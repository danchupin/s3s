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
