# Phase 1 Data Model: Connection Management UX Fixes

UI-only state. No storage/config entities change. Three small pieces of UI state.

## textField (NEW — `internal/ui/textfield.go`)

The shared single-line rune-aware editor. The single source of caret/paste/render logic for
both the add-connection form and the typed-confirm input.

| Field | Type | Notes |
|-------|------|-------|
| `Value` | `string` | the field contents |
| `Caret` | `int` | caret position as a **rune** index, invariant `0 ≤ Caret ≤ len([]rune(Value))` |

Behaviours (methods):

- `Insert(s string)` — insert `s` at the caret; advance caret by `len([]rune(s))`. Used by both keypress (`msg.Text`) and paste.
- `Backspace()` — delete the rune **before** the caret (no-op at start); caret−−.
- `DeleteFwd()` — delete the rune **at** the caret (no-op at end). (Optional; bind to Delete key.)
- `Left()` / `Right()` — move caret by one rune, clamped.
- `Home()` / `End()` — caret to 0 / to end.
- `Render(width int, masked bool) string` — horizontally-scrolled window of width `width`
  that always contains the caret; draws a caret glyph; when `masked`, renders `•` per rune
  (count = rune length, never the raw secret).

Validation / invariants:
- Caret never splits a rune (rune-indexed).
- Single line: callers sanitize pasted newlines before `Insert` (R2).
- Masked render exposes only the length, never the secret bytes (FR-007).

## connForm (EDIT — `internal/ui/connections.go`)

The add-connection form. The five text fields become `textField` editors; the two booleans
and cursor are unchanged.

| Field | Before | After |
|-------|--------|-------|
| `name, endpoint, region, accessKey, secret` | `string` each | `textField` each |
| `pathStyle, readOnly` | `bool` | unchanged |
| `cursor` | `int` (0..fldReadOnly) | unchanged — selects the focused field |
| `err, tested, testOK` | as today | unchanged |

- The focused text field (per `cursor`) receives Insert/Backspace/Left/Right/Home/End/paste.
- Caret-movement / paste on a boolean row (`fldPathStyle`/`fldReadOnly`) is a no-op (FR-008).
- `draft()` reads each field's `.Value` (TrimSpace as today); the secret is wrapped in
  `logging.Secret` only at draft time (unchanged).
- Switching fields (up/down/tab) leaves each field's caret as-is, or resets to `End()` — see
  contract; default: caret follows the field (each `textField` keeps its own caret).

## operation.input (EDIT — `internal/ui/confirm.go` / `operation`)

The typed-confirm input gains caret + paste by becoming a `textField` (or keeping a string +
a caret int that the shared editor operates on). The byte-exact match compares `input.Value`
to `op.expect` (unchanged semantics — exact, no trim, no case-fold).

| Field | Before | After |
|-------|--------|-------|
| `op.input` | `string` | `textField` (compare `.Value` to `expect`) |

- `confirmview.typedConfirmForm` renders via `input.Render(avail, masked=false)` so the
  caret is visible at its real position (today it only shows the tail + a fixed caret glyph).

## Command bar render state (EDIT — `internal/ui/commandbar.go`)

No new fields. The change is render-only:
- ALL THREE `blockTitleStyle` heading rows are removed (FR-013): `"INFO"` (commandbar.go:162 in
  `infoColumn`), `"READ"` (line 148 in `commandBarView`), `"WRITE"` (line 191 in `writeColumn`).
  The columns + the existing ≥2-space inter-column gap keep the grouping visible.
- The read-only `(w to arm)` cue (today appended to the `WRITE` title) is relocated to a
  lead row of the write column rendered only when `!writable()` — the literal text "w to arm"
  (amber, NO_COLOR-safe) is the defined surface for FR-014.

## Post-mutation cache invalidation (EDIT — `internal/ui/operation.go`, `tree.go`) — US6

No new type. The level cache `cache.Cache[*levelState]` is keyed by
`cache.Key{Context,Bucket,Prefix,Search}`. Today `refresh()` invalidates only
`m.levelKey()` (current view). Change:

- After a **copy** / **move** / **bulk copy**, invalidate PRECISELY the two affected level keys
  (clarified — NOT a whole-cache clear). copy/move are SAME-BUCKET (`CopyKey`/`MoveObject` take
  one bucket), so both keys share the bucket: source = `{ctx,bucket,prefixOf(srcKey),""}` and
  destination = `{ctx,bucket,prefixOf(target),""}` (search empty — a cached filtered view of
  either is also stale). For `bulk_copy`, the SOURCE prefix is `op.parent` (= `m.prefix` where
  the bulk started, bulk.go:71) and the DESTINATION prefix is `op.dstKey` (the entered
  destination prefix, used at bulk.go:111) — NOT `op.parent`.
- Same-level mutations (folder/upload/delete/recursive/bucket) keep their existing
  `refresh()`/`refreshBuckets()` — already correct (no regression).
- Add a small helper (e.g. `invalidateLevel(key)`) so the destination key can be cleared
  without navigating there. `prefixOf` reuses `parentPrefix` (tree.go).
- NO cross-bucket case: there is no cross-bucket copy/move in the storage contract, so no
  dst-bucket key is ever needed — keeps US6 strictly UI-only (FR-018).

| Behaviour | Before | After |
|-----------|--------|-------|
| copy/move/bulk_copy post-op | `refresh()` (current level only) | invalidate source + destination prefix keys (same bucket, precise), then `refresh()` current |

## Command-bar affordances (EDIT — `internal/ui/commandbar.go`, `hintbar.go`) — US7/US8/US9

Render-state only; no new persistent fields.

- **US7 connection affordance** (clarified — relabel, no new entry): relabel "new conn" →
  "connections" at BOTH render sites — `infoColumn` (commandbar.go:172) AND the collapsed read
  row (`collapsedBarView`, commandbar.go:220). Still bound to `m.keys.AddConn`; still opens the
  manager (switch/add/delete). No separate switch entry; `c` context-switch key unchanged, not
  separately surfaced.
- **US7 collapse order (FR-020)**: in `collapsedBarView`, the "connections" entry MUST be placed
  ahead of droppable read entries (or added to `fitEntries` keep-min). Today it is appended LAST
  and `fitEntries` drops trailing entries first — so a relabel alone would still drop it first.
- **US8 filter-reset**: in `readEntries`, append `{key: glyph(Back/Esc), label: "clear"}` when a
  filter term exists and the input is closed. Predicate: `m.searchActive() && !m.searching`
  (note `searchActive()` itself ORs in `m.searching`, so the `&& !m.searching` reduces it to a
  non-empty filter term with the input closed — the FR-021 "applied, not typing" condition).
- **US9 duplicate delete** (clarified — show only applicable): in `writeEntries`, suppress the
  delete entry whose `avail(m,kind)` is false so only the selection-applicable delete shows
  (object/group delete for an object cursor; recursive delete for a folder cursor). Targeted
  exception to 007's "all write actions always shown" — for the delete pair only; other write
  actions still render dimmed/inapplicable as before.

## Key glyph table (EDIT — `internal/ui/keys.go`)

No structural change — two map values flip:

| Key | Before | After |
|-----|--------|-------|
| `ctrl+x` | `^x` | `Ctrl+X` |
| `ctrl+o` | `^o` | `Ctrl+O` |
| `ctrl+c` | `Ctrl+C` | `Ctrl+C` (unchanged, the precedent) |
