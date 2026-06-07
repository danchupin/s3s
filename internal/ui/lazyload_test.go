package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// dualApp builds a bucket-list app on a Dual-tier terminal (objects zone visible) with the
// bucket list delivered. Width ≥100 so afterBucketMove arms the objects-zone preview (011 US1).
func dualApp(f *storage.Fake) App {
	m := newApp(f, nil, nil)
	m.width, m.height = 120, 30
	bs, _ := f.ListBuckets(context.Background())
	return deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
}

func updateCmd(m App, msg tea.Msg) (App, tea.Cmd) {
	mm, cmd := m.Update(msg)
	return mm.(App), cmd
}

// loadObjects delivers a settle tick for the highlighted bucket then its level page,
// populating the objects zone as the event loop would (011 US1).
func loadObjects(m App, f *storage.Fake, bucket string) App {
	m = deliver(m, bucketTickMsg{gen: m.bucketLoadGen, bucket: bucket})
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: bucket})
	return deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
}

// T007: startup lists only bucket NAMES — zero object-level listings (FR-002a / SC-010).
func TestStartupNoObjectListing(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	f.Seed("c", "x.txt")
	before := f.ListLevelCalls
	m := dualApp(f)
	if f.ListLevelCalls != before {
		t.Errorf("startup must issue no object-level listings, got %d", f.ListLevelCalls-before)
	}
	if m.level != nil || m.bucket != "" {
		t.Errorf("no bucket level loaded at startup (bucket=%q level=%v)", m.bucket, m.level)
	}
}

// T008: highlighting a bucket renders its first-level entries in the objects zone (SC-001).
func TestObjectsZoneShowsBucketContents(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "logs/x.log", "readme.txt")
	m := dualApp(f) // single bucket "b", bucketSel=0
	m = loadObjects(m, f, "b")
	v := viewOf(m)
	if !strings.Contains(v, "readme.txt") || !strings.Contains(v, "logs") {
		t.Errorf("objects zone must show bucket b's first level:\n%s", v)
	}
}

// T009: a stale (scrolled-past) settle tick is dropped; only the settled selection loads (FR-004).
func TestObjectsZoneStaleTickDropped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("a", "x")
	f.Seed("b", "y")
	f.Seed("c", "z")
	m := dualApp(f)      // sorted a,b,c; bucketSel=0 "a", bucketLoadGen=1
	m = press(m, "down") // "b", gen=2
	m = press(m, "down") // "c", gen=3
	if m2, _ := updateCmd(m, bucketTickMsg{gen: 2, bucket: "b"}); m2.bucket != "" {
		t.Errorf("stale tick (gen 2) must be dropped, bucket=%q", m2.bucket)
	}
	m3, cmd := updateCmd(m, bucketTickMsg{gen: m.bucketLoadGen, bucket: "c"})
	if m3.bucket != "c" {
		t.Errorf("settled tick should load bucket c, got %q", m3.bucket)
	}
	if cmd == nil {
		t.Errorf("settled tick (cache miss) should dispatch a load")
	}
}

// T010: revisiting a cached bucket issues no additional load — cache hit (FR-006 / SC-004).
func TestObjectsZoneRevisitCacheHit(t *testing.T) {
	f := storage.NewFake()
	f.Seed("a", "x")
	f.Seed("b", "y")
	m := dualApp(f)
	m = loadObjects(m, f, "a") // load + cache "a"
	m = press(m, "down")       // cursor "b"
	m = loadObjects(m, f, "b") // m.bucket="b"
	m = press(m, "up")         // cursor back on "a"
	_, cmd := updateCmd(m, bucketTickMsg{gen: m.bucketLoadGen, bucket: "a"})
	if cmd != nil {
		t.Errorf("revisiting a cached bucket must not dispatch a load (cache hit)")
	}
}

// T011a: an empty bucket renders the exact (empty) marker (FR-005).
func TestObjectsZoneEmpty(t *testing.T) {
	f := storage.NewFake()
	f.Seed("empty") // bucket, no objects
	m := dualApp(f)
	m = loadObjects(m, f, "empty")
	if v := viewOf(m); !strings.Contains(v, "(empty)") {
		t.Errorf("empty bucket must render (empty) marker:\n%s", v)
	}
}

// T011b: an in-flight objects load renders the loading marker (FR-005).
func TestObjectsZoneLoading(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f)
	m = deliver(m, bucketTickMsg{gen: m.bucketLoadGen, bucket: "b"}) // load dispatched, level nil
	if v := viewOf(m); !strings.Contains(v, "loading…") {
		t.Errorf("in-flight objects load must render loading marker:\n%s", v)
	}
}

// T011c: a denied objects listing shows an in-pane error and does NOT flip bucketsScoped
// (the connection is not mislabeled), and the error is not cached (FR-006c / FR-018).
func TestObjectsZoneDeniedNotScoped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("secret", "x")
	f.AccessDeniedBuckets = map[string]bool{"secret": true}
	m := dualApp(f)
	m, _ = updateCmd(m, bucketTickMsg{gen: m.bucketLoadGen, bucket: "secret"}) // beginLoad, m.gen bumped
	m = deliver(m, errMsg{gen: m.gen, err: storage.ErrAccessDenied})
	if !strings.Contains(viewOf(m), "error:") {
		t.Errorf("denied objects listing must show an in-pane error")
	}
	if m.bucketsScoped {
		t.Errorf("an objects-zone listing error must NOT flip bucketsScoped (connection mislabeled)")
	}
}
