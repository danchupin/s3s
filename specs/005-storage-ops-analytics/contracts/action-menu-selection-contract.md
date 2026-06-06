# Contract: Action menu, selection, bulk, download, analyze, sort (US1–US4)

Extends the 004 action-menu model. New operations are reachable through the contextual menu
(`a`); selection/write gating mirrors the existing single-object rules.

## C1 — New menu items (FR-023)

`menuItemsFor()` gains, gated by mode / selection / capability:

| Item | Availability |
|------|--------------|
| download | object selected, OR ≥1 marked object → bulk download. **Read** (always, incl. RO). |
| analyze | bucket selected (bucket list) or folder/level selected → `du`. **Read** (always). |
| bulk delete | ≥1 marked object AND `writable`. Hidden in RO. |
| bulk copy | ≥1 marked object AND `writable`. Hidden in RO. |

Single-object delete/copy/move and recursive-delete remain as in 004. Recursive delete is **not**
reachable via multi-select (FR-016). Refresh stays last.

Download/analyze/bulk are **menu-only** (no top-level keys) to preserve the 004 footer declutter
(FR-023); only mark / sort / write-toggle are dedicated keys.

**Tests**: RO menu offers download/analyze but no bulk delete/copy; armed menu offers them when a
selection exists; folder selection offers analyze + recursive-delete (unchanged), not "mark".

## C2 — Selection (US3 / FR-014/019)

- `Mark` (space) toggles the current row in `sel` (objects only; folders ignored).
- Header/footer shows `<n> selected · <combined size>`; marked rows show a marker glyph.
- `sel` is cleared on every navigation: enter level, back/up, bucket entry, context switch.

**Tests**: marking an object updates count/size; marking a folder is a no-op; navigating away
clears `sel`; the marker renders for marked rows.

## C3 — Bulk execution (US3 / FR-015/017/018/015a)

- A `bulk` op iterates marked keys applying the per-item backend call; it **continues past
  failures** and ends with a truthful `succeeded/failed` summary.
- **Download**: recreates each key's hierarchy as local subdirs under the destination (FR-015a).
  Read — available RO.
- **Delete**: typed confirmation on the **count**; each delete logged before execution (FR-017);
  requires `writable`.
- **Copy**: destination-prefix entry; requires `writable`.

**Tests**: bulk download of N keys writes N files in mirrored subdirs with a per-item summary; one
failing item does not abort the rest; bulk delete requires the typed count + logs each op; bulk
delete/copy refused while RO.

## C4 — Download (US1 / FR-003/004/005/006/007)

- Object selected → download streams to `dest+".partial"`, renamed on success; cancel/failure
  removes the partial.
- Overwrite of an existing **local** file requires a simple confirm (FR-005).
- Live progress (bytes/%) during `phaseRunning`; Esc cancels (FR-003/004).
- Default destination = working dir (configurable); override via the existing file browser (FR-007).
- Uses a non-`Mutator` dispatch path (download is a read; no `--write`).

**Tests**: download writes a byte-identical file; existing local file prompts before overwrite;
cancel leaves no `.partial`; runs in a RO context.

## C5 — Analyze view (US2 / FR-008..013)

- `modeUsage` renders totals + ranked children (size + % of parent), with live progress during the
  scan and Esc-to-cancel.
- Enter on a child sub-prefix drills down (re-analyze under it); back returns (FR-013).
- Sizes human-readable; empty prefix shows zero, not an error.

**Tests**: totals + ranking match seeded data; drill-down re-analyzes the child; cancel shows
partial totals; empty prefix shows zero.

## C6 — Sort (US4 / FR-020/021)

- `Sort` key cycles column (name→size→modified); a direction toggle flips asc/desc.
- Applied at render time to a copy of the level; the active column+direction is visible.
- Session-persistent across navigation; composes with active search/filter; dirs ordered
  consistently when sorting by size/modified.

**Tests**: size-desc puts the largest object first; toggling reverses; sort persists into a newly
entered level; sort + active search both apply.
