package storage

import (
	"context"
	"errors"
	"testing"
)

// TestNormalizeFolderKey: normalises to exactly one trailing "/" and rejects
// empty/whitespace/control-char names (FR-010).
func TestNormalizeFolderKey(t *testing.T) {
	ok := map[string]string{
		"reports":  "reports/",
		"reports/": "reports/",
		"a/b":      "a/b/",
		"a/b//":    "a/b/",
	}
	for in, want := range ok {
		got, err := normalizeFolderKey(in)
		if err != nil {
			t.Errorf("normalizeFolderKey(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("normalizeFolderKey(%q) = %q, want %q", in, got, want)
		}
	}

	for _, bad := range []string{"", "   ", "\t", "a\x00b", "x\x7f"} {
		if _, err := normalizeFolderKey(bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("normalizeFolderKey(%q) = %v, want ErrInvalidName", bad, err)
		}
	}
}

// TestFakeCreateFolder: the fake creates the normalised key, validates input, and
// maps a missing bucket to ErrNotFound.
func TestFakeCreateFolder(t *testing.T) {
	f := NewFake()
	f.Seed("b")

	if err := f.CreateFolder(context.Background(), "b", "reports"); err != nil {
		t.Fatalf("CreateFolder error: %v", err)
	}
	if _, ok := f.Buckets["b"].Objects["reports/"]; !ok {
		t.Errorf("want key reports/; objects=%v", f.Buckets["b"].Objects)
	}

	if err := f.CreateFolder(context.Background(), "b", "  "); !errors.Is(err, ErrInvalidName) {
		t.Errorf("blank name = %v, want ErrInvalidName", err)
	}
	if err := f.CreateFolder(context.Background(), "missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing bucket = %v, want ErrNotFound", err)
	}
}
