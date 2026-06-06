package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// markObjects marks the given object keys in the level.
func markObjects(m *App, keys ...string) {
	if m.sel == nil {
		m.sel = map[string]bool{}
	}
	for _, k := range keys {
		m.sel[k] = true
	}
}

// runBulkToDone starts the bulk producer (dispatchBulk's batch cmd is not executed in
// tests) and drives it to the terminal summary.
func runBulkToDone(t *testing.T, m App) App {
	t.Helper()
	op := m.op
	if op == nil || m.opCh == nil {
		t.Fatal("bulk op not dispatched")
	}
	msg := bulkCmd(m.loadCtx, m.activeStore(), op.kind, op.bucket, op.parent, op.dstKey, op.bulkKeys, m.downloadDir(), m.opCh, m.gen)()
	for {
		switch ev := msg.(type) {
		case operationProgressMsg:
			mm, _ := m.Update(ev)
			m = mm.(App)
			msg = waitForProgress(m.opCh, m.gen)()
		case operationDoneMsg:
			mm, _ := m.Update(ev)
			return mm.(App)
		default:
			t.Fatalf("unexpected msg %#v", msg)
		}
	}
}

// TestSelectionMarkAndClear: marking toggles objects (not folders); navigating away
// clears the selection (005 FR-014/FR-019).
func TestSelectionMarkAndClear(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "a.txt", storage.FakeObject{Data: make([]byte, 10)})
	f.SeedObject("b", "c.txt", storage.FakeObject{Data: make([]byte, 30)})
	f.Seed("b", "sub/x") // creates a sub/ folder
	m := treeApp(f, false)

	selectObject(&m, "a.txt")
	mm, _ := m.toggleMark()
	m = mm.(App)
	selectObject(&m, "c.txt")
	mm, _ = m.toggleMark()
	m = mm.(App)
	if m.selCount() != 2 || m.selSize() != 40 {
		t.Errorf("selection = %d items / %d bytes, want 2/40", m.selCount(), m.selSize())
	}
	// Marking a folder is a no-op.
	selectDir(&m, "sub/")
	mm, _ = m.toggleMark()
	m = mm.(App)
	if m.selCount() != 2 {
		t.Errorf("marking a folder changed the selection: %d", m.selCount())
	}
	// Navigating into a folder clears the selection.
	m.prefix = "sub/"
	mm, _ = m.enterLevel()
	if got := mm.(App).selCount(); got != 0 {
		t.Errorf("navigation should clear selection, got %d", got)
	}
}

// TestBulkDownloadMirrorsHierarchy: every marked object is written into local subdirs
// mirroring its key path, with a truthful per-item summary; a failing item does not
// abort the rest (005 FR-015a/FR-018).
func TestBulkDownloadMirrorsHierarchy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("S3S_DOWNLOAD_DIR", dir)
	f := storage.NewFake()
	f.SeedObject("b", "logs/a.log", storage.FakeObject{Data: []byte("aaa")})
	f.SeedObject("b", "logs/2026/b.log", storage.FakeObject{Data: []byte("bbbb")})
	f.SeedObject("b", "bad", storage.FakeObject{Data: []byte("x"), AccessDenied: true})
	m := treeApp(f, false) // read-only — bulk download still works

	markObjects(&m, "logs/a.log", "logs/2026/b.log", "bad")
	mm, _ := m.startBulkDownload()
	m = runBulkToDone(t, mm.(App))

	if _, err := os.Stat(filepath.Join(dir, "logs", "a.log")); err != nil {
		t.Errorf("logs/a.log not downloaded to a mirrored subdir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "logs", "2026", "b.log")); err != nil {
		t.Errorf("logs/2026/b.log not downloaded to a mirrored subdir: %v", err)
	}
	// 2 ok, 1 failed — and the selection is consumed.
	if m.notice == "" || m.selCount() != 0 {
		t.Errorf("expected a summary notice + cleared selection; notice=%q sel=%d", m.notice, m.selCount())
	}
}

// TestBulkDeleteWriteGatedTypedConfirm: bulk delete needs the armed write state and a
// typed count confirmation; it is refused read-only (005 FR-016/FR-017).
func TestBulkDeleteWriteGatedTypedConfirm(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("x")})
	f.SeedObject("b", "y", storage.FakeObject{Data: []byte("y")})

	// Read-only: refused.
	ro := treeApp(f, false)
	markObjects(&ro, "x", "y")
	mm, _ := ro.startBulkDelete()
	if got := mm.(App); got.op != nil {
		t.Errorf("bulk delete must be refused read-only; op=%+v", got.op)
	}

	// Writable: typed-count confirm, then it deletes both.
	rw := treeApp(f, true)
	markObjects(&rw, "x", "y")
	mm, _ = rw.startBulkDelete()
	m := mm.(App)
	if m.op == nil || m.op.kind != "bulk_delete" || m.op.tier != confirmTyped || m.op.expect != strconv.Itoa(2) {
		t.Fatalf("bulk delete should require a typed count confirm; op=%+v", m.op)
	}
	m.op.input = "2" // type the count
	mm, _ = m.onConfirmKey("enter", keyMsgFor("enter"))
	m = runBulkToDone(t, mm.(App))
	if len(f.Buckets["b"].Objects) != 0 {
		t.Errorf("bulk delete should remove both objects; remaining=%v", f.Buckets["b"].Objects)
	}
}
