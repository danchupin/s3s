# Contract: footer / command-bar spacing

**Feature**: 013 | Governs FR-014..017, SC-005/006 | Constitution VI, VII

## Rule

Widen the gaps between footer/command-bar elements so adjacent elements no longer merge — consistently across
the wide (3-column) and collapsed (drop-trailing) paths — WITHOUT any footer line wrapping or scrolling off at
any width tier.

## Single-sourced gaps

- **S1 (separator token)**: one package-level token, ` · ` (w3) → `  ·  ` (w5). Replaces literals at
  commandbar.go:63 (`barGlobals`), :262 (collapsed globals), :276 (`fitEntries`), styles.go:469/472
  (`renderHintRow`), :518/521 (`footerIdentityCompact`), pane.go:54/71 (details hints).
- **S2 (key↔label gap)**: `entryStyled` (commandbar.go:159) 1 → 2 spaces.
- **S3 (inter-column gap)**: `commandBarView` (commandbar.go:179) 2 → 3 spaces, via a single `colGap` const.

## Derived math (the ONE non-self-measuring site)

- **S4**: `natural := Width(info)+Width(read)+Width(write)+2*len(colGap)` (commandbar.go:175) and
  `JoinHorizontal(Top, info, colGap, read, colGap, write)` (commandbar.go:179). The `+4` magic constant is
  REPLACED by `2*len(colGap)` so the gap literal and the guard can never drift (eliminates the double-count
  that would let a wide bar render past `w`).

## Invariants

- **S5 (self-accounting)**: every other fitter — `fitEntries` (commandbar.go:272-308), `renderHintRow`
  (styles.go:446-477), `footerIdentityCompact` cluster-append (styles.go:520) — re-measures its separator via
  `lipgloss.Width`. Widening S1 needs NO math change there; the drop loop simply drops one more entry to fit.
- **S6 (horizontal-only)**: widening inserts NO newline → `footerH` (app.go:1139) and the body budget
  (app.go:1142) are unchanged; `boxViewWith` `minRows` cap (styles.go:347) still holds.
- **S7 (no wrap)**: every footer line stays ≤ `w` at every tier (the sole failure mode is a line exceeding
  `w` → visual wrap → uncounted row → footer scroll; S4+S5 prevent it).

## MUST NOT

- **S8**: MUST NOT widen S3 without updating S4 in lockstep (use the derived form).
- **S9**: MUST NOT introduce a new hue or per-surface style for spacing (VII) — spacing is whitespace only.

## Tests

- `assertWidthSweep(treeApp, 40, 200, 9)` (footer_test.go:150) stays green: every footer line ≤ w, ≤ 9 rows.
- `TestFooterFitsWidthAndShowsHints` (w=60), narrow-drop (w=30), `TestFooterVisibleAcrossTiers`,
  `TestFooterVisibleMinHeight` stay green.
- NEW: `commandBarView(140)` (on `stripANSI`) contains the widened separator / `colGap`; a boundary width
  where old-natural fit but new-natural does not now renders the collapsed 3-row bar (proves S4 updated).
