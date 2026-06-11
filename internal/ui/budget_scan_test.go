package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// seedKeys seeds n one-byte objects with deterministic keys.
func seedKeys(f *storage.Fake, bucket string, n int) {
	for i := 0; i < n; i++ {
		f.SeedObject(bucket, fmt.Sprintf("obj-%06d", i), storage.FakeObject{Data: []byte("x")})
	}
}

// driveUsageCmd pumps a scan cmd's event stream through Update to its terminal done msg
// (the production drain discipline), returning the settled model.
func driveUsageCmd(t *testing.T, m App, cmd tea.Cmd) App {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a scan cmd, got nil")
	}
	msg := cmd()
	for {
		switch ev := msg.(type) {
		case usageProgressMsg:
			mm, _ := m.Update(ev)
			m = mm.(App)
			msg = waitForUsage(ev.ch, ev.gen, ev.key)()
		case usageDoneMsg:
			mm, _ := m.Update(ev)
			return mm.(App)
		default:
			t.Fatalf("unexpected msg %#v", ev)
		}
	}
}

// TestAmbientScanBudgetBounded: the dwell-tick scan is capped at the budget — a target
// bigger than the budget yields a CACHED lower bound rendered with "≥" + "partial", and
// the enumeration stops within one page of the cap (017 US1/FR-001/FR-002).
func TestAmbientScanBudgetBounded(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1500)
	m := withBuckets(f, []string{"ctx"}, nil)
	m.usageBudget = 1000

	mm, cmd := m.Update(usageTickMsg{gen: m.usageGen, bucket: "b", prefix: ""})
	m = driveUsageCmd(t, mm.(App), cmd)

	rep, ok := m.usageResults.Get(m.usageKey("b", ""))
	if !ok {
		t.Fatal("budget-bounded report must be cached (017 FR-002)")
	}
	if !rep.Bounded || rep.Complete {
		t.Errorf("got Bounded=%v Complete=%v, want Bounded=true Complete=false", rep.Bounded, rep.Complete)
	}
	if rep.TotalCount != 1000 {
		t.Errorf("TotalCount = %d, want 1000 (one page)", rep.TotalCount)
	}
	if f.UsagePages != 1 {
		t.Errorf("UsagePages = %d, want 1 (stop within one page of the cap)", f.UsagePages)
	}
	line := stripANSI(m.usageLine("b", ""))
	if !strings.Contains(line, "≥") || !strings.Contains(line, "partial") {
		t.Errorf("usageLine = %q, want a ≥ lower bound with a partial marker", line)
	}
}

// TestAmbientScanBoundaryExact: count == budget is an exact result — no marker.
func TestAmbientScanBoundaryExact(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1000)
	m := withBuckets(f, []string{"ctx"}, nil)
	m.usageBudget = 1000

	mm, cmd := m.Update(usageTickMsg{gen: m.usageGen, bucket: "b", prefix: ""})
	m = driveUsageCmd(t, mm.(App), cmd)

	rep, ok := m.usageResults.Get(m.usageKey("b", ""))
	if !ok || rep.Bounded || !rep.Complete {
		t.Fatalf("boundary scan: ok=%v rep=%+v, want exact (Complete, not Bounded)", ok, rep)
	}
	if line := stripANSI(m.usageLine("b", "")); strings.Contains(line, "≥") || strings.Contains(line, "partial") {
		t.Errorf("usageLine = %q, want NO lower-bound marker at the exact boundary", line)
	}
}

// TestBudgetZeroDisablesAmbient: budget 0 ⇒ the dwell path arms nothing; cached results
// still render (017 FR-006).
func TestBudgetZeroDisablesAmbient(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 10)
	m := withBuckets(f, []string{"ctx"}, nil)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40}) // full tier → pane visible
	m.usageBudget = 0

	if cmd := (&m).armUsageScan(); cmd != nil {
		t.Error("budget=0 must arm NO ambient scan tick")
	}
	if f.UsagePages != 0 {
		t.Errorf("UsagePages = %d, want 0 (no enumeration)", f.UsagePages)
	}
	m.usageResults.Put(m.usageKey("b", ""), &storage.UsageReport{TotalSize: 7, TotalCount: 1, Complete: true})
	if line := m.usageLine("b", ""); line == "" {
		t.Error("cached results must still render with ambient scanning off")
	}
}

// TestAmbientPartialIsCacheHit: a cached partial is a HIT for the ambient path — dwell
// renders it instantly and rescans nothing; only the explicit full scan upgrades it
// (contracts/budgeted-usage-scan.md).
func TestAmbientPartialIsCacheHit(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 10)
	m := withBuckets(f, []string{"ctx"}, nil)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m.usageBudget = 1000
	m.usageResults.Put(m.usageKey("b", ""), &storage.UsageReport{TotalSize: 999, TotalCount: 5, Bounded: true})

	if cmd := (&m).armUsageScan(); cmd != nil {
		t.Error("a partial cache entry must be an ambient HIT — no rescan tick")
	}
	if f.UsagePages != 0 {
		t.Errorf("UsagePages = %d, want 0", f.UsagePages)
	}
	if line := stripANSI(m.usageLine("b", "")); !strings.Contains(line, "≥") {
		t.Errorf("usageLine = %q, want the cached partial's ≥ totals", line)
	}
}

// TestBackendBudgetPlumb: the resolved budget reaches the App via Backend, never via
// config reads in the UI (017 T010, constitution I).
func TestBackendBudgetPlumb(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := New(Backend{Store: f, UsageScanBudget: 7}, "ctx", []string{"ctx"}, nil, nil, preview.ProtoNone)
	if m.usageBudget != 7 {
		t.Errorf("usageBudget = %d, want 7 from Backend", m.usageBudget)
	}
	mm, _ := m.Update(contextResolvedMsg{gen: m.gen, target: "ctx2", be: Backend{Store: f, UsageScanBudget: 3}})
	if got := mm.(App).usageBudget; got != 3 {
		t.Errorf("usageBudget after switch = %d, want 3 from the new Backend", got)
	}
}

// TestCancelledScanCachesLowerBound: a cancelled scan's partial progress IS cached and
// renders as a lower bound — never discarded (017 FR-004, inverts 016).
func TestCancelledScanCachesLowerBound(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := treeApp(f, false)
	key := m.usageKey("b", "")

	partial := storage.UsageReport{Bucket: "b", TotalSize: 50, TotalCount: 5, Complete: false}
	mm, _ := m.Update(usageDoneMsg{gen: m.usageGen, key: key, report: partial, err: context.Canceled})
	m = mm.(App)

	rep, ok := m.usageResults.Get(key)
	if !ok {
		t.Fatal("cancelled scan's partial progress must be cached (017 FR-004)")
	}
	if rep.TotalCount != 5 || rep.Complete {
		t.Errorf("cached partial = %+v, want the 5-object lower bound", rep)
	}
	if line := stripANSI(m.usageLine("b", "")); !strings.Contains(line, "≥") {
		t.Errorf("usageLine = %q, want ≥ for a cancelled partial", line)
	}
}

// TestCancelledEmptyScanNotCached: a cancellation before anything was learned leaves no
// entry (a zero lower bound is noise, not information).
func TestCancelledEmptyScanNotCached(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := treeApp(f, false)
	key := m.usageKey("b", "")

	mm, _ := m.Update(usageDoneMsg{gen: m.usageGen, key: key, report: storage.UsageReport{Bucket: "b"}, err: context.Canceled})
	if _, ok := mm.(App).usageResults.Get(key); ok {
		t.Error("a zero-progress cancelled scan must not create a cache entry")
	}
}

// TestExactNeverOverwrittenByPartial: an exact entry survives a late partial for the
// same key (017 data-model §2 transitions).
func TestExactNeverOverwrittenByPartial(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := treeApp(f, false)
	key := m.usageKey("b", "")
	m.usageResults.Put(key, &storage.UsageReport{TotalCount: 3, TotalSize: 30, Complete: true})

	mm, _ := m.Update(usageDoneMsg{gen: m.usageGen, key: key,
		report: storage.UsageReport{TotalCount: 1, TotalSize: 10, Bounded: true}})
	rep, ok := mm.(App).usageResults.Get(key)
	if !ok || !rep.Complete || rep.TotalCount != 3 {
		t.Errorf("exact entry was overwritten by a partial: %+v", rep)
	}
}

// TestFullScanResultReplacesPartial: a completed full scan overwrites the partial entry
// for its target (017 FR-005).
func TestFullScanResultReplacesPartial(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := treeApp(f, false)
	key := m.usageKey("b", "")
	m.usageResults.Put(key, &storage.UsageReport{TotalCount: 1000, TotalSize: 1000, Bounded: true})

	mm, _ := m.Update(usageDoneMsg{gen: m.usageGen, key: key,
		report: storage.UsageReport{TotalCount: 1500, TotalSize: 1500, Complete: true}})
	m = mm.(App)
	rep, ok := m.usageResults.Get(key)
	if !ok || !rep.Complete || rep.TotalCount != 1500 {
		t.Errorf("full result must replace the partial: %+v", rep)
	}
	if line := stripANSI(m.usageLine("b", "")); strings.Contains(line, "≥") {
		t.Errorf("usageLine = %q, want no marker after the full scan", line)
	}
}
