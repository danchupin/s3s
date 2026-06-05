package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/storage"
)

func TestBucketFilter(t *testing.T) {
	f := storage.NewFake()
	f.Seed("assets")
	f.Seed("archive")
	f.Seed("backups")
	m := withBuckets(f, []string{"ctx"}, nil)

	m = press(m, "/")
	if !m.searching {
		t.Fatal("'/' should open the bucket filter")
	}
	m = press(m, "a") // filter "a" → assets, archive, backups (all contain 'a')
	if len(m.filteredBuckets()) != 3 {
		t.Fatalf("filter 'a' = %d buckets, want 3", len(m.filteredBuckets()))
	}
	m = press(m, "r") // "ar" → only archive contains "ar"
	got := m.filteredBuckets()
	if len(got) != 1 || got[0].Name != "archive" {
		t.Fatalf("filter 'ar' = %+v, want [archive]", got)
	}

	// Esc clears the filter, restoring all buckets.
	m = press(m, "esc")
	if m.bucketFilter != "" || len(m.filteredBuckets()) != 3 {
		t.Fatalf("esc should clear filter; got %q / %d", m.bucketFilter, len(m.filteredBuckets()))
	}
}

func TestDigitContextSwitch(t *testing.T) {
	f := storage.NewFake()
	f.Seed("a")
	other := storage.NewFake()
	other.Seed("b")
	resolve := func(name string) (Backend, error) {
		if name == "two" {
			return Backend{Store: other}, nil
		}
		return Backend{Store: f}, nil
	}
	m := withBuckets(f, []string{"one", "two", "three"}, resolve)
	m.ctxName = "one"

	m = press(m, "2") // jump to second context
	if m.ctxName != "two" {
		t.Fatalf("digit '2' should switch to second context, got %q", m.ctxName)
	}
}

func TestFooterFitsWidthAndShowsHints(t *testing.T) {
	f := storage.NewFake()
	f.Seed("hot")
	m := New(Backend{Store: f, Cluster: "c", User: "u",
		Endpoint: "https://very-long-endpoint.example.storage.internal:9000", Region: "us-east-1"},
		"my-context", []string{"my-context"}, nil, 0)
	m.width, m.height = 60, 16
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: []storage.Bucket{{Name: "hot"}}})

	v := m.View().Content
	for _, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line exceeds width 60 (=%d): %q", w, line)
		}
	}
	if !strings.Contains(v, "quit") || !strings.Contains(v, "filter") {
		t.Errorf("footer hints missing from view:\n%s", v)
	}
}

// --- US2 footer: helpers (T002) ---

func footerLineCount(m App, w int) int {
	return strings.Count(m.footerBlock(w), "\n") + 1
}

// assertWidthSweep checks that, across widths [lo,hi], no FOOTER line exceeds the width
// and the footer never exceeds 3 rows (FR-006/FR-019, SC-002). FR-019 scopes the
// no-overflow guarantee to footer/menu/help/status — not the table body.
func assertWidthSweep(t *testing.T, build func(w int) App, lo, hi int) {
	t.Helper()
	for w := lo; w <= hi; w++ {
		m := build(w)
		m.width, m.height = w, 24
		for _, ln := range strings.Split(m.footerBlock(w), "\n") {
			if lipgloss.Width(ln) > w {
				t.Fatalf("w=%d: footer line exceeds width (%d): %q", w, lipgloss.Width(ln), ln)
			}
		}
		if fr := footerLineCount(m, w); fr > 3 {
			t.Fatalf("w=%d: footer rows=%d, want ≤3", w, fr)
		}
	}
}

func containsAny(s string, subs ...string) string {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return sub
		}
	}
	return ""
}

// --- US2 footer contract obligations ---

func TestFooterHintsAdvertiseActionsNotWriteKeys(t *testing.T) { // obligation 1/2
	h := footerHints(hintCtx{mode: modeTree, width: 80})
	if strings.Count(h, "\n") != 0 {
		t.Fatalf("hint row must be a single line; got %q", h)
	}
	for _, want := range []string{"actions", "help", "quit"} {
		if !strings.Contains(h, want) {
			t.Errorf("hint row missing %q; got %q", want, h)
		}
	}
	if bad := containsAny(h, "del", "upload", "copy", "move", "rmdir", "folder", "refresh"); bad != "" {
		t.Errorf("hint row must not advertise individual write/refresh ops; found %q in %q", bad, h)
	}
}

func TestFooterHintsSingleContextHidesSwitch(t *testing.T) { // obligation 3 (FR-003)
	// A single context makes the numeric quick-switch inapplicable → never shown.
	single := footerHints(hintCtx{mode: modeTree, multiContext: false, width: 200})
	if strings.Contains(single, "switch") || strings.Contains(single, "context") {
		t.Errorf("single-context footer must omit context/switch hints; got %q", single)
	}
	// With multiple contexts the hint is applicable; with no competing nav (object
	// view has only enter/back) it surfaces within the 6-cap.
	multi := footerHints(hintCtx{mode: modeObject, multiContext: true, width: 200})
	if !strings.Contains(multi, "context") {
		t.Errorf("multi-context footer should surface the context hint when room allows; got %q", multi)
	}
}

func TestFooterWidthSweepNoOverflow(t *testing.T) { // obligation 4
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	assertWidthSweep(t, func(int) App {
		m := treeApp(f, true)
		selectObject(&m, "a.txt")
		return m
	}, 40, 200)
}

func TestFooterHintsNarrowDropsWithMoreCue(t *testing.T) { // obligation 5
	h := footerHints(hintCtx{mode: modeTree, multiContext: true, width: 30})
	if !strings.Contains(h, "? more") {
		t.Errorf("narrow footer should append a '? more' cue; got %q", h)
	}
	if !strings.Contains(h, "help") || !strings.Contains(h, "quit") {
		t.Errorf("help/quit must survive every drop; got %q", h)
	}
	if lipgloss.Width(h) > 30 {
		t.Errorf("narrow hint row overflows width 30 (=%d): %q", lipgloss.Width(h), h)
	}
}

func TestFooterHintsArrowPrimaryNoVim(t *testing.T) { // obligation 6 (FR-031)
	h := footerHints(hintCtx{mode: modeTree, width: 120})
	if !strings.Contains(h, "↵") {
		t.Errorf("nav cue should use the arrow/enter glyph; got %q", h)
	}
	if bad := containsAny(h, " j", " k", " g", " G", "Home", "End", "hjkl"); bad != "" {
		t.Errorf("footer must not advertise vim/Top-Bottom keys; found %q in %q", bad, h)
	}
}

func TestFooterHintsSearchClearVsBack(t *testing.T) { // obligation 7 (FR-009)
	active := footerHints(hintCtx{mode: modeTree, searchActive: true, width: 120})
	if !strings.Contains(active, "clear") || strings.Contains(active, "back") {
		t.Errorf("search-active footer should show 'esc clear' and not 'esc back'; got %q", active)
	}
	idle := footerHints(hintCtx{mode: modeTree, searchActive: false, width: 120})
	if !strings.Contains(idle, "back") || strings.Contains(idle, "clear") {
		t.Errorf("idle footer should show 'esc back' and not 'esc clear'; got %q", idle)
	}
}

func TestFooterHintsCapSixDropsLowestPrio(t *testing.T) { // obligation 8 (SC-003)
	h := footerHints(hintCtx{mode: modeTree, multiContext: true, width: 200})
	// 8 applicable > 6: the lowest-prio (context/switch, prio 40) are dropped first.
	if strings.Contains(h, "switch") {
		t.Errorf("cap should drop the lowest-prio switch hint; got %q", h)
	}
	if !strings.Contains(h, "? more") {
		t.Errorf("capped footer should show '? more'; got %q", h)
	}
	for _, want := range []string{"open", "actions", "help", "quit"} {
		if !strings.Contains(h, want) {
			t.Errorf("high-prio hint %q should survive the cap; got %q", want, h)
		}
	}
}

func TestBoxLongTitleNoOverflow(t *testing.T) {
	f := storage.NewFake()
	m := withBuckets(f, []string{"ctx"}, nil)
	m.width, m.height = 40, 14
	m.mode = modeObject
	longKey := strings.Repeat("verylongsegment/", 12) + "file.bin"
	md := storage.ObjectMetadata{Key: longKey, ContentType: "application/octet-stream"}
	m.meta = &md

	v := m.View().Content
	for _, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("long-title line exceeds width 40 (=%d)", w)
		}
	}
}
