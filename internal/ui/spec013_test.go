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

// T013: a bucket-list filter, once committed, shows as a chip on the buckets box; reopening
// and clearing the term removes it.
func TestBucketFilterChipCommitted(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	m := dualApp(f) // focus buckets

	m = press(m, "/")
	for _, r := range "alph" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if m.bucketFilter != "alph" {
		t.Fatalf("setup: committed bucket filter = %q, want \"alph\"", m.bucketFilter)
	}
	if !strings.Contains(stripANSI(viewOf(m)), "filter: alph") {
		t.Errorf("a committed bucket filter must show a border chip:\n%s", stripANSI(viewOf(m)))
	}

	// Reopen and clear the term → chip gone.
	m = press(m, "/")
	for range "alph" {
		m = press(m, "backspace")
	}
	m = press(m, "enter")
	if m.bucketFilter != "" {
		t.Fatalf("clearing the term should empty the bucket filter, got %q", m.bucketFilter)
	}
	if strings.Contains(stripANSI(viewOf(m)), "filter: ") {
		t.Errorf("clearing the filter must remove the chip:\n%s", stripANSI(viewOf(m)))
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
}
