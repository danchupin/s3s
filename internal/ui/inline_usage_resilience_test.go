package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// TestUsageDwellGateDropsStaleTick: a usage tick whose generation is stale (the cursor
// moved on) starts NO scan; only a current tick on the still-focused target does
// (016 US2/FR-005 — rapid transit spawns no scan).
func TestUsageDwellGateDropsStaleTick(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := withBuckets(f, []string{"ctx"}, nil)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40}) // full tier → pane visible
	mm, _ := m.afterBucketMove()
	m = mm.(App)
	gen := m.usageGen

	mm, _ = m.Update(usageTickMsg{gen: gen - 1, bucket: "b", prefix: ""})
	if mm.(App).usageCh != nil {
		t.Error("a stale-generation tick must not start a scan")
	}

	mm, cmd := m.Update(usageTickMsg{gen: gen, bucket: "b", prefix: ""})
	if mm.(App).usageCh == nil || cmd == nil {
		t.Error("a current tick on the focused target should start a scan")
	}
}

// TestUsageRefreshInvalidates: tree 'r' invalidates the level's cached usage so the next
// focus rescans (016 US2/FR-007).
func TestUsageRefreshInvalidates(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: make([]byte, 10)})
	m := treeApp(f, false)
	m = driveUsageScan(t, m, "b", "")
	if _, ok := m.usageResults.Get(m.usageKey("b", "")); !ok {
		t.Fatal("precondition: usage should be cached")
	}
	mm, _ := m.refresh()
	if _, ok := mm.(App).usageResults.Get(m.usageKey("b", "")); ok {
		t.Error("tree refresh ('r') must invalidate the cached usage total")
	}
}

// TestUsageRefreshBucketsInvalidates: bucket-list 'r' invalidates the highlighted bucket's
// cached usage (the path that otherwise touches no cache) — 016 US2/FR-007.
func TestUsageRefreshBucketsInvalidates(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: make([]byte, 10)})
	m := withBuckets(f, []string{"ctx"}, nil)
	m = driveUsageScan(t, m, "b", "")
	if _, ok := m.usageResults.Get(m.usageKey("b", "")); !ok {
		t.Fatal("precondition: usage should be cached")
	}
	mm, _ := m.refreshBuckets()
	if _, ok := mm.(App).usageResults.Get(m.usageKey("b", "")); ok {
		t.Error("bucket-list refresh ('r') must invalidate the highlighted bucket's usage")
	}
}

// TestUsageContextSwitchClears: switching context clears the usage cache so figures never
// bleed across contexts (016 US2 edge case).
func TestUsageContextSwitchClears(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: make([]byte, 10)})
	m := treeApp(f, false)
	m = driveUsageScan(t, m, "b", "")
	if m.usageResults.Len() == 0 {
		t.Fatal("precondition: usage cache populated")
	}
	mm, _ := m.Update(contextResolvedMsg{gen: m.gen, target: "ctx2", be: Backend{Store: f, Cluster: "c2"}})
	if mm.(App).usageResults.Len() != 0 {
		t.Error("context switch must clear the usage cache")
	}
}
