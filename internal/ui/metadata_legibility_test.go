package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

func bigReport(n int) *storage.UsageReport {
	rep := &storage.UsageReport{Bucket: "b", TotalCount: n, TotalSize: int64(n) * 100, Complete: true}
	for i := 0; i < n; i++ {
		rep.Children = append(rep.Children, storage.UsageChild{Name: fmt.Sprintf("c%02d/", i), IsDir: true, Size: 100})
	}
	return rep
}

// TestBreakdownOverflowAffordance: more children than fit are summarised by a
// "… +N more (i to reveal)" line — nothing is silently clipped (constitution VI / FR-017).
func TestBreakdownOverflowAffordance(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := withBuckets(f, []string{"ctx"}, nil)
	m.usageResults.Put(m.usageKey("b", ""), bigReport(12))
	m.detailSection = sectBreakdown

	v := stripANSI(m.detailBreakdownView("b", "", 60))
	if !strings.Contains(v, "more") || !strings.Contains(v, "reveal") {
		t.Errorf("breakdown must summarise overflow with a '+N more (i to reveal)' line:\n%s", v)
	}
}

// TestGroupedObjectView130x24: at the supported minimum (130×24) every seeded enriched
// value is readable in the full-screen object view (the pane's reveal path), the grouped
// pane keeps its headers, and the footer never scrolls off (017 US2/FR-013, T021).
func TestGroupedObjectView130x24(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "docs/report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	m = deliver(m, tea.WindowSizeMsg{Width: 130, Height: 24})
	md := enrichedMeta()
	m.meta = &md
	m.mode = modeObject

	v := stripANSI(viewOf(m))
	for _, want := range []string{
		"docs/report.pdf", "aws:kms", "GOVERNANCE", "COMPLETE", "gzip", "max-age=3600",
		"Identity", "Security & governance", "Delivery",
	} {
		if !strings.Contains(v, want) {
			t.Errorf("object view at 130×24 must show %q (or its group):\n%s", want, v)
		}
	}
	lines := strings.Split(v, "\n")
	if len(lines) > 24 {
		t.Errorf("view exceeds 24 rows (%d) — footer scrolled off", len(lines))
	}
}

// TestBreakdownRendersInFullView: the inline breakdown shows in the real composed view at
// the full tier — and the footer/command-hint bar stays present (016 US3, constitution VI).
func TestBreakdownRendersInFullView(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m.usageResults.Put(m.usageKey("b", ""), bigReport(3))
	m.detailSection = sectBreakdown

	v := stripANSI(viewOf(m))
	if !strings.Contains(v, "breakdown") || !strings.Contains(v, "c00/") {
		t.Errorf("breakdown must render inline in the composed view:\n%s", v)
	}
	if !strings.Contains(v, "quit") {
		t.Errorf("footer/command-hint bar must remain visible (constitution VI):\n%s", v)
	}
}
