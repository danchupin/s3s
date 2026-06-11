# Contract: Health Card View (US4)

## Entry / exit

- `H` (`keys.Health`) or `:health` from a bucket or prefix/level focus (same target resolution
  as `focusedUsageTarget`, `internal/ui/analyze.go:43-62`; object focus → no-op + footer note).
- New `modeHealth`; Esc returns via `prevMode` to the exact prior position (selection,
  scroll, zone preserved — nothing about browse state is mutated by the card).
- Entering NEVER starts a full scan (FR-003). Data resolution order: cached exact → cached
  partial (labelled) → start a BUDGETED scan if nothing cached. The `A full scan` affordance
  is visible in the card whenever data is absent/partial.

## Layout (top→bottom), 130×24 reference

```
┌ Health: bucket/prefix ────────────────────────────┐
│ totals line: N objects · X GiB [≥ partial]        │
│ Age        ▇▇▇░░  42%  <1d      …  (6 rows)       │
│ Size       ▇░░░░  12%  <128KiB  …  (6 rows)       │
│ Classes    STANDARD 90% · COLD 10%  (≤3 rows)     │
│ Incomplete uploads: 14 · ≥1.2 GiB (12 of 14 sized)│
│            oldest 41d  [state: …/denied/unsup.]   │
│ ⚠ 61% of objects < 128 KiB — index pressure       │
└───────────────────────────────────────────────────┘
footer (hints incl. A full scan · Y copy/export · Esc back)
```

- Histogram rows reuse `usageBar` + shared table styles; six rows per histogram, fixed
  boundaries (data-model §2).
- Height budget: card body composes under the same `View()` arithmetic as other modes; when
  rows < needed (e.g. 24-row terminal with warnings), sections collapse in priority order
  (classes → size → age) into one-line summaries with `i` reveal recovery — the footer is
  NEVER scrolled off (constitution VI).
- NO_COLOR: bars are glyph-based already; states carry text labels.

## Totals-line states

| Data | Render |
|---|---|
| exact report | plain totals |
| partial report | `≥` totals + `partial — A to full scan` |
| scan in flight (card-started budgeted or running full) | `scanning… N objects · X GiB so far` (running totals from the usage progress stream) |

## MPU block states

| State | Render |
|---|---|
| loading | `incomplete uploads: loading…` (probe under `healthGen`, cancellable) |
| none (honest zero) | `incomplete uploads: none` |
| present | count · `[≥]size (N of M sized)` · oldest age |
| denied | `incomplete uploads: denied` (warn role) |
| unsupported | `incomplete uploads: unsupported` (dim) |
| error | footer error line; block shows `unavailable` |

The probe result is cached per `(context,bucket,prefix)` and invalidated with `usageResults`
(refresh `r` / context switch).

## Warning lines

- Small-object: fires per data-model §4 (`share(<KiB) > Share`); text names both numbers:
  `⚠ 61% of objects < 128 KiB — small-object index pressure` (warn role + `⚠` text marker).
- Partial data: every figure derived from a `Bounded`/cancelled report renders with `≥` and
  the card header carries `partial — A to full scan`.
- Empty bucket: zeros render plainly; no warnings (exact zero).

## Test obligations (RED first)

1. Seeded mix (Fake distributions) → card shows exact bucket rows; Σ percentages sane.
2. Partial report → every figure `≥`-marked; header `partial`; affordance present.
3. MPU: each of the 6 states renders per table (incl. denied ≠ none — never zero-as-clean).
4. Small-object warning: fires at >50% below threshold, silent at ≤50%; numbers in text.
5. 130×24 sweep: all seeded values present or revealable; footer last line intact; collapse
   order honored at 24 rows.
6. Esc restores exact prior selection/zone/scroll; entering from object focus is a no-op.
7. `H` with nothing cached starts a budgeted (never full) scan — Fake asserts page cap.
