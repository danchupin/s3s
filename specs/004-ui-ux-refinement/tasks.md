---
description: "Task list for 004-ui-ux-refinement"
---

# Tasks: UI/UX Refinement — Action Menu, Footer Redesign & Key Discoverability

**Input**: Design documents from `/specs/004-ui-ux-refinement/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
(action-menu-contract.md, footer-hints-contract.md, help-surface-contract.md), quickstart.md

**Tests**: REQUIRED — Constitution III (Test-First, NON-NEGOTIABLE). Every slice writes
failing white-box UI tests first (Red), then implements to Green.

**Organization**: By user story (US1 P1 → US2 P1 → US3 P2 → US4 P3). All work is in
`internal/ui`; no `internal/storage`, SDK, or `cmd/s3s` changes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Parallelizable — different file, no dependency on an incomplete task.
- File-conflict note: US1, US2, US4 all edit `internal/ui/styles.go` and `internal/ui/app.go`
  → those slices are NOT mutually [P]; run US1 → US2 → US4. US1 and US3 both edit
  `internal/ui/keys.go`. New files (`actionmenu.go`, `keys_test.go`) are conflict-free.

---

## Phase 1: Setup

**Purpose**: Establish a green baseline so each slice's Red→Green is observable.

- [X] T001 Verify pre-change baseline is green: run `make test fmt vet lint check-readonly` and record all pass (no source file change).

**Checkpoint**: Baseline green.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared primitives used by the menu (US1) and the footer (US2).

**⚠️ CRITICAL**: Complete before US1/US2 test tasks.

- [X] T002 Add a shared selection-kind primitive `type selKind int` ({selNone, selObject, selFolder}) and `func (m App) selKind() selKind` (derived from `m.selected()`/`isDir`) in `internal/ui/app.go`, plus white-box test helpers `footerRows(content string) int` and `assertWidthSweep(t, build func(w int) App, lo, hi int)` (every line width ≤ w AND footer rows ≤ 3) in `internal/ui/footer_test.go`.

**Checkpoint**: Shared helpers ready.

---

## Phase 3: User Story 1 - Contextual action menu + keymap reduction (Priority: P1) 🎯 MVP

**Goal**: `a` opens a contextual menu of the operations valid for the selection/context;
the six write keys + `r` refresh are removed from top level; `x` cancel folds into `Esc`.
Menu items dispatch the existing `start*`/`refresh` flows unchanged.

**Independent Test**: In writable/read-only contexts with object/folder/empty selections,
press `a` and assert the menu lists exactly the applicable actions; choosing one enters the
existing op flow; removed top-level keys do nothing; `Esc` cancels an in-flight load.

### Tests for User Story 1 (write FIRST, confirm FAIL) ⚠️

- [X] T003 [US1] Add action-menu contract tests (contracts/action-menu-contract.md C1–C8, all 10 obligations) in new file `internal/ui/actionmenu_test.go`: (1) writable tree + object → menu has Delete/Copy/Move/Upload/New folder/Refresh, not Recursive delete; (2) folder → Recursive delete (+Upload/New folder/Refresh), not Delete/Copy/Move; (3) read-only tree → Refresh only; (4) buckets → Refresh only; (5) choosing Delete → `m.op.tier==confirmTyped` & `phaseConfirm`, choosing Copy → `phaseDest` (existing flow); (6) Esc in menu → closes, mode restored, no `m.op`; (7) pressing `d/u/y/m/D/+/r/x` at top level → no state change, no op; (8) Esc with in-flight load → `cancelLoad`; with running op → op cancelled; else back; (9) top-level interactive action count ≤ 12 (aliases/`1-9` count once); (10) **modal precedence**: menu open + load in flight → Esc closes menu and load NOT cancelled (`m.loading` still true), second Esc → load cancelled (FR-029). Confirm all FAIL.

### Implementation for User Story 1

- [X] T004 [US1] In new file `internal/ui/actionmenu.go`: add `modeActionMenu`; menu state (`menuItems []MenuItem{label,invoke,writeOnly}`, `menuSel int`); `func (m App) openActionMenu()` building the contextual item list from `menuCtx` (mode, writable, `selKind`) per contract C2 — items bind the EXISTING `startRemoveObject`/`startCopy`/`startMove`/`startUpload`/`startCreateFolder`/`startRecursiveDelete`/`refresh`; Refresh always last/present; write items omitted when `!writable`.
- [X] T005 [US1] In `internal/ui/actionmenu.go`: add the menu overlay view (boxView-style, title `actions: <selection>`, states Esc-closes) and `onMenuKey` (↑/↓ + vim move, Enter/→ invoke selected item, Esc/← close → restore `prevMode`); wire rendering for `modeActionMenu` into `App.View()` in `internal/ui/app.go` (alt-screen overlay like help).
- [X] T006 [US1] In `internal/ui/keys.go`: add `Menu: []string{"a"}` to `keyMap`/`defaultKeys()`; remove the `Cancel: []string{"x"}` binding. Keep the write/refresh keyMap fields (used by menu items + help text), just no longer matched at top level.
- [X] T007 [US1] Routing: in `internal/ui/tree.go` `onTreeKey` remove the cases for NewFolder/Delete/Upload/Copy/Move/DeleteAll/Refresh and add `case matches(key, m.keys.Menu): return m.openActionMenu()`; in `internal/ui/app.go` `onBucketsKey` add the same Menu case; in `internal/ui/app.go` `onKey` replace the cancel paths (the `Cancel && m.loading` global check ~L302 and the `phaseRunning` modal `Cancel` ~L270) with the Back key (Esc/Back cancels when `m.loading` or `op.phase==phaseRunning`, else normal back); route `modeActionMenu` to `onMenuKey` in the `Update`/`onKey` switch.
- [X] T008 [US1] Run `go test ./internal/ui/ -run 'Menu|Actions|Cancel'` → Green; then `make fmt vet lint check-readonly`. Fix until pass.

**Checkpoint**: US1 functional — `a` menu replaces the write keys; top-level keymap reduced. MVP shippable.

---

## Phase 4: User Story 2 - A calm, context-aware footer (Priority: P1)

**Goal**: Footer ≤ 3 rows; one contextual hint row capped at 6 advertising `a actions`
(not the write keys), arrow glyphs as primary nav, `? more` on overflow; compact identity
row; endpoint/region/user/version removed from the footer.

**Independent Test**: At several widths/modes assert footer ≤ 3 rows, hint row one line with
`a actions` and no individual write keys, arrow glyphs (not vim), `esc clear/back`, cap 6,
`? more`, zero overflow.

### Tests for User Story 2 (write FIRST, confirm FAIL) ⚠️

- [X] T009 [US2] Add footer contract tests (contracts/footer-hints-contract.md C1–C6, all 8 obligations) in `internal/ui/footer_test.go`: writable tree @w80 → one-line hint row with `a`/`? help`/`q quit`, none of `d/u/y/m/D/+/r/x`, footer ≤ 3 rows; read-only → no individual write hints; single context hides `c`/`1-9`; width sweep 40→200 (use T002 helper); narrow → `? more` + `? help`/`q quit` kept; nav cues use arrow glyphs not vim letters (FR-031); footer contains no Top/Bottom token (`g`/`G`/Home/End unadvertised, FR-031); search active → `esc clear` & not `esc back` (inverse when inactive); >6 applicable @w200 → ≤ 6 tokens + `? more`. Confirm FAIL.

### Implementation for User Story 2

- [X] T010 [US2] In `internal/ui/styles.go`: define `hintCtx` + the hint catalog (per contract C3 — `a actions` replacing write hints; arrow-glyph nav tokens; `c`/`1-9` multi-context; P0 `? help`/`q quit`) and the single-row priority packer (cap `maxHints = 6`, drop lowest-prio first, append dim `? more` when any dropped); add the compact identity builder `● <ctx> [RW|RO]` + optional `· <cluster>`. Retire `footerEndpointLine`/old `footerHintsLine` from footer use; do NOT use `wrapSegs` for hints.
- [X] T011 [US2] In `internal/ui/app.go` `footerBlock`: rebuild as identity row + hint row + optional status row (≤ 3 lines); remove the separator rule and the endpoint line; construct `hintCtx` from model (mode, writable, `selKind()`, search state, `len(contexts)>1`, width) and call the new builders.
- [X] T012 [US2] Run `go test ./internal/ui/ -run 'Footer|Hint'` → Green; then `make fmt vet lint`. Fix until pass.

**Checkpoint**: US1 + US2 — calm footer advertising `a actions`, arrows primary.

---

## Phase 5: User Story 3 - Discover every shortcut on demand (Priority: P2)

**Goal**: One categorized help surface — Navigation / Search & View / Actions (the menu) /
Context / Global / Connection — listing all key aliases incl. vim, documenting the menu's
contents, reflecting write capability, with an explicit close hint.

**Independent Test**: Open help in each mode; assert the 6 sections, all `defaultKeys()`
aliases incl. vim, an Actions section describing the `a` menu + items, a Connection section
(endpoint/region/user/version), writable reflection, and `press any key to close`.

### Tests for User Story 3 (write FIRST, confirm FAIL) ⚠️

- [X] T013 [P] [US3] Add help contract tests (contracts/help-surface-contract.md H1–H5, obligations 1/1a/2/2a/3/4) in new file `internal/ui/keys_test.go`: help lists all `defaultKeys()` actions grouped under Navigation/Search & View/Actions/Context/Global with aliases INCLUDING vim (`↑/k`, `←/h/Esc`, `g/Home`); Actions section documents `a` + items (new folder/delete/upload/copy/move/recursive delete/refresh) marking write-only; Connection section with endpoint/region/user/`s3s ver`; redaction guard (2a — credential-like sentinel never appears); `!writable` → write items hidden/marked; contains `press any key to close`. ([P]: new file, conflict-free.) Confirm FAIL.

### Implementation for User Story 3

- [X] T014 [US3] In `internal/ui/keys.go`: convert `helpLines()` → method `m.helpLines()` returning categorized sections; build the key column from `defaultKeys()` (incl. vim aliases — help is the sole vim advertising point per FR-031/FR-014c); add an Actions section documenting the `a` menu key + its items (mark write-only per `m.writable`, H4); add a Connection section from `m.ctxName`/`m.info.*`/`Version` (non-secret fields only, FR-021); keep `press any key to close`.
- [X] T015 [US3] In `internal/ui/keys.go` `helpView`: render section titles + rows from `m.helpLines()`; ensure call sites compile. (Same file as T014 → sequential.)
- [X] T016 [US3] Run `go test ./internal/ui/ -run 'Help'` → Green; then `make fmt vet lint`. Fix until pass.

**Checkpoint**: US1 + US2 + US3 — full keymap + menu + connection discoverable in help.

---

## Phase 6: User Story 4 - Clearer status, loading & confirmation feedback (Priority: P3)

**Goal**: Loading names what is loading and says `Esc to cancel`; debounced search shows a
pending indicator; typed-confirm keeps the target visible; success notices are green vs red
errors.

**Independent Test**: Trigger each state; assert loading text differs by mode + `Esc to
cancel`, `searching…` pending, notice green vs error red, typed-confirm target + safe mismatch.

### Tests for User Story 4 (write FIRST, confirm FAIL) ⚠️

- [X] T017 [US4] Add status feedback tests (contracts/help-surface-contract.md S1–S5) in `internal/ui/app_test.go`: loading contains `buckets`/`contents`/`object` by mode and `Esc to cancel` (not `x`, FR-029); `searching…` while search scheduled-but-unfired; success notice green hue vs error red hue (distinct ANSI); typed-confirm shows required target beside input and a mismatch dispatches no command (S3); **status precedence (S5/FR-018a)**: with loading + notice both set → row shows loading (not notice); with an op prompt active during a load → row shows the op prompt; assert exactly one status line by the priority order op-prompt > running > loading > search-pending > notice > error. Confirm FAIL.

### Implementation for User Story 4

- [X] T018 [US4] In `internal/ui/styles.go`: add `noticeStyle` (foreground `colOK` green), distinct from `errStyle` red.
- [X] T019 [US4] In `internal/ui/app.go` `statusLine`: name the loading target by state (`loading buckets…`/`loading contents…`/`loading object…`) with `(Esc to cancel)`; add a `searching…` pending branch; render `m.notice` with `noticeStyle`. Ensure the single-row precedence order (op-prompt > running > loading > search-pending > notice > error, FR-018a/S5) is explicit in the branch ordering. Leave `opPromptLine` (typed-confirm target) intact.
- [X] T020 [US4] Run `go test ./internal/ui/ -run 'Status|Loading|Notice|Confirm'` → Green; then `make fmt vet lint check-readonly`. Fix until pass.

**Checkpoint**: All four stories independently functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T021 [P] Update `README.md` keybindings/footer section: the `a` action menu, removed top-level write/refresh keys, `Esc` cancel, arrows-primary (vim in help), 3-row footer with `a actions`, categorized help.
- [X] T022 Run `specs/004-ui-ux-refinement/quickstart.md` manual verification (read-only & `--write`, object/folder menus, removed keys inert, Esc-cancel, narrow/wide footer, help sections).
- [X] T023 Final gate: `make test fmt vet lint check-readonly` all green; assert top-level interactive action count ≤ 12 (SC-008) and footer suite passes at widths 40–200; confirm `scripts/check-readonly.sh` unaffected (no SDK changes).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: none.
- **Foundational (T002)**: after T001; blocks US1/US2 tests (`selKind`, helpers).
- **US1 (T003–T008)**: after T002. The MVP and the keystone — the footer (US2) and help (US3) reference the menu it introduces.
- **US2 (T009–T012)**: after US1 (footer advertises `a actions`; shares `styles.go`/`app.go` with US1).
- **US3 (T013–T016)**: after US1 (help documents the menu; `keys.go` edited by US1 first). T013 (new file) may be written in parallel.
- **US4 (T017–T020)**: after US2 (shares `styles.go`/`app.go`).
- **Polish (T021–T023)**: after all stories.

### Within Each Story

- Test task FIRST (confirm FAIL) → implementation → Green + gates.
- US1: T003 → T004 → T005 → T006 → T007 → T008.
- US2: T009 → T010 → T011 → T012.
- US3: T013 → T014 → T015 → T016 (T014/T015 same file → sequential).
- US4: T017 → T018 → T019 → T020.

### Parallel Opportunities

- T013 (US3 help tests, new `keys_test.go`) is file-disjoint and may be authored alongside US1/US2 code.
- T021 (README) is [P] vs any code task.
- Intra-story impl tasks share files → mostly sequential.

---

## Parallel Example

```bash
# Stories are largely serial due to shared styles.go/app.go/keys.go, BUT:
# - T013 (keys_test.go, new) can be written in parallel with US1/US2 implementation.
# - T021 (README) parallel with final code tasks.
# Recommended order: T001 → T002 → US1(T003–T008) → US2(T009–T012) → US3(T013–T016) → US4(T017–T020) → Polish.
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. T001 Setup → T002 Foundational.
2. US1 (T003–T008): the `a` action menu + keymap reduction — the user's core "fewer keys" ask.
3. **STOP & VALIDATE**: `a` opens contextual menu; write/refresh keys gone from top level;
   Esc cancels; ≤ 12 top-level keys. Ship/demo.

### Incremental Delivery

1. Setup + Foundational.
2. + US1 → action menu + reduced keymap (MVP) → demo.
3. + US2 → calm footer advertising `a actions`, arrows primary → demo.
4. + US3 → categorized help (menu + connection + vim) → demo.
5. + US4 → clearer loading/search/notice feedback → demo.
6. Polish → README + quickstart + final gates.

---

## Notes

- All edits confined to `internal/ui`; `make check-readonly` MUST stay green (no SDK writes added).
- The action menu adds NO operation/confirmation logic — items re-enter the existing `start*`/`refresh` flows; two-tier confirmation + secret redaction preserved (FR-020/021/026).
- [P] = different file, no incomplete-task dependency.
- Verify each test FAILS before implementing (Constitution III).
- Commit after each task or logical group.
