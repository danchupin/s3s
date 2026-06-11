package storage

import (
	"context"
	"testing"
	"time"
)

// fixedScanStart pins the age-histogram reference for deterministic buckets.
var fixedScanStart = time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

// distSeed places one object per age AND size bucket so each histogram row is hit
// exactly once (boundaries per data-model.md §2 / research D5).
func distSeed(f *Fake) {
	mk := func(key string, age time.Duration, size int64, class string) {
		f.SeedObject("b", key, FakeObject{
			Size:         size,
			LastModified: fixedScanStart.Add(-age),
			StorageClass: class,
		})
	}
	const (
		kib = int64(1024)
		mib = 1024 * kib
		gib = 1024 * mib
	)
	mk("a0", 1*time.Hour, 1*kib, "")                    // <1d      · <128KiB     · "" → STANDARD
	mk("a1", 3*24*time.Hour, 200*kib, "STANDARD")       // 1–7d     · 128KiB–1MiB
	mk("a2", 10*24*time.Hour, 2*mib, "COLD")            // 7–30d    · 1–16MiB
	mk("a3", 45*24*time.Hour, 20*mib, "COLD")           // 30–90d   · 16–128MiB
	mk("a4", 200*24*time.Hour, 200*mib, "STANDARD")     // 90–365d  · 128MiB–1GiB
	mk("a5", 2*365*24*time.Hour, gib+1, "DEEP_ARCHIVE") // >1y      · ≥1GiB
}

// TestUsageOfDistributions: age/size/class histograms accumulate in the SAME pass
// with fixed boundaries (017 US4, FR-020; research D5).
func TestUsageOfDistributions(t *testing.T) {
	f := NewFake()
	f.UsageScanStart = fixedScanStart
	distSeed(f)

	rep, err := f.UsageOf(context.Background(), "b", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if !rep.ScanStart.Equal(fixedScanStart) {
		t.Errorf("ScanStart = %v, want %v (injectable in Fake)", rep.ScanStart, fixedScanStart)
	}

	for i := 0; i < 6; i++ {
		if rep.AgeDist[i].Count != 1 {
			t.Errorf("AgeDist[%d].Count = %d, want 1", i, rep.AgeDist[i].Count)
		}
		if rep.SizeDist[i].Count != 1 {
			t.Errorf("SizeDist[%d].Count = %d, want 1", i, rep.SizeDist[i].Count)
		}
	}

	// "" normalizes to STANDARD: a0("") + a1 + a4 = 3 STANDARD objects.
	wantClasses := map[string]int{"STANDARD": 3, "COLD": 2, "DEEP_ARCHIVE": 1}
	if len(rep.ClassDist) != len(wantClasses) {
		t.Fatalf("ClassDist classes = %v, want %v", rep.ClassDist, wantClasses)
	}
	for class, n := range wantClasses {
		if rep.ClassDist[class].Count != n {
			t.Errorf("ClassDist[%q].Count = %d, want %d", class, rep.ClassDist[class].Count, n)
		}
	}
}

// TestUsageOfDistInvariants: Σ distribution counts/sizes == totals — the single-pass
// aggregation invariant (data-model §2).
func TestUsageOfDistInvariants(t *testing.T) {
	f := NewFake()
	f.UsageScanStart = fixedScanStart
	distSeed(f)

	rep, err := f.UsageOf(context.Background(), "b", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}

	sum := func(d [6]DistBucket) (int, int64) {
		var c int
		var s int64
		for _, b := range d {
			c += b.Count
			s += b.Size
		}
		return c, s
	}
	if c, s := sum(rep.AgeDist); c != rep.TotalCount || s != rep.TotalSize {
		t.Errorf("AgeDist Σ = %d/%d, want %d/%d", c, s, rep.TotalCount, rep.TotalSize)
	}
	if c, s := sum(rep.SizeDist); c != rep.TotalCount || s != rep.TotalSize {
		t.Errorf("SizeDist Σ = %d/%d, want %d/%d", c, s, rep.TotalCount, rep.TotalSize)
	}
	var cc int
	var cs int64
	for _, b := range rep.ClassDist {
		cc += b.Count
		cs += b.Size
	}
	if cc != rep.TotalCount || cs != rep.TotalSize {
		t.Errorf("ClassDist Σ = %d/%d, want %d/%d", cc, cs, rep.TotalCount, rep.TotalSize)
	}
}

// TestFakeObjectSizeOverride: FakeObject.Size lets tests model multi-GiB objects
// without allocating; len(Data) stays authoritative when Size is zero.
func TestFakeObjectSizeOverride(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "big", FakeObject{Size: 5 << 30})
	f.SeedObject("b", "small", FakeObject{Data: []byte("abc")})

	rep, err := f.UsageOf(context.Background(), "b", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if rep.TotalSize != (5<<30)+3 {
		t.Errorf("TotalSize = %d, want %d", rep.TotalSize, int64(5<<30)+3)
	}
}
