package preview

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func envFunc(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDetectProtocol(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Protocol
	}{
		{"kitty via window id", map[string]string{"KITTY_WINDOW_ID": "1"}, ProtoKitty},
		{"kitty via TERM", map[string]string{"TERM": "xterm-kitty"}, ProtoKitty},
		{"ghostty", map[string]string{"TERM": "ghostty"}, ProtoKitty},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, ProtoKitty},
		{"iterm2", map[string]string{"TERM_PROGRAM": "iTerm.app"}, ProtoITerm2},
		{"iterm2 via LC", map[string]string{"LC_TERMINAL": "iTerm2"}, ProtoITerm2},
		{"sixel", map[string]string{"TERM": "xterm-sixel"}, ProtoSixel},
		{"none", map[string]string{"TERM": "xterm-256color"}, ProtoNone},
		{"empty", map[string]string{}, ProtoNone},
	}
	for _, c := range cases {
		if got := DetectProtocol(envFunc(c.env)); got != c.want {
			t.Errorf("%s: DetectProtocol = %v, want %v", c.name, got, c.want)
		}
	}
}

// makePNG returns a small solid-color PNG.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestRenderHalfBlock(t *testing.T) {
	data := makePNG(t, 16, 16)
	out, err := RenderHalfBlock(data, 20, 10)
	if err != nil {
		t.Fatalf("RenderHalfBlock: %v", err)
	}
	if out == "" {
		t.Fatal("half-block render produced no output")
	}
	// truecolor half-block uses the upper-half-block rune and ANSI escapes.
	if !bytes.Contains([]byte(out), []byte("\x1b[")) {
		t.Error("expected ANSI escape codes in half-block output")
	}
}

func TestRenderImageProtocols(t *testing.T) {
	png := makePNG(t, 8, 8)

	if out, err := RenderImage(png, ProtoNone, 20, 10); err != nil || out == "" {
		t.Errorf("ProtoNone should render half-block: out=%d err=%v", len(out), err)
	}
	iterm, err := RenderImage(png, ProtoITerm2, 20, 10)
	if err != nil || !strings.Contains(iterm, "\x1b]1337;File=inline=1") {
		t.Errorf("iTerm2 render missing escape: err=%v out=%q", err, iterm[:min(40, len(iterm))])
	}
	kitty, err := RenderImage(png, ProtoKitty, 20, 10)
	if err != nil || !strings.Contains(kitty, "\x1b_Gf=100,a=T") || !strings.Contains(kitty, "q=2") {
		t.Errorf("kitty render missing escape: err=%v", err)
	}
}

func TestRenderImageBadDataFallsBack(t *testing.T) {
	bad := []byte("not an image")
	// Protocol paths surface an error so the caller shows a summary.
	if _, err := RenderImage(bad, ProtoITerm2, 20, 10); err == nil {
		t.Error("iTerm2 with undecodable data should error")
	}
	if _, err := RenderImage(bad, ProtoKitty, 20, 10); err == nil {
		t.Error("kitty with undecodable data should error")
	}
}

func TestRenderHalfBlockBadImage(t *testing.T) {
	if _, err := RenderHalfBlock([]byte("not an image"), 20, 10); err == nil {
		t.Error("undecodable bytes should return an error so caller can fall back")
	}
}

func TestRenderHalfBlockEvenHeight(t *testing.T) {
	// Odd rows are bumped to even internally; should not error.
	if _, err := RenderHalfBlock(makePNG(t, 8, 8), 10, 5); err != nil {
		t.Errorf("odd rows should be handled, got %v", err)
	}
}
