---
description: "Task list for 004-ui-ux-refinement"
---

# Tasks: UI/UX Refinement — Footer Redesign & Key Discoverability

**Input**: Design documents from `/specs/004-ui-ux-refinement/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
(footer-hints-contract.md, help-surface-contract.md), quickstart.md

**Tests**: REQUIRED — Constitution III (Test-First, NON-NEGOTIABLE). Every slice writes
failing white-box UI tests first, confirms Red, then implements to Green.

**Organization**: By user story (US1 P1 → US2 P2 → US3 P3). All work is in
`internal/ui`; no `internal/storage`, SDK, or `cmd/s3s` changes.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Parallelizable — different file, no dependency on an incomplete task.
- File-conflict note: US1 and US3 both edit `internal/ui/styles.go` and
  `internal/ui/app.go`, so they are NOT mutually [P]. US2 edits `internal/ui/keys.go` +
  new `internal/ui/keys_test.go` and IS independent (can proceed alongside US1).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish a green baseline so each slice's Red→Green is observable.

- [ ] T001 Verify pre-change baseline is green: run `make test fmt vet lint check-readonly` and record all pass (no source file change).

**Checkpoint**: Baseline green — TDD slices can begin.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared test scaffolding used by US1 and US3 footer/status assertions.

**⚠️ CRITICAL**: Complete before US1/US3 test tasks.

- [ ] T002 [US-shared] Add white-box test helpers `footerRows(content string) int` and `assertWidthSweep(t, build func(w int) App, lo, hi int)` (asserts every rendered line width ≤ w and footer rows ≤ 3) in `internal/ui/footer_test.go`.

**Checkpoint**: Helpers available — story implementation can begin.

---

## Phase 3: User Story 1 - A calm, context-aware footer (Priority: P1) 🎯 MVP

**Goal**: Footer capped at 3 rows (compact identity + single contextual hint row +
optional status); hints adapt to mode/selection/context/width; drop lowest-priority
first with a `? more` cue; `? help`/`q quit` always survive; zero write hints when
read-only.

**Independent Test**: At several widths and in each mode (read-only vs write,
object/folder selected, search active, single vs multi context), assert footer ≤ 3 rows,
hint row is one line, correct hints present/absent, `? more` appears on overflow, no
horizontal overflow. Verifies SC-001/002/003/005/007, FR-001..009.

### Tests for User Story 1 (write FIRST, confirm FAIL) ⚠️

- [ ] T003 [US1] Add US1 footer contract tests (all 8 cases of contracts/footer-hints-contract.md C1–C6) in `internal/ui/footer_test.go`: (1) writable tree + object selected @w80 → hint row one line, contains `d`/`u`/`y`/`m` + `? help` + `q quit`, footer ≤ 3 rows; (2) read-only any mode → none of `d/u/y/m/D/+`; (3) single context hides `c`/`1-9`, multi shows; (4) width sweep 40→200 via `assertWidthSweep`; (5) narrow width drops low-prio + appends `? more`, keeps `? help`+`q quit`; (6) folder selected → `D rmdir` present & `d/y/m` absent, object selected → inverse; (7) search active in tree → `esc clear` present AND `esc back` absent; search inactive → inverse (C5/FR-009); (8) state with >6 applicable hints (writable tree, object selected, multi-context) @w200 → at most **6** rendered key tokens AND `? more` cue present (count cap, C3/C4, SC-003). Confirm all FAIL.

### Implementation for User Story 1

- [ ] T004 [US1] In `internal/ui/styles.go`: define `hintCtx` struct (mode, writable, selKind, searchActive, searching, multiContext, opActive, width per data-model.md), the static hint catalog `[]hint{key,label,prio,visible(hintCtx)bool}` with priorities P0–P4 and visibility predicates per contract C3 (incl. the `esc clear` vs `esc back` swap on `searchActive`, C5), and a single-row priority packer that: sorts desc by prio, **caps to the top `maxHints = 6` (const)**, then greedily fits those within width dropping lowest-prio first, and appends a reserved-width dim `? more` segment when ≥1 hint was dropped for EITHER reason (cap or width) (C4). Reuse `accentStyle`/`dimCellStyle`; do NOT use `wrapSegs` (no multi-row).
- [ ] T005 [US1] In `internal/ui/styles.go`: add compact identity builder `footerIdentityCompact(width, ctx, cluster string, writable bool)` → `● <ctx> [RW|RO]` + `· <cluster>` only if it fits (width-capped via `truncate`); retire `footerEndpointLine` from footer use (keep the func only if still referenced, else remove). (Same file as T004 → sequential.)
- [ ] T006 [US1] In `internal/ui/app.go` `footerBlock`: rebuild as identity row + hints row + optional status row (≤ 3 lines); remove the `strings.Repeat("─", w)` separator and the `footerEndpointLine` line; construct `hintCtx` from model (`m.mode`, `m.writable`, `m.selected()`→selKind {none/object/folder via `isDir`}, `m.search`/`m.bucketFilter`→searchActive, `m.searching`, `len(m.contexts)>1`, `m.op!=nil`, `w`) and call the new identity + packer builders. (Depends on T004, T005.)
- [ ] T007 [US1] Run `go test ./internal/ui/ -run 'Footer|Hint'` → Green; then `make fmt vet lint check-readonly`. Fix until all pass.

**Checkpoint**: US1 fully functional — footer is calm, contextual, ≤ 3 rows. MVP shippable.

---

## Phase 4: User Story 2 - Discover every shortcut on demand (Priority: P2)

**Goal**: Single categorized help surface reachable from every mode, listing every action
with all key aliases (derived from `defaultKeys()`), a Connection section housing the
footer-evicted metadata, write actions reflecting `m.writable`, and an explicit close
hint.

**Independent Test**: Open help in each mode; assert the 6 section titles, every
`defaultKeys()` action with aliases, the Connection section (endpoint/region/user/
version), writable reflection, and `press any key to close`. Verifies FR-010..014a.

### Tests for User Story 2 (write FIRST, confirm FAIL) ⚠️

- [ ] T008 [P] [US2] Add help contract tests (contracts/help-surface-contract.md H1–H5, incl. obligations 1–7 + 2a) in new file `internal/ui/keys_test.go`: help in a mode lists all `defaultKeys()` actions grouped under titles Navigation/Search & View/Context/Write/Global, multi-key aliases shown (e.g. `→/l/Enter`, `q/Ctrl+C`); Connection section present with endpoint/region/user/`s3s ver`; **redaction guard (2a)**: Connection renders only the known non-secret `Backend` display fields + `ctxName`/`Version` (assert a distinctive credential-like sentinel never appears in `App.View().Content`); `!writable` → Write actions hidden or marked unavailable; output contains `press any key to close`. Confirm FAIL. ([P]: distinct file from US1/US3 work.)

### Implementation for User Story 2

- [ ] T009 [US2] In `internal/ui/keys.go`: convert `helpLines()` → method `m.helpLines()` returning categorized sections; build the key column for each action from `defaultKeys()` (single source of truth, no drift); group actions into Navigation / Search & View / Context / Write / Global; mark or hide Write actions per `m.writable` (H4); append a Connection section from `m.ctxName`, `m.info.Cluster/Endpoint/Region/User`, `Version` (omit empties; keep redaction); keep the trailing `press any key to close` line.
- [ ] T010 [US2] In `internal/ui/keys.go` `helpView`: render section titles (e.g. `titleStyle`) and rows from `m.helpLines()`; ensure call sites compile (`helpView` already a method on `App`). (Same file as T009 → sequential.)
- [ ] T011 [US2] Run `go test ./internal/ui/ -run 'Help'` → Green; then `make fmt vet lint`. Fix until pass.

**Checkpoint**: US1 + US2 both work — hidden footer shortcuts fully discoverable in help.

---

## Phase 5: User Story 3 - Clearer status, loading & confirmation feedback (Priority: P3)

**Goal**: Loading names what is loading; debounced search shows a pending indicator;
typed-confirm keeps the required target visible with safe mismatch handling; success
notices are green and visually distinct from red errors.

**Independent Test**: Trigger each state; assert loading text differs by mode
(buckets/contents/object), `searching…` pending indicator, notice green vs error red,
typed-confirm shows target + mismatch dispatches nothing. Verifies FR-015..018.

### Tests for User Story 3 (write FIRST, confirm FAIL) ⚠️

- [ ] T012 [US3] Add status feedback tests (contracts/help-surface-contract.md S1–S4) in `internal/ui/app_test.go`: loading line contains `buckets` in modeBuckets, `contents` in modeTree, `object` in modeObject (S1); `searching…` shown while search scheduled-but-unfired (S2); success notice uses green hue and error uses red hue — distinct ANSI (S4); typed-confirm prompt shows the exact required target beside input and a mismatch on submit dispatches no command (S3, regression-locks existing two-tier safety). Confirm FAIL.

### Implementation for User Story 3

- [ ] T013 [US3] In `internal/ui/styles.go`: add `noticeStyle` (foreground `colOK` green) for success notices, distinct from `errStyle` red.
- [ ] T014 [US3] In `internal/ui/app.go` `statusLine`: name the loading target by state (`loading buckets…` / `loading contents…` / `loading object…`, keep `(x to cancel)`); add a `searching…` pending branch while `m.searching` and a debounced search is scheduled-but-unfired; render `m.notice` with `noticeStyle` (green) instead of accent. Leave typed-confirm prompt behavior (`opPromptLine`) intact (already shows the target). (Same file conflict with US1 T006 → run after US1 or coordinate.)
- [ ] T015 [US3] Run `go test ./internal/ui/ -run 'Status|Loading|Notice|Confirm'` → Green; then `make fmt vet lint check-readonly`. Fix until pass.

**Checkpoint**: All three stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T016 [P] Update `README.md` footer/keybindings section to reflect the 3-row footer, contextual hints + `? more`, and the categorized help (incl. Connection section).
- [ ] T017 Run `specs/004-ui-ux-refinement/quickstart.md` manual verification (read-only & `--write`, narrow/wide widths, object vs folder selection, help sections, named loading).
- [ ] T018 Final gate: `make test fmt vet lint check-readonly` all green; confirm full footer test suite passes at widths 40–200 and `scripts/check-readonly.sh` is unaffected (no SDK changes introduced).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: none.
- **Foundational (T002)**: after T001; blocks US1/US3 tests.
- **US1 (T003–T007)**: after T002.
- **US2 (T008–T011)**: after T001 (does not need T002 helpers); independent of US1 (distinct files) — may run in parallel with US1.
- **US3 (T012–T015)**: after T002; shares `styles.go`+`app.go` with US1 → run after US1 (or serialize edits to those files).
- **Polish (T016–T018)**: after all desired stories complete.

### Within Each Story

- Test task FIRST, confirm FAIL → implementation → Green + gates.
- US1: T003 → T004 → T005 → T006 → T007 (T004/T005/T006 all touch styles.go/app.go → sequential).
- US2: T008 → T009 → T010 → T011 (T009/T010 same file → sequential).
- US3: T012 → T013 → T014 → T015.

### Parallel Opportunities

- **US2 ∥ US1**: T008–T011 (keys.go/keys_test.go) are file-disjoint from US1 (styles.go/app.go/footer_test.go) → a second developer can run US2 while US1 proceeds.
- T016 (README) is [P] vs any code task.
- Within a story, impl tasks touch shared files → mostly sequential (no intra-story [P]).

---

## Parallel Example

```bash
# After T002, two developers in parallel:
# Dev A — US1 (footer):    T003 → T004 → T005 → T006 → T007   (styles.go, app.go, footer_test.go)
# Dev B — US2 (help):      T008 → T009 → T010 → T011          (keys.go, keys_test.go)
# Then US3 (status) after US1 frees styles.go/app.go:  T012 → T013 → T014 → T015
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. T001 Setup → T002 Foundational.
2. US1 (T003–T007): the calm contextual footer — the user's primary complaint.
3. **STOP & VALIDATE**: footer ≤ 3 rows, contextual, no overflow at any width. Ship/demo.

### Incremental Delivery

1. Setup + Foundational → baseline + helpers.
2. + US1 → calm footer (MVP) → demo.
3. + US2 → full keymap + connection metadata in help → demo.
4. + US3 → clearer loading/search/notice feedback → demo.
5. Polish → README + quickstart + final gates.

---

## Notes

- All edits confined to `internal/ui`; `make check-readonly` MUST stay green (no SDK writes added).
- [P] = different file, no incomplete-task dependency.
- Verify each test FAILS before implementing (Constitution III).
- Two-tier confirmation + secret redaction preserved (FR-017/020/021) — US3 locks this with a regression test, adds no new behavior.
- Commit after each task or logical group.
