package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Regression tests for the T053 manual-validation findings (fix/017): the usage line
// overflowed the details pane (the "(partial)" marker clipped at the box border), and a
// full scan running over a CACHED partial was invisible — the cache hit outranked the
// in-flight progress, so the UI looked dead until the scan finished.

// TestUsageLineFitsNarrowPane: the rendered line never exceeds the pane width, and the
// lower-bound semantics (≥ + partial) survive the compact form (constitution VI — no
// silent clipping at the box border).
func TestUsageLineFitsNarrowPane(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := treeApp(f, false)
	m.usageResults.Put(m.usageKey("b", ""), &storage.UsageReport{
		TotalSize: 1 << 30, TotalCount: 20901, Bounded: true, // the exact T053 repro figures
	})

	for _, w := range []int{38, 30, 24} {
		line := m.usageLine("b", "", w)
		if got := lipgloss.Width(line); got > w {
			t.Errorf("w=%d: rendered width %d overflows the pane (clips at the border)", w, got)
		}
		plain := stripANSI(line)
		if !strings.Contains(plain, "≥") {
			t.Errorf("w=%d: lower bound lost: %q", w, plain)
		}
	}
	// At a comfortable width the partial marker is spelled out in full.
	if plain := stripANSI(m.usageLine("b", "", 38)); !strings.Contains(plain, "partial") {
		t.Errorf("w=38 must carry the partial marker: %q", plain)
	}
}

// TestUsageLineScanningOutranksCachedPartial: an in-flight scan for the SAME target
// renders its running totals INSTEAD of the stale cached entry — a full scan over a
// cached partial must be visibly alive (T053 finding 2).
func TestUsageLineScanningOutranksCachedPartial(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1500)
	m := treeApp(f, false)
	key := m.usageKey("b", "")
	m.usageResults.Put(key, &storage.UsageReport{TotalSize: 100, TotalCount: 1000, Bounded: true})

	mm, _ := m.startFullScan()
	m = mm.(App)
	m.usageProg = storage.UsageProgress{ScannedCount: 1234, ScannedSize: 4096}

	plain := stripANSI(m.usageLine("b", "", 60))
	if !strings.Contains(plain, "full scan") {
		t.Errorf("an in-flight FULL scan must announce itself: %q", plain)
	}
	if !strings.Contains(plain, "1234") {
		t.Errorf("running totals must be visible during the scan: %q", plain)
	}
	if strings.Contains(plain, "partial") {
		t.Errorf("the stale cached partial must not mask the live progress: %q", plain)
	}
}

// TestUsageLineProgressUpdatesLive: each usageProgressMsg advances the rendered totals
// — the scan visibly ticks in the pane (T053 finding 2).
func TestUsageLineProgressUpdatesLive(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 2500) // 3 Fake pages → ≥2 progress events before done
	m := treeApp(f, false)

	mm, cmd := m.startFullScan()
	m = mm.(App)
	msg := cmd()
	ev, ok := msg.(usageProgressMsg)
	if !ok {
		t.Fatalf("first event = %#v, want usageProgressMsg", msg)
	}
	m = deliver(m, ev)
	first := stripANSI(m.usageLine("b", "", 60))
	if !strings.Contains(first, "1000") {
		t.Errorf("after page 1 the line must show 1000 scanned: %q", first)
	}
	msg = waitForUsage(ev.ch, ev.gen, ev.key)()
	ev2, ok := msg.(usageProgressMsg)
	if !ok {
		t.Fatalf("second event = %#v, want usageProgressMsg", msg)
	}
	m = deliver(m, ev2)
	if second := stripANSI(m.usageLine("b", "", 60)); !strings.Contains(second, "2000") {
		t.Errorf("after page 2 the line must advance to 2000: %q", second)
	}
}

// TestPaneBucketLinesFitWidth: every line the bucket pane renders fits the width it was
// given — nothing left to be clipped by the box border (constitution VI).
func TestPaneBucketLinesFitWidth(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("st-img-range-bucket-1403628312", "x", storage.FakeObject{Data: []byte("y")})
	m := withBuckets(f, []string{"ctx"}, nil)
	m.usageResults.Put(m.usageKey("st-img-range-bucket-1403628312", ""), &storage.UsageReport{
		TotalSize: 1 << 30, TotalCount: 20901, Bounded: true,
	})

	const w = 38
	for i, line := range strings.Split(m.paneBucket(w), "\n") {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("pane line %d width %d > %d (border clip): %q", i, got, w, stripANSI(line))
		}
	}
}
