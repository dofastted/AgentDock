// pf-serve runs OpenAI's Privacy Filter for magpie's masking (internal/pii):
// a process of its own, so the model's 1.6 GB is let go by ending it and a
// crash in a GPU driver ends no gateway.
//
//   pf-serve <model.gguf> [threshold]
//
// It loads the model on a GPU when there is one (PF_DEVICE=cpu keeps it on
// the CPU), runs a few texts so the GPU's kernels are built before the
// first request, and writes one JSON line: {"device":…,"gpu":…,"ms":…}, or
// {"error":…} and exits 1. Then, until stdin ends, each request is
//
//   u32 n, then n times: u32 len, len bytes of UTF-8
//
// and each reply is u32 0 and, for each text in order, u32 count and count
// times: i32 start, i32 end (byte offsets), f32 score, u8 len, the label;
// or u32 1, u32 len and an error's text. Little-endian throughout.
#include "pf.h"
#include "ggml-backend.h"

#include <chrono>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <string>
#include <vector>

#ifdef _WIN32
#include <fcntl.h>
#include <io.h>
#endif

static bool read_all(void * p, size_t n) {
    return n == 0 || std::fread(p, 1, n, stdin) == n;
}

static uint32_t le32(const unsigned char * b) {
    return (uint32_t) b[0] | (uint32_t) b[1] << 8 | (uint32_t) b[2] << 16 | (uint32_t) b[3] << 24;
}

static void put32(std::string & out, uint32_t v) {
    const char b[4] = {(char) (v & 0xff), (char) (v >> 8 & 0xff), (char) (v >> 16 & 0xff), (char) (v >> 24 & 0xff)};
    out.append(b, 4);
}

static std::string json_str(const std::string & s) {
    std::string o = "\"";
    for (unsigned char c : s) {
        if (c == '"' || c == '\\') {
            o += '\\';
            o += (char) c;
        } else if (c < 0x20) {
            char b[8];
            std::snprintf(b, sizeof b, "\\u%04x", c);
            o += b;
        } else {
            o += (char) c;
        }
    }
    return o + "\"";
}

static void hello(const std::string & line) {
    std::fwrite(line.data(), 1, line.size(), stdout);
    std::fputc('\n', stdout);
    std::fflush(stdout);
}

// gpu_name is the GPU "gpu" picks, as pf's backend picks it: the first
// discrete one, else the first integrated one.
static std::string gpu_name() {
    for (int want : {(int) GGML_BACKEND_DEVICE_TYPE_GPU, (int) GGML_BACKEND_DEVICE_TYPE_IGPU}) {
        for (size_t i = 0; i < ggml_backend_dev_count(); i++) {
            ggml_backend_dev_t d = ggml_backend_dev_get(i);
            if ((int) ggml_backend_dev_type(d) == want) {
                std::string reg = ggml_backend_reg_name(ggml_backend_dev_backend_reg(d));
                return reg + ": " + ggml_backend_dev_description(d);
            }
        }
    }
    return "gpu";
}

int main(int argc, char ** argv) {
#ifdef _WIN32
    _setmode(_fileno(stdin), _O_BINARY);
    _setmode(_fileno(stdout), _O_BINARY);
#endif
    if (argc < 2) {
        hello("{\"error\":\"usage: pf-serve <model.gguf> [threshold]\"}");
        return 2;
    }
    const float threshold = argc > 2 ? (float) std::atof(argv[2]) : 0.5f;
    const auto t0 = std::chrono::steady_clock::now();

    const char * env = std::getenv("PF_DEVICE");
    const bool cpu_only = env && std::strcmp(env, "cpu") == 0;
    pf_ctx * ctx = nullptr;
    std::string device, gpu_error;
    if (!cpu_only) {
        ctx = pf_load(argv[1], "gpu", 0);
        if (ctx && !pf_last_error(ctx)) {
            device = gpu_name();
        } else {
            gpu_error = ctx && pf_last_error(ctx) ? pf_last_error(ctx) : "no GPU";
            pf_free(ctx);
            ctx = nullptr;
        }
    }
    if (!ctx) {
        ctx = pf_load(argv[1], "cpu", 0);
        if (!ctx || pf_last_error(ctx)) {
            hello("{\"error\":" + json_str(ctx && pf_last_error(ctx) ? pf_last_error(ctx) : "the model didn't load") + "}");
            return 1;
        }
        device = "cpu";
    }

    // the GPU's kernels are built on first use, a second or more: here,
    // not on a request
    // in a short text and longer ones, as kernels are picked by size
    std::string warm = "Hi Bob";
    for (int n : {0, 1, 12, 48}) {
        while ((int) warm.size() < n * 160) {
            warm += " Ask Jane Doe at jane.doe@example.com or +1 415 555 0100 about 221B Baker Street, account 12345678.";
        }
        pf_entity * ents = nullptr;
        size_t ne = 0;
        if (pf_classify(ctx, warm.data(), warm.size(), threshold, &ents, &ne) != 0) {
            hello("{\"error\":" + json_str(std::string("warm-up: ") + (pf_last_error(ctx) ? pf_last_error(ctx) : "failed")) + "}");
            return 1;
        }
        pf_entities_free(ents, ne);
    }
    const long long ms = (long long) std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() - t0).count();
    std::string h = "{\"device\":" + json_str(device) + ",\"gpu\":" + (device == "cpu" ? "false" : "true") + ",\"ms\":" + std::to_string(ms);
    if (!gpu_error.empty()) {
        h += ",\"gpuError\":" + json_str(gpu_error);
    }
    hello(h + "}");

    std::vector<std::string> texts;
    std::string out;
    for (;;) {
        unsigned char b[4];
        if (!read_all(b, 4)) {
            break;
        }
        const uint32_t n = le32(b);
        texts.assign(n, std::string());
        bool ok = true;
        for (uint32_t i = 0; i < n && ok; i++) {
            ok = read_all(b, 4);
            if (ok) {
                texts[i].resize(le32(b));
                ok = read_all(texts[i].data(), texts[i].size());
            }
        }
        if (!ok) {
            break;
        }
        out.clear();
        put32(out, 0);
        for (const std::string & t : texts) {
            pf_entity * ents = nullptr;
            size_t ne = 0;
            if (!t.empty() && pf_classify(ctx, t.data(), t.size(), threshold, &ents, &ne) != 0) {
                const std::string e = pf_last_error(ctx) ? pf_last_error(ctx) : "classify failed";
                out.clear();
                put32(out, 1);
                put32(out, (uint32_t) e.size());
                out += e;
                break;
            }
            put32(out, (uint32_t) ne);
            for (size_t i = 0; i < ne; i++) {
                put32(out, (uint32_t) ents[i].start);
                put32(out, (uint32_t) ents[i].end);
                uint32_t f;
                std::memcpy(&f, &ents[i].score, 4);
                put32(out, f);
                const size_t ll = std::strlen(ents[i].label);
                out += (char) (ll > 255 ? 255 : ll);
                out.append(ents[i].label, ll > 255 ? 255 : ll);
            }
            pf_entities_free(ents, ne);
        }
        if (std::fwrite(out.data(), 1, out.size(), stdout) != out.size() || std::fflush(stdout) != 0) {
            break;
        }
    }
    pf_free(ctx);
    return 0;
}
