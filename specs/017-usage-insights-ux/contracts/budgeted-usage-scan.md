# Contract: Budgeted Usage Scan (US1)

Governs the ambient (dwell) scan, the explicit full scan, and partial-result caching.
Supersedes the 016 inline-usage contract's unbounded behaviour; everything not stated here
(dwell gate, generation guard, channel drain, single in-flight scan) carries over unchanged
from `specs/016-metadata-enrichment/contracts/inline-usage.md`.

## Ambient (dwell) scan

- Trigger: unchanged — `armUsageScan` dwell tick (`paneDebounce` 180 ms) on an uncached
  bucket/dir/level target at Dual/Full tiers.
- Work bound: `UsageOf(ctx, bucket, prefix, budget, onProgress)` with `budget` = resolved
  `usageScanBudget` (default 20 000). The enumeration MUST stop within one listing page of
  reaching `budget` enumerated objects.
- `budget == 0` ⇒ `armUsageScan` arms NOTHING (no tick, no scan). Cached results still render.
- Result: `Bounded=true, Complete=false` report when capped; exact report when the target was
  smaller than the budget (`Bounded=false, Complete=true` — boundary case: count == budget is
  exact, no marker).
- A partial cache entry IS a hit for the ambient path: dwell never rescans a partial-cached
  target (same budget ⇒ same bound). Only the explicit full scan upgrades it.

## Explicit full scan

- Triggers: `A` (`keys.FullScan`) and `:scan` — ONE dispatcher (`startFullScan`). No other
  code path may start an unbounded scan: NOT `a`/`:detail` (breakdown shows
  collected/budgeted data only), NOT `H`/`:health` (card renders partial with affordance).
- Runs under `usageGen`/`usageCancel` exactly like today's scan (one in-flight enumeration per
  app, cancel-on-navigate + cancel-on-`beginLoad`).
- Progress: running totals stream to the pane/card (existing `usageProgressMsg` path);
  cancellable via navigation or Esc semantics already in place.

## Partial caching (inverts 016 behaviour)

- `onUsageDone` MUST cache any report carrying data: exact, `Bounded`, or cancelled
  (`Complete=false`). Today's discard branch (`internal/ui/analyze.go:185-187` gen-mismatch
  drop for superseded scans stays; the cancelled-report discard goes).
- Superseded-generation results: still dropped UNLESS the report carries data for the key it
  was started for — the report is cached under `usageScanKey` even when the user has moved
  on (the data is valid for that target; the VIEW gate stays generation-guarded).
- Exact entries are never overwritten by partial ones; full-scan completion overwrites the
  partial for its key.
- Invalidation: unchanged — manual refresh (`r`) on the surface, context switch clears all.

## Pane rendering

- Partial: totals prefixed `≥`, suffix `partial`, plus affordance line using the shared
  keyHint vocabulary: `A full scan`. Exact: unchanged rendering.
- The affordance line appears whenever the focused target's cache entry is absent or partial
  AND the pane is visible.
- NO_COLOR: `≥`/`partial` are text markers, not colour-only.

## Test obligations (RED first)

1. Fake seeded budget+1 objects → dwell → `Bounded=true`, enumeration request count ≤
   ⌈(budget+999)/1000⌉ pages (Fake counts `ListLevel`-equivalent calls).
2. Fake seeded budget objects exactly → exact result, no `≥` in `View().Content`.
3. Cancel mid-full-scan → cache contains lower bound; revisit renders `≥` totals instantly.
4. `budget=0` → no scan armed on dwell; `A` still scans.
5. `a` on uncached target → budgeted scan only (Fake asserts max pages), affordance present.
6. Full scan after partial → exact replaces `≥` in pane and cache.
7. MinIO integration: cap honored against real pagination (`s3client_integration_test.go`).
