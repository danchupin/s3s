package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// --- 006 visual contract: non-color cues (FR-034/FR-041), badge sole-loud (FR-035) ---

func TestNonColorCuesPresent(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "c.txt")
	m := treeApp(f, true)
	m.width, m.height = 120, 30
	selectObject(&m, "a.txt")
	m = press(m, " ") // mark a.txt
	v := viewOf(m)

	// Selection gutter, multi-select mark, and the badge TEXT are all non-color cues
	// so meaning survives without color (FR-034).
	if !strings.Contains(v, "▶") {
		t.Errorf("selected row should carry the ▶ gutter cue:\n%s", v)
	}
	if !strings.Contains(v, "✓") {
		t.Errorf("a marked object should carry the ✓ glyph cue:\n%s", v)
	}
	if !strings.Contains(v, "[RW]") {
		t.Errorf("an armed session should show the [RW] badge TEXT:\n%s", v)
	}
}

func TestBadgeIsReadOnlyTextWhenDisarmed(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false) // read-only
	m.width, m.height = 120, 30
	if v := viewOf(m); !strings.Contains(v, "[RO]") {
		t.Errorf("a read-only session should show the [RO] badge text:\n%s", v)
	}
}
