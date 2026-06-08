package ui

import (
	"context"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Feature 013: mode chip dedup, footer breathing room, applied-filter state.
// White-box tests (package ui), failing-first per user story. Helpers reused:
// dualApp/treeApp/buildApp/crossToObjects/selectObject/press/deliver/viewOf/stripANSI.

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }

// --- Foundational: two-chip border slot (border-chip-contract C1–C5/C7) ---

// T002: boxViewWith carries BOTH a filter chip (inboard) and a mode chip (right-most).
func TestBorderTwoChips(t *testing.T) {
	filter := warnStyle.Render("filter: abc")
	mode := writeBadgeStyle.Render("WRITE")
	top := stripANSI(firstLine(boxViewChip("title", "", filter, mode, "body", 60, 3)))
	if !strings.Contains(top, "filter: abc") {
		t.Errorf("two-chip border missing the filter chip:\n%q", top)
	}
	if !strings.Contains(top, "WRITE") {
		t.Errorf("two-chip border missing the mode chip:\n%q", top)
	}
	if strings.Index(top, "WRITE") < strings.Index(top, "filter: abc") {
		t.Errorf("mode chip must be right-most (after the filter chip):\n%q", top)
	}
	if w := lipgloss.Width(firstLine(boxViewChip("title", "", filter, mode, "body", 60, 3))); w > 60 {
		t.Errorf("top border width %d exceeds 60", w)
	}
}

// T002: degrade order — drop the filter chip before the safety-critical mode chip (C3/C7).
func TestBorderChipDegradeOrder(t *testing.T) {
	filter := warnStyle.Render("filter: abcdefgh")
	mode := writeBadgeStyle.Render("WRITE")
	top := stripANSI(firstLine(boxViewChip("lvl", "", filter, mode, "body", 30, 3)))
	if !strings.Contains(top, "WRITE") {
		t.Errorf("the mode chip must survive a narrow border (safety):\n%q", top)
	}
	if strings.Contains(top, "filter:") {
		t.Errorf("the filter chip must drop before the mode chip on a narrow border:\n%q", top)
	}
}

// --- US1: one universal mode chip; no duplicate footer tag ---

// T006: the opened-object box carries the mode chip (RO read-only, WRITE armed).
func TestModeChipOnObjectView(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")

	m := treeApp(f, false) // read-only
	m.width, m.height = 100, 30
	selectObject(&m, "a.txt")
	m = press(m, "enter")
	if m.mode != modeObject {
		t.Fatalf("enter on an object should open modeObject, got %v", m.mode)
	}
	if top := stripANSI(firstLine(viewOf(m))); !strings.Contains(top, "RO") {
		t.Errorf("opened-object box border must show the RO chip:\n%q", top)
	}

	ma := treeApp(f, true) // armed
	ma.width, ma.height = 100, 30
	selectObject(&ma, "a.txt")
	ma = press(ma, "enter")
	if top := stripANSI(firstLine(viewOf(ma))); !strings.Contains(top, "WRITE") {
		t.Errorf("opened-object box border must show the WRITE chip when armed:\n%q", top)
	}
}

// T007: the footer no longer carries the duplicate [RW]/[RO] tag — the chip is the sole
// mode indicator (FR-001/FR-002, SC-001).
func TestFooterModeTagRemoved(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt")

	ro := viewOf(dualApp(f))
	if strings.Contains(ro, "[RO]") || strings.Contains(ro, "[RW]") {
		t.Errorf("read-only view must not carry the old footer [RO]/[RW] tag:\n%s", ro)
	}

	m := dualApp(f)
	m = press(m, "w")
	m = press(m, "y")
	if !m.writable() {
		t.Fatal("setup: should be armed")
	}
	if armed := viewOf(m); strings.Contains(armed, "[RW]") || strings.Contains(armed, "[RO]") {
		t.Errorf("armed view must not carry the old footer [RW]/[RO] tag:\n%s", armed)
	}
}

// --- US2: applied-filter chip on the filtered pane ---

// T012: an objects-level filter, once committed, shows as a chip on the objects box; it is
// hidden while typing and removed on clear; the chip is presentation-only (no backend load).
func TestObjectsFilterChipCommitted(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "alpha.txt", "beta.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")

	m = press(m, "/")
	for _, r := range "alpha" {
		m = press(m, string(r))
	}
	if strings.Contains(stripANSI(viewOf(m)), "filter: alpha") {
		t.Errorf("the filter chip must be hidden while the input is open:\n%s", stripANSI(viewOf(m)))
	}

	m = press(m, "enter")
	if m.searching {
		t.Fatal("enter should close the filter input")
	}
	// Complete the (filtered) level load so a later Back clears the search instead of just
	// cancelling the in-flight load.
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "alpha"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
	if !strings.Contains(stripANSI(viewOf(m)), "filter: alpha") {
		t.Errorf("a committed objects filter must show a border chip:\n%s", stripANSI(viewOf(m)))
	}

	m = press(m, "esc") // Back → objectsBack clears the search
	if m.search != "" {
		t.Fatalf("Back should clear the committed search, got %q", m.search)
	}
	if strings.Contains(stripANSI(viewOf(m)), "filter: alpha") {
		t.Errorf("clearing the filter must remove the chip:\n%s", stripANSI(viewOf(m)))
	}
}

// T010 / 015 US1: a bucket-list filter, once committed, is shown per-pane — its count-bearing
// chip is produced focus-agnostically (the chip may degrade off the narrow bucket border under
// width pressure, in which case the always-visible strip still echoes the focused scope's term);
// reopening and clearing the term removes both. Migrated from 013's chip-in-view assertion to the
// 015 always-visible model (the chip CAN drop per the applied-filter-chip-count contract).
func TestBucketFilterChipCommitted(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	m := dualApp(f) // focus buckets

	m = press(m, "/")
	for _, r := range "alph" {
		m = press(m, string(r))
	}
	// While editing the bucket scope the chip is hidden; the live term shows in the strip.
	if m.bucketFilterChip() != "" {
		t.Errorf("the bucket chip must hide while its scope is edited; got %q", stripANSI(m.bucketFilterChip()))
	}
	if s := stripANSI(m.filterStripView(120)); !strings.Contains(s, "filter buckets: alph") {
		t.Errorf("the live term must show in the strip while editing; got %q", s)
	}

	m = press(m, "enter")
	if m.bucketFilter != "alph" {
		t.Fatalf("setup: committed bucket filter = %q, want \"alph\"", m.bucketFilter)
	}
	// Committed: the count-bearing chip is produced focus-agnostically, and the committed term
	// stays visible in the view (the strip echoes the focused bucket scope).
	if got := stripANSI(m.bucketFilterChip()); !strings.Contains(got, "filter: alph") {
		t.Errorf("a committed bucket filter must produce a chip; got %q", got)
	}
	if v := stripANSI(viewOf(m)); !strings.Contains(v, "filter buckets: alph") {
		t.Errorf("the committed bucket filter must stay visible in the view:\n%s", v)
	}

	// Reopen and clear the term → chip gone, strip back to the placeholder.
	m = press(m, "/")
	for range "alph" {
		m = press(m, "backspace")
	}
	m = press(m, "enter")
	if m.bucketFilter != "" {
		t.Fatalf("clearing the term should empty the bucket filter, got %q", m.bucketFilter)
	}
	if m.bucketFilterChip() != "" {
		t.Errorf("clearing the filter must remove the chip; got %q", stripANSI(m.bucketFilterChip()))
	}
	if strings.Contains(stripANSI(viewOf(m)), "filter buckets: alph") {
		t.Errorf("clearing the filter must remove the strip echo:\n%s", stripANSI(viewOf(m)))
	}
}

// T010 / 015 US1: both committed chips render simultaneously, focus-agnostic — a bucket filter
// and an object filter are visible at once, and moving focus does not hide either.
func TestBothChipsVisibleTogether(t *testing.T) {
	f := storage.NewFake()
	f.Seed("dev-a", "log1.txt", "log2.txt")
	f.Seed("dev-b")
	f.Seed("prod")
	m := dualApp(f) // focus buckets, Dual tier (both boxes render)

	// Commit a bucket filter "dev" (matches dev-a, dev-b → 2 of 3).
	m = press(m, "/")
	for _, r := range "dev" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if m.bucketFilter != "dev" {
		t.Fatalf("setup: bucket filter = %q, want dev", m.bucketFilter)
	}

	// Cross into objects (highlighted bucket = dev-a) and commit an object filter "log".
	m = crossToObjects(m, f, "dev-a")
	m = press(m, "/")
	for _, r := range "log" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "dev-a", Search: "log"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})

	if m.focusZone != zoneObjects {
		t.Fatalf("setup: focus should be on objects, got %v", m.focusZone)
	}
	// Both committed chips are produced AT ONCE, focus-agnostic (focus is on objects, yet the
	// bucket chip is not hidden — the 015 US1 fix over 013's global searching-gate).
	if got := stripANSI(m.bucketFilterChip()); !strings.Contains(got, "filter: dev") {
		t.Errorf("the bucket chip must stay present while focus is on objects; got %q", got)
	}
	if got := stripANSI(m.objectsFilterChip()); !strings.Contains(got, "filter: log") {
		t.Errorf("the object chip must be present at the same time; got %q", got)
	}
	// The wide objects box renders its chip in the composed view.
	if v := stripANSI(viewOf(m)); !strings.Contains(v, "filter: log") {
		t.Errorf("the object chip must render in the view:\n%s", v)
	}

	// US1 core: editing the OBJECT scope hides ONLY the object chip; the bucket chip stays put.
	m = press(m, "/")
	if !m.searching {
		t.Fatal("'/' should reopen the object filter")
	}
	if m.objectsFilterChip() != "" {
		t.Errorf("the object chip must hide while its own scope is edited; got %q", stripANSI(m.objectsFilterChip()))
	}
	if got := stripANSI(m.bucketFilterChip()); !strings.Contains(got, "filter: dev") {
		t.Errorf("the bucket chip must NOT hide while the OBJECT scope is edited (US1 fix); got %q", got)
	}
}

// T015 / 015 US4: the committed chip carries a match count — bucket "M/T" (local total), object
// "N matched" (no total fetched); the count narrows live while typing (via the box title).
func TestChipShowsMatchCount(t *testing.T) {
	f := storage.NewFake()
	f.Seed("dev-a")
	f.Seed("dev-b")
	f.Seed("prod")
	m := dualApp(f)

	// Bucket chip: matched/total.
	m = press(m, "/")
	for _, r := range "dev" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if got := stripANSI(m.bucketFilterChip()); !strings.Contains(got, "filter: dev · 2/3") {
		t.Errorf("bucket chip should read 'filter: dev · 2/3'; got %q", got)
	}

	// Object chip: N matched only (no total).
	f2 := storage.NewFake()
	f2.Seed("b", "log1.txt", "log2.txt", "data.txt")
	mt := treeApp(f2, true)
	mt.search = "log"
	page, _ := f2.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "log"})
	mt.level = &levelState{dirs: page.Dirs, objects: page.Objects, complete: true}
	if got := stripANSI(mt.objectsFilterChip()); !strings.Contains(got, "filter: log · 2") {
		t.Errorf("object chip should read 'filter: log · 2'; got %q", got)
	}
	if strings.Contains(stripANSI(mt.objectsFilterChip()), "/") {
		t.Errorf("object chip must NOT carry a level total (paginated); got %q", stripANSI(mt.objectsFilterChip()))
	}

	// Live narrowing while typing: the box title shows the running M/T count (the chip is hidden
	// mid-edit; its live term shows in the strip).
	m2 := dualApp(f)
	m2 = press(m2, "/")
	m2 = press(m2, "d") // dev-a, dev-b, prod all contain 'd'
	if n := len(m2.filteredBuckets()); n != 3 {
		t.Errorf("typing 'd' should match 3 buckets live, got %d", n)
	}
	m2 = press(m2, "e") // "de" → dev-a, dev-b
	if n := len(m2.filteredBuckets()); n != 2 {
		t.Errorf("typing 'de' should narrow live to 2, got %d", n)
	}
	if !strings.Contains(m2.resourceTitle(), "[2/3]") {
		t.Errorf("the box title should show the live match count while typing; got %q", m2.resourceTitle())
	}
}

// --- US3: widened footer / command-bar spacing ---

// T017: the command bar gives elements breathing room — a widened key↔label gap (2 spaces)
// and a widened inter-column gap (colGap, 3 spaces).
func TestFooterSpacingWidened(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt")
	m := treeApp(f, true)
	m.width = 140
	bar := stripANSI(m.commandBarView(140))
	if !strings.Contains(bar, "Enter  open") { // key↔label gap widened 1→2
		t.Errorf("command-bar entries should have a 2-space key↔label gap:\n%s", bar)
	}
	if len(colGap) < 3 {
		t.Errorf("inter-column gap should be widened to ≥3 spaces, got %d", len(colGap))
	}
}

// T018: after widening, the footer still never overflows width nor exceeds the row budget at
// any tier (FR-016 / layout-visibility-contract L3).
func TestFooterNoOverflowAfterWidening(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt")
	for w := 40; w <= 200; w++ {
		m := treeApp(f, true)
		m.width = w
		fb := m.footerBlock(w)
		lines := strings.Split(fb, "\n")
		if len(lines) > 9 {
			t.Fatalf("footer at w=%d has %d lines (>9 budget)", w, len(lines))
		}
		for _, ln := range lines {
			if lipgloss.Width(ln) > w {
				t.Fatalf("footer line at w=%d exceeds width (%d > %d): %q",
					w, lipgloss.Width(ln), w, stripANSI(ln))
			}
		}
	}
}

// --- Cross-cutting: NO_COLOR-safe chip text (SC-008, FR-006) ---

// T023: the mode chip and the applied-filter chip carry their state as TEXT.
func TestChipsTextNoColor(t *testing.T) {
	if !strings.Contains((App{}).modeChip(), "RO") {
		t.Error("read-only mode chip must contain the literal 'RO'")
	}
	f := storage.NewFake()
	f.Seed("b", "x")
	if !strings.Contains(buildApp(f, true, false).modeChip(), "WRITE") {
		t.Error("armed mode chip must contain the literal 'WRITE'")
	}
	m := buildApp(f, false, false)
	m.search = "needle"
	if got := stripANSI(m.objectsFilterChip()); !strings.Contains(got, "filter: needle") {
		t.Errorf("objects filter chip text = %q, want it to contain 'filter: needle'", got)
	}

	// 015 SC-007: the match COUNT is carried as TEXT (not color alone), legible under NO_COLOR.
	mb := buildApp(f, false, false)
	bs, _ := f.ListBuckets(context.Background())
	mb = deliver(mb, bucketsMsg{gen: mb.gen, buckets: bs})
	mb.bucketFilter = "b"
	if got := stripANSI(mb.bucketFilterChip()); !strings.Contains(got, "filter: b · ") {
		t.Errorf("bucket chip must carry the match count as text; got %q", got)
	}
}
