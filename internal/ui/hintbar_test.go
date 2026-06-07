package ui

import (
	"strings"
	"testing"

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

	if m.mode != modeUsage {
		t.Fatalf("'a' on a folder should analyze (modeUsage), not open a menu; mode=%v", m.mode)
	}
}

// --- 007 US1: the three-block command bar ---

func TestCommandBarHasThreeBlocks(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.width = 140
	bar := m.commandBarView(140)
	for _, want := range []string{"INFO", "READ", "WRITE"} {
		if !strings.Contains(bar, want) {
			t.Errorf("command bar missing block %q; got:\n%s", want, bar)
		}
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
	// The "(w to arm)" cue marks the dimmed write block (NO_COLOR-safe, FR-015).
	if !strings.Contains(bar, "w to arm") {
		t.Errorf("read-only write block must carry a non-color 'w to arm' cue; got:\n%s", bar)
	}
}

func TestCommandBarShowsChordLabels(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m.width = 140
	bar := m.commandBarView(140)
	if !strings.Contains(bar, "^x") {
		t.Errorf("write block must advertise the delete chord '^x'; got:\n%s", bar)
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
	// Read-only → every write entry is dimmed.
	ro := treeApp(f, false)
	selectObject(&ro, "a.txt")
	for _, e := range ro.writeEntries() {
		if e.state != entryDimmed {
			t.Errorf("read-only write entry %q state=%v, want entryDimmed", e.label, e.state)
		}
	}
	// Writable, object selected → recursive delete is inapplicable (needs a folder),
	// DISTINCT from dimmed; other write entries are active.
	rw := treeApp(f, true)
	selectObject(&rw, "a.txt")
	var sawInapplicable bool
	for _, e := range rw.writeEntries() {
		if e.state == entryInapplicable {
			sawInapplicable = true
			if e.role == roleWriteDimmed {
				t.Errorf("inapplicable entry %q must not use the dimmed role", e.label)
			}
		}
	}
	if !sawInapplicable {
		t.Error("recursive delete on an object selection should be entryInapplicable")
	}
}

// --- US1: bucket list offers analyze directly + a chorded bucket delete ---

func TestBucketListDirectAnalyze(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, "a")
	if m.mode != modeUsage {
		t.Fatalf("'a' in the bucket list should analyze the bucket; mode=%v", m.mode)
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
