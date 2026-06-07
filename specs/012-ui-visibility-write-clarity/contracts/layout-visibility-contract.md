# Contract: Layout visibility (col-grow, active-row wrap, breadcrumb, mode chip)

**Stories**: US1, US3, US2(chip). FR-001, FR-003, FR-010..013, FR-022, FR-038. Source: `app.go`
`listWithPane`, `styles.go` `renderTable`/`boxView`/`boxViewWith`/`windowBounds`.

## Bucket-column auto-grow (FR-001)

- In `listWithPane`, size the buckets column to fit the longest **visible** bucket name, bounded by a max
  AND by measured objects-zone slack (`slack = objW − objMinW`; allocate ≤ ~half), per tier. No slack →
  keep the base `[24,40]` clamp.
- Measure the visible window only (lazy-load model). Never shrink the objects zone below a legible minimum.

## Active-row wrap (FR-003)

- Only the SELECTED row wraps across multiple display lines when a cell overflows; other rows truncate.
- Stays within the existing `rows` budget; `windowBounds` remains stateless (one data row → N display
  lines does not change the window).
- Pre-measure wrapped height; if it would exceed the body budget, fall back to truncation + the reveal
  popup. `boxView` `minRows` hard-cap is never exceeded → footer never scrolls off.
- Active-row wrap only in Dual/Full tiers; Single tier keeps single-line truncation.

## Breadcrumb (FR-010..013)

- Full path `ctx → bucket → prefix-chain` (+ `(search: …)` after elision).
- Render: objects-zone center label (Dual/Full) / box title (Single).
- `elideMiddle(path, maxW)` keeps bucket + deepest segment, drops middle, falls back to end-truncation.
- Respect the `boxViewWith` center-width budget; full path revealable via the reveal popup.

## Mode chip on box border (FR-038)

- New right-aligned label slot in `boxViewWith` (mirror of the left slot, occupying the right border dashes).
- Text `WRITE` (accent `writeBadgeStyle`/`warnStyle`) when armed; `RO` (neutral `roStyle`/`dimCellStyle`)
  read-only. Rendered on the PRIMARY list box only — the leftmost bucket-list box in multi-zone tiers, the
  sole box in the single tier (one fixed location, clarified 2026-06-07). Active-row wrap is automatic when
  the highlighted value is truncated (no keypress); reveal popup is the overflow/copy fallback.
- Width precedence on a narrow border: drop the centered breadcrumb before the mode chip (chip is
  safety-critical). NO_COLOR-safe (text).

## Invariant (all of the above)

Footer + command bar fully visible at every supported width and tier — `boxView` body hard-capped to
`minRows` (FR-022 / SC-008). Tested at e.g. 60×10, 120×8, 140×12.

## Tests

`TestBucketsColumnGrowsWithSlack`, `TestBucketsColumnCappedAtMax`, `TestActiveRowWrapsNoFooterClip`,
`TestActiveRowFallsBackToRevealWhenTooTall`, `TestBreadcrumbFullPath`, `TestBreadcrumbMiddleElision`,
`TestModeChipWriteAccent` / `TestModeChipReadonlyNeutral`, `TestModeChipNoColor`,
`TestFooterVisibleAcrossTiers`.
