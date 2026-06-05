package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// enterTree drills into the (only/first) bucket and delivers the root level page.
func enterTree(t *testing.T, f *storage.Fake, bucket string) App {
	t.Helper()
	m := withBuckets(f, []string{"ctx"}, nil)
	// position on the requested bucket
	for i, b := range m.buckets {
		if b.Name == bucket {
			m.bucketSel = i
		}
	}
	m = press(m, "enter") // drill into bucket → arms a level load
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: bucket})
	return deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
}

func TestTreeDrillDownAndBack(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "docs/a.md", "docs/deep/x.txt", "top.txt")
	m := enterTree(t, f, "b")

	if m.mode != modeTree {
		t.Fatalf("mode = %v, want tree", m.mode)
	}
	if !strings.Contains(m.breadcrumb(), "b") {
		t.Errorf("breadcrumb = %q, want bucket name", m.breadcrumb())
	}
	entries := m.treeEntries()
	if len(entries) != 2 || !entries[0].isDir || entries[0].label != "docs/" {
		t.Fatalf("root entries = %+v, want [docs/ dir, top.txt obj]", entries)
	}

	// Drill into docs/.
	m = press(m, "enter")
	sub, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Prefix: "docs/"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: sub})
	if m.prefix != "docs/" {
		t.Fatalf("prefix = %q, want docs/", m.prefix)
	}
	if !strings.Contains(m.breadcrumb(), "docs/") {
		t.Errorf("breadcrumb = %q, want docs/", m.breadcrumb())
	}

	// Back to root — served from cache (no new load).
	m = press(m, "left")
	if m.prefix != "" {
		t.Fatalf("after back, prefix = %q, want root", m.prefix)
	}
	if m.loading {
		t.Error("returning to a cached level should not reload (FR-011)")
	}
	if len(m.treeEntries()) != 2 {
		t.Errorf("root level not restored from cache: %+v", m.treeEntries())
	}
}

func TestTreeBackFromRootReturnsToBuckets(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt")
	m := enterTree(t, f, "b")
	m = press(m, "esc")
	if m.mode != modeBuckets {
		t.Errorf("esc at root should return to buckets, mode = %v", m.mode)
	}
}

func TestTreeEmptyAndNoMatch(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b") // empty bucket
	m := enterTree(t, f, "b")
	if !strings.Contains(viewOf(m), "Empty") {
		t.Errorf("empty level should show empty message:\n%s", viewOf(m))
	}
}

func TestTreePagingOnScrollTriggersOneLoad(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a", "z")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, "enter")
	// Deliver a partial page (NextToken set) to simulate more pages.
	tok := "next"
	partial := storage.Page{
		Objects:   []storage.ObjectRef{{Key: "a"}, {Key: "z"}},
		NextToken: &tok,
	}
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: partial})
	if m.level.complete {
		t.Fatal("level should be incomplete (NextToken set)")
	}

	// Move to last entry, then Down again → exactly one next-page load.
	m = press(m, "down") // sel 0 -> 1 (last)
	genBefore := m.gen
	m, cmd := pressCmd(m, "down") // at end → fetch next page
	if cmd == nil {
		t.Fatal("scrolling past end should trigger a load")
	}
	if m.gen != genBefore+1 {
		t.Fatalf("expected exactly one new load (gen %d -> %d)", genBefore, m.gen)
	}
	if !m.loading {
		t.Error("should be loading after paging")
	}

	// A second Down while loading must NOT trigger another load.
	genDuring := m.gen
	m, cmd2 := pressCmd(m, "down")
	if cmd2 != nil || m.gen != genDuring {
		t.Errorf("second scroll while loading should not load again (gen %d -> %d)", genDuring, m.gen)
	}
}

func TestTreeStaleLevelDropped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt")
	m := enterTree(t, f, "b")
	countBefore := len(m.treeEntries())

	// A stale page from a superseded generation must be ignored.
	stale := storage.Page{Objects: []storage.ObjectRef{{Key: "ghost"}}}
	m = deliver(m, levelMsg{gen: m.gen - 1, key: m.levelKey(), page: stale})
	if len(m.treeEntries()) != countBefore {
		t.Errorf("stale level msg should be dropped; entries changed to %+v", m.treeEntries())
	}
}

func TestTreeRefreshInvalidatesCache(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt")
	m := enterTree(t, f, "b")
	key := m.levelKey()
	if _, ok := m.cache.Get(key); !ok {
		t.Fatal("level should be cached after load")
	}

	m = viaMenu(t, m, "refresh")
	if _, ok := m.cache.Get(key); ok {
		t.Error("refresh should invalidate the cached level (FR-011a)")
	}
	if !m.loading || m.level != nil {
		t.Error("refresh should arm a fresh load (loading, level cleared)")
	}
}

func TestParentPrefix(t *testing.T) {
	cases := map[string]string{
		"a/b/": "a/",
		"a/":   "",
		"":     "",
	}
	for in, want := range cases {
		if got := parentPrefix(in); got != want {
			t.Errorf("parentPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeLabel(t *testing.T) {
	in := "ab\x00c\td\x07e" + string([]byte{0xff, 0xfe})
	out := sanitizeLabel(in)
	if strings.ContainsRune(out, 0x00) || strings.ContainsRune(out, 0x07) {
		t.Errorf("control chars leaked: %q", out)
	}
	if !strings.Contains(out, "ab") || !strings.Contains(out, "c d") {
		t.Errorf("tab should become space, printable kept: %q", out)
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		0:       "0 B",
		512:     "512 B",
		1024:    "1.0 KiB",
		1048576: "1.0 MiB",
	}
	for n, want := range cases {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}
