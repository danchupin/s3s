//go:build integration

// Integration coverage for the 005 read methods (GetObject full-stream, UsageOf
// recursive aggregation) against a real MinIO (Constitution IV).
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
)

// TestIntegrationGetObjectFull downloads a multi-megabyte object and verifies the
// full body length + content, and that cancellation surfaces ctx.Err() (005 FR-001,
// storage-read-ops-contract C1).
func TestIntegrationGetObjectFull(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "dlbucket")

	// ~3 MiB body (larger than the 5 MiB preview cap is unnecessary; this proves the
	// full stream is returned, not a bounded slice).
	body := bytes.Repeat([]byte("s3s-download-payload-"), 150_000) // ~3.15 MiB
	b.put(t, "dlbucket", "big/data.bin", string(body), "application/octet-stream")

	rc, err := b.store.GetObject(context.Background(), "dlbucket", "big/data.bin")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(got) != len(body) || !bytes.Equal(got, body) {
		t.Errorf("GetObject body len=%d, want %d (content match=%v)", len(got), len(body), bytes.Equal(got, body))
	}

	// Cancellation mid-flight surfaces ctx.Err().
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.store.GetObject(ctx, "dlbucket", "big/data.bin"); err == nil {
		t.Error("GetObject with cancelled ctx returned nil error")
	}
}

// TestIntegrationUsageOfPagination seeds more than one ListObjectsV2 page worth of
// keys across nested prefixes and verifies totals + ranked children survive the
// pagination boundary; a cancelled scan returns Complete=false (005 FR-008/FR-011).
func TestIntegrationUsageOfPagination(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "usage")

	// 1200 keys (> the 1000 default page size) split across two sub-prefixes, plus a
	// couple of direct objects, so the immediate-child breakdown crosses a page edge.
	const n = 1200
	for i := 0; i < n; i++ {
		sub := "alpha"
		if i%2 == 0 {
			sub = "beta"
		}
		b.put(t, "usage", fmt.Sprintf("%s/deep/%04d.txt", sub, i), "x", "")
	}
	b.put(t, "usage", "top.bin", "0123456789", "") // 10 bytes, direct object

	rep, err := b.store.UsageOf(context.Background(), "usage", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf: %v", err)
	}
	if rep.TotalCount != n+1 {
		t.Errorf("TotalCount = %d, want %d (pagination lost keys)", rep.TotalCount, n+1)
	}
	// 3 immediate children: alpha/, beta/, top.bin.
	if len(rep.Children) != 3 {
		t.Fatalf("children = %d, want 3 (%+v)", len(rep.Children), rep.Children)
	}
	for _, c := range rep.Children {
		if c.IsDir && c.Count != n/2 {
			t.Errorf("child %q count = %d, want %d", c.Name, c.Count, n/2)
		}
	}
	if !rep.Complete {
		t.Error("Complete = false on a finished scan")
	}

	// Cancelled scan: partial + Complete=false.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	crep, cerr := b.store.UsageOf(ctx, "usage", "", 0, nil)
	if cerr == nil {
		t.Error("cancelled UsageOf returned nil error")
	}
	if crep.Complete {
		t.Error("cancelled UsageOf Complete = true, want false")
	}
}

// TestIntegrationUsageOfBudgetCap seeds more keys than the cap and verifies the real
// client stops within one ListObjectsV2 page of maxObjects, reporting an honest lower
// bound — and that the uncapped scan of the same bucket is exact (017 US1/FR-001, T017).
func TestIntegrationUsageOfBudgetCap(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "usagecap")

	const n = 1200 // > one 1000-key page
	for i := 0; i < n; i++ {
		b.put(t, "usagecap", fmt.Sprintf("k/%04d.txt", i), "x", "")
	}

	rep, err := b.store.UsageOf(context.Background(), "usagecap", "", 1000, nil)
	if err != nil {
		t.Fatalf("UsageOf(capped): %v", err)
	}
	if !rep.Bounded || rep.Complete {
		t.Errorf("capped scan: Bounded=%v Complete=%v, want Bounded=true Complete=false", rep.Bounded, rep.Complete)
	}
	// Stop within one page of the cap: the first 1000-key page reaches the cap.
	if rep.TotalCount != 1000 {
		t.Errorf("capped TotalCount = %d, want exactly one page (1000)", rep.TotalCount)
	}

	full, err := b.store.UsageOf(context.Background(), "usagecap", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf(full): %v", err)
	}
	if !full.Complete || full.Bounded || full.TotalCount != n {
		t.Errorf("full scan = %+v, want exact %d objects", full, n)
	}
}
