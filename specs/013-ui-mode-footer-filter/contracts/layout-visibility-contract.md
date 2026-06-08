# Contract: layout visibility invariant (carried over from 012)

**Feature**: 013 | Governs FR-012, FR-016 | Constitution VI

## Rule

Every footer line and the command/hint bar stay fully visible at every supported terminal width AND height
tier. The three 013 changes (universal chip, applied-filter chip, wider spacing) MUST preserve this.

## MUST

- **L1**: Chips ride the box TOP BORDER → cost zero body rows. The body budget `rows = height - footerH - 2`
  (app.go:1142) and the `boxViewWith` `minRows` hard-cap (styles.go:347-349) are unchanged by either chip.
- **L2**: `footerH = strings.Count(footer,"\n")+1` (app.go:1139) is unchanged by spacing (horizontal-only,
  footer-spacing-contract S6) and by chips (border-only).
- **L3**: No footer line may exceed `w` (would wrap → uncounted row → footer scrolls off). Guaranteed by the
  self-measuring fitters + the derived inter-column constant (footer-spacing-contract S4/S5/S7).
- **L4**: A filter term too long for its chip is dropped whole by `boxViewWith` (border-chip-contract C5), so
  it MUST be capped with an explicit `…`; the full committed term remains recoverable by re-opening the filter
  input (`/`, which pre-fills it). The filter term is the user's own query — recoverable by re-opening — so no
  resource identifier is permanently hidden with no way to reveal it (constitution VI).
- **L5**: Under two chips, the safety-critical mode chip survives narrow widths; the filter chip yields first
  (border-chip-contract C3).

## Tiers (unchanged)

Single ≤99 / Dual 100–129 / Full ≥130 (tier_test.go); `commandBarView` columns only ≥ `blockColMin=100`,
else the collapsed 3-row bar.

## Tests

- `assertWidthSweep(treeApp, 40, 200, 9)` and `TestFooterVisibleMinHeight` (height=8, `quit` visible) stay
  green with both chips mounted and spacing widened.
- At a narrow width, the object-view + tree boxes still show the mode chip (mode chip never dropped first).
