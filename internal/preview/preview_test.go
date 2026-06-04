package preview

import (
	"strings"
	"testing"
)

func TestClassifyText(t *testing.T) {
	cases := []struct {
		ct   string
		data []byte
	}{
		{"text/plain", []byte("hello world")},
		{"application/json", []byte(`{"a":1}`)},
		{"", []byte("plain sniffed text\nsecond line")},
		{"application/octet-stream", []byte("actually utf-8 text")},
	}
	for _, c := range cases {
		if k := Classify(c.ct, c.data); k != KindText {
			t.Errorf("Classify(%q) = %v, want text", c.ct, k)
		}
	}
}

func TestClassifyImage(t *testing.T) {
	if k := Classify("image/png", []byte{0x89, 0x50, 0x4e, 0x47}); k != KindImage {
		t.Errorf("png content-type = %v, want image", k)
	}
	// PNG magic, no content-type → sniffed as image
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if k := Classify("", png); k != KindImage {
		t.Errorf("sniffed png = %v, want image", k)
	}
}

func TestClassifyBinary(t *testing.T) {
	withNUL := []byte("text\x00with-nul")
	if k := Classify("text/plain", withNUL); k != KindBinary {
		t.Errorf("NUL byte should force binary, got %v", k)
	}
	if k := Classify("application/octet-stream", []byte{0x00, 0x01, 0x02, 0xff}); k != KindBinary {
		t.Errorf("binary blob = %v, want binary", k)
	}
}

func TestBuildAndTruncationFlag(t *testing.T) {
	p := Build("k.txt", "text/plain", []byte("data"), true)
	if p.Kind != KindText {
		t.Errorf("Kind = %v, want text", p.Kind)
	}
	if !p.Truncated {
		t.Error("Truncated flag not carried")
	}
	if Limit != 5*1024*1024 {
		t.Errorf("Limit = %d, want 5 MiB", Limit)
	}
}

func TestSummarySafe(t *testing.T) {
	blob := []byte{0x00, 0x01, 0x02, 0x03}
	p := Build("k.bin", "application/octet-stream", blob, true)
	s := Summary(p)
	if strings.ContainsRune(s, 0x00) {
		t.Error("summary leaked raw NUL byte")
	}
	if !strings.Contains(s, "binary") {
		t.Errorf("summary = %q, want it to mention binary", s)
	}
	if !strings.Contains(s, "truncated") {
		t.Errorf("summary = %q, want truncation notice", s)
	}
}
