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

// --- US1/US2: always-visible filter FORMS (015 redesign — boxed input under each pane) ---

// 015: an object-level filter renders in the labeled object form — live while typing, the
// committed term when idle, gone on clear.
func TestObjectFilterFormCommitted(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "alpha.txt", "beta.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")

	m = press(m, "/")
	for _, r := range "alpha" {
		m = press(m, string(r))
	}
	if s := stripANSI(m.objectFilterField(80)); !strings.Contains(s, "alpha") {
		t.Errorf("the object form must show the live input while typing; got %q", s)
	}

	m = press(m, "enter")
	if m.searching {
		t.Fatal("enter should close the filter input")
	}
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "alpha"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
	if s := stripANSI(m.objectFilterField(80)); !strings.Contains(s, "alpha") {
		t.Errorf("a committed object filter must show its term in the form; got %q", s)
	}
	if v := stripANSI(viewOf(m)); !strings.Contains(v, "filter objects") {
		t.Errorf("the object filter form must render in the view:\n%s", v)
	}

	m = press(m, "esc") // Back → objectsBack clears the search
	if m.search != "" {
		t.Fatalf("Back should clear the committed search, got %q", m.search)
	}
	if s := stripANSI(m.objectFilterField(80)); strings.Contains(s, "alpha") {
		t.Errorf("clearing the filter must remove the term from the form; got %q", s)
	}
}

// 015: a bucket-list filter renders in the labeled bucket form — live while typing, the committed
// term when idle, gone on clear.
func TestBucketFilterFormCommitted(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	m := dualApp(f) // focus buckets

	m = press(m, "/")
	for _, r := range "alph" {
		m = press(m, string(r))
	}
	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "alph") {
		t.Errorf("the bucket form must show the live input while typing; got %q", s)
	}

	m = press(m, "enter")
	if m.bucketFilter != "alph" {
		t.Fatalf("setup: committed bucket filter = %q, want \"alph\"", m.bucketFilter)
	}
	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "filter buckets") || !strings.Contains(s, "alph") {
		t.Errorf("a committed bucket filter must show its term in the form; got %q", s)
	}
	if v := stripANSI(viewOf(m)); !strings.Contains(v, "filter buckets") {
		t.Errorf("the bucket filter form must render in the view:\n%s", v)
	}

	// Reopen and clear the term → form back to the placeholder.
	m = press(m, "/")
	for range "alph" {
		m = press(m, "backspace")
	}
	m = press(m, "enter")
	if m.bucketFilter != "" {
		t.Fatalf("clearing the term should empty the bucket filter, got %q", m.bucketFilter)
	}
	if s := stripANSI(m.bucketFilterField(40)); strings.Contains(s, "alph") {
		t.Errorf("clearing the filter must remove the term from the form; got %q", s)
	}
}

// 015 US1 (the user's core ask): BOTH filter forms render at once in the two-pane browse — the
// bucket form and the object form, both with their committed terms, regardless of which pane has
// focus.
func TestBothFormsVisibleTogether(t *testing.T) {
	f := storage.NewFake()
	f.Seed("dev-a", "log1.txt", "log2.txt")
	f.Seed("dev-b")
	f.Seed("prod")
	m := dualApp(f) // focus buckets, Dual tier (both panes render)

	// Commit a bucket filter "dev".
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
	// BOTH forms render at once — focus is on objects, yet the bucket form stays put with its term.
	v := stripANSI(viewOf(m))
	if !strings.Contains(v, "filter buckets") {
		t.Errorf("the bucket form must stay visible while focus is on objects:\n%s", v)
	}
	if !strings.Contains(v, "filter objects") {
		t.Errorf("the object form must be visible at the same time:\n%s", v)
	}
	if got := stripANSI(m.bucketFilterField(40)); !strings.Contains(got, "dev") {
		t.Errorf("the bucket form must keep its committed term 'dev'; got %q", got)
	}
	if got := stripANSI(m.objectFilterField(80)); !strings.Contains(got, "log") {
		t.Errorf("the object form must show its committed term 'log'; got %q", got)
	}
}

// 015 US4: the committed term lives in the FORM; the match count rides the box TITLE — bucket
// "[M/T]" (local total), object "[N]" (no total fetched). The title count narrows live as you type.
func TestFilterFormTermCountInTitle(t *testing.T) {
	f := storage.NewFake()
	f.Seed("dev-a")
	f.Seed("dev-b")
	f.Seed("prod")
	m := dualApp(f)

	m = press(m, "/")
	for _, r := range "dev" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if got := stripANSI(m.bucketFilterField(40)); !strings.Contains(got, "dev") {
		t.Errorf("the bucket form must show the committed term; got %q", got)
	}
	if got := m.resourceTitle(); !strings.Contains(got, "[2/3]") {
		t.Errorf("the bucket box title must carry the match count [2/3]; got %q", got)
	}

	// Object count rides the tree title ("[N]").
	f2 := storage.NewFake()
	f2.Seed("b", "log1.txt", "log2.txt", "data.txt")
	mt := treeApp(f2, true)
	mt.search = "log"
	page, _ := f2.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "log"})
	mt.level = &levelState{dirs: page.Dirs, objects: page.Objects, complete: true}
	if got := stripANSI(mt.objectFilterField(80)); !strings.Contains(got, "log") {
		t.Errorf("the object form must show the committed term; got %q", got)
	}
	if got := mt.resourceTitle(); !strings.Contains(got, "[2]") {
		t.Errorf("the tree title must carry the object match count [2]; got %q", got)
	}

	// Live narrowing while typing: the title count updates per keystroke.
	m2 := dualApp(f)
	m2 = press(m2, "/")
	m2 = press(m2, "d") // dev-a, dev-b, prod all contain 'd'
	if n := len(m2.filteredBuckets()); n != 3 {
		t.Errorf("typing 'd' should match 3 buckets live, got %d", n)
	}
	m2 = press(m2, "e") // "de" → dev-a, dev-b
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

// --- Cross-cutting: NO_COLOR-safe text (SC-008, FR-006) ---

// T023: the mode chip and the filter FORM carry their state as TEXT (legible under NO_COLOR).
func TestChipsTextNoColor(t *testing.T) {
	if !strings.Contains((App{}).modeChip(), "RO") {
		t.Error("read-only mode chip must contain the literal 'RO'")
	}
	f := storage.NewFake()
	f.Seed("b", "x")
	if !strings.Contains(buildApp(f, true, false).modeChip(), "WRITE") {
		t.Error("armed mode chip must contain the literal 'WRITE'")
	}
	// 015 SC-007: the filter form carries its label + term as TEXT, not color alone.
	m := buildApp(f, false, false)
	m.search = "needle"
	if got := stripANSI(m.objectFilterField(80)); !strings.Contains(got, "filter objects") || !strings.Contains(got, "needle") {
		t.Errorf("object filter form text = %q, want 'filter objects' + 'needle'", got)
	}
}
