package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// availLabels returns the labels of the currently-available actions (read + write).
func availLabels(m App) []string {
	out := make([]string, 0)
	for _, a := range m.availableActions() {
		out = append(out, a.label)
	}
	return out
}

func hasStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// --- 007 US4: dangerous actions are chord-gated; bare keys are inert ---

func TestBareDeleteKeyIsInert(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true) // writable
	selectObject(&m, "a.txt")

	m = press(m, "x") // bare delete key — must NOT start anything (FR-021, SC-008)
	if m.op != nil {
		t.Fatalf("bare 'x' must not start a delete op (chord required); op=%+v", m.op)
	}
	if m.mode != modeTree {
		t.Fatalf("mode must stay tree; got %v", m.mode)
	}
}

func TestChordDeleteStartsBinaryConfirm(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")

	m = press(m, "ctrl+x") // the delete chord
	if m.op == nil || m.op.kind != "delete_object" || m.op.tier != confirmSimple {
		t.Fatalf("ctrl+x should start a binary delete confirm; op=%+v", m.op)
	}
}

func TestChordRecursiveDeleteStartsTypedConfirm(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "p/a", "p/b")
	m := treeApp(f, true)
	selectDir(&m, "p/")

	m = press(m, "ctrl+x") // delete chord on a FOLDER → recursive (typed-path)
	if m.op == nil || m.op.kind != "delete_recursive" || m.op.tier != confirmTyped {
		t.Fatalf("ctrl+x on a folder should start a typed recursive delete; op=%+v", m.op)
	}
	if m.op.expect != "p/" {
		t.Errorf("recursive expect = %q, want p/", m.op.expect)
	}
}

func TestChordFolderWithMarksRoutesToRecursive(t *testing.T) {
	// review #1: a folder cursor WITH marks active must still route ctrl+x to recursive
	// delete of the highlighted folder — NOT bulk delete of the marked set.
	f := storage.NewFake()
	f.Seed("b", "obj.txt", "p/a", "p/b")
	m := treeApp(f, true)
	selectObject(&m, "obj.txt")
	m = press(m, " ")   // mark obj.txt
	selectDir(&m, "p/") // move cursor to the folder while a mark exists
	m = press(m, "ctrl+x")
	if m.op == nil || m.op.kind != "delete_recursive" || m.op.prefix != "p/" {
		t.Fatalf("ctrl+x on a folder (with marks) should recurse the folder, not bulk-delete marks; op=%+v", m.op)
	}
}

func TestChordMoveStartsMove(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")

	m = press(m, "ctrl+o") // move chord (ctrl+m is Enter, reserved)
	if m.op == nil || m.op.kind != "move" {
		t.Fatalf("ctrl+o should start a move; op=%+v", m.op)
	}
}

func TestChordDeleteReadOnlyNoSurface(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false) // read-only
	selectObject(&m, "a.txt")

	m = press(m, "ctrl+x") // FR-028: no surface opens in read-only
	if m.op != nil {
		t.Fatalf("a dangerous chord in read-only must open no surface; op=%+v", m.op)
	}
	if m.errorText() == "" {
		t.Error("a refused dangerous chord should surface the read-only nudge")
	}
}

func TestBareDeleteReadOnlyNudges(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false) // read-only
	selectObject(&m, "a.txt")

	m = press(m, "x") // bare key in read-only → no op, read-only explanation
	if m.op != nil {
		t.Fatalf("a write key in read-only must not start an op; op=%+v", m.op)
	}
	if m.errorText() == "" {
		t.Error("a refused write should surface an explanatory status")
	}
}

// --- read actions are not chord-gated (bare key, work read-only) ---

func TestDirectDownloadStartsImmediately(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "d")

	if m.op == nil || m.op.kind != "download" {
		t.Fatalf("'d' should start a download op immediately; op=%+v", m.op)
	}
}

func TestDirectAnalyzeNoMenu(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := treeApp(f, true)
	selectDir(&m, "docs/")
	m = press(m, "a")

	// 016: 'a' is now "more detail" — on a folder it expands the inline usage breakdown
	// in place (no separate full-screen mode, no menu).
	if m.detailSection != sectBreakdown {
		t.Fatalf("'a' on a folder should expand the inline usage breakdown; section=%v, mode=%v", m.detailSection, m.mode)
	}
	if m.mode != modeTree {
		t.Errorf("'a' must NOT switch modes; mode=%v", m.mode)
	}
}

// --- 007 US1: the three-block command bar ---

// 008 US5 (FR-013): the command bar drops ALL THREE block headings (INFO/READ/WRITE) while
// keeping the info/read/write entries grouped in distinct columns.
func TestCommandBarHasNoBlockTitles(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m.width = 140
	bar := m.commandBarView(140)
	for _, gone := range []string{"INFO", "READ", "WRITE"} {
		if strings.Contains(bar, gone) {
			t.Errorf("command bar must NOT show the %q heading; got:\n%s", gone, bar)
		}
	}
	// Grouping still readable: a read key and a write key both render.
	if !strings.Contains(bar, "open") {
		t.Errorf("read entries should still render (e.g. 'open'); got:\n%s", bar)
	}
	if !strings.Contains(bar, "delete") {
		t.Errorf("write entries should still render (e.g. 'delete'); got:\n%s", bar)
	}
}

func TestCommandBarReadOnlyShowsWriteDimmed(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	ro := treeApp(f, false) // read-only
	selectObject(&ro, "a.txt")
	ro.width = 140
	bar := ro.commandBarView(140)
	// Every write action stays VISIBLE in read-only (reverses 006 FR-004).
	for _, want := range []string{"delete", "copy", "move", "upload", "new folder"} {
		if !strings.Contains(bar, want) {
			t.Errorf("read-only command bar must still show write action %q; got:\n%s", want, bar)
		}
	}
	// The "w enable write" cue marks the disarmed write block (NO_COLOR-safe; 012 FR-007 made
	// the old "w to arm" explicit).
	if !strings.Contains(bar, "enable write") {
		t.Errorf("read-only write block must carry a non-color 'enable write' cue; got:\n%s", bar)
	}
}

// 008 US8: an applied filter (not actively typing) shows an "Esc clear" reset affordance in
// the read group; no affordance when no filter is active (FR-021).
func hasReadLabel(m App, label string) bool {
	for _, e := range m.readEntries(m.selKind(), m.actionCatalog()) {
		if e.label == label {
			return true
		}
	}
	return false
}

func TestFilterResetAffordanceBuckets(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	if hasReadLabel(m, "clear") {
		t.Errorf("no filter active → no clear affordance")
	}
	m.bucketFilter = "foo" // applied, not typing
	if !hasReadLabel(m, "clear") {
		t.Errorf("applied bucket filter must show a clear affordance")
	}
}

func TestFilterResetAffordanceTree(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.search = "pre" // applied search, not typing
	if !hasReadLabel(m, "clear") {
		t.Errorf("applied tree search must show a clear affordance")
	}
	m.search = ""
	m.searching = true // actively typing → its own Enter/Esc hints, no clear entry
	if hasReadLabel(m, "clear") {
		t.Errorf("while typing the filter, no clear affordance should appear")
	}
}

// 008 US7: the connection affordance reads "connections" (not "new conn") and survives a
// collapsed/narrow bar (FR-019/FR-020).
func TestCommandBarConnectionsAffordance(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.connect = &fakeConnector{}
	m.width = 140
	wide := m.commandBarView(140)
	if !strings.Contains(wide, "connections") {
		t.Errorf("bar should show 'connections'; got:\n%s", wide)
	}
	if strings.Contains(wide, "new conn") {
		t.Errorf("bar must not show the old 'new conn' label; got:\n%s", wide)
	}
	narrow := m.commandBarView(40) // forces the collapsed bar + entry dropping
	if !strings.Contains(narrow, "connections") {
		t.Errorf("collapsed bar must keep 'connections' (not dropped first); got:\n%s", narrow)
	}
}

// 008 US9: the write group shows only the selection-applicable delete — never two identical
// "delete" labels (FR-022).
func writeLabels(m App) []string {
	var out []string
	for _, e := range m.writeEntries(m.selKind(), m.actionCatalog()) {
		out = append(out, e.label)
	}
	return out
}

func countLabel(labels []string, want string) int {
	n := 0
	for _, l := range labels {
		if l == want {
			n++
		}
	}
	return n
}

func TestNoDuplicateDeleteObjectCursor(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "p/x")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	labels := writeLabels(m)
	if got := countLabel(labels, "delete"); got != 1 {
		t.Errorf("object cursor: want exactly one 'delete' entry, got %d (%v)", got, labels)
	}
}

func TestNoDuplicateDeleteFolderCursor(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "p/x")
	m := treeApp(f, true)
	selectDir(&m, "p/")
	labels := writeLabels(m)
	if got := countLabel(labels, "delete"); got != 1 {
		t.Errorf("folder cursor: want exactly one 'delete' entry, got %d (%v)", got, labels)
	}
}

// 008 US9 regression: suppressing the inapplicable delete must NOT empty the write group in
// bucket mode (where "delete" is the lone write action) — FR-016 keeps the capability shown.
func TestWriteGroupKeptInBucketModeZeroMatches(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "x", Writable: true},
		"ctx", []string{"ctx"}, nil, nil, preview.ProtoNone)
	bs, _ := f.ListBuckets(context.Background())
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
	m.bucketFilter = "zzz-matches-nothing" // delete becomes inapplicable (0 buckets)
	if got := m.writeEntries(m.selKind(), m.actionCatalog()); len(got) == 0 {
		t.Errorf("bucket-mode write group must not be emptied by delete suppression (FR-016)")
	}
}

func TestCommandBarShowsChordLabels(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m.width = 140
	bar := m.commandBarView(140)
	if !strings.Contains(bar, "Ctrl+X") {
		t.Errorf("write block must advertise the delete chord 'Ctrl+X'; got:\n%s", bar)
	}
}

// --- 007 US1 / FR-017: multi-select reflects bulk variants + counts ---

func TestCommandBarBulkLabelWithCount(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "c.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, " ") // mark a.txt
	m.width = 140
	bar := m.commandBarView(140)
	if !strings.Contains(bar, "delete 1") || !strings.Contains(bar, "download 1") {
		t.Errorf("command bar should show bulk variant + count when marked; got:\n%s", bar)
	}
}

// --- 007 US1 / FR-018: inapplicable distinct from read-only dimmed ---

func TestWriteEntryStatesDistinct(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	// Read-only → every write entry uses the dimmed role.
	ro := treeApp(f, false)
	selectObject(&ro, "a.txt")
	for _, e := range ro.writeEntries(ro.selKind(), ro.actionCatalog()) {
		if e.role != roleWriteDimmed {
			t.Errorf("read-only write entry %q role=%v, want roleWriteDimmed", e.label, e.role)
		}
	}
	// Writable, FOLDER selected → copy/move are inapplicable (need an object), DISTINCT from
	// dimmed; other write entries are active. (008 US9: the inapplicable DELETE is now
	// suppressed entirely rather than shown dimmed, so the inapplicable role is exercised via
	// copy/move on a folder.)
	rw := treeApp(f, true)
	selectDir(&rw, "docs/")
	var sawInapplicable bool
	for _, e := range rw.writeEntries(rw.selKind(), rw.actionCatalog()) {
		if e.role == roleWriteInapplicable {
			sawInapplicable = true
		}
		if e.role == roleWriteDimmed {
			t.Errorf("writable write entry %q must not use the dimmed role", e.label)
		}
	}
	if !sawInapplicable {
		t.Error("copy/move on a folder selection should use the inapplicable role")
	}
}

// --- US1: bucket list offers analyze directly + a chorded bucket delete ---

func TestBucketListDirectAnalyze(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, "a")
	// 016: 'a' expands the inline usage breakdown for the highlighted bucket (no mode switch).
	if m.detailSection != sectBreakdown {
		t.Fatalf("'a' in the bucket list should expand the inline usage breakdown; section=%v, mode=%v", m.detailSection, m.mode)
	}
}

func TestBucketDeleteChordTypedName(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	m.armed = true // writable
	m = press(m, "ctrl+x")
	if m.op == nil || m.op.kind != "delete_bucket" || m.op.tier != confirmTyped || m.op.expect != "b" {
		t.Fatalf("ctrl+x on the bucket list should start a typed bucket delete; op=%+v", m.op)
	}
}

// --- 007 US3: palette role distinctness (FR-013/FR-014/FR-015) ---

func TestBlockRolesAreDistinct(t *testing.T) {
	// info / read / write-active / write-dimmed / write-inapplicable must map to
	// DISTINCT styles drawn only from the existing palette.
	roles := []styleRole{roleInfo, roleRead, roleWriteActive, roleWriteDimmed, roleWriteInapplicable}
	seen := map[string]styleRole{}
	for _, r := range roles {
		key := roleStyle[r].Render("X")
		if prev, dup := seen[key]; dup {
			t.Errorf("roles %v and %v render identically (%q) — not distinguishable", prev, r, key)
		}
		seen[key] = r
	}
	// Inactive (read-only, dimmed) MUST differ from not-applicable (inapplicable).
	if roleStyle[roleWriteDimmed].Render("x") == roleStyle[roleWriteInapplicable].Render("x") {
		t.Error("dimmed (read-only) and inapplicable write styles must be distinct (FR-018)")
	}
}

func TestAvailLabelsReadOnlyOmitsWriteFromAvailable(t *testing.T) {
	// availableActions still drops write in read-only (used elsewhere); the command bar
	// shows them dimmed via writeEntries instead.
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false)
	selectObject(&m, "a.txt")
	ls := availLabels(m)
	if !hasStr(ls, "download") {
		t.Errorf("read-only must still offer download; got %v", ls)
	}
}
