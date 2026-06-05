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
