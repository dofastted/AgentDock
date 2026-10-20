package gateway

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

func pngB64(t *testing.T, w, h int) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, shotImage(w, h)); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func anthBody(images ...string) []byte {
	var b strings.Builder
	b.WriteString(`{"model":"m1","max_tokens":16,"messages":[{"role":"user","content":[{"type":"text","text":"see"}`)
	for _, im := range images {
		b.WriteString(`,{"type":"image","source":{"type":"base64","media_type":"image/png","data":"`)
		b.WriteString(im)
		b.WriteString(`"}}`)
	}
	b.WriteString(`]}]}`)
	return []byte(b.String())
}

func imageSize(t *testing.T, b64 string) (int, int) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func imageDatas(t *testing.T, body []byte) []string {
	t.Helper()
	var out []string
	gjson.GetBytes(body, "messages.0.content").ForEach(func(_, block gjson.Result) bool {
		if block.Get("type").String() == "image" {
			out = append(out, block.Get("source.data").String())
		}
		return true
	})
	return out
}

// The gateway, with Fit large images off, sends the same image bytes with
// four images and with five. Nothing starts compressing at the fifth.
func TestGatewayImageBytesSurviveAddingImages(t *testing.T) {
	big := pngB64(t, 2400, 1200)
	small := pngB64(t, 8, 8)
	f := &fake{t: t, ctype: "application/json", reply: `{"id":"msg","type":"message","content":[]}`}
	setup(t, provider.Anthropic, f)
	t.Setenv("AGENTDOCK_DATA_DIR", "")
	var four []string
	for _, n := range []int{4, 5} {
		imgs := []string{big, small, small, small}
		if n == 5 {
			imgs = append(imgs, small)
		}
		code, resp := post(t, "/v1/messages", string(anthBody(imgs...)))
		if code != 200 {
			t.Fatalf("%d images: %d %s", n, code, resp)
		}
		got := imageDatas(t, f.got)
		if len(got) != n || got[0] != big {
			t.Fatalf("%d images: upstream rewrote them (%d datas, first changed %v)", n, len(got), len(got) > 0 && got[0] != big)
		}
		for _, d := range got[1:] {
			if d != small {
				t.Fatal("a small image was rewritten")
			}
		}
		if n == 4 {
			four = append([]string(nil), got...)
		}
	}
	five := imageDatas(t, f.got)
	if strings.Join(five[:4], "\n") != strings.Join(four, "\n") {
		t.Fatal("adding a fifth image changed the four already sent")
	}
}

// On, the gateway scales an oversized image on the first request, not when
// a later image arrives. A second send of the same image is the same bytes.
func TestGatewayFitsAnImageFromTheFirst(t *testing.T) {
	t.Setenv("AGENTDOCK_DATA_DIR", "")
	big := pngB64(t, 2400, 1200)
	small := pngB64(t, 8, 8)
	f := &fake{t: t, ctype: "application/json", reply: `{"id":"msg","type":"message","content":[]}`}
	setup(t, provider.Anthropic, f)
	if err := settings.Save(settings.Settings{ImageFit: true}); err != nil {
		t.Fatal(err)
	}
	body := string(anthBody(big, small))
	code, resp := post(t, "/v1/messages", body)
	if code != 200 {
		t.Fatalf("%d %s", code, resp)
	}
	got := imageDatas(t, f.got)
	if len(got) != 2 || got[1] != small {
		t.Fatalf("images: %d, small kept %v", len(got), len(got) == 2 && got[1] == small)
	}
	if got[0] == big {
		t.Fatal("the setting on left the oversized image as sent")
	}
	w, h := imageSize(t, got[0])
	if w != 2000 || h != 1000 {
		t.Fatalf("scaled to %dx%d, want 2000x1000", w, h)
	}
	if !bytes.Contains(f.got, []byte(`"text":"see"`)) {
		t.Fatalf("the rest of the request moved: %.120s", f.got)
	}
	first := got[0]
	if code, resp = post(t, "/v1/messages", body); code != 200 {
		t.Fatalf("again: %d %s", code, resp)
	}
	if again := imageDatas(t, f.got); len(again) != 2 || again[0] != first {
		t.Fatal("the same image was scaled to different bytes")
	}
}

func TestGatewayFitsAToolResultImage(t *testing.T) {
	t.Setenv("AGENTDOCK_DATA_DIR", "")
	big := pngB64(t, 800, 3000)
	f := &fake{t: t, ctype: "application/json", reply: `{"id":"msg","type":"message","content":[]}`}
	setup(t, provider.Anthropic, f)
	if err := settings.Save(settings.Settings{ImageFit: true}); err != nil {
		t.Fatal(err)
	}
	body := `{"model":"m1","max_tokens":16,"messages":[` +
		`{"role":"user","content":[{"type":"text","text":"look"}]},` +
		`{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"shot","input":{}}]},` +
		`{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[` +
		`{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + big + `"}}]}]}]}`
	code, resp := post(t, "/v1/messages", body)
	if code != 200 {
		t.Fatalf("%d %s", code, resp)
	}
	got := gjson.GetBytes(f.got, "messages.2.content.0.content.0.source.data").String()
	if got == "" || got == big {
		t.Fatal("the tool image was not scaled")
	}
	w, h := imageSize(t, got)
	if w != 533 || h != 2000 {
		t.Fatalf("tool image scaled to %dx%d, want 533x2000", w, h)
	}
}
