package storage

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestFakeRemoveObject: delete removes the key; a missing key is ErrNotFound; a
// FailDelete key is ErrAccessDenied (US1, FR-001/FR-015).
func TestFakeRemoveObject(t *testing.T) {
	f := NewFake()
	f.Seed("b", "a.txt", "keep.txt")

	if err := f.RemoveObject(context.Background(), "b", "a.txt"); err != nil {
		t.Fatalf("RemoveObject error: %v", err)
	}
	if _, ok := f.Buckets["b"].Objects["a.txt"]; ok {
		t.Error("a.txt should be deleted")
	}
	if _, ok := f.Buckets["b"].Objects["keep.txt"]; !ok {
		t.Error("keep.txt must be untouched")
	}
	if err := f.RemoveObject(context.Background(), "b", "gone.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing key = %v, want ErrNotFound", err)
	}
}

// TestFakeUploadFile: stores the exact bytes (SC-003 fidelity at the fake level).
func TestFakeUploadFile(t *testing.T) {
	f := NewFake()
	f.Seed("b")
	want := []byte("hello world payload")
	if err := f.UploadFile(context.Background(), "b", "dir/x.bin", bytes.NewReader(want), int64(len(want))); err != nil {
		t.Fatalf("UploadFile error: %v", err)
	}
	got := f.Buckets["b"].Objects["dir/x.bin"].Data
	if !bytes.Equal(got, want) {
		t.Errorf("stored bytes = %q, want %q", got, want)
	}
}

// TestFakeCopyKey: duplicates and leaves the source; rejects invalid/identical dst
// (US3, FR-004/FR-013).
func TestFakeCopyKey(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "src.txt", FakeObject{Data: []byte("data")})

	if err := f.CopyKey(context.Background(), "b", "src.txt", "dst.txt"); err != nil {
		t.Fatalf("CopyKey error: %v", err)
	}
	if !bytes.Equal(f.Buckets["b"].Objects["dst.txt"].Data, []byte("data")) {
		t.Error("dst must hold the copied bytes")
	}
	if _, ok := f.Buckets["b"].Objects["src.txt"]; !ok {
		t.Error("source must remain after copy")
	}
	if err := f.CopyKey(context.Background(), "b", "src.txt", "src.txt"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("dst==src = %v, want ErrInvalidName", err)
	}
	if err := f.CopyKey(context.Background(), "b", "src.txt", "  "); !errors.Is(err, ErrInvalidName) {
		t.Errorf("blank dst = %v, want ErrInvalidName", err)
	}
}

// TestFakeMoveObjectClean: a clean move leaves only the destination (US4 AS1).
func TestFakeMoveObjectClean(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "old/x", FakeObject{Data: []byte("d")})
	if err := f.MoveObject(context.Background(), "b", "old/x", "new/x"); err != nil {
		t.Fatalf("MoveObject error: %v", err)
	}
	if _, ok := f.Buckets["b"].Objects["old/x"]; ok {
		t.Error("source must be gone after a clean move")
	}
	if _, ok := f.Buckets["b"].Objects["new/x"]; !ok {
		t.Error("destination must exist after a move")
	}
}

// TestFakeMoveObjectPartial: copy succeeds but source delete fails => ErrMovePartial
// with BOTH keys present (no data loss) — US4 AS2, FR-007.
func TestFakeMoveObjectPartial(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "old/x", FakeObject{Data: []byte("d")})
	f.FailDelete = map[string]bool{"b/old/x": true}

	err := f.MoveObject(context.Background(), "b", "old/x", "new/x")
	if !errors.Is(err, ErrMovePartial) {
		t.Fatalf("partial move = %v, want ErrMovePartial", err)
	}
	if _, ok := f.Buckets["b"].Objects["new/x"]; !ok {
		t.Error("destination must hold the data (no loss)")
	}
	if _, ok := f.Buckets["b"].Objects["old/x"]; !ok {
		t.Error("source must remain when its delete failed (no loss)")
	}
}

// TestFakeDeleteRecursive: removes all under a prefix and reports accurate counts;
// a FailDelete key is counted Failed while the rest still delete (best-effort) —
// US5, FR-009/FR-011.
func TestFakeDeleteRecursive(t *testing.T) {
	f := NewFake()
	f.Seed("b", "p/a", "p/b", "p/c", "other")
	f.FailDelete = map[string]bool{"b/p/b": true}

	var lastProgress DeleteSummary
	sum, err := f.DeleteRecursive(context.Background(), "b", "p/", func(s DeleteSummary) { lastProgress = s })
	if err != nil {
		t.Fatalf("DeleteRecursive error: %v", err)
	}
	if sum.Deleted != 2 || sum.Failed != 1 {
		t.Errorf("summary = %+v, want Deleted=2 Failed=1", sum)
	}
	if lastProgress != sum {
		t.Errorf("final progress %+v != summary %+v", lastProgress, sum)
	}
	if _, ok := f.Buckets["b"].Objects["p/b"]; !ok {
		t.Error("the failing key must remain")
	}
	if _, ok := f.Buckets["b"].Objects["other"]; !ok {
		t.Error("a key outside the prefix must be untouched")
	}
	if _, ok := f.Buckets["b"].Objects["p/a"]; ok {
		t.Error("deletable keys under the prefix must be gone")
	}
}

// TestFakeDeleteRecursiveCancel: a cancelled context returns the partial counts and
// ctx.Err() (US5 AS4).
func TestFakeDeleteRecursiveCancel(t *testing.T) {
	f := NewFake()
	f.Seed("b", "p/a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.DeleteRecursive(ctx, "b", "p/", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled DeleteRecursive = %v, want context.Canceled", err)
	}
}

// TestGuardRefusesAllMutations: the read-only guard refuses EVERY mutating method
// with ErrReadOnly and never touches storage (FR-012, SC-008). Adding a Mutator
// method without a guard override would fail here.
func TestGuardRefusesAllMutations(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "x", FakeObject{Data: []byte("d")})
	ro := Guard(f, false).(Mutator)
	ctx := context.Background()

	checks := []struct {
		name string
		err  error
	}{
		{"RemoveObject", ro.RemoveObject(ctx, "b", "x")},
		{"UploadFile", ro.UploadFile(ctx, "b", "y", strings.NewReader("z"), 1)},
		{"CopyKey", ro.CopyKey(ctx, "b", "x", "z")},
		{"MoveObject", ro.MoveObject(ctx, "b", "x", "z")},
	}
	for _, c := range checks {
		if !errors.Is(c.err, ErrReadOnly) {
			t.Errorf("%s on read-only guard = %v, want ErrReadOnly", c.name, c.err)
		}
	}
	if _, err := ro.DeleteRecursive(ctx, "b", "", nil); !errors.Is(err, ErrReadOnly) {
		t.Errorf("DeleteRecursive on read-only guard = %v, want ErrReadOnly", err)
	}

	// Storage must be completely untouched.
	if got := len(f.Buckets["b"].Objects); got != 1 {
		t.Errorf("read-only guard mutated storage: %d objects, want 1", got)
	}
	if _, ok := f.Buckets["b"].Objects["x"]; !ok {
		t.Error("the seeded object must still exist")
	}
}
