package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// TestFullScanKeyDispatch: `A` starts the UNCAPPED scan for the focused target; the
// result is exact even past the ambient budget (017 US1/FR-003).
func TestFullScanKeyDispatch(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1500)
	m := treeApp(f, false)
	m.usageBudget = 1000

	mm, cmd := pressCmd(m, "A")
	m = mm
	if m.usageCh == nil {
		t.Fatal("A must start a scan")
	}
	m = driveUsageCmd(t, m, cmd)

	rep, ok := m.usageResults.Get(m.usageKey("b", ""))
	if !ok || !rep.Complete || rep.Bounded || rep.TotalCount != 1500 {
		t.Errorf("full scan result = ok=%v %+v, want exact 1500 (uncapped)", ok, rep)
	}
}

// TestScanCommandSameDispatcher: `:scan` reaches the same dispatcher as `A` — identical
// uncapped behavior (017 FR-003: ONE dedicated explicit action).
func TestScanCommandSameDispatcher(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1500)
	m := treeApp(f, false)
	m.usageBudget = 1000

	m = press(m, ":")
	for _, r := range "scan" {
		m = press(m, string(r))
	}
	mm, cmd := pressCmd(m, "enter")
	m = driveUsageCmd(t, mm, cmd)

	rep, ok := m.usageResults.Get(m.usageKey("b", ""))
	if !ok || !rep.Complete || rep.TotalCount != 1500 {
		t.Errorf(":scan result = ok=%v %+v, want exact 1500", ok, rep)
	}
}

// TestFullScanStreamsProgress: the explicit scan streams running totals through the
// usage progress path (017 FR-003).
func TestFullScanStreamsProgress(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1500) // 2 Fake pages → ≥2 progress events
	m := treeApp(f, false)

	mm, cmd := m.startFullScan()
	m = mm.(App)
	msg := cmd()
	ev, okType := msg.(usageProgressMsg)
	if !okType {
		t.Fatalf("first scan event = %#v, want usageProgressMsg", msg)
	}
	m = deliver(m, ev)
	if m.usageProg.ScannedCount == 0 {
		t.Error("running totals must stream into usageProg during the full scan")
	}
}

// TestMoreDetailRunsBudgetedOnly: `a` (breakdown) on an uncached target triggers at most
// the BUDGETED scan — never the unbounded one (017 FR-003, inverts 016 startMoreDetail).
func TestMoreDetailRunsBudgetedOnly(t *testing.T) {
	f := storage.NewFake()
	for i := 0; i < 1500; i++ {
		f.SeedObject("b", fmt.Sprintf("dir/obj-%06d", i), storage.FakeObject{Data: []byte("x")})
	}
	m := treeApp(f, false) // selection lands on the "dir/" folder → a folder usage target
	m.usageBudget = 1000

	mm, cmd := m.startMoreDetail()
	m = driveUsageCmd(t, mm.(App), cmd)

	rep, ok := m.usageResults.Get(m.usageKey("b", "dir/"))
	if !ok || !rep.Bounded {
		t.Fatalf("breakdown scan = ok=%v %+v, want a budget-bounded report", ok, rep)
	}
	if f.UsagePages != 1 {
		t.Errorf("UsagePages = %d, want 1 — `a` must never run unbounded work", f.UsagePages)
	}
}

// TestFullScanAffordanceHint: the pane advertises `A full scan` while the target is
// uncached or partial, and drops the hint once the result is exact (017 FR-001).
func TestFullScanAffordanceHint(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := withBuckets(f, []string{"ctx"}, nil)

	if v := stripANSI(m.paneBucket(60)); !strings.Contains(v, "full scan") {
		t.Errorf("uncached target: pane must show the full-scan affordance:\n%s", v)
	}
	m.usageResults.Put(m.usageKey("b", ""), &storage.UsageReport{TotalCount: 5, TotalSize: 50, Bounded: true})
	if v := stripANSI(m.paneBucket(60)); !strings.Contains(v, "full scan") {
		t.Errorf("partial target: pane must keep the full-scan affordance:\n%s", v)
	}
	m.usageResults.Put(m.usageKey("b", ""), &storage.UsageReport{TotalCount: 5, TotalSize: 50, Complete: true})
	if v := stripANSI(m.paneBucket(60)); strings.Contains(v, "full scan") {
		t.Errorf("exact target: the full-scan affordance must disappear:\n%s", v)
	}
}
