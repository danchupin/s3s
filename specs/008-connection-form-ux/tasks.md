---

description: "Task list for 008-connection-form-ux implementation"
---

# Tasks: Connection Management UX Fixes

**Input**: Design documents from `/specs/008-connection-form-ux/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED (Constitution III — Test-First, non-negotiable). White-box `package ui`
tests via `deliver`/`press` helpers + a new `tea.PasteMsg` delivery helper; `textField` unit
tests. No integration tests (no storage/config contract change).

**Organization**: One phase per user story, ordered by priority (P1 → P2 → P3). All changes are
in `internal/ui` (+ its tests) — NO `internal/storage` / `internal/config` edits (FR-018);
`make check-readonly` MUST stay green throughout.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1..US9 (maps to spec.md user stories)
- Exact file paths included.

## Path Conventions

Single Go project. All work under `internal/ui/`. Test files are `*_test.go` (white-box,
`package ui`).

---

## Phase 1: Setup (baseline)

**Purpose**: Establish a green baseline so each story's new test starts Red against known-good.

- [x] T001 Confirm baseline green: run `make test`, `make fmt vet lint`, `make check-readonly` from repo root; record that they pass before any change.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The shared single-line text editor + paste plumbing that US3 (and US4, and the
typed-confirm) build on. This is the only hard cross-story prerequisite; the remaining stories
(US1/US2/US5/US6/US7/US8/US9) are independent and need nothing here.

**⚠️ CRITICAL**: US3 and US4 cannot begin until T002–T003 are complete.

- [x] T002 [P] Write FAILING unit tests for the text editor in `internal/ui/textfield_test.go` per `contracts/text-input-contract.md`: insert-at-caret, `Left`/`Right`/`Home`/`End` bounds, `Backspace`/`DeleteFwd` at caret, multi-rune `Insert` (paste), rune-aware caret (`héllo` len 5), masked `Render` length-only, windowed `Render` keeps caret visible.
- [x] T003 Create the rune-aware single-line editor in `internal/ui/textfield.go`: type `textField{Value string; Caret int}` (rune index) with `Insert`/`Backspace`/`DeleteFwd`/`Left`/`Right`/`Home`/`End` and `Render(width int, masked bool) string` (horizontal-scroll window + caret glyph; `•`×runeLen when masked). Make T002 pass.

**Checkpoint**: `textField` exists and is unit-green — US3/US4 can proceed.

---

## Phase 3: User Story 3 - Usable text entry in the add-connection form (Priority: P1) 🎯 MVP

**Goal**: Caret movement + clipboard paste in the add-connection form AND the typed-confirm
input; secret stays masked & keychain-only.

**Independent Test**: Open the add-connection form, paste a value into a field (whole value
lands), move the caret mid-field and insert/delete at the caret; repeat in the delete-connection
typed-confirm input.

### Tests for User Story 3 ⚠️ (write first, must FAIL)

- [x] T004 [P] [US3] FAILING tests in `internal/ui/connections_test.go`: paste `"https://h:9000\n"` into endpoint → field `"https://h:9000"` (trailing newline stripped, form NOT submitted); caret mid-field insert lands at caret; `Backspace` removes the char before the caret; secret field paste stays masked (`•`×len) and `draft().Secret` redacts; `←/→/Home/End`/paste on a toggle row are no-ops and `space` still toggles.
- [x] T005 [P] [US3] FAILING tests in `internal/ui/confirm_test.go` (+ `confirmview` assertion): paste into and caret-move within the typed-confirm input (delete-connection name); byte-exact match still compares the full value; `typedConfirmForm` renders the caret at its real position.

### Implementation for User Story 3

- [x] T006 [US3] Convert the five `connForm` text fields (name/endpoint/region/accessKey/secret) from `string` to `textField` in `internal/ui/connections.go`; update `textField()` selector, `formAppend`/`formBackspace` to act at the caret, and `draft()` to read `.Value` (TrimSpace as today; secret wrapped in `logging.Secret` only at draft).
- [x] T007 [US3] Extend `onConnFormKey` in `internal/ui/connections.go` to handle `left`/`right`/`home`/`end` on the focused text field, and to no-op caret keys on the boolean rows (per `contracts/connection-ui-contract.md`).
- [x] T008 [US3] Update `connFormView` in `internal/ui/connections.go` to render the focused field via `textField.Render(width, masked)` (masked for the secret; caret visible).
- [x] T009 [US3] Convert `op.input` to `textField` in `internal/ui/confirm.go` (`onConfirmKey`: `left`/`right`/`home`/`end`, insert/backspace at caret; compare `op.input.Value` to `op.expect`); render via `textField.Render` in `internal/ui/confirmview.go` (`typedConfirmForm`).
- [x] T010 [US3] Add `case tea.PasteMsg` to `App.Update` in `internal/ui/app.go`; route the content (strip trailing `\r`/`\n`, replace interior newlines with spaces) to the active text surface — search input, command input, `connForm` focused field, or `op.input`. Make T004/T005 paste assertions pass.
- [x] T011 [US3] Run `make test` (US3 green) and `make check-readonly` (green).

**Checkpoint**: Forms + typed-confirm are fully editable (caret + paste); MVP usable.

---

## Phase 4: User Story 1 - Discoverable connection delete (Priority: P1)

**Goal**: The connection list shows, inline, how to delete the highlighted connection.

**Independent Test**: Render the connections list — delete keystroke+label visible for a
non-active selection; absent on `+ add`/empty; active selection press → guard notice.

### Tests for User Story 1 ⚠️ (write first, must FAIL)

- [x] T012 [P] [US1] FAILING tests in `internal/ui/connections_test.go`: `connectionsView` shows `Ctrl+X` + "delete" for a non-active existing selection; the delete segment is ABSENT (not rendered) on the `+ add connection` row and the empty list (FR-003 single behaviour); pressing the delete chord on the active connection yields the "cannot delete the active connection" notice (FR-002, already implemented).

### Implementation for User Story 1

- [x] T013 [US1] Add an inline delete hint to `connectionsView` in `internal/ui/connections.go` (alongside the existing help line, NOT via the command-bar catalog) per `contracts/connection-ui-contract.md`: active for a non-active connection, ABSENT (not rendered) on add-row/empty; keep correct when the surface is narrow (FR-012).
- [x] T014 [US1] Run `make test` for the connections suite; verify.

**Checkpoint**: Connection delete is discoverable from the screen alone.

---

## Phase 5: User Story 2 - Keystroke labels spell out modifiers (Priority: P1)

**Goal**: `^x`/`^o` → `Ctrl+X`/`Ctrl+O` everywhere (consistent with `Ctrl+C`).

**Independent Test**: `glyph("ctrl+x")=="Ctrl+X"`; rendered list/connections/confirm/help views
contain no `^x`/`^o`.

### Tests for User Story 2 ⚠️ (write first, must FAIL)

- [x] T015 [P] [US2] FAILING tests in `internal/ui/keys_test.go` per `contracts/key-label-contract.md`: `glyph("ctrl+x")=="Ctrl+X"`, `glyph("ctrl+o")=="Ctrl+O"`; rendered bucket/object/connections views contain no `"^x"`/`"^o"`; bare-`x` nudge contains `Ctrl+X`.

### Implementation for User Story 2

- [x] T016 [US2] Change `keyGlyph` in `internal/ui/keys.go`: `"ctrl+x"→"Ctrl+X"`, `"ctrl+o"→"Ctrl+O"`.
- [x] T017 [US2] Trim the redundant "(Ctrl chord required)" tail in the bare-key nudge in `dispatchActionKey` (`internal/ui/hintbar.go`) so it reads naturally (e.g. "press Ctrl+X to delete").
- [x] T018 [US2] Update the existing `^x`/`^o` assertions at `internal/ui/hintbar_test.go:199-200` to expect `Ctrl+X`/`Ctrl+O` (these are the ONLY chord-glyph assertions; `footer_test.go` has none). Run `make test`.

**Checkpoint**: No caret-style chord labels remain.

---

## Phase 6: User Story 7 - Connection affordance labelled "connections" (Priority: P1)

**Goal**: Relabel the command-bar connection entry "new conn" → "connections" (it opens the
manager = switch/add/delete). No separate switch entry.

**Independent Test**: Command bar shows a "connections" entry (not "new conn") in the info group
and the collapsed path.

### Tests for User Story 7 ⚠️ (write first, must FAIL)

- [x] T019 [P] [US7] FAILING test in `internal/ui/hintbar_test.go` per `contracts/command-bar-contract.md`: the wide info group and the collapsed bar contain "connections" and not "new conn"; AND at a width that forces dropping read entries, the "connections" entry SURVIVES (not dropped first — FR-020).

### Implementation for User Story 7

- [x] T020 [US7] Relabel the connection affordance from "new conn" to "connections" in BOTH `infoColumn` (commandbar.go:172) and `collapsedBarView` (commandbar.go:220) in `internal/ui/commandbar.go` (binding stays `m.keys.AddConn`; it still opens the manager). AND in `collapsedBarView`, place the "connections" entry ahead of droppable read entries (or add it to `fitEntries` keep-min) so width-trimming does not drop it first (FR-020).
- [x] T021 [US7] Update any test/snapshot referencing "new conn"; run `make test`.

**Checkpoint**: Switching connection is discoverable from the bar.

---

## Phase 7: User Story 6 - Changes visible after every action (Priority: P1)

**Goal**: Every mutation reflects without manual refresh, including a same-bucket cross-prefix
copy/move/bulk-copy (precisely invalidate the source + destination prefix keys, same bucket).

**Independent Test**: Copy `a/x`→`b/x`; navigate `b/` → present (no manual refresh). Move →
source absent, destination present. Pre-cached empty `b/` is invalidated after a copy into `b/`.

### Tests for User Story 6 ⚠️ (write first, must FAIL)

- [x] T022 [P] [US6] FAILING tests in `internal/ui/operation_test.go` per `contracts/post-mutation-visibility-contract.md` (using `storage.Fake`, all same-bucket): (a) copy to a different prefix → destination shows the new object after navigation, no manual refresh; (b) move → source loses it, destination shows it; (c) pre-visit the destination so it is cached empty, then copy into it → re-navigation shows the new object (proves the destination key was invalidated); (d) bulk_copy to a destination prefix → that prefix shows the copied objects on navigation; (e) a same-level new-folder/delete still reflects immediately (no regression).

### Implementation for User Story 6

- [x] T023 [US6] Add an `invalidateLevel(key cache.Key)` helper in `internal/ui/tree.go` (sibling to `refresh()`) that invalidates an arbitrary cache key without navigating to it.
- [x] T024 [US6] In `onOperationDone` (`internal/ui/operation.go`), for a successful `copy`/`move`, invalidate the precise source key `{ctx,bucket,prefixOf(srcKey),""}` and destination key `{ctx,bucket,prefixOf(target),""}` — SAME bucket (`CopyKey`/`MoveObject` are single-bucket; no dst-bucket) — NOT a whole-cache `Clear()`; then keep the existing `refresh()` of the current level. Apply the same to `bulk_copy` (kind exists; NO `bulk_move`): source prefix = `op.parent`, destination prefix = `op.dstKey` (bulk.go:71/111) — invalidate both.
- [x] T025 [US6] Run `make test`; confirm same-level mutations (folder/upload/delete/recursive/bucket) are unchanged.

**Checkpoint**: No stale view after any mutation, same- or cross-prefix (same bucket).

---

## Phase 8: User Story 4 - Guidance for entering secrets (Priority: P2)

**Goal**: Per-field guidance in the form; secret field names keychain storage + config-file-only
alternatives (no ${ENV} resolution promise).

**Independent Test**: Focus the secret field → hint names the keychain + the config-file sources;
other fields show a one-line expectation.

**Dependency**: US3 (edits the same `connFormView`).

### Tests for User Story 4 ⚠️ (write first, must FAIL)

- [x] T026 [P] [US4] FAILING test in `internal/ui/connections_test.go`: focused secret field hint names the secret access key, OS keychain storage, and that env var / cmd / AWS profile are config-file-only; each focusable field shows a one-line expectation; the hint does NOT promise `${ENV}` resolution.

### Implementation for User Story 4

- [x] T027 [US4] Add a focused-field guidance line in `connFormView` (`internal/ui/connections.go`): secret → "stored in OS keychain · env/cmd/AWS profile via config file"; name/endpoint/region/accessKey → short expectations. Run `make test`.

**Checkpoint**: Secret entry is no longer a guessing game.

---

## Phase 9: User Story 8 - Reset an active filter (Priority: P2)

**Goal**: Show an `Esc clear` affordance in the command-bar read group when a filter is applied
(and not actively typing).

**Independent Test**: Apply a filter → read group shows "clear"; no filter → absent; trigger →
full list restored.

### Tests for User Story 8 ⚠️ (write first, must FAIL)

- [x] T028 [P] [US8] FAILING tests in `internal/ui/hintbar_test.go`: with a bucket filter applied (`searchActive() && !searching`) the read group contains an `Esc` + "clear" entry; with no filter, no "clear" entry; cover both the bucket-list filter and a tree-level search.

### Implementation for User Story 8

- [x] T029 [US8] In `readEntries` (`internal/ui/commandbar.go`), append `{key: glyph(Back/Esc), label: "clear", role: roleRead}` when `m.searchActive() && !m.searching`; reuse the existing Esc-clear path (no new binding).
- [x] T030 [US8] Run `make test`; verify clearing restores the unfiltered list (existing behaviour, no regression).

**Checkpoint**: An active filter is always escapable from the visible bar.

---

## Phase 10: User Story 9 - No duplicate delete entries (Priority: P2)

**Goal**: The write group shows only the selection-applicable delete (object delete for an object
cursor; recursive delete for a folder cursor) — never two identical "delete".

**Independent Test**: Object cursor → object delete only; folder cursor → recursive only; no two
identical labels.

### Tests for User Story 9 ⚠️ (write first, must FAIL)

- [x] T031 [P] [US9] FAILING tests in `internal/ui/hintbar_test.go`: object cursor → write group has the object/group delete and NOT the recursive-delete entry; folder cursor → recursive delete and NOT the object-delete entry; assert no two write-group entries share an identical label.

### Implementation for User Story 9

- [x] T032 [US9] In `writeEntries` (`internal/ui/commandbar.go`), suppress the delete entry whose `a.avail(m,kind)` is false for the delete pair (object delete vs recursive delete), so only the applicable one renders — a targeted exception to 007's "all write actions always shown" (other write actions still render dimmed/inapplicable).
- [x] T033 [US9] Verify non-delete write actions are unchanged (still shown dimmed when inapplicable); run `make test`.

**Checkpoint**: Write group never duplicates "delete".

---

## Phase 11: User Story 5 - Quieter command bar without block headings (Priority: P3)

**Goal**: Drop ALL THREE headings (INFO/READ/WRITE); keep column grouping; preserve the
read-only `w to arm` cue as literal text.

**Independent Test**: Wide bar has no INFO/READ/WRITE titles but info/read/write entries stay in
distinct columns; read-only context still shows literal `w to arm`.

### Tests for User Story 5 ⚠️ (write first, must FAIL)

- [x] T034 [P] [US5] FAILING tests in `internal/ui/hintbar_test.go` per `contracts/command-bar-contract.md`: rendered bar (wide, writable) contains no "INFO"/"READ"/"WRITE" title text while info keys, read keys (`open`,`/`) and write keys remain in distinct column areas; read-only context contains literal `w to arm`; collapsed bar has no orphan title text.

### Implementation for User Story 5

- [x] T035 [US5] Remove ALL THREE `blockTitleStyle` heading rows in `internal/ui/commandbar.go`: `"INFO"` in `infoColumn` (line 162), `"READ"` in `commandBarView` (line 148), `"WRITE"` in `writeColumn` (line 191). Keep the inter-column gap so grouping stays visible. Relocate the `w to arm` cue to the write column's lead row (literal amber text), shown only when `!writable()`.
- [x] T036 [US5] Update the existing title assertion at `internal/ui/hintbar_test.go:166` (currently requires `{"INFO","READ","WRITE"}` present) to assert their ABSENCE; run `make test`.

**Checkpoint**: Calmer bar, grouping + read-only cue intact.

---

## Phase 12: Polish & Cross-Cutting Concerns

**Purpose**: Whole-feature validation and wrap-up.

- [x] T037 [P] Run `quickstart.md` manually (`make build && ./bin/s3s`) — walk US1–US9 verification steps.
- [x] T038 [P] `go test -cover ./internal/ui/` — confirm coverage is not regressed; tidy any new helper docs/comments to match surrounding style.
- [x] T039 `make fmt vet lint && make check-readonly` — all green (no new write-capable S3 symbol leaves `internal/storage`).
- [x] T040 Update the `<!-- SPECKIT START -->` block in `CLAUDE.md`: status PLANNED → IMPLEMENTED with the final task count.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: after Setup. Blocks US3 + US4 only.
- **US3 (Phase 3)**: after Foundational. **MVP.**
- **US1, US2, US7, US6 (Phases 4–7, P1)**: after Setup; independent of Foundational and of each other (different files / concerns).
- **US4 (Phase 8, P2)**: after US3 (same `connFormView`).
- **US8, US9, US5 (Phases 9–11)**: after Setup; independent logically, BUT all edit `internal/ui/commandbar.go` (US7 too) → serialize edits to that file to avoid conflicts.
- **Polish (Phase 12)**: after all desired stories.

### Same-file serialization (not logical dependencies)

- `internal/ui/commandbar.go`: US7 (T020), US8 (T029), US9 (T032), US5 (T035) — do sequentially.
- `internal/ui/connections.go`: US3 (T006–T008), US1 (T013), US4 (T027) — do sequentially.

### Parallel Opportunities

- All `[P]` test-authoring tasks within a phase run in parallel.
- Across stories with TRULY disjoint files: US2 (keys.go/hintbar.go) and US6 (operation.go/tree.go) can proceed in parallel with each other. NOTE: US3/US1/US4 all share `connections.go` AND `connections_test.go` → they are NOT parallel (serialize per the Same-file section). US7/US8/US9/US5 all share `commandbar.go` AND `hintbar_test.go` → also serialize. The `[P]` markers are intra-phase only (each is alone in its phase here), NOT a cross-story parallel guarantee.

---

## Parallel Example: User Story 3 (tests first)

```bash
# Author the failing tests together (different files):
Task: "T004 connForm paste/caret tests in internal/ui/connections_test.go"
Task: "T005 typed-confirm paste/caret tests in internal/ui/confirm_test.go"
# (T002 textField unit tests already authored in Foundational)
```

---

## Implementation Strategy

### MVP First

1. Phase 1 Setup → Phase 2 Foundational (`textField`) → Phase 3 **US3** (usable forms + paste).
2. **STOP and VALIDATE**: paste + caret work in the form and the typed-confirm. This alone
   resolves the loudest complaint ("формы неюзабельны").

### Incremental Delivery (priority order)

1. MVP = US3.
2. Add the remaining P1: US1 (delete hint), US2 (Ctrl+X), US7 (connections label), US6
   (post-mutation visibility) — each independently testable.
3. Add P2: US4 (secret guidance), US8 (filter reset), US9 (no dup delete).
4. Add P3: US5 (quieter bar).
5. Polish (Phase 12).

### Constitution guardrails (every phase)

- Test-First (III): the `[P]` test task in each phase is authored and FAILS before its
  implementation task.
- Core/UI Separation (I) + read-only guard: no edits outside `internal/ui`; `make check-readonly`
  green after every phase.
- Non-Blocking (II): no new blocking I/O; paste is in-process input.

---

## Notes

- `[P]` = different files, no incomplete-task dependency.
- Total: 40 tasks across 12 phases (5×P1 stories, 3×P2, 1×P3 + setup/foundational/polish).
- Commit after each task or logical group.
- Every story is independently testable and leaves the suite green.
