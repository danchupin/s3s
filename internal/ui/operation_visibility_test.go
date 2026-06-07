package ui

import (
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// 008 US6 (FR-015/FR-016): a copy/move/bulk-copy whose destination prefix differs from the
// current view must precisely invalidate the source AND destination level caches (same
// bucket), so a later navigation shows fresh contents without a manual refresh. Tests drive
// onOperationDone directly with a completed op (the established pattern in write_ops_test.go).

func TestCopyInvalidatesDestPrefix(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "src.txt")
	m := treeApp(f, true)
	dest := m.levelKeyFor("b", "sub/")
	m.cache.Put(dest, &levelState{complete: true}) // stale, cached-empty destination
	m.op = &operation{kind: "copy", bucket: "b", srcKey: "src.txt", target: "sub/src.txt", phase: phaseRunning}

	out, _ := m.onOperationDone(operationDoneMsg{gen: m.gen})
	if _, ok := out.(App).cache.Get(dest); ok {
		t.Error("copy to a different prefix must invalidate the destination level cache")
	}
}

func TestMoveInvalidatesSourceAndDest(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a/src.txt")
	m := treeApp(f, true)
	src := m.levelKeyFor("b", "a/")
	dest := m.levelKeyFor("b", "z/")
	m.cache.Put(src, &levelState{complete: true})
	m.cache.Put(dest, &levelState{complete: true})
	m.op = &operation{kind: "move", bucket: "b", srcKey: "a/src.txt", target: "z/src.txt", phase: phaseRunning}

	out, _ := m.onOperationDone(operationDoneMsg{gen: m.gen})
	mm := out.(App)
	if _, ok := mm.cache.Get(src); ok {
		t.Error("move must invalidate the SOURCE prefix cache (item left it)")
	}
	if _, ok := mm.cache.Get(dest); ok {
		t.Error("move must invalidate the DESTINATION prefix cache (item arrived)")
	}
}

func TestBulkCopyInvalidatesDestPrefix(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x.txt", "y.txt")
	m := treeApp(f, true)
	dest := m.levelKeyFor("b", "bulkdst/")
	m.cache.Put(dest, &levelState{complete: true})
	m.op = &operation{kind: "bulk_copy", bucket: "b", parent: "", dstKey: "bulkdst/", phase: phaseRunning}

	out, _ := m.onOperationDone(operationDoneMsg{gen: m.gen, summary: &storage.DeleteSummary{Deleted: 2}})
	if _, ok := out.(App).cache.Get(dest); ok {
		t.Error("bulk_copy to a prefix must invalidate that destination level cache")
	}
}

func TestPartialMoveInvalidatesDest(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a/src.txt")
	m := treeApp(f, true)
	dest := m.levelKeyFor("b", "z/")
	m.cache.Put(dest, &levelState{complete: true})
	m.op = &operation{kind: "move", bucket: "b", srcKey: "a/src.txt", target: "z/src.txt", phase: phaseRunning}
	// Partial move: copied to dest, source remains → dest is stale and MUST be invalidated.
	out, _ := m.onOperationDone(operationDoneMsg{gen: m.gen, err: storage.ErrMovePartial})
	if _, ok := out.(App).cache.Get(dest); ok {
		t.Error("a partial move must still invalidate the destination level cache")
	}
}

func TestFailedCopyLeavesCacheForReload(t *testing.T) {
	// A hard failure should not falsely report success; invalidation is gated on err==nil,
	// but a failed op must never leave a stale SUCCESS view — here we only assert no panic
	// and that the op is cleared.
	f := storage.NewFake()
	f.Seed("b", "src.txt")
	m := treeApp(f, true)
	m.op = &operation{kind: "copy", bucket: "b", srcKey: "src.txt", target: "sub/src.txt", phase: phaseRunning}
	out, _ := m.onOperationDone(operationDoneMsg{gen: m.gen, err: storage.ErrAccessDenied})
	if out.(App).op != nil {
		t.Error("a failed copy should still clear the op")
	}
}
