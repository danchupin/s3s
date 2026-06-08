package ui

// White-box tests for feature 012 (UI legibility, hotkey parity, breadcrumbs, write-mode
// clarity). Drives the model with the shared deliver/press/viewOf helpers + storage.Fake.

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// --- US1: names never hidden ---

// T005: a long bucket name grows the bucket column into the objects-zone slack rather than
// truncating (FR-001).
func TestBucketsColumnGrowsWithSlack(t *testing.T) {
	f := storage.NewFake()
	long := "very-long-bucket-name-exceeding-default-column"
	f.Seed(long, "a.txt")
	m := dualApp(f)
	if !strings.Contains(viewOf(m), long) {
		t.Errorf("long bucket name must be fully visible (column grows into slack):\n%s", viewOf(m))
	}
}

// T005: the grown column is capped so the objects zone stays legible (>= objMinW).
func TestBucketsColumnCappedAtMax(t *testing.T) {
	f := storage.NewFake()
	f.Seed(strings.Repeat("x", 200), "a.txt")
	m := dualApp(f)
	bw := m.bucketsColWidth(120, 40)
	if bw > 120-30 {
		t.Errorf("bucket column must cap to leave the objects zone >= 30; got %d", bw)
	}
	if bw < 40 {
		t.Errorf("bucket column must not shrink below the base width; got %d", bw)
	}
}

// T006: the active row wraps its full value onto continuation lines when truncated and spare
// rows exist (FR-003).
func TestActiveRowWrapsWhenTruncated(t *testing.T) {
	cols := []column{{"name", 0}, {"size", 6}}
	long := "this-is-a-very-long-object-key-that-will-not-fit"
	out := renderTableActive(30, cols, [][]string{{long, "1 B"}}, nil, 0, 5)
	if !strings.Contains(out, long[len(long)-10:]) {
		t.Errorf("active row should wrap to show the full value tail:\n%s", out)
	}
}

// T006: with no spare rows the active row does NOT wrap (reveal popup is the fallback).
func TestActiveRowFallsBackWhenFull(t *testing.T) {
	cols := []column{{"name", 0}, {"size", 6}}
	out := renderTableActive(30, cols, [][]string{{strings.Repeat("x", 50), "1 B"}}, nil, 0, 0)
	if strings.Count(out, "\n") > 2 {
		t.Errorf("no spare rows → must not wrap; got:\n%s", out)
	}
}

// T006: wrapped output never exceeds the header+dataRows budget (footer stays visible, FR-022).
func TestActiveRowWrapStaysInBudget(t *testing.T) {
	cols := []column{{"name", 0}}
	dataRows := 6
	out := renderTableActive(20, cols, [][]string{{strings.Repeat("y", 100)}}, nil, 0, dataRows-1)
	if lines := strings.Count(out, "\n") + 1; lines > 2+dataRows {
		t.Errorf("wrapped output %d lines exceeds budget %d", lines, 2+dataRows)
	}
}

// T007: the reveal popup ('i') shows the full object key and copies it.
func TestRevealShowsFullValue(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "very-long-object-key-name-at-root-exceeding-width.bin")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = press(m, "i")
	if m.reveal == nil {
		t.Fatal("'i' should open the reveal popup")
	}
	if !strings.Contains(viewOf(m), "exceeding-width.bin") {
		t.Errorf("reveal popup must show the full key:\n%s", viewOf(m))
	}
}

// T007: reveal emits a clipboard command (best-effort OSC52).
func TestRevealEmitsClipboardCmd(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "k.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	if _, cmd := pressCmd(m, "i"); cmd == nil {
		t.Fatal("reveal must emit a clipboard command")
	}
}

// T007: reveal works in the bucket list too (zone-agnostic), revealing the bucket name.
func TestRevealBucketName(t *testing.T) {
	f := storage.NewFake()
	long := "very-long-bucket-name-for-reveal"
	f.Seed(long, "a.txt")
	m := dualApp(f)
	m = press(m, "i")
	if m.reveal == nil || m.reveal.kind != revealBucket {
		t.Fatalf("reveal on the bucket list should reveal the bucket name; got %+v", m.reveal)
	}
	if !strings.Contains(viewOf(m), long) {
		t.Errorf("reveal must show the full bucket name:\n%s", viewOf(m))
	}
}

// T007: any key dismisses the reveal popup.
func TestRevealDismiss(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "k.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = press(m, "i")
	if m.reveal == nil {
		t.Fatal("reveal should be open")
	}
	if m = press(m, "esc"); m.reveal != nil {
		t.Error("any key should dismiss the reveal popup")
	}
}

// T007: the footer stays visible under the reveal popup on a small terminal (FR-022).
func TestRevealFooterNotClipped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", strings.Repeat("z", 300)+".bin")
	m := dualApp(f)
	m.width, m.height = 80, 24
	m = crossToObjects(m, f, "b")
	m = press(m, "i")
	if v := viewOf(m); !strings.Contains(v, "quit") && !strings.Contains(v, "help") {
		t.Errorf("footer must remain visible under the reveal popup:\n%s", v)
	}
}

// --- US2: write state legible + mode chip ---

// T011→013: only the mode chip TEXT carries the state color — not an adjacent space (FR-006).
// (The footer [RW]/[RO] tag was removed in 013 US1; the invariant now lives on the chip.)
func TestBadgeDoesNotColorAdjacentSpace(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	chip := buildApp(f, true, false).modeChip()
	if strings.Contains(chip, writeBadgeStyle.Render(" WRITE")) {
		t.Error("a space adjacent to the chip must not carry the state color")
	}
	if !strings.Contains(chip, writeBadgeStyle.Render("WRITE")) {
		t.Errorf("chip text must carry the state color:\n%q", chip)
	}
}

// T011: disarmed (writable context) shows a clear "enable write" cue (FR-007).
func TestEnableWriteLabelWhenDisarmed(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := buildApp(f, false, false)
	if bar := m.commandBarView(140); !strings.Contains(bar, "enable write") {
		t.Errorf("disarmed write column must say 'enable write':\n%s", bar)
	}
}

// T011: armed shows a visible disarm cue back to read-only (FR-008).
func TestDisarmCueWhenArmed(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := buildApp(f, false, false)
	m = press(m, "w")
	m = press(m, "y")
	if !m.writable() {
		t.Fatal("setup: should be armed")
	}
	if bar := m.commandBarView(140); !strings.Contains(bar, "read-only") {
		t.Errorf("armed write column must show a disarm cue (→ read-only):\n%s", bar)
	}
}

// T011: a read-only context communicates that writes are unavailable (FR-009).
func TestReadonlyContextCue(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := buildApp(f, false, true)
	if bar := m.commandBarView(140); !strings.Contains(bar, "read-only context") {
		t.Errorf("read-only context cue missing:\n%s", bar)
	}
}

// T012: the read-only mode chip is inset into the primary box top border (FR-038).
func TestModeChipReadonlyNeutral(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := dualApp(f)
	top := strings.SplitN(viewOf(m), "\n", 2)[0]
	if !strings.Contains(top, "RO") {
		t.Errorf("primary box top border must show the RO mode chip:\n%q", top)
	}
}

// T012: arming flips the chip to the WRITE accent (FR-038).
func TestModeChipWriteAccent(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := dualApp(f)
	m = press(m, "w")
	m = press(m, "y")
	top := strings.SplitN(viewOf(m), "\n", 2)[0]
	if !strings.Contains(top, "WRITE") {
		t.Errorf("primary box top border must show the WRITE chip when armed:\n%q", top)
	}
}

// T012: the chip text is present regardless of color (NO_COLOR-safe).
func TestModeChipNoColor(t *testing.T) {
	if !strings.Contains((App{}).modeChip(), "RO") {
		t.Error("read-only chip must contain the literal text 'RO'")
	}
}

// --- US6: objects-zone hotkey parity (regression) ---

// T016: marking works in the objects zone, and the selection kind is recognised.
func TestMarkObjectsInObjectsZone(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "z.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	if m.selKind() != selObject {
		t.Fatalf("objects zone with an object highlighted: selKind=%v, want selObject", m.selKind())
	}
	m = press(m, " ")
	if m.selCount() != 1 {
		t.Errorf("space in objects zone should mark the object; selCount=%d", m.selCount())
	}
	if m = press(m, " "); m.selCount() != 0 {
		t.Errorf("space again should unmark; selCount=%d", m.selCount())
	}
}

// T016: sort + sort-direction cycle in the objects zone.
func TestSortCycleInObjectsZone(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	if before := m.sortBy; press(m, "s").sortBy == before {
		t.Error("'s' in objects zone should cycle the sort column")
	}
	if before := m.sortAsc; press(m, "S").sortAsc == before {
		t.Error("'S' in objects zone should toggle sort direction")
	}
}

// T016: context switch fires from the objects zone (returns focus to the bucket list).
func TestContextFromObjectsZone(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	if m = press(m, "c"); m.focusZone != zoneBuckets {
		t.Error("'c' (context) in the objects zone must fire, not be a dead key")
	}
}

// T017: the objects zone uses the OBJECT action catalog, not the bucket one (parity).
func TestObjectsZoneUsesObjectCatalog(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	labels := map[string]bool{}
	for _, a := range m.actionCatalog() {
		labels[a.label] = true
	}
	for _, want := range []string{"download", "copy", "move", "upload", "new folder"} {
		if !labels[want] {
			t.Errorf("objects zone must use the object catalog; missing %q (got %v)", want, labels)
		}
	}
}

// T017: a per-item action key is live in the objects zone (download starts a flow).
func TestDownloadInObjectsZone(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	if _, cmd := pressCmd(m, "d"); cmd == nil {
		t.Error("'d' (download) in the objects zone must start the download flow, not be a dead key")
	}
}

// T017: marks clear when the bucket/level changes.
func TestMarksClearOnBucketChange(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	f.Seed("c", "x.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = press(m, " ")
	if m.selCount() != 1 {
		t.Fatal("setup: should have 1 mark")
	}
	m = press(m, "esc")  // back to the bucket list
	m = press(m, "down") // highlight bucket "c"
	m = crossToObjects(m, f, "c")
	if m.selCount() != 0 {
		t.Errorf("marks must clear on bucket/level change; selCount=%d", m.selCount())
	}
}

// --- US7: filter the current level + prominent input ---

// T021: the objects-zone filter runs a server-side level load and stays multi-pane.
func TestFilterScopesToObjectsLevel(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple.txt", "banana.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = press(m, "/")
	m = press(m, "a") // type → schedules debounce
	m2, cmd := updateCmd(m, searchFireMsg{searchGen: m.searchGen, term: m.searchInput})
	if cmd == nil {
		t.Error("objects-zone filter must schedule a server-side level load")
	}
	if m2.mode != modeBuckets || m2.focusZone != zoneObjects {
		t.Errorf("objects-zone filter must stay multi-pane; mode=%v focus=%v", m2.mode, m2.focusZone)
	}
	if m2.bucketFilter != "" {
		t.Errorf("objects-zone filter must NOT set the bucket filter; got %q", m2.bucketFilter)
	}
}

// T021: the bucket-list filter stays local (no backend load).
func TestBucketFilterStillLocal(t *testing.T) {
	f := storage.NewFake()
	f.Seed("apple", "x")
	f.Seed("banana", "y")
	m := dualApp(f)
	m = press(m, "/")
	m2, cmd := pressCmd(m, "a")
	if cmd != nil {
		t.Error("bucket filter must be local (no backend load cmd)")
	}
	if m2.bucketFilter != "a" {
		t.Errorf("bucket filter should narrow locally; bucketFilter=%q", m2.bucketFilter)
	}
}

// T022: Enter commits the objects-zone filter, closes the input, and keeps focus on the pane.
func TestFilterInputCommitMovesFocus(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = press(m, "/")
	m = press(m, "a")
	m = press(m, "enter")
	if m.searching {
		t.Error("Enter should close the filter input")
	}
	if m.focusZone != zoneObjects {
		t.Error("after commit, focus stays in the filtered (objects) pane")
	}
}

// T022: re-opening the filter pre-fills the committed term for refinement.
func TestFilterReopenPrefilled(t *testing.T) {
	f := storage.NewFake()
	f.Seed("apple", "x")
	f.Seed("banana", "y")
	m := dualApp(f)
	m = press(m, "/")
	m = press(m, "a")
	m = press(m, "enter")
	if m.bucketFilter != "a" {
		t.Fatalf("commit: bucketFilter=%q", m.bucketFilter)
	}
	if m = press(m, "/"); m.searchInput != "a" {
		t.Errorf("re-open should pre-fill the committed term; searchInput=%q", m.searchInput)
	}
}

// T022: Esc cancels the in-progress edit and reverts to the committed term.
func TestFilterEscRevertsToCommitted(t *testing.T) {
	f := storage.NewFake()
	f.Seed("apple", "x")
	f.Seed("apricot", "y")
	f.Seed("banana", "z")
	m := dualApp(f)
	m = press(m, "/")
	m = press(m, "a")
	m = press(m, "enter") // commit "a"
	m = press(m, "/")     // re-open prefilled
	m = press(m, "p")     // live edit → "ap"
	if m.bucketFilter != "ap" {
		t.Fatalf("live edit: bucketFilter=%q", m.bucketFilter)
	}
	if m = press(m, "esc"); m.bucketFilter != "a" {
		t.Errorf("Esc should revert to the committed filter; bucketFilter=%q", m.bucketFilter)
	}
}

// --- US4: prominent arm confirmation ---

// T028: arming opens a prominent centered popup stating the consequence + keys.
func TestArmConfirmationIsCenteredPopup(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := buildApp(f, false, false)
	m.width, m.height = 120, 30
	m = press(m, "w")
	if !m.armConfirm {
		t.Fatal("'w' should open the arm confirmation")
	}
	v := viewOf(m)
	if !strings.Contains(v, "mutations will be enabled") || !strings.Contains(v, "arm WRITE mode") {
		t.Errorf("arm popup must state the consequence + action:\n%s", v)
	}
}

// T028: the badge stays visible while the arm popup is shown (FR-017).
func TestArmConfirmBadgeVisible(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := buildApp(f, false, false)
	m.width, m.height = 120, 30
	m = press(m, "w")
	if v := viewOf(m); !strings.Contains(v, "RO") {
		t.Errorf("read/write badge must stay visible during the arm popup:\n%s", v)
	}
}

// T028: disarming is instant — no confirmation popup (FR-016).
func TestDisarmIsInstantNoPopup(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := buildApp(f, false, false)
	m = press(m, "w")
	m = press(m, "y")
	if !m.writable() {
		t.Fatal("setup: should be armed")
	}
	m = press(m, "w")
	if m.armConfirm {
		t.Error("disarm must not open a confirmation")
	}
	if m.writable() {
		t.Error("disarm should be instant")
	}
}

// --- US8: sort surfaced in the command bar ---

// T033: the command bar advertises the sort key + current field/direction (FR-032).
func TestSortIndicatorInCommandBar(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false)
	if bar := m.commandBarView(140); !strings.Contains(bar, "name") {
		t.Errorf("command bar must show the current sort field:\n%s", bar)
	}
}

// T033: cycling sort reaches modification date and the bar reflects it (FR-031).
func TestSortCycleReachesModified(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false)
	m = press(m, "s") // size
	m = press(m, "s") // modified
	if !strings.Contains(m.sortIndicator(), "modified") {
		t.Errorf("sort cycle should reach modified; indicator=%q", m.sortIndicator())
	}
	if !strings.Contains(m.commandBarView(140), "modified") {
		t.Error("command bar must reflect the modified sort field")
	}
}

// --- US3: location breadcrumb ---

// T026: elideMiddle keeps the first + deepest segment.
func TestElideMiddle(t *testing.T) {
	if got := elideMiddle("a/b/c/d/e", 7); got != "a/…/e" {
		t.Errorf("elideMiddle keeps first+deepest; got %q", got)
	}
	if elideMiddle("short", 20) != "short" {
		t.Error("elideMiddle should not change a fitting string")
	}
}

// T025: the objects-zone label shows the context → bucket path (FR-010/011).
func TestBreadcrumbFullPath(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a/b/c/file.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	if v := viewOf(m); !strings.Contains(v, "→") || !strings.Contains(v, "b") {
		t.Errorf("breadcrumb should show context → bucket:\n%s", v)
	}
}

// T025: the breadcrumb updates when drilling into a prefix (FR-012).
func TestBreadcrumbUpdatesOnDrill(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "docs/readme.txt")
	m := dualApp(f)
	m = crossToObjects(m, f, "b")
	m = drillObjects(m, f)
	if m.prefix != "docs/" {
		t.Fatalf("prefix=%q want docs/", m.prefix)
	}
	if !strings.Contains(viewOf(m), "docs") {
		t.Errorf("breadcrumb should show the drilled prefix:\n%s", viewOf(m))
	}
}

// --- US5: keymap single-source, no stale glyphs ---

// T030: no hardcoded key literals remain in any rendered surface (FR-035).
func TestNoStaleLiteralsInAnyView(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "docs/x", "file.txt")
	m := treeApp(f, true)
	m.width = 140
	selectObject(&m, "file.txt")
	views := map[string]string{
		"view":    viewOf(m),
		"help":    m.helpView(),
		"bar":     m.commandBarView(140),
		"pane":    m.paneView(40, 20),
		"status":  m.statusLine(140),
		"panedir": func() string { selectDir(&m, "docs/"); return m.paneView(40, 20) }(),
	}
	// The old chord format (^x/^o) must never appear — chords render via glyph() as Ctrl+X/O.
	for name, v := range views {
		for _, bad := range []string{"^x", "^o"} {
			if strings.Contains(v, bad) {
				t.Errorf("%s contains the old chord literal %q:\n%s", name, bad, v)
			}
		}
	}
}

// T030: rebinding an action key propagates to the command-bar entries (FR-035). Asserted on
// the structured entries (the rendered key+label are separate styled spans).
func TestRebindPropagatesToCommandBar(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "file.txt")
	m := treeApp(f, true)
	m.keys.Download = []string{"f"} // rebind download d → f
	selectObject(&m, "file.txt")
	found := false
	for _, e := range m.readEntries(m.selKind(), m.actionCatalog()) {
		if e.label == "download" {
			found = true
			if e.key != "f" {
				t.Errorf("rebound download key must be 'f', got %q", e.key)
			}
		}
	}
	if !found {
		t.Error("download entry missing from the command-bar read entries")
	}
}

// --- US9: declutter (no affordance lost, no on-screen duplication) ---

// T035: every per-item action stays advertised in the command bar (declutter loses nothing).
func TestEveryActionAdvertised(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "file.txt")
	m := treeApp(f, true)
	selectObject(&m, "file.txt")
	kind, cat := m.selKind(), m.actionCatalog()
	labels := map[string]bool{}
	for _, e := range m.readEntries(kind, cat) {
		labels[e.label] = true
	}
	for _, e := range m.writeEntries(kind, cat) {
		labels[e.label] = true
	}
	for _, want := range []string{"download", "analyze", "copy", "move", "upload", "new folder"} {
		if !labels[want] {
			t.Errorf("action %q must stay advertised in the command bar; got %v", want, labels)
		}
	}
}

// T035: the sort indicator is not duplicated in the box title (it moved to the command bar).
func TestSortNotDuplicatedInTitle(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false)
	if title := m.resourceTitle(); strings.Contains(title, "↑") || strings.Contains(title, "↓") {
		t.Errorf("sort indicator must not duplicate in the box title; got %q", title)
	}
}

// --- Polish: NO_COLOR + footer visibility ---

// T038: the new state cues carry their meaning as literal TEXT (legible under NO_COLOR).
func TestCuesAreTextBased(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := dualApp(f)
	m = press(m, "/") // open the filter input
	// 015 FR-005: the filter input cue moved from statusLine to the always-visible strip.
	if s := m.filterStripView(120); !strings.Contains(s, "filter") {
		t.Errorf("filter input cue must be text: %q", s)
	}
	if !strings.Contains((App{}).modeChip(), "RO") || !strings.Contains((App{armed: true}).modeChip(), "WRITE") {
		t.Error("mode chip must carry RO/WRITE as text")
	}
}

// T039: the footer/command bar stays visible across all supported tiers (FR-022 / SC-008).
func TestFooterVisibleAcrossTiers(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "file.txt")
	for _, dim := range []struct{ w, h int }{{60, 10}, {120, 8}, {140, 12}} {
		m := dualApp(f)
		m.width, m.height = dim.w, dim.h
		v := viewOf(m)
		if !strings.Contains(v, "quit") && !strings.Contains(v, "help") && !strings.Contains(v, "?") {
			t.Errorf("footer must remain visible at %dx%d:\n%s", dim.w, dim.h, v)
		}
	}
}
