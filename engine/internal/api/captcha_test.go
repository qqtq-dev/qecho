package api

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestRandomCode(t *testing.T) {
	for i := 0; i < 50; i++ {
		code := randomCode()
		if len(code) != captchaLength {
			t.Fatalf("code length = %d, want %d", len(code), captchaLength)
		}
		for _, c := range code {
			if !strings.ContainsRune(captchaCharset, c) {
				t.Fatalf("char %q not in charset", c)
			}
			if _, ok := captchaGlyphs[byte(c)]; !ok {
				t.Fatalf("char %q has no glyph", c)
			}
		}
	}
}

func TestRenderCaptcha(t *testing.T) {
	data, err := renderCaptcha("A2B9")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantW := captchaLength*captchaCell*captchaScale + (captchaLength-1)*captchaGap + 16
	wantH := 7*captchaScale + 16
	if b := img.Bounds(); b.Dx() != wantW || b.Dy() != wantH {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), wantW, wantH)
	}
	bg := img.At(0, 0)
	ink := false
	for y := 0; y < wantH && !ink; y++ {
		for x := 0; x < wantW; x++ {
			if img.At(x, y) != bg {
				ink = true
				break
			}
		}
	}
	if !ink {
		t.Fatal("image is blank")
	}
}

func TestCheckCaptcha(t *testing.T) {
	s := &Server{captchas: map[string]captchaEntry{}}
	s.captchas["ok"] = captchaEntry{code: "A2B9", expires: time.Now().Add(time.Minute)}
	s.captchas["old"] = captchaEntry{code: "A2B9", expires: time.Now().Add(-time.Minute)}

	if !s.checkCaptcha("ok", "a2b9 ") {
		t.Fatal("lowercase answer with spaces should pass")
	}
	if s.checkCaptcha("ok", "A2B9") {
		t.Fatal("captcha must be single-use")
	}
	if s.checkCaptcha("", "A2B9") || s.checkCaptcha("missing", "A2B9") {
		t.Fatal("unknown id should fail")
	}
	if s.checkCaptcha("old", "A2B9") {
		t.Fatal("expired captcha should fail")
	}

	s.captchas["again"] = captchaEntry{code: "QW34", expires: time.Now().Add(time.Minute)}
	if s.checkCaptcha("again", "ZZZZ") {
		t.Fatal("wrong answer should fail")
	}
	if s.checkCaptcha("again", "") {
		t.Fatal("empty answer should fail")
	}
}
