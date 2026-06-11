package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// TestHealthCard130x24: at the supported minimum the card keeps the footer visible and
// collapses sections (classes → size → age) instead of clipping values (017 US4
// acceptance 6, contract health-card-view.md, T040).
func TestHealthCard130x24(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	f.SeedIncompleteUpload("b", storage.FakeIncompleteUpload{Key: "k", Initiated: mpuTime(), PartSizes: []int64{5}})
	m := healthApp(t, f)
	m = deliver(m, tea.WindowSizeMsg{Width: 130, Height: 24})

	mm, cmd := pressCmd(m, "H")
	m = drainBatch(t, mm, cmd)
	v := stripANSI(viewOf(m))
	lines := strings.Split(v, "\n")
	if len(lines) > 24 {
		t.Fatalf("view = %d rows at a 24-row terminal — chrome scrolled off", len(lines))
	}
	if !strings.Contains(v, "quit") {
		t.Errorf("footer/hints must stay visible:\n%s", v)
	}
	// The age histogram survives (highest priority); lower-priority sections collapse
	// with an explicit marker rather than silently vanishing.
	if !strings.Contains(v, "Age") {
		t.Errorf("age histogram must survive the collapse:\n%s", v)
	}
	if !strings.Contains(v, "collapsed") {
		t.Errorf("collapsed sections must be explicitly marked:\n%s", v)
	}
}

// TestHealthCard40Rows: with room to spare nothing collapses — all sections render.
func TestHealthCard40Rows(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := healthApp(t, f)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40})

	mm, cmd := pressCmd(m, "H")
	m = drainBatch(t, mm, cmd)
	v := stripANSI(viewOf(m))
	for _, want := range []string{"Age", "Size", "Classes"} {
		if !strings.Contains(v, want) {
			t.Errorf("tall terminal must render all sections, missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "collapsed") {
		t.Errorf("nothing should collapse at 40 rows:\n%s", v)
	}
}
