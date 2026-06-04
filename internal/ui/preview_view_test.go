package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

func TestPreviewTextScrollable(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "file.txt", storage.FakeObject{Data: []byte("hello")})
	m := enterTree(t, f, "b")
	m.height = 10 // small window to make scrolling observable

	m = press(m, "p")
	if !m.loading {
		t.Fatal("'p' should arm a preview load")
	}
	body := "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7"
	pl := preview.Build("file.txt", "text/plain", []byte(body), false)
	m = deliver(m, previewMsg{gen: m.gen, payload: pl})

	if m.mode != modePreview {
		t.Fatalf("mode = %v, want preview", m.mode)
	}
	if !strings.Contains(viewOf(m), "line0") {
		t.Errorf("preview should show first line:\n%s", viewOf(m))
	}

	// Scroll down advances the offset.
	m = press(m, "down")
	if m.prevOff != 1 {
		t.Errorf("scroll offset = %d, want 1", m.prevOff)
	}
	m = press(m, "up")
	if m.prevOff != 0 {
		t.Errorf("scroll offset = %d, want 0 after up", m.prevOff)
	}

	m = press(m, "esc")
	if m.mode != modeTree {
		t.Errorf("esc should leave preview, mode = %v", m.mode)
	}
}

func TestPreviewTruncatedNotice(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "big.txt", storage.FakeObject{Data: []byte("x")})
	m := enterTree(t, f, "b")
	m = press(m, "p")
	pl := preview.Build("big.txt", "text/plain", []byte("data"), true)
	m = deliver(m, previewMsg{gen: m.gen, payload: pl})
	if !strings.Contains(viewOf(m), "truncated at 5 MiB") {
		t.Errorf("truncated preview should show notice:\n%s", viewOf(m))
	}
}

func TestPreviewImageFallbackSummary(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "logo.png", storage.FakeObject{Data: []byte("not a real png")})
	m := enterTree(t, f, "b")
	m = press(m, "p")
	// image/* content type but undecodable bytes → classified image, render fails.
	pl := preview.Build("logo.png", "image/png", []byte{0x89, 0x50, 0x4e, 0x47, 0x00}, false)
	if pl.Kind != preview.KindImage {
		t.Fatalf("payload Kind = %v, want image", pl.Kind)
	}
	m = deliver(m, previewMsg{gen: m.gen, payload: pl})
	v := viewOf(m)
	if !strings.Contains(v, "Image preview unavailable") || !strings.Contains(v, "image") {
		t.Errorf("undecodable image should fall back to summary:\n%s", v)
	}
}
