package ui

import (
	"context"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// crossToObjects crosses focus into the objects zone (→) and delivers the bucket's root level,
// as the event loop would (011 US2).
func crossToObjects(m App, f *storage.Fake, bucket string) App {
	m = press(m, "right") // cross focus → loadObjectsLevel
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: bucket})
	return deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
}

// drillObjects descends into the selected folder in the objects zone and delivers its level.
func drillObjects(m App, f *storage.Fake) App {
	m = press(m, "enter")
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: m.bucket, Prefix: m.prefix})
	return deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
}

// T017: →/Tab moves focus into the objects zone; the objects cursor moves independently of bucketSel.
func TestCrossFocusIntoObjects(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt", "y.txt")
	m := dualApp(f)
	if m.focusZone != zoneBuckets {
		t.Fatalf("browse must start with focus on buckets")
	}
	m = crossToObjects(m, f, "b")
	if m.focusZone != zoneObjects {
		t.Fatalf("→ must cross focus into the objects zone")
	}
	bsel := m.bucketSel
	m = press(m, "down")
	if m.bucketSel != bsel {
		t.Errorf("bucket cursor must not move while focus is in objects (got %d want %d)", m.bucketSel, bsel)
	}
	if m.treeSel != 1 {
		t.Errorf("objects cursor should advance to 1, got %d", m.treeSel)
	}
}

// T024/T006: the focused zone renders an active title; crossing focus changes the styling.
func TestActiveZoneStyling(t *testing.T) {
	if boxViewFocus("zone", "", "body", 20, 3, true) == boxViewFocus("zone", "", "body", 20, 3, false) {
		t.Errorf("active and inactive zone titles must render differently (T006)")
	}
	f := storage.NewFake()
	f.Seed("b", "x.txt")
	m := dualApp(f)
	before := viewOf(m)
	m = crossToObjects(m, f, "b")
	if viewOf(m) == before {
		t.Errorf("crossing focus into objects must change the active-zone styling")
	}
}

// T018: Enter on a folder descends the objects level; the bucket list (bucketSel) is unchanged.
func TestObjectsDrillFolder(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "logs/a.log", "logs/b.log", "top.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b") // root: logs/ (dir) + top.txt
	bsel := m.bucketSel
	m = drillObjects(m, f) // enter on logs/
	if m.prefix != "logs/" {
		t.Errorf("drilling a folder should descend to logs/, prefix=%q", m.prefix)
	}
	if m.bucketSel != bsel {
		t.Errorf("bucket cursor must be unchanged while drilling objects")
	}
	if m.mode != modeBuckets {
		t.Errorf("drilling in the objects zone must stay multi-pane (modeBuckets), got %d", m.mode)
	}
}

// T019: Enter on an object opens the full-screen view; Esc restores modeBuckets + objects focus.
func TestObjectsOpenAndReturn(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "file.txt", storage.FakeObject{Data: []byte("hi")})
	m := dualApp(f)
	m = crossToObjects(m, f, "b") // entries: file.txt
	m, _ = updateCmd(m, keyMsgFor("enter"))
	if m.mode != modeObject {
		t.Fatalf("enter on an object should open the full-screen view, got mode %d", m.mode)
	}
	if m.objReturn != modeBuckets {
		t.Errorf("objReturn should be modeBuckets when opened from the objects zone, got %d", m.objReturn)
	}
	m.loading = false // simulate the object metadata/preview load completing
	m = press(m, "esc")
	if m.mode != modeBuckets {
		t.Errorf("esc should return to modeBuckets, got %d", m.mode)
	}
	if m.focusZone != zoneObjects {
		t.Errorf("focus should remain on the objects zone after returning")
	}
}

// T020: ←/Esc precedence in the objects zone — ascend one level, then (at root) return focus to buckets.
func TestObjectsBackPrecedence(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "logs/a.log", "top.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = drillObjects(m, f) // into logs/
	if m.prefix != "logs/" {
		t.Fatalf("setup: should be in logs/, prefix=%q", m.prefix)
	}
	m = press(m, "esc") // ascend to root (cache hit)
	if m.prefix != "" {
		t.Errorf("esc at depth should ascend to the root, prefix=%q", m.prefix)
	}
	if m.focusZone != zoneObjects {
		t.Errorf("focus should still be in objects after ascending one level")
	}
	m = press(m, "esc") // at root → return focus to buckets
	if m.focusZone != zoneBuckets {
		t.Errorf("esc at the root must return focus to the bucket list")
	}
}

// T021: Tab is a symmetric toggle — from a deep objects level it jumps straight back to buckets,
// and Tab again resumes the objects zone at the same position (FR-007 "preserve cursor").
func TestTabSymmetricToggle(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "logs/a.log", "top.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = drillObjects(m, f) // deep in logs/
	bsel := m.bucketSel
	m = press(m, "tab")
	if m.focusZone != zoneBuckets {
		t.Errorf("tab from a deep objects level should jump straight back to buckets")
	}
	if m.bucketSel != bsel {
		t.Errorf("bucket cursor must be preserved across the toggle")
	}
	m = press(m, "tab")
	if m.focusZone != zoneObjects {
		t.Errorf("tab again should return to the objects zone")
	}
	if m.prefix != "logs/" {
		t.Errorf("the objects position must be preserved at logs/, got %q", m.prefix)
	}
}
