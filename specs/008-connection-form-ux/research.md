# Phase 0 Research: Connection Management UX Fixes

All nine user stories (US1–US9) are UI-local. No NEEDS CLARIFICATION remained after
`/speckit-clarify` (two sessions, 6 questions resolved). R1–R6 cover the original five fixes +
heading removal; R7–R10 cover the US6–US9 follow-up defects added in the scope expansion.

## R1 — Single-line text editor (FR-005, FR-006, FR-007, FR-008)

**Decision**: Hand-roll a small rune-aware `textField` type in `internal/ui/textfield.go`:
fields `Value string` + `Caret int` (caret is a **rune** index, 0..len([]rune(Value))).
Methods: `Insert(s string)`, `Backspace()`, `DeleteFwd()`, `Left()`, `Right()`, `Home()`,
`End()`, and `Render(width int, masked bool) string` (a horizontally-scrolled window that
keeps the caret visible and draws a caret glyph). Shared by the add-connection form fields
and the typed-confirm input.

**Rationale**: The codebase has no `bubbles` dependency (go.mod lists only bubbletea v2 +
lipgloss v2). The two affected surfaces (`connForm` fields, `op.input`) already hand-roll
append-tail editing — replacing both with one tested helper is DRY and avoids pulling in
`bubbles/textinput` (heavier, its own styling model, inconsistent with the project's render
approach). Rune indexing avoids splitting multi-byte input (endpoints/keys are ASCII, but
pasted values may not be).

**Alternatives considered**:
- `bubbles/textinput` — full-featured but a new dependency + a foreign styling/update model
  inside an otherwise hand-rolled UI; rejected for weight and inconsistency.
- Byte-index caret — simpler but corrupts on multi-byte paste; rejected.

## R2 — Clipboard paste (FR-005)

**Decision**: Handle `tea.PasteMsg` (Content string). Add a `case tea.PasteMsg` in
`App.Update` that routes the content to the active text surface: search query, command
input, `connForm` focused field, or `op.input` (typed-confirm). For single-line fields,
strip trailing `\r`/`\n` (clipboards commonly append one) before `Insert`, and replace any
interior newlines with spaces so a paste never breaks the single line or premature-submits.

**Rationale**: Bubble Tea v2 emits `tea.PasteMsg` via bracketed paste, which the v2 renderer
**enables by default** (`SetModeBracketedPaste` unless `DisableBracketedPasteMode`). Today
`Update` only handles `tea.KeyPressMsg`, so paste is silently dropped — adding the case is
the whole fix on the input side. Insert-at-caret reuses the R1 editor.

**Alternatives considered**:
- OS clipboard library (e.g. atotto/clipboard) — unnecessary; the terminal already delivers
  the paste as input. Rejected (new dependency, no benefit).
- Treating paste as a burst of key events — v2 already separates paste into one message;
  re-synthesizing keypresses would lose the atomic-insert semantics FR-005 requires.

## R3 — Chord label format (FR-004, clarified no-space)

**Decision**: Change the single source `keyGlyph` in `keys.go`: `"ctrl+x" → "Ctrl+X"`,
`"ctrl+o" → "Ctrl+O"` (drop the `^x`/`^o` carets). Every surface renders chords through
`glyph()`, so the command bar, the bare-key nudge in `hintbar.go`, and help all update at
once. Audit the nudge string in `dispatchActionKey` ("press Ctrl+X to delete (Ctrl chord
required)") and trim the now-redundant "(Ctrl chord required)" tail.

**Rationale**: One map entry is the entire fix; consistency with the existing `"ctrl+c" →
"Ctrl+C"`. No-space form chosen in clarification (matches `Ctrl+C`, narrower).

**Alternatives considered**: per-surface string edits — rejected (drift risk; `glyph()`
already centralizes it).

## R4 — Connection-delete hint placement (FR-001, FR-003, clarified inline)

**Decision**: Render the delete hint **inline in the connections list view**, not by adding
a `delete_connection` action to the command-bar catalog. In `connectionsView`, append a help
line (mirroring `connFormView`'s help line) whose delete segment is:
- shown active (`Ctrl+X delete`) when a **non-active existing** connection is selected;
- ABSENT (not rendered) on the `+ add connection` row and the empty list (FR-003 single behaviour, clarified — not "marked inapplicable");
- the active-connection case keeps the existing guard message on press (FR-002).

**Rationale**: Clarified choice. The command-bar catalog is keyed to bucket/object modes;
`modeConnections` delete lives in `onConnectionsKey`, so a local inline hint is the minimal,
honest surface and avoids wiring a connections-mode entry through the whole catalog/predicate
machinery.

**Alternatives considered**: command-bar write-block entry — rejected per clarification
(more plumbing, the bar isn't the connections surface).

## R5 — Secret + per-field guidance (FR-009, FR-010, clarified hint-only)

**Decision**: In `connFormView`, render a focused-field hint line. For the secret field:
"secret access key — stored in your OS keychain · other sources (env var · cmd · AWS
profile) via the config file". For the other fields, a one-line expectation (e.g. endpoint →
"https://host:port", name → "unique context name"). The save path is unchanged
(keychain-only); the hint explicitly does NOT promise `${ENV}` resolution from the form, so
no one is misled into typing a reference the form would store verbatim (FR-010).

**Rationale**: Clarified hint-only scope. Text guidance is cheap, removes the guessing game,
and keeps the config-writer untouched.

**Alternatives considered**: a credential-source selector (keychain/env/cmd/awsProfile) —
rejected in clarification (larger scope, config-writer changes).

## R6 — Remove command-bar block headings (FR-013, FR-014, US5)

**Decision**: In `commandbar.go`, stop rendering ALL THREE `blockTitleStyle` rows —
`"INFO"` (line 162), `"READ"` (148), `"WRITE"` (191) — keep the three columns joined by the
existing gap so grouping stays visually distinct. Preserve the read-only cue: when
`!writable()`, render the literal text `w to arm` (amber `warnStyle`, NO_COLOR-safe) as the
write column's lead row in place of the dropped title; when writable, no lead row. The
collapsed (narrow) path already has no titles — leave it, just ensure no orphaned heading text.

**Rationale**: Columns + inter-column gap already communicate the grouping; the titles are
redundant noise (user feedback). Removing only READ/WRITE while keeping INFO would leave an
inconsistent bar (analysis finding), so all three go. FR-014 mandates keeping the read-only
cue at a defined surface, hence the literal `w to arm` atop the write column.

**Alternatives considered**: keep `INFO` for the identity column — rejected for inconsistency;
move `w to arm` onto the `[RO]` identity badge — spreads the cue away from the write entries.

## R7 — Post-mutation visibility, same-bucket cross-prefix (US6, FR-015/016/018)

**Decision**: After a successful `copy`/`move`/`bulk_copy`, in `onOperationDone`, precisely
invalidate the source prefix key `{ctx,bucket,parentPrefix(srcKey),""}` and destination prefix
key `{ctx,bucket,parentPrefix(target),""}` via a new `invalidateLevel(key)` helper, then keep
the existing `refresh()` of the current level. Same-level mutations are already correct.

**Rationale**: `refresh()` invalidates only the current `levelKey()`, so a copy/move into a
different (cached) prefix shows stale contents until manual `r`. `CopyKey`/`MoveObject` are
SAME-BUCKET (storage.go:79,84), so there is no cross-bucket case — both keys share the bucket,
no storage change, stays UI-only (FR-018). `bulk_copy` (bulk.go) targets a destination prefix
and has the same bug; `bulk_move` does not exist.

**Alternatives considered**: whole-cache `cache.Clear()` after any mutation — rejected
(clarified): precise two-key invalidation preserves the rest of the session cache.

## R8 — Connection affordance relabel + collapse order (US7, FR-019/020)

**Decision**: Relabel the existing connection entry "new conn" → "connections" at BOTH render
sites (`infoColumn`:172 and `collapsedBarView`:220), still bound to `AddConn`. In
`collapsedBarView`, place it ahead of droppable read entries (or in `fitEntries` keep-min) so
trimming does not drop it first.

**Rationale**: Clarified — relabel, no separate `c switch` entry; the manager covers
switch/add/delete. FR-020 needs the reorder because the entry is appended LAST today and
`fitEntries` drops trailing entries first (analysis finding) — a relabel alone would fail it.

**Alternatives considered**: separate `c switch` entry / both — rejected in clarification.

## R9 — Filter-reset affordance (US8, FR-021)

**Decision**: In `readEntries`, append `{key: glyph(Back/Esc), label: "clear"}` when
`m.searchActive() && !m.searching`. Reuses the existing Esc-clear path (no new binding).

**Rationale**: List modes render the command bar, not the legacy `footerHints` that already had
an "esc clear" cue — so the cue was lost. `searchActive()` ORs in `m.searching`, so the
`&& !m.searching` reduces the gate to "a filter term exists AND the input is closed" (the
FR-021 applied-not-typing predicate, now stated in the spec).

**Alternatives considered**: a dedicated reset key — rejected; Esc already clears.

## R10 — No duplicate delete labels (US9, FR-022)

**Decision**: In `writeEntries`, suppress the delete entry whose `a.avail(m,kind)` is false for
the delete pair (object delete vs recursive delete), so only the selection-applicable one
renders. Other write actions still render dimmed/inapplicable as before.

**Rationale**: Clarified — show only applicable (not relabel). Object delete (`avail`=object
cursor) and recursive (`avail`=folder cursor) are mutually exclusive by selection, so
suppression always leaves exactly one delete. Targeted exception to 007's "all write always
shown", scoped to the delete pair to avoid hiding other capabilities.

**Alternatives considered**: relabel recursive to "delete dir" — rejected in clarification
(showing only the applicable one is cleaner and matches the user's "remove the duplicate").

## Cross-cutting: testing (Constitution III)

White-box `package ui` tests, written first (Red):
- `textfield_test.go`: insert at caret, left/right/home/end bounds, backspace/delete-fwd at
  caret, multi-rune paste, masked render length, window keeps caret visible.
- `connections_test.go`: paste into a field lands whole; caret-mid edit; secret stays masked;
  delete hint visible for non-active selection, absent on add-row, guard on active.
- `confirm`/`confirmview` tests: paste + caret in the typed-confirm input.
- `commandbar`/`hintbar`/`footer` tests: no `READ`/`WRITE`/`^x`/`^o` strings; `Ctrl+X`
  present; `(w to arm)` still shown read-only.

No integration test (no storage/config contract change).
