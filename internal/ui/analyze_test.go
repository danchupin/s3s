package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// runAnalyzeToDone starts the scan producer (runAnalyze's batch cmd is not executed in
// tests) and drives its progress events to the terminal report.
func runAnalyzeToDone(t *testing.T, m App) App {
	t.Helper()
	if m.usageCh == nil {
		t.Fatal("analyze did not arm a scan channel")
	}
	msg := analyzeCmd(m.loadCtx, m.activeStore(), m.usageBucket, m.usagePrefix, m.usageCh, m.gen)()
	for {
		switch ev := msg.(type) {
		case usageProgressMsg:
			mm, _ := m.Update(ev)
			m = mm.(App)
			msg = waitForUsage(m.usageCh, m.gen)()
		case usageDoneMsg:
			mm, _ := m.Update(ev)
			return mm.(App)
		default:
			t.Fatalf("unexpected msg %#v", msg)
		}
	}
}

// TestAnalyzeTotalsAndView: analyzing a level shows totals and the ranked children in
// the rendered view (005 US2, FR-008/FR-009).
func TestAnalyzeTotalsAndView(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "logs/a.log", storage.FakeObject{Data: make([]byte, 300)})
	f.SeedObject("b", "media/x.png", storage.FakeObject{Data: make([]byte, 100)})
	f.SeedObject("b", "big.bin", storage.FakeObject{Data: make([]byte, 250)})
	m := treeApp(f, false)

	mm, _ := m.runAnalyze("b", "")
	m = runAnalyzeToDone(t, mm.(App))

	if m.mode != modeUsage || m.usage == nil {
		t.Fatalf("analyze should land in modeUsage with a report; mode=%v usage=%v", m.mode, m.usage)
	}
	if m.usage.TotalCount != 3 || m.usage.TotalSize != 650 {
		t.Errorf("totals = %d/%d, want 3/650", m.usage.TotalCount, m.usage.TotalSize)
	}
	// Ranked largest-first: logs/ (300) before big.bin (250) before media/ (100).
	if len(m.usage.Children) != 3 || m.usage.Children[0].Name != "logs/" || m.usage.Children[1].Name != "big.bin" {
		t.Errorf("children ranking wrong: %+v", m.usage.Children)
	}
	v := viewOf(m)
	if !strings.Contains(v, "logs/") || !strings.Contains(v, "total") {
		t.Errorf("usage view missing ranked children or total:\n%s", v)
	}
}

// TestAnalyzeDrillDown: Enter on a sub-prefix child re-analyzes under it (005 FR-013).
func TestAnalyzeDrillDown(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "logs/2026/a.log", storage.FakeObject{Data: make([]byte, 40)})
	f.SeedObject("b", "logs/2026/b.log", storage.FakeObject{Data: make([]byte, 60)})
	f.SeedObject("b", "other/x", storage.FakeObject{Data: make([]byte, 5)})
	m := treeApp(f, false)

	mm, _ := m.runAnalyze("b", "")
	m = runAnalyzeToDone(t, mm.(App)) // top level: logs/ (100), other/ (5)
	if m.usage.Children[0].Name != "logs/" {
		t.Fatalf("precondition: logs/ should rank first; %+v", m.usage.Children)
	}
	// Drill into logs/ (selected row 0).
	mm, _ = m.onUsageKey("enter")
	m = runAnalyzeToDone(t, mm.(App))
	if m.usagePrefix != "logs/" {
		t.Errorf("drill-down prefix = %q, want logs/", m.usagePrefix)
	}
	if m.usage.TotalCount != 2 || m.usage.TotalSize != 100 {
		t.Errorf("drilled totals = %d/%d, want 2/100", m.usage.TotalCount, m.usage.TotalSize)
	}
	if m.usage.Children[0].Name != "2026/" {
		t.Errorf("drilled child = %+v, want 2026/", m.usage.Children)
	}
}

// TestAnalyzeBackReturnsToOrigin: leaving the analytics view returns to the mode it was
// launched from — the bucket list, not a phantom tree (regression for the 005 review).
func TestAnalyzeBackReturnsToOrigin(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: make([]byte, 3)})
	m := withBuckets(f, []string{"ctx"}, nil) // bucket list

	mm, _ := m.startAnalyze() // analyze the highlighted bucket
	m = runAnalyzeToDone(t, mm.(App))
	if m.mode != modeUsage {
		t.Fatalf("analyze should be in modeUsage; got %v", m.mode)
	}
	mm, _ = m.onUsageKey("esc")
	if got := mm.(App).mode; got != modeBuckets {
		t.Errorf("Back from a bucket-list analyze should return to modeBuckets, got %v", got)
	}
}

// TestAnalyzeEmptyPrefix: analyzing an empty prefix shows zero, not an error (FR-012).
func TestAnalyzeEmptyPrefix(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b") // no objects
	m := treeApp(f, false)

	mm, _ := m.runAnalyze("b", "nothing/")
	m = runAnalyzeToDone(t, mm.(App))
	if m.usage == nil || m.usage.TotalCount != 0 || m.usage.TotalSize != 0 {
		t.Errorf("empty analyze = %+v, want zero totals", m.usage)
	}
	if m.err != nil {
		t.Errorf("empty analyze should not error: %v", m.err)
	}
}
