package gateway

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"slices"
	"sync"

	"github.com/tidwall/gjson"

	"github.com/yetone/magpie/internal/settings"
)

// imageFitSide is the longest side an image is scaled to when Fit large
// images is on. Anthropic applies this limit itself only above 20 images.
const imageFitSide = 2000

// anthropicImagesFit scales inline images for an Anthropic messages body
// when Settings' ImageFit is on. Off, which is the default, the body is
// returned as it came. A long chat that starts scaling only after several
// images (the report was five) rewrites images already sent, and
// Anthropic's prompt cache misses from the first changed image on.
// Anthropic's own stricter 2000px limit applies above 20 images, not at 5;
// this does not wait for that count, because waiting is what changes bytes
// already in the cache. On, every inline image whose long side is over
// 2000px is scaled to 2000, from the first one, so a later image does not
// rewrite the ones already sent.
func anthropicImagesFit(body []byte) []byte {
	if !bytes.Contains(body, []byte(`"image"`)) || !settings.Load().ImageFit {
		return body
	}
	return fitAnthropicImages(body)
}

// fitAnthropicImages is anthropicImagesFit with the setting already on.
// An image that fits, or that is not inline, is left as it was, and the
// rest of the JSON stays byte for byte.
func fitAnthropicImages(body []byte) []byte {
	slots := anthropicImageSlots(body)
	if len(slots) == 0 {
		return body
	}
	type swap struct {
		at, end int
		with    string
	}
	var swaps []swap
	for _, s := range slots {
		if !s.inline {
			continue
		}
		b64, media, ok := fitAnthropicImage(s.data.Str, s.media.Str)
		if !ok {
			continue
		}
		swaps = append(swaps, swap{s.data.Index, s.data.Index + len(s.data.Raw), `"` + b64 + `"`})
		if media != s.media.Str && s.media.Index > 0 {
			swaps = append(swaps, swap{s.media.Index, s.media.Index + len(s.media.Raw), `"` + media + `"`})
		}
	}
	if len(swaps) == 0 {
		return body
	}
	slices.SortFunc(swaps, func(a, b swap) int { return a.at - b.at })
	var out bytes.Buffer
	last := 0
	for _, s := range swaps {
		if s.at < last || s.end > len(body) || s.end < s.at {
			return body
		}
		out.Write(body[last:s.at])
		out.WriteString(s.with)
		last = s.end
	}
	out.Write(body[last:])
	return out.Bytes()
}

// anthImage is one image block in an Anthropic messages body.
type anthImage struct {
	data, media gjson.Result
	inline      bool
}

// anthropicImageSlots is every image block, including one nested in a
// tool_result. A URL image is counted and left as it was.
func anthropicImageSlots(body []byte) []anthImage {
	var slots []anthImage
	gjson.GetBytes(body, "messages").ForEach(func(i, _ gjson.Result) bool {
		collectAnthropicImages(body, "messages."+i.String()+".content", &slots)
		return true
	})
	return slots
}

func collectAnthropicImages(body []byte, path string, slots *[]anthImage) {
	content := gjson.GetBytes(body, path)
	if !content.IsArray() {
		return
	}
	content.ForEach(func(j, block gjson.Result) bool {
		p := path + "." + j.String()
		switch block.Get("type").String() {
		case "image":
			if s, ok := anthImageOf(body, p); ok {
				*slots = append(*slots, s)
			}
		case "tool_result":
			collectAnthropicImages(body, p+".content", slots)
		}
		return true
	})
}

func anthImageOf(body []byte, path string) (anthImage, bool) {
	src := gjson.GetBytes(body, path+".source")
	if src.Get("type").String() != "base64" {
		// a url or file image still counts
		if gjson.GetBytes(body, path+".type").String() == "image" {
			return anthImage{}, true
		}
		return anthImage{}, false
	}
	data := src.Get("data")
	if data.Type != gjson.String || data.Index <= 0 {
		return anthImage{}, false
	}
	return anthImage{data: data, media: src.Get("media_type"), inline: true}, true
}

// fittedAnthropic remembers a scaled image so the next turn, which sends
// it again, gets the same bytes.
var fittedAnthropic = struct {
	sync.Mutex
	m map[[32]byte]fittedImage
}{m: map[[32]byte]fittedImage{}}

type fittedImage struct {
	b64, media string
	same       bool
}

// fitAnthropicImage is b64 scaled so its long side is at most imageFitSide,
// written again in its own format (a GIF as PNG), and whether that differs
// from what came. One that already fits, or that cannot be read, is left.
func fitAnthropicImage(b64, media string) (string, string, bool) {
	key := sha256.Sum256([]byte(media + "\n" + b64))
	fittedAnthropic.Lock()
	got, ok := fittedAnthropic.m[key]
	fittedAnthropic.Unlock()
	if ok {
		return got.b64, got.media, !got.same
	}
	nb, nm, same := scaleAnthropicImage(b64, media)
	fittedAnthropic.Lock()
	if len(fittedAnthropic.m) >= 64 {
		clear(fittedAnthropic.m)
	}
	if same {
		fittedAnthropic.m[key] = fittedImage{same: true}
	} else {
		fittedAnthropic.m[key] = fittedImage{b64: nb, media: nm}
	}
	fittedAnthropic.Unlock()
	return nb, nm, !same
}

func scaleAnthropicImage(b64, media string) (string, string, bool) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", "", true
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > codexImageMaxPixels {
		return "", "", true
	}
	w, h := fitLongSide(cfg.Width, cfg.Height, imageFitSide)
	if w == cfg.Width && h == cfg.Height && format != "gif" {
		return "", "", true
	}
	var src image.Image
	switch format {
	case "png":
		src, err = png.Decode(bytes.NewReader(raw))
	case "jpeg":
		src, err = jpeg.Decode(bytes.NewReader(raw))
	case "gif":
		src, err = gif.Decode(bytes.NewReader(raw))
	default:
		return "", "", true
	}
	if err != nil {
		return "", "", true
	}
	if w != cfg.Width || h != cfg.Height {
		src = triangleResize(src, w, h)
	}
	var out bytes.Buffer
	outMedia := "image/png"
	if format == "jpeg" {
		outMedia = "image/jpeg"
		err = jpeg.Encode(&out, src, &jpeg.Options{Quality: 85})
	} else {
		err = png.Encode(&out, src)
	}
	if err != nil {
		return "", "", true
	}
	// a GIF becomes PNG; without a media_type to correct, leave it
	if format == "gif" && media == "" {
		return "", "", true
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), outMedia, false
}

// fitLongSide is w,h scaled so the longer side is at most side, the shorter
// at least 1.
func fitLongSide(w, h, side int) (int, int) {
	if w <= side && h <= side {
		return w, h
	}
	scale := float64(side) / float64(max(w, h))
	nw := max(1, int(math.Round(float64(w)*scale)))
	nh := max(1, int(math.Round(float64(h)*scale)))
	if nw > side {
		nw = side
	}
	if nh > side {
		nh = side
	}
	return nw, nh
}
