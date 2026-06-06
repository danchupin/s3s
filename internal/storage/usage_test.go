package storage

import (
	"context"
	"errors"
	"io"
	"testing"
)

// TestGetObjectFull: GetObject returns the full body byte-for-byte; a missing key is
// ErrNotFound (005 FR-001, storage-read-ops-contract C1).
func TestGetObjectFull(t *testing.T) {
	f := NewFake()
	want := []byte("the whole object body, not a 5 MiB slice")
	f.SeedObject("b", "dir/data.bin", FakeObject{Data: want})

	rc, err := f.GetObject(context.Background(), "b", "dir/data.bin")
	if err != nil {
		t.Fatalf("GetObject error: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != string(want) {
		t.Errorf("GetObject body = %q, want %q", got, want)
	}

	if _, err := f.GetObject(context.Background(), "b", "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetObject(missing) = %v, want ErrNotFound", err)
	}
}

// TestUsageOfTotalsAndRanking: UsageOf aggregates totals and ranks immediate children
// largest-first, classifying sub-prefixes vs direct objects (005 FR-008/FR-009).
func TestUsageOfTotalsAndRanking(t *testing.T) {
	f := NewFake()
	// Under prefix "" : logs/ (300), big.bin (250), media/ (100), small.txt (10).
	f.SeedObject("b", "logs/2026/a.log", FakeObject{Data: make([]byte, 200)})
	f.SeedObject("b", "logs/2026/b.log", FakeObject{Data: make([]byte, 100)})
	f.SeedObject("b", "media/img.png", FakeObject{Data: make([]byte, 100)})
	f.SeedObject("b", "big.bin", FakeObject{Data: make([]byte, 250)})
	f.SeedObject("b", "small.txt", FakeObject{Data: make([]byte, 10)})

	rep, err := f.UsageOf(context.Background(), "b", "", nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if rep.TotalCount != 5 || rep.TotalSize != 660 {
		t.Fatalf("totals = %d objs / %d bytes, want 5/660", rep.TotalCount, rep.TotalSize)
	}
	wantOrder := []struct {
		name  string
		isDir bool
		size  int64
	}{
		{"logs/", true, 300},
		{"big.bin", false, 250},
		{"media/", true, 100},
		{"small.txt", false, 10},
	}
	if len(rep.Children) != len(wantOrder) {
		t.Fatalf("children = %d, want %d (%+v)", len(rep.Children), len(wantOrder), rep.Children)
	}
	for i, w := range wantOrder {
		c := rep.Children[i]
		if c.Name != w.name || c.IsDir != w.isDir || c.Size != w.size {
			t.Errorf("child[%d] = {%q dir=%v %d}, want {%q dir=%v %d}", i, c.Name, c.IsDir, c.Size, w.name, w.isDir, w.size)
		}
	}
	if !rep.Complete {
		t.Error("Complete = false, want true for a finished scan")
	}
}

// TestUsageOfEmptyPrefix: an empty prefix yields a zero report, not an error (FR-012).
func TestUsageOfEmptyPrefix(t *testing.T) {
	f := NewFake()
	f.Seed("b") // no objects
	rep, err := f.UsageOf(context.Background(), "b", "nothing/", nil)
	if err != nil {
		t.Fatalf("UsageOf(empty) error: %v", err)
	}
	if rep.TotalCount != 0 || rep.TotalSize != 0 || len(rep.Children) != 0 {
		t.Errorf("empty report = %d/%d/%d children, want all zero", rep.TotalCount, rep.TotalSize, len(rep.Children))
	}
	if !rep.Complete {
		t.Error("empty scan Complete = false, want true")
	}
}

// TestUsageOfNestedPrefix: analyzing under a sub-prefix only counts what's beneath it
// and excludes the prefix placeholder object (005 data-model).
func TestUsageOfNestedPrefix(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "logs/", FakeObject{})                              // folder placeholder
	f.SeedObject("b", "logs/app.log", FakeObject{Data: make([]byte, 40)}) // direct object
	f.SeedObject("b", "logs/2026/x.log", FakeObject{Data: make([]byte, 60)})
	f.SeedObject("b", "other/y.bin", FakeObject{Data: make([]byte, 99)})

	rep, err := f.UsageOf(context.Background(), "b", "logs/", nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if rep.TotalCount != 2 || rep.TotalSize != 100 {
		t.Errorf("nested totals = %d/%d, want 2/100 (placeholder + other excluded)", rep.TotalCount, rep.TotalSize)
	}
}

// TestUsageOfCancelled: a cancelled context returns the partial report with
// Complete=false and ctx.Err() (005 FR-011).
func TestUsageOfCancelled(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "a", FakeObject{Data: make([]byte, 1)})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep, err := f.UsageOf(ctx, "b", "", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if rep.Complete {
		t.Error("cancelled scan Complete = true, want false")
	}
}
