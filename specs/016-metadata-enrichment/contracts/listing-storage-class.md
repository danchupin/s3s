# Contract: Listing Storage-Class Marker (US5 — FR-015)

**Surface**: `treeView` data generation (`tree.go:224-240`).

## Inputs

- Tree entries; each object carries `e.obj.StorageClass` (from `ListLevel` mapping
  `o.StorageClass`, on `ObjectRef`, `storage.go:179-184`).
- Columns `{"name",0},{"type",5},{"size",11},{"modified",17}` (`tree.go:224`).

## Rendered shape — fixed (lossy) marker map within the 5-char `type` cell

Real classes exceed 5 chars (GLACIER=7, DEEP_ARCHIVE=12, INTELLIGENT_TIERING=19), so the
cell is deliberately lossy with a CLOSED, documented token set; the FULL class is
recoverable via `keys.Reveal` (`i`) on the row:

| StorageClass | marker | | StorageClass | marker |
|---|---|---|---|---|
| STANDARD / "" | `obj` (no marker) | | STANDARD_IA | `ia` |
| GLACIER | `glac` | | ONEZONE_IA | `1zia` |
| GLACIER_IR | `gir` | | REDUCED_REDUNDANCY | `rr` |
| DEEP_ARCHIVE | `arch` | | any other non-standard | `cls*` |
| INTELLIGENT_TIERING | `int` | | (directory) | `dir` (never marked) |

## Invariants

- I1 (FR-015): non-standard class visible; STANDARD adds no noise.
- I2 (VI/SC-005): width math holds at 80 columns — the flexible `name` column never drops
  below the legibility floor; no horizontal overflow. Verified at widths 80/120/160.
- I3 (VI — "fully visible OR revealable"): the cell is lossy by design, so the marker is
  TIED to the reveal affordance — `i` on a non-standard row shows the full class string;
  asserted in the US5 test.
- I4: the marker reuses the existing column + `renderTable`/`renderTableActive`; no new
  column steals `name` width, no new palette role.

## Testable assertions

- A level with one STANDARD + one GLACIER object: the GLACIER row's `type` cell shows
  `glac`; the STANDARD row's shows `obj`; column widths align at 80 cols.
- Reveal recovery: `i` on the GLACIER row shows `GLACIER` (the full, lossless class).
