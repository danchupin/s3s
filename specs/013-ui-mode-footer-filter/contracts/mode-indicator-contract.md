# Contract: single universal read/write mode indicator

**Feature**: 013 | Governs FR-001..006, SC-001/002/008 | Constitution V, VI, VII

## Rule

Exactly ONE read/write mode indicator per browse screen — the border-mounted chip (`modeChip()`,
app.go:1294). The older footer/identity `[RW]/[RO]` tag is removed everywhere.

## MUST

- **M1**: `modeChip()` is mounted on every BROWSE box's top border: bucket list, object level, opened object.
  - bucket list / tree boxes: already mounted (app.go:1256/1270/1286).
  - opened object: `modeObject` render (app.go:1178) switches `boxView` → `boxViewChip` with `m.modeChip()`.
- **M2**: The chip reads `WRITE` (armed, `writeBadgeStyle`) or `RO` (read-only, `roStyle`); state carried by
  text → NO_COLOR-safe (FR-006).
- **M3**: The `[RW]/[RO]` tag is REMOVED from `footerIdentityCompact` (styles.go:512-524). The identity row
  becomes `● ctx · cluster`. Callers updated: `footerBlock` (app.go:1382), `infoColumn` (commandbar.go:185),
  `collapsedBarView` (commandbar.go:254).
- **M4**: In every browse mode, mode state remains visible after M3 (FR-003) — guaranteed by M1.

## MUST NOT (exemptions — these STAY)

- **M5**: MUST NOT remove the modal write badges (`writeBadge`): confirm popup (confirmview.go:36), inline
  typed-confirm (confirmview.go:69), arm popup (writemode.go:56). They are the safety redundancy (FR-005),
  shown on surfaces where the chip is not.
- **M6**: MUST NOT remove the help-overlay badge prefix (app.go:1130) — the help screen has no box/chip.
- **M7**: MUST NOT show the mode in two places on the same browse screen (FR-001 / SC-001).

## Scope

Overlay/menu modes (`contextSwitch`/`usage`/`connections`/`connForm`/`addBucket`/`filebrowser`) are not
browse boxes and remain chip-less (write state cannot change inside them; return to a browse view shows it).

## Tests

- Chip present on the object-view border in RO (`RO`) and armed (`WRITE`) — fails today.
- Footer no longer contains `[RW]`/`[RO]` on a chip-bearing view.
- Migrate existing `[RW]/[RO]` footer asserts to the chip (operation_test.go, visual_test.go,
  writemode_test.go, spec012_test.go) — keep the help branch on `writeBadge`.
- SC-001: no browse view's `viewOf` contains the mode indicator twice.
