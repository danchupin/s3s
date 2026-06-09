package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// TestStorageClassMarkerInListing: a non-standard class shows its compact marker in the
// `type` cell; a STANDARD object shows the neutral "obj" (no per-row noise); the full
// class is NOT printed inline (it is revealable). US5 / FR-015.
func TestStorageClassMarkerInListing(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "cold.dat", storage.FakeObject{Data: []byte("x"), StorageClass: "GLACIER"})
	f.SeedObject("b", "warm.txt", storage.FakeObject{Data: []byte("y"), StorageClass: "STANDARD"})
	m := enterTree(t, f, "b")
	m = deliver(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	v := stripANSI(viewOf(m))
	if !strings.Contains(v, "glac") {
		t.Errorf("GLACIER row must show the 'glac' marker:\n%s", v)
	}
	if !strings.Contains(v, "obj") {
		t.Errorf("STANDARD row must show the neutral 'obj' marker:\n%s", v)
	}
	if strings.Contains(v, "GLACIER") {
		t.Errorf("the listing must not print the full class inline (revealed via 'i'):\n%s", v)
	}
}

// TestStorageClassRevealShowsFullClass: 'i' on a non-standard row reveals the full,
// lossless class string (constitution VI — lossy marker tied to a reveal affordance).
func TestStorageClassRevealShowsFullClass(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "cold.dat", storage.FakeObject{Data: []byte("x"), StorageClass: "GLACIER"})
	m := enterTree(t, f, "b")
	m = deliver(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = press(m, "i") // reveal the selected (only) object row
	if m.reveal == nil {
		t.Fatal("'i' should open the reveal popup")
	}
	if !strings.Contains(stripANSI(viewOf(m)), "GLACIER") {
		t.Errorf("reveal must show the full class GLACIER:\n%s", viewOf(m))
	}
}

func TestStorageClassMarkerToken(t *testing.T) {
	cases := map[string]string{
		"": "obj", "STANDARD": "obj", "GLACIER": "glac", "GLACIER_IR": "gir",
		"DEEP_ARCHIVE": "arch", "INTELLIGENT_TIERING": "int", "STANDARD_IA": "ia",
		"ONEZONE_IA": "1zia", "REDUCED_REDUNDANCY": "rr", "WEIRD_FUTURE": "cls*",
	}
	for in, want := range cases {
		if got := storageClassMarker(in); got != want {
			t.Errorf("storageClassMarker(%q) = %q, want %q", in, got, want)
		}
	}
}
