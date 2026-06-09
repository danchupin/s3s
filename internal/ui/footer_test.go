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

	m = press(m, "2") // jump to second context (resolve runs off the event loop)
	m = finishSwitch(m, resolve, "two")
	if m.ctxName != "two" {
		t.Fatalf("digit '2' should switch to second context, got %q", m.ctxName)
	}
}

func TestFooterFitsWidthAndShowsHints(t *testing.T) {
	f := storage.NewFake()
	f.Seed("hot")
	m := New(Backend{Store: f, Cluster: "c", User: "u",
		Endpoint: "https://very-long-endpoint.example.storage.internal:9000", Region: "us-east-1"},
		"my-context", []string{"my-context"}, nil, nil, 0)
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
// and the footer never exceeds maxRows (FR-006/FR-019, SC-002/SC-005). FR-019 scopes the
// no-overflow guarantee to footer/menu/help/status — not the table body. 007 US1: the
// list-mode command bar is a multi-row block (columns when wide, ≤3 stacked rows + status
// when narrow), so the row budget is larger than the old single-strip footer.
func assertWidthSweep(t *testing.T, build func(w int) App, lo, hi, maxRows int) {
	t.Helper()
	for w := lo; w <= hi; w++ {
		m := build(w)
		m.width, m.height = w, 24
		for _, ln := range strings.Split(m.footerBlock(w), "\n") {
			if lipgloss.Width(ln) > w {
				t.Fatalf("w=%d: footer line exceeds width (%d): %q", w, lipgloss.Width(ln), ln)
			}
		}
		if fr := footerLineCount(m, w); fr > maxRows {
			t.Fatalf("w=%d: footer rows=%d, want ≤%d", w, fr, maxRows)
		}
		// 015: the always-visible filter form fits — each of its 3 lines is within the width.
		for _, fl := range strings.Split(m.scopeFilterField(w), "\n") {
			if lipgloss.Width(fl) > w {
				t.Fatalf("w=%d: filter-form line exceeds width (%d): %q", w, lipgloss.Width(fl), stripANSI(fl))
			}
		}
	}
}

func lineCount(s string) int { return strings.Count(s, "\n") + 1 }

// assertHeightSweep checks that as the terminal shrinks vertically, the footer AND the filter
// form stay fully rendered and only the LIST gives up rows (015 layout-budget-contract). Within
// the floor-free range the composed view fills EXACTLY the available height — proof that the
// footer/form are never clipped and the list absorbs every row of loss.
func assertHeightSweep(t *testing.T, build func() App, w int) {
	t.Helper()
	base := build()
	base.width = w
	footerH := strings.Count(base.footerBlock(w), "\n") + 1
	loH := footerH + 8 // smallest height with no row-floor clamp (list inner ≥ 3 + form band 3 + 2)
	for h := loH; h <= loH+24; h++ {
		m := build()
		m.width, m.height = w, h
		view := m.View().Content
		if lc := lineCount(view); lc != h {
			t.Fatalf("w=%d h=%d: view fills %d lines, want exactly %d (footer/form must stay; list absorbs)", w, h, lc, h)
		}
		if !strings.Contains(view, m.footerBlock(w)) {
			t.Fatalf("w=%d h=%d: footer clipped", w, h)
		}
		if !strings.Contains(stripANSI(view), "filter objects") {
			t.Fatalf("w=%d h=%d: filter form clipped", w, h)
		}
	}
}

// 015 US2: the filter form fits at every width and keeps a usable input field (≥10 cols) even at
// the narrowest supported width (FR-006).
func TestFilterFormWidthSweepUsable(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple", "apricot")
	const longTerm = "abcdefghijklmnopqrst" // 20 chars
	build := func(w int) App {
		m := treeApp(f, true)
		m.width, m.height = w, 24
		m.searching = true
		m.searchInput = longTerm
		return m
	}
	for w := 40; w <= 200; w++ {
		field := build(w).scopeFilterField(w)
		if lc := lineCount(field); lc != 3 {
			t.Fatalf("w=%d: filter form must be exactly 3 lines, got %d", w, lc)
		}
		for _, fl := range strings.Split(field, "\n") {
			if lipgloss.Width(fl) > w {
				t.Fatalf("w=%d: form line overflows (%d): %q", w, lipgloss.Width(fl), stripANSI(fl))
			}
		}
	}
	// At the narrowest supported width the editable field keeps ≥10 visible columns: the first 10
	// chars of the typed term stay visible (FR-006).
	if narrow := stripANSI(build(40).scopeFilterField(40)); !strings.Contains(narrow, longTerm[:10]) {
		t.Errorf("at w=40 the input field must keep ≥10 visible columns; got %q", narrow)
	}
}

// 015 US2: shrinking the height never clips the footer or the filter form — only the list shrinks.
func TestFilterFormHeightSweep(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "b.txt", "c.txt", "d.txt")
	assertHeightSweep(t, func() App { return treeApp(f, true) }, 100)
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
	// footerHints now serves overlay/list modes (the buckets/tree hints moved to the 006
	// hint bar); it must still always show help/quit and never advertise write ops.
	h := footerHints(hintCtx{mode: modeContextSwitch, width: 80})
	if strings.Count(h, "\n") != 0 {
		t.Fatalf("hint row must be a single line; got %q", h)
	}
	for _, want := range []string{"help", "quit"} {
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
	// Wide widths render the three-block columns (~8 rows); narrow widths collapse to
	// identity + read + write rows (+ optional status). The hard guarantee is no line
	// exceeds the width (SC-005); the row budget is generous for the grouped bar.
	assertWidthSweep(t, func(int) App {
		m := treeApp(f, true)
		selectObject(&m, "a.txt")
		return m
	}, 40, 200, 9)
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
	h := footerHints(hintCtx{mode: modeContextSwitch, multiContext: true, width: 120})
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
