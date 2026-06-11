package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// seedN seeds n zero-byte objects with deterministic keys under prefix.
func seedN(f *Fake, bucket, prefix string, n int) {
	for i := 0; i < n; i++ {
		f.SeedObject(bucket, fmt.Sprintf("%sobj-%06d", prefix, i), FakeObject{Data: []byte("x")})
	}
}

// TestUsageOfBudgetCap: enumeration stops within one listing page of reaching
// maxObjects and reports Bounded=true / Complete=false (017 US1, FR-001;
// contracts/budgeted-usage-scan.md obligation 1).
func TestUsageOfBudgetCap(t *testing.T) {
	f := NewFake()
	seedN(f, "b", "", 2500)

	rep, err := f.UsageOf(context.Background(), "b", "", 1500, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if !rep.Bounded {
		t.Error("Bounded = false, want true for a capped scan")
	}
	if rep.Complete {
		t.Error("Complete = true, want false for a capped scan")
	}
	// Cap reached during page 2 (1000 + 1000 ≥ 1500): exactly 2 pages listed,
	// totals reflect everything actually enumerated.
	if rep.TotalCount != 2000 {
		t.Errorf("TotalCount = %d, want 2000 (two full pages)", rep.TotalCount)
	}
	if f.UsagePages != 2 {
		t.Errorf("UsagePages = %d, want 2 (stop within one page of the cap)", f.UsagePages)
	}
}

// TestUsageOfBudgetBoundaryExact: a target whose object count equals the budget is an
// EXACT result — no lower-bound marker (spec edge case; budgeted-usage-scan.md).
func TestUsageOfBudgetBoundaryExact(t *testing.T) {
	f := NewFake()
	seedN(f, "b", "", 1000)

	rep, err := f.UsageOf(context.Background(), "b", "", 1000, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if rep.Bounded {
		t.Error("Bounded = true, want false when count == budget (exact)")
	}
	if !rep.Complete {
		t.Error("Complete = false, want true when count == budget (exact)")
	}
	if rep.TotalCount != 1000 {
		t.Errorf("TotalCount = %d, want 1000", rep.TotalCount)
	}
}

// TestUsageOfBudgetPlusOne: one object over the budget caps after the first page.
func TestUsageOfBudgetPlusOne(t *testing.T) {
	f := NewFake()
	seedN(f, "b", "", 1001)

	rep, err := f.UsageOf(context.Background(), "b", "", 1000, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if !rep.Bounded || rep.Complete {
		t.Errorf("got Bounded=%v Complete=%v, want Bounded=true Complete=false", rep.Bounded, rep.Complete)
	}
	if rep.TotalCount != 1000 {
		t.Errorf("TotalCount = %d, want 1000 (lower bound after one page)", rep.TotalCount)
	}
	if f.UsagePages != 1 {
		t.Errorf("UsagePages = %d, want 1", f.UsagePages)
	}
}

// TestUsageOfUnlimited: maxObjects == 0 keeps the pre-017 full-scan semantics.
func TestUsageOfUnlimited(t *testing.T) {
	f := NewFake()
	seedN(f, "b", "", 2500)

	rep, err := f.UsageOf(context.Background(), "b", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf error: %v", err)
	}
	if rep.Bounded || !rep.Complete {
		t.Errorf("got Bounded=%v Complete=%v, want Bounded=false Complete=true", rep.Bounded, rep.Complete)
	}
	if rep.TotalCount != 2500 {
		t.Errorf("TotalCount = %d, want 2500", rep.TotalCount)
	}
}

// TestUsageOfCancelledNotBounded: a cancelled scan keeps Complete=false but is NOT
// Bounded (the cap was never the reason it stopped) and returns ctx.Err() — the
// invalid state Complete==true && Bounded==true never occurs (data-model §2).
func TestUsageOfCancelledNotBounded(t *testing.T) {
	f := NewFake()
	seedN(f, "b", "", 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rep, err := f.UsageOf(ctx, "b", "", 0, nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if rep.Complete {
		t.Error("cancelled scan Complete = true, want false")
	}
	if rep.Bounded {
		t.Error("cancelled scan Bounded = true, want false")
	}
}
