package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// T040: resizing across the Full ↔ Single ↔ Full tier boundaries preserves the highlighted
// bucket, the objects cursor, and the focused zone, and never panics (011 SC-008).
func TestResizeReflowPreservesState(t *testing.T) {
	f := storage.NewFake()
	f.Seed("a", "x")
	f.Seed("b", "y.txt", "z.txt")
	m := fullApp(f)
	m = press(m, "down") // bucketSel → 1 ("b")
	m = crossToObjects(m, f, "b")
	m = press(m, "down") // objects cursor → 1
	bsel, tsel, fz := m.bucketSel, m.treeSel, m.focusZone

	m = deliver(m, tea.WindowSizeMsg{Width: 80, Height: 24})  // → Single
	_ = viewOf(m)                                             // must not panic
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 30}) // → Full
	_ = viewOf(m)

	if m.bucketSel != bsel {
		t.Errorf("bucketSel not preserved across resize: got %d want %d", m.bucketSel, bsel)
	}
	if m.treeSel != tsel {
		t.Errorf("objects cursor not preserved across resize: got %d want %d", m.treeSel, tsel)
	}
	if m.focusZone != fz {
		t.Errorf("focusZone not preserved across resize")
	}
}

// T041: the footer (incl. the quit hint) stays visible even at a small terminal height (FR-017).
func TestFooterVisibleMinHeight(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := dualApp(f)
	m.height = 8
	v := viewOf(m)
	if !strings.Contains(v, "quit") {
		t.Errorf("footer (quit) must stay visible at small height:\n%s", v)
	}
}

// T047: Single-tier parity (SC-007) — at ≤99 cols Enter-on-a-bucket drills full-screen
// (modeTree) as before, no objects-zone preview is armed, and focus stays inert.
func TestSingleTierParity(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, nil, nil) // width 80 → Single
	if m.bucketLoadGen != 0 {
		t.Errorf("Single tier must not arm the objects-zone preview (bucketLoadGen=%d)", m.bucketLoadGen)
	}
	m = press(m, "enter") // drill full-screen
	if m.mode != modeTree {
		t.Errorf("Single tier: Enter on a bucket must drill full-screen (modeTree), got mode %d", m.mode)
	}
	if m.focusZone != zoneBuckets {
		t.Errorf("focusZone must stay inert (zoneBuckets) in the Single tier")
	}
}
