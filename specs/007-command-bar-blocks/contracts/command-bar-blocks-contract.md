# Contract: three-block command bar (US1/US2/US3)

Behavioral contract for the `info · read · write` blocked command bar. Drives white-box UI
tests (`package ui`, assert on `App.View().Content`).

## Structure (FR-001..FR-006)

- The footer in `modeBuckets`/`modeTree` renders **three labelled blocks** in order:
  **info**, **read**, **write**, laid out as side-by-side columns (info left).
- **info** shows: active context, cluster, user, region, s3s version, AND a visible
  add-connection affordance with its key (FR-003/FR-011).
- **read** lists: download, analyze, filter/search, refresh, open.
- **write** lists: delete, copy, move, recursive delete (rm), upload, new folder.
- All three blocks are present at every render unless collapsed by width (FR-016).

## Read-only visibility (FR-007..FR-010) — the headline

- In a read-only context (disarmed OR `readonly:true`) the **write block is still shown**,
  every write entry rendered **dimmed/inactive** — NOT hidden (reverses 006 FR-004).
- In a writable (armed) context the write block renders in its **active (caution) style**,
  visually distinct from dimmed.
- Pressing a dimmed write key mutates nothing and surfaces the read-only nudge ("read-only
  — press `w` to arm"); it never auto-arms.
- Toggling `w` flips the write block dimmed↔active immediately.

## Add-connection affordance (FR-011/FR-012)

- The info block shows a labelled key (e.g. `+ new connection`) whenever a `Connector` is
  wired; activating it opens the existing connection manager (`modeConnections`/form) with
  the 006 add/test/save flow unchanged.

## Color & calm (FR-013..FR-015, SC-004/SC-007)

- Blocks use ONLY existing palette tokens; info/read/write/dimmed/caution are visually
  distinct roles, no new hue.
- The inactive write block uses the faint/dim role uniformly.
- Every color meaning has a redundant text cue (gutter, `(w)`, `^`, `[RW]/[RO]`) so it
  survives `NO_COLOR`.

## Label rule (FR-005a, SC-014)

- Every read/write label: single imperative verb, ≤2 words, lowercase, no articles, no
  trailing punctuation. A table-driven test asserts the whole catalog conforms.

## Responsiveness (FR-016, SC-005)

- `width ≥ blockColMin`: three columns.
- `width < blockColMin`: collapse to a compact wrapped single row that STILL lists the
  write entries (dimmed) and keeps the loud `[RW]/[RO]` badge — never clips the list or
  drops the write block.
- Renders 80×24 → large without clipping the list or badge.

## Preservation (FR-017..FR-020, SC-006)

- Multi-select reflected: marked objects → bulk variants + counts in read/write.
- Every 006-reachable action stays reachable; action flows/confirmations unchanged except
  the dangerous-action surface split (see dangerous-actions-contract).
- The loud write/read-only badge remains present and unmistakable.

## Test checklist

- [ ] read-only render lists all six write actions, dimmed
- [ ] armed render shows write active (caution), distinct from dimmed
- [ ] dimmed write key → no mutation + nudge; no auto-arm
- [ ] `w` flips dimmed↔active
- [ ] info block shows add-connection key; activating opens the manager
- [ ] three columns ≥ blockColMin; collapsed row < blockColMin still shows write (dimmed)
- [ ] all labels pass the FR-005a rule (table-driven)
- [ ] NO_COLOR: active vs inactive write distinguishable by text cue
- [ ] 80×24 renders without clip
