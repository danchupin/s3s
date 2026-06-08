# Contract: border-mounted chip slots (`boxViewWith`)

**Feature**: 013 | Governs FR-001..003, FR-007..009, FR-012, FR-016 | Constitution VI, VII

## Component

`boxViewWith(left, center, body, width, minRows, titleSt, …chips)` — the single function that physically
draws chips into a box's top border (styles.go:334-406). Wrappers `boxViewChip` / `boxViewFocusChip` (and the
new objects-pane variant) delegate here. Plain `boxView` / `boxViewFocus` pass no chips.

## Border composition (top line, left → right)

```
╭─ <left/breadcrumb> ──── «center» ──── ‹filterChip›  ‹modeChip› ╮
```

- `left` = title/breadcrumb (existing).
- `center` = selection label (existing).
- **`filterChip`** = inboard chip (NEW slot) — the applied-filter chip, or empty.
- **`modeChip`** = right-most chip (existing slot) — the RO/WRITE chip, or empty.
- Chips are already-styled strings; each is rendered as ` <chip> ` inset, the right-most immediately before
  `╮` (existing mechanic, styles.go:361-366/397).

## MUST

- **C1**: Support TWO independent chips. Either may be empty (`""`) → that slot emits nothing.
- **C2**: Right-most chip is the **mode chip**; the inboard chip is the **filter chip**.
- **C3**: **Degrade order** under insufficient border width (extends styles.go:375-385): drop `center` first,
  then `filterChip`, then `modeChip` LAST. The mode chip is safety-critical and survives longest.
- **C4**: Width accounting (`avail`, dash split) MUST subtract the sum of both rendered chip widths so the
  border never overflows `width` (no wrap → footer never scrolls, FR-016).
- **C5**: Chip text is NOT elided by `boxViewWith`; an over-long chip is dropped whole (C3). Callers MUST cap
  chip text themselves before the call (see applied-filter-contract C-term).
- **C6**: No new hue introduced here; chips arrive pre-styled by the caller (VII).

## MUST NOT

- **C7**: MUST NOT drop the mode chip before the filter chip.
- **C8**: MUST NOT add body rows or a newline (chips are border-only → zero body-height cost, FR-016).

## Wrappers

- `boxViewChip(left, center, body, w, rows, …chips)` — title style.
- `boxViewFocusChip(left, center, body, w, rows, active, …chips)` — focus title style; used by buckets AND now
  objects pane (objects pane previously used `boxViewFocus`, no chip).

## Tests (white-box, failing-first)

- Two-chip render: a box given both a filter chip and a mode chip shows both on the first border line, mode
  chip right-most.
- Degrade: at a width where only one chip fits, the **mode chip** survives and the **filter chip** is the one
  dropped (assert mode text present, filter text absent); at a width where neither fits, center already gone
  and mode chip dropped last.
- No-overflow: first border line width ≤ `width` at every tier (feeds `assertWidthSweep`).
