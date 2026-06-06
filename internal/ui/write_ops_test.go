package ui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/localfs"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// treeApp builds an App in modeTree on bucket "b" with the fake's level loaded,
// writable per the flag. White-box: it sets the level state directly.
func treeApp(f *storage.Fake, writable bool) App {
	m := New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "x", Writable: writable},
		"ctx", []string{"ctx"}, nil, nil, preview.ProtoNone)
	bs, _ := f.ListBuckets(context.Background())
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
	m.mode = modeTree
	m.bucket = "b"
	m.prefix = ""
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b"})
	m.level = &levelState{dirs: page.Dirs, objects: page.Objects, complete: true}
	m.loading = false
	return m
}

// selectObject positions the selection on the object with the given key.
func selectObject(m *App, key string) {
	for i, e := range m.treeEntries() {
		if !e.isDir && e.full == key {
			m.treeSel = i
			return
		}
	}
}

// viaMenu drives an operation through its 006 entry point: the direct single key on the
// selection (the modal action menu was removed in US1). It maps the former menu label to
// the new key and presses it, so the large existing op-flow test suite keeps exercising
// the SAME start* entry points.
func viaMenu(t *testing.T, m App, label string) App {
	t.Helper()
	key, ok := map[string]string{
		"download":         "d",
		"analyze":          "a",
		"delete":           "x",
		"copy":             "y",
		"move / rename":    "m",
		"recursive delete": "X",
		"upload here":      "u",
		"new folder":       "+",
		"refresh":          "r",
	}[label]
	if !ok {
		t.Fatalf("viaMenu: no direct key for label %q", label)
	}
	return press(m, key)
}

// selectDir positions the selection on the dir with the given prefix.
func selectDir(m *App, dir string) {
	for i, e := range m.treeEntries() {
		if e.isDir && e.full == dir {
			m.treeSel = i
			return
		}
	}
}

// --- US1: delete a single object ---

func TestDeleteObjectTypedConfirmFlow(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "keep.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")

	m = viaMenu(t, m, "delete")
	if m.op == nil || m.op.kind != "delete_object" || m.op.tier != confirmTyped {
		t.Fatalf("menu delete should start a typed delete op; got %+v", m.op)
	}
	if m.op.expect != "a.txt" {
		t.Errorf("typed expect = %q, want a.txt", m.op.expect)
	}

	// Wrong entry aborts with no dispatch.
	m2 := m
	for _, r := range "wrong" {
		m2 = press(m2, string(r))
	}
	m2 = press(m2, "enter")
	if m2.op != nil {
		t.Error("mismatch must abort the op")
	}
	if !errors.Is(m2.err, errConfirmMismatch) {
		t.Errorf("mismatch err = %v, want errConfirmMismatch", m2.err)
	}
}

func TestDeleteObjectReadOnlyRefused(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false)
	selectObject(&m, "a.txt")
	// In a read-only context the menu offers no delete item; the defensive guard in
	// the entry point still refuses if invoked directly.
	mm, _ := m.startRemoveObject()
	m = mm.(App)
	if m.op != nil {
		t.Error("read-only context must not start a delete op")
	}
	if !errors.Is(m.err, storage.ErrReadOnly) {
		t.Errorf("err = %v, want ErrReadOnly", m.err)
	}
}

func TestRemoveObjectCmdDeletesAndBenignNotFound(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	msg := removeObjectCmd(context.Background(), f, "b", "a.txt", 7)()
	done, ok := msg.(operationDoneMsg)
	if !ok || done.err != nil || done.gen != 7 {
		t.Fatalf("remove done = %#v, want clean done gen=7", msg)
	}
	if _, ok := f.Buckets["b"].Objects["a.txt"]; ok {
		t.Error("object should be deleted")
	}
	// Already-gone delete is benign (clean done, no error).
	msg2 := removeObjectCmd(context.Background(), f, "b", "a.txt", 7)()
	if d := msg2.(operationDoneMsg); d.err != nil {
		t.Errorf("not-found delete should be benign; got err %v", d.err)
	}
}

// --- US2: upload ---

func TestUploadBrowserOverwriteEscalation(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "exists.txt")
	dir := t.TempDir()
	mkFile(t, filepath.Join(dir, "exists.txt"), "new")
	mkFile(t, filepath.Join(dir, "fresh.txt"), "data")

	m := treeApp(f, true)
	m = viaMenu(t, m, "upload here")
	if m.op == nil || m.op.phase != phaseBrowse {
		t.Fatalf("menu upload should open the file browser; got %+v", m.op)
	}
	// Point the browser at our temp dir directly.
	mm, _ := (&m).openBrowser(dir)
	m = mm.(App)

	// Choosing a colliding key escalates to the typed overwrite tier.
	collide := findEntry(t, m, "exists.txt")
	mc, _ := m.chooseUploadFile(collide)
	mcApp := mc.(App)
	if mcApp.op.tier != confirmTyped || mcApp.op.expect != "exists.txt" {
		t.Errorf("overwrite should be typed with expect=exists.txt; got tier=%v expect=%q", mcApp.op.tier, mcApp.op.expect)
	}

	// A non-colliding key uses the simple tier (the upload op is still in browse).
	mm, _ = (&m).openBrowser(dir)
	m = mm.(App)
	fresh := findEntry(t, m, "fresh.txt")
	mf, _ := m.chooseUploadFile(fresh)
	if mf.(App).op.tier != confirmSimple {
		t.Errorf("non-colliding upload should be simple tier; got %v", mf.(App).op.tier)
	}
}

func TestUploadCmdStreamsAndStores(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	dir := t.TempDir()
	path := filepath.Join(dir, "x.bin")
	want := strings.Repeat("payload-", 4096) // multi-read
	mkFile(t, path, want)

	ch := make(chan progressEvent, 8)
	cmd := uploadCmd(context.Background(), f, "b", "up/x.bin", path, int64(len(want)), ch, 5)
	msg := cmd()
	sawProgress := false
	var done operationDoneMsg
	for {
		switch ev := msg.(type) {
		case operationProgressMsg:
			sawProgress = true
			msg = waitForProgress(ch, 5)()
		case operationDoneMsg:
			done = ev
			goto finished
		default:
			t.Fatalf("unexpected msg %#v", msg)
		}
	}
finished:
	if done.err != nil {
		t.Fatalf("upload err: %v", done.err)
	}
	if got := string(f.Buckets["b"].Objects["up/x.bin"].Data); got != want {
		t.Errorf("uploaded bytes mismatch (len got=%d want=%d)", len(got), len(want))
	}
	if !sawProgress {
		t.Error("expected at least one progress event for a multi-read upload")
	}
}

// --- US3: copy ---

func TestCopyDestEntryAndTiers(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "src.txt", "taken.txt")
	m := treeApp(f, true)
	selectObject(&m, "src.txt")

	m = viaMenu(t, m, "copy")
	if m.op == nil || m.op.kind != "copy" || m.op.phase != phaseDest {
		t.Fatalf("menu copy should start a copy dest entry; got %+v", m.op)
	}
	if m.op.dstKey != "src.txt" {
		t.Errorf("dest should be prefilled with source key; got %q", m.op.dstKey)
	}

	// dst == src is rejected (stays in dest phase).
	rej := m
	rej = press(rej, "enter")
	if rej.op == nil || rej.op.phase != phaseDest || !errors.Is(rej.err, storage.ErrInvalidName) {
		t.Errorf("dst==src must be rejected and stay in dest phase; op=%+v err=%v", rej.op, rej.err)
	}

	// Edit to a free destination → simple tier.
	free := m
	free.op.dstKey = "copy.txt"
	mf, _ := free.submitDest()
	if mf.(App).op.tier != confirmSimple {
		t.Errorf("free dst should be simple tier; got %v", mf.(App).op.tier)
	}

	// Edit to an existing key → typed overwrite.
	taken := m
	taken.op.dstKey = "taken.txt"
	mt, _ := taken.submitDest()
	if mt.(App).op.tier != confirmTyped || mt.(App).op.expect != "taken.txt" {
		t.Errorf("colliding dst should be typed overwrite; got tier=%v expect=%q", mt.(App).op.tier, mt.(App).op.expect)
	}
}

// --- US4: move ---

func TestMoveAlwaysTypedAndPartial(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "src.txt")
	m := treeApp(f, true)
	selectObject(&m, "src.txt")

	m = viaMenu(t, m, "move / rename")
	if m.op == nil || m.op.kind != "move" {
		t.Fatalf("menu move should start a move; got %+v", m.op)
	}
	// Even a free destination is typed (the source is removed).
	m.op.dstKey = "dst.txt"
	mm, _ := m.submitDest()
	if mm.(App).op.tier != confirmTyped {
		t.Errorf("move must always be typed; got %v", mm.(App).op.tier)
	}

	// A partial move (ErrMovePartial) renders a non-success notice, not a clean done.
	f.FailDelete = map[string]bool{"b/src.txt": true}
	msg := moveObjectCmd(context.Background(), f, "b", "src.txt", "dst.txt", 9)()
	done := msg.(operationDoneMsg)
	if !errors.Is(done.err, storage.ErrMovePartial) {
		t.Fatalf("move cmd err = %v, want ErrMovePartial", done.err)
	}
	app := treeApp(f, true)
	app.op = &operation{kind: "move", phase: phaseRunning}
	app.gen = done.gen
	out, _ := app.onOperationDone(done)
	if outApp := out.(App); outApp.notice == "" || strings.Contains(strings.ToLower(outApp.notice), "done ok") {
		t.Errorf("partial move should surface a partial notice; got %q", outApp.notice)
	}
}

// --- US5: recursive delete ---

func TestRecursiveDeleteTypedAndPartialCounts(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "p/a", "p/b", "p/c")
	m := treeApp(f, true)
	selectDir(&m, "p/")

	m = viaMenu(t, m, "recursive delete")
	if m.op == nil || m.op.kind != "delete_recursive" || m.op.tier != confirmTyped {
		t.Fatalf("menu recursive delete should start a typed recursive delete; got %+v", m.op)
	}
	if m.op.expect != "p/" {
		t.Errorf("recursive expect = %q, want p/", m.op.expect)
	}

	// Best-effort: one key fails, the rest delete; the outcome is partial.
	f.FailDelete = map[string]bool{"b/p/b": true}
	ch := make(chan progressEvent, 8)
	cmd := recursiveDeleteCmd(context.Background(), f, "b", "p/", ch, 3)
	msg := cmd()
	var done operationDoneMsg
	for {
		if d, ok := msg.(operationDoneMsg); ok {
			done = d
			break
		}
		msg = waitForProgress(ch, 3)()
	}
	if done.summary == nil || done.summary.Deleted != 2 || done.summary.Failed != 1 {
		t.Fatalf("recursive summary = %+v, want Deleted=2 Failed=1", done.summary)
	}
	if !done.partial {
		t.Error("a recursive delete with failures must be marked partial")
	}
}

// TestWriteOpsReadOnlyRefused: every new write trigger is refused on a read-only
// context with the ErrReadOnly hint and no op (FR-012, SC-008).
func TestWriteOpsReadOnlyRefused(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "obj.txt", "p/a")
	cases := []struct {
		name   string
		sel    func(*App)
		invoke func(App) (tea.Model, tea.Cmd)
	}{
		{"upload", func(*App) {}, App.startUpload},
		{"copy", func(m *App) { selectObject(m, "obj.txt") }, App.startCopy},
		{"move", func(m *App) { selectObject(m, "obj.txt") }, App.startMove},
		{"recursive", func(m *App) { selectDir(m, "p/") }, App.startRecursiveDelete},
	}
	for _, c := range cases {
		m := treeApp(f, false)
		c.sel(&m)
		// The read-only menu offers no write item; the entry point's defensive guard
		// still refuses with ErrReadOnly if invoked directly.
		mm, _ := c.invoke(m)
		m = mm.(App)
		if m.op != nil {
			t.Errorf("%s: read-only must not start an op; got %+v", c.name, m.op)
		}
		if !errors.Is(m.err, storage.ErrReadOnly) {
			t.Errorf("%s: err = %v, want ErrReadOnly", c.name, m.err)
		}
	}
}

// TestStreamingCancelNotSuccess: a cancelled upload / recursive delete clears the op
// and surfaces a non-success notice, never a clean done (FR-011, US2 AS4/US5 AS4).
func TestStreamingCancelNotSuccess(t *testing.T) {
	for _, kind := range []string{"upload", "delete_recursive"} {
		f := storage.NewFake()
		f.Seed("b", "x")
		m := treeApp(f, true)
		m.op = &operation{kind: kind, phase: phaseRunning}
		out, _ := m.onOperationDone(operationDoneMsg{gen: m.gen, err: context.Canceled})
		app := out.(App)
		if app.op != nil {
			t.Errorf("%s: cancelled op must be cleared", kind)
		}
		if !strings.Contains(strings.ToLower(app.notice), "cancel") {
			t.Errorf("%s: notice = %q, want a cancellation notice", kind, app.notice)
		}
	}
}

// TestRunningOpIsModal: while a mutation runs, navigation and new mutations are
// blocked; only cancel works (prevents superseding the progress reader — review #1).
func TestRunningOpIsModal(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "p/x")
	m := treeApp(f, true)
	m.op = &operation{kind: "upload", phase: phaseRunning}
	m.loading = true

	// A navigation key (Enter) is ignored — no new op, still running, mode unchanged.
	before := m.mode
	m = press(m, "enter")
	if m.mode != before || m.op == nil || m.op.phase != phaseRunning {
		t.Errorf("navigation must be blocked during a running op; mode=%v op=%+v", m.mode, m.op)
	}
	// A new-op key (d) is ignored too.
	m = press(m, "d")
	if m.op.kind != "upload" {
		t.Errorf("starting another op must be blocked during a running op; got kind=%q", m.op.kind)
	}
}

// TestMoveOntoExistingWarnsOverwrite: a move whose destination already exists is
// flagged as an overwrite (clobber warning) and stays typed (review #2, US4 AS3).
func TestMoveOntoExistingWarnsOverwrite(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "src.txt", "dst.txt")
	m := treeApp(f, true)
	selectObject(&m, "src.txt")
	m = viaMenu(t, m, "move / rename")
	m.op.dstKey = "dst.txt"
	out, _ := m.submitDest()
	app := out.(App)
	if !app.op.overwrite {
		t.Error("move onto an existing key must set the overwrite flag")
	}
	if app.op.tier != confirmTyped {
		t.Errorf("move-overwrite tier = %v, want typed", app.op.tier)
	}
	if !strings.Contains(app.opPromptLine(120), "OVERWRITES") {
		t.Errorf("typed prompt should warn of overwrite; got %q", app.opPromptLine(120))
	}
}

// TestCountingReaderSeekable: countingReader must stay seekable (SigV4 hash-rewind),
// and a rewind to start resets the byte counter so progress tracks the send pass.
// A non-seekable body causes XAmzContentSHA256Mismatch on real backends.
func TestCountingReaderSeekable(t *testing.T) {
	ch := make(chan progressEvent, 64)
	cr := &countingReader{rs: bytes.NewReader([]byte("0123456789")), total: 10, ch: ch}

	var _ io.ReadSeeker = cr // must satisfy io.ReadSeeker for the SDK to sign the body

	buf := make([]byte, 4)
	_, _ = cr.Read(buf)
	if cr.read != 4 {
		t.Fatalf("after read, counter = %d, want 4", cr.read)
	}
	if pos, err := cr.Seek(0, io.SeekStart); err != nil || pos != 0 {
		t.Fatalf("Seek(0) = %d, %v; want 0, nil", pos, err)
	}
	if cr.read != 0 {
		t.Errorf("rewind must reset the byte counter; got %d", cr.read)
	}
}

// --- helpers ---

func mkFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func findEntry(t *testing.T, m App, name string) localfs.Entry {
	t.Helper()
	for _, en := range m.fbEntries {
		if en.Name == name {
			return en
		}
	}
	t.Fatalf("entry %q not found in browser", name)
	return localfs.Entry{}
}
