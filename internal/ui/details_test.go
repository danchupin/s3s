package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// fullApp builds a bucket-list app on a Full-tier terminal (≥130 cols → three zones).
func fullApp(f *storage.Fake) App {
	m := newApp(f, nil, nil)
	m.width, m.height = 140, 30
	bs, _ := f.ListBuckets(context.Background())
	return deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
}

// T026: at the Full tier all three zones render; details shows the highlighted bucket's
// metadata while focus is on the bucket list (011 US3).
func TestFullTierThreeZonesBucketMeta(t *testing.T) {
	f := storage.NewFake()
	f.Seed("mybucket", "a.txt")
	m := fullApp(f)
	v := viewOf(m)
	if !strings.Contains(v, "details") {
		t.Errorf("Full tier must render the details zone:\n%s", v)
	}
	if !strings.Contains(v, "Bucket") || !strings.Contains(v, "mybucket") {
		t.Errorf("details zone must show bucket metadata when focus is on buckets:\n%s", v)
	}
}

// T027: with focus in the objects zone the details zone shows the selected object's metadata
// (and would fill in its bounded preview via the debounced pane load) — 011 US3.
func TestFullTierObjectMeta(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "data.bin", storage.FakeObject{Data: make([]byte, 1234)})
	m := fullApp(f)
	m = crossToObjects(m, f, "b") // focus objects, entry 0 = data.bin
	if !strings.Contains(viewOf(m), "Size") {
		t.Errorf("details zone must show object metadata (Size) when focus is in objects:\n%s", viewOf(m))
	}
}

// T028: at the Dual tier the details zone collapses (only buckets│objects remain) — 011 US3/FR-015.
func TestDualTierNoDetails(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f) // width 120 → Dual
	if strings.Contains(viewOf(m), "details") {
		t.Errorf("Dual tier must collapse the details zone (no details box):\n%s", viewOf(m))
	}
}
