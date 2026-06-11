package storage

import (
	"context"
	"errors"
	"testing"
)

// TestGuardRefusesMutation: a read-only-guarded backend returns ErrReadOnly from
// CreateFolder and leaves storage unchanged; a writable one passes through (SC-002,
// SC-007, FR-003/FR-012).
func TestGuardRefusesMutation(t *testing.T) {
	f := NewFake()
	f.Seed("b") // empty bucket

	ro, ok := Guard(f, false).(Mutator)
	if !ok {
		t.Fatal("guarded backend must satisfy Mutator")
	}
	if err := ro.CreateFolder(context.Background(), "b", "x"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("read-only CreateFolder = %v, want ErrReadOnly", err)
	}
	// Storage must be untouched (the call never reached the client).
	if got := len(f.Buckets["b"].Objects); got != 0 {
		t.Errorf("read-only guard mutated storage: %d objects", got)
	}

	// Writable: passes through and creates the folder.
	rw := Guard(f, true).(Mutator)
	if err := rw.CreateFolder(context.Background(), "b", "x"); err != nil {
		t.Fatalf("writable CreateFolder error: %v", err)
	}
	if _, ok := f.Buckets["b"].Objects["x/"]; !ok {
		t.Errorf("writable CreateFolder did not create key x/; objects=%v", f.Buckets["b"].Objects)
	}

	// Reads still pass through the guard.
	if _, err := Guard(f, false).ListBuckets(context.Background()); err != nil {
		t.Errorf("guard must delegate reads: %v", err)
	}
}

// TestGuardRefusesRemoveBucket: the read-only guard refuses bucket delete (007 US4 /
// FR-024b) without touching the backend.
func TestGuardRefusesRemoveBucket(t *testing.T) {
	f := NewFake()
	f.Seed("b") // empty bucket
	ro := Guard(f, false).(Mutator)
	if err := ro.RemoveBucket(context.Background(), "b"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("read-only RemoveBucket = %v, want ErrReadOnly", err)
	}
	if _, ok := f.Buckets["b"]; !ok {
		t.Error("read-only guard must not remove the bucket")
	}
}

// TestFakeRemoveBucket: empty bucket is removed; a non-empty one returns
// ErrBucketNotEmpty and is left intact (007 FR-024b / SC-015).
func TestFakeRemoveBucket(t *testing.T) {
	f := NewFake()
	f.Seed("empty")
	f.SeedObject("full", "a.txt", FakeObject{Data: []byte("x")})

	if err := f.RemoveBucket(context.Background(), "empty"); err != nil {
		t.Fatalf("RemoveBucket(empty) = %v, want nil", err)
	}
	if _, ok := f.Buckets["empty"]; ok {
		t.Error("empty bucket should be removed")
	}
	if err := f.RemoveBucket(context.Background(), "full"); !errors.Is(err, ErrBucketNotEmpty) {
		t.Fatalf("RemoveBucket(full) = %v, want ErrBucketNotEmpty", err)
	}
	if _, ok := f.Buckets["full"]; !ok {
		t.Error("non-empty bucket must be left intact")
	}
}

// TestGuardDelegatesNewReads: the 005 read methods (GetObject, UsageOf) pass through
// the read-only guard unchanged — no ErrReadOnly (storage-read-ops-contract C3).
func TestGuardDelegatesNewReads(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "a.txt", FakeObject{Data: []byte("hello")})

	ro := Guard(f, false)
	rc, err := ro.GetObject(context.Background(), "b", "a.txt")
	if err != nil {
		t.Fatalf("guarded GetObject = %v, want pass-through", err)
	}
	_ = rc.Close()

	rep, err := ro.UsageOf(context.Background(), "b", "", 0, nil)
	if err != nil {
		t.Fatalf("guarded UsageOf = %v, want pass-through", err)
	}
	if rep.TotalCount != 1 || rep.TotalSize != 5 {
		t.Errorf("UsageOf through guard = %d objs / %d bytes, want 1/5", rep.TotalCount, rep.TotalSize)
	}
}
