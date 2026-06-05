# Contract: Action Menu & Keymap Reduction

Observable behavior of the contextual action menu and the reduced keymap (US1).
Verified white-box on `App.View().Content` and model state. Maps to FR-023..FR-031,
FR-020, SC-004/005/008.

## C1 — Open & dismiss

- Pressing `a` in `modeBuckets` or `modeTree` opens the action menu (`mode==modeActionMenu`).
- The menu renders as an overlay box titled `actions: <selection>` and states how to close
  (Esc).
- Pressing Esc/Back (or the menu key again) closes the menu and restores the previous mode
  with no side effect.
- `a` does nothing in `modeObject`, `modeContextSwitch`, `modeHelp`, while a search input is
  open, or while an operation is mid-flow (`m.op != nil`).

## C2 — Contextual items

The item list is built from `menuCtx` (mode, writable, selKind):

| Context | Items (in order) |
|---------|------------------|
| buckets (any capability) | Refresh |
| tree, read-only | Refresh |
| tree, writable, **object** selected | Delete, Copy, Move / Rename, Upload here, New folder, Refresh |
| tree, writable, **folder** selected | Recursive delete, Upload here, New folder, Refresh |
| tree, writable, empty / nothing selected | Upload here, New folder, Refresh |

Rules:
- **FR-024**: object-only items (Delete, Copy, Move) appear only when an object is selected;
  Recursive delete only when a folder is selected.
- **FR-025**: Refresh is always present and always last; it is the sole refresh entry point
  (top-level `r` is removed). In buckets it reloads buckets; in tree it reloads the level.
- **FR-005/SC-005**: in read-only contexts the menu contains Refresh only — no write item
  appears or is invokable.

## C3 — Navigation & invocation

- The menu is keyboard-navigable: `↑`/`↓` (and vim `k`/`j`) move; `Enter`/`→` invoke; `Esc`/
  `←`/`h` close.
- Invoking an item calls the EXISTING entry point (`startRemoveObject`, `startCopy`,
  `startMove`, `startUpload`, `startCreateFolder`, `startRecursiveDelete`, `refresh`) — the
  subsequent name/destination entry and two-tier (simple vs typed) confirmation are unchanged
  (FR-026/FR-020).
- Nav cues rendered in the menu use arrow glyphs, not vim letters (FR-031).

## C4 — Keymap reduction

- Top-level keys `+`, `d`, `u`, `y`, `m`, `D` (write ops) and `r` (refresh) are NOT routed at
  top level: pressing them in `modeBuckets`/`modeTree` does nothing (FR-028).
- The standalone cancel key `x` is removed (FR-029).
- Cancellation of an in-flight load OR a running op (`op.phase==phaseRunning`) is performed by
  Esc/Back: when loading/running, Esc cancels; otherwise Esc performs back/navigation (FR-029).
- **Modal precedence (FR-029)**: when the action menu (or any modal overlay) is open, Esc
  closes that overlay FIRST and MUST NOT cancel a background load; the load/run-cancel meaning
  of Esc applies only when no modal overlay is open.
- The reduced top-level interactive action set is ≤ 12, counted as distinct logical top-level
  actions (aliases count once; `1-9` counts as one; within-menu/within-mode actions excluded)
  (FR-030/SC-008).

## C5 — Reachability (no capability lost)

- Every operation removed from a top-level key remains reachable: write ops + refresh via the
  menu (≤ 2 keypresses: `a` then select), cancel via Esc (FR-022/SC-004).

## Test obligations (TDD — write first, must FAIL before impl)

1. `a` in writable tree, object selected → menu lists Delete, Copy, Move/Rename, Upload here,
   New folder, Refresh; NOT Recursive delete.
2. `a` with a folder selected → lists Recursive delete (+ Upload/New folder/Refresh); NOT
   Delete/Copy/Move.
3. `a` in read-only tree → Refresh only; no write item present.
4. `a` in bucket list → Refresh only.
5. Choosing Delete (typed-confirm op) from the menu → enters `phaseConfirm` with the existing
   typed tier (assert `m.op.tier==confirmTyped`); choosing Copy → `phaseDest`; i.e. the
   existing op flow runs unchanged.
6. Esc in the open menu → menu closes, `mode` restored, no `m.op` created.
7. Pressing `d`/`u`/`y`/`m`/`D`/`+`/`r`/`x` at top level (no menu open) → no state change, no
   op created (removed-keys inert).
8. With an in-flight load, Esc → `cancelLoad` invoked (load cancelled); with a running op,
   Esc → op cancelled; with neither, Esc → back/navigation.
9. Top-level interactive action count ≤ 12 (assert against the routed key set; aliases and
   `1-9` each count once; menu/mode-internal actions excluded).
10. **Modal precedence**: with the menu OPEN AND a load in flight, Esc → menu closes and the
    load is NOT cancelled (`m.loading` still true, `mode` restored); a second Esc (menu now
    closed, load still running) → load cancelled (FR-029).
