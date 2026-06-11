package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

var mpuT0 = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// TestIncompleteUploadsAggregates: count, oldest age and part-size totals over the
// seeded in-progress uploads, prefix-scoped (017 US4/FR-021).
func TestIncompleteUploadsAggregates(t *testing.T) {
	f := NewFake()
	f.Seed("b", "x")
	f.SeedIncompleteUpload("b", FakeIncompleteUpload{Key: "logs/a.bin", Initiated: mpuT0, PartSizes: []int64{100, 200}})
	f.SeedIncompleteUpload("b", FakeIncompleteUpload{Key: "logs/b.bin", Initiated: mpuT0.Add(48 * time.Hour), PartSizes: []int64{50}})
	f.SeedIncompleteUpload("b", FakeIncompleteUpload{Key: "media/c.bin", Initiated: mpuT0.Add(time.Hour), PartSizes: []int64{1000}})

	got, err := f.ListIncompleteUploads(context.Background(), "b", "logs/")
	if err != nil {
		t.Fatalf("ListIncompleteUploads: %v", err)
	}
	if got.State != ConfigConfigured {
		t.Errorf("State = %q, want configured (uploads present)", got.State)
	}
	if got.Count != 2 || got.SizedCount != 2 || got.TotalSize != 350 {
		t.Errorf("got %+v, want Count=2 SizedCount=2 TotalSize=350", got)
	}
	if !got.OldestInitiated.Equal(mpuT0) {
		t.Errorf("OldestInitiated = %v, want %v", got.OldestInitiated, mpuT0)
	}
}

// TestIncompleteUploadsHonestZero: a successful empty listing is ConfigNone with
// Count=0 — an HONEST zero, never a fallback for failures (017 FR-022).
func TestIncompleteUploadsHonestZero(t *testing.T) {
	f := NewFake()
	f.Seed("b", "x")
	got, err := f.ListIncompleteUploads(context.Background(), "b", "")
	if err != nil {
		t.Fatalf("ListIncompleteUploads: %v", err)
	}
	if got.State != ConfigNone || got.Count != 0 {
		t.Errorf("got %+v, want State=none Count=0", got)
	}
}

// TestIncompleteUploadsSizingCap: only the first 100 uploads are size-enriched —
// beyond that the count/age stay exact but TotalSize is a lower bound (research D6).
func TestIncompleteUploadsSizingCap(t *testing.T) {
	f := NewFake()
	f.Seed("b", "x")
	for i := 0; i < 130; i++ {
		f.SeedIncompleteUpload("b", FakeIncompleteUpload{
			Key: fmt.Sprintf("k-%03d", i), Initiated: mpuT0.Add(time.Duration(i) * time.Minute),
			PartSizes: []int64{10},
		})
	}
	got, err := f.ListIncompleteUploads(context.Background(), "b", "")
	if err != nil {
		t.Fatalf("ListIncompleteUploads: %v", err)
	}
	if got.Count != 130 {
		t.Errorf("Count = %d, want 130 (counting is never capped)", got.Count)
	}
	if got.SizedCount != 100 || got.TotalSize != 1000 {
		t.Errorf("SizedCount=%d TotalSize=%d, want 100/1000 (sizing capped)", got.SizedCount, got.TotalSize)
	}
}

// TestIncompleteUploadsDeniedAndUnsupported: tri-state classification — denied and
// unsupported are states, never rendered as zero (017 FR-022).
func TestIncompleteUploadsDeniedAndUnsupported(t *testing.T) {
	f := NewFake()
	f.Seed("b", "x")
	f.FailListUploads = true
	got, err := f.ListIncompleteUploads(context.Background(), "b", "")
	if err != nil {
		t.Fatalf("denied listing must classify, not error: %v", err)
	}
	if got.State != ConfigDenied {
		t.Errorf("State = %q, want denied", got.State)
	}

	f.FailListUploads = false
	f.UnsupportedListUploads = true
	got, err = f.ListIncompleteUploads(context.Background(), "b", "")
	if err != nil {
		t.Fatalf("unsupported listing must classify, not error: %v", err)
	}
	if got.State != ConfigUnsupported {
		t.Errorf("State = %q, want unsupported", got.State)
	}
}

// TestIncompleteUploadsCancelled: a cancelled ctx returns what was accumulated with
// ctx.Err() (017 FR-021 — the probe is cancellable).
func TestIncompleteUploadsCancelled(t *testing.T) {
	f := NewFake()
	f.Seed("b", "x")
	f.SeedIncompleteUpload("b", FakeIncompleteUpload{Key: "k", Initiated: mpuT0, PartSizes: []int64{1}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.ListIncompleteUploads(ctx, "b", ""); err == nil {
		t.Error("cancelled ctx must surface ctx.Err()")
	}
}
