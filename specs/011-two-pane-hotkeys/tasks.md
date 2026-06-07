---
description: "Task list for 011-two-pane-hotkeys"
---

# Tasks: Two-Pane Browse + Hotkey Mnemonic Review

**Input**: Design documents from `/specs/011-two-pane-hotkeys/`

**Prerequisites**: plan.md, spec.md, research.md (R1–R10), data-model.md, contracts/ (keymap-contract, two-pane-layout, lazy-load-cache), quickstart.md

**Tests**: REQUIRED — Constitution III (Test-First, NON-NEGOTIABLE). Every story's tests are written FIRST and MUST fail before implementation (Red→Green→refactor). All tests are white-box `package ui` (`deliver`/`press`/`viewOf` helpers, in-memory `storage.Fake`); no integration test (no storage-contract change — research R10).

**Organization**: grouped by user story for independent implementation/testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[Story]**: US1/US2/US3/US4 (Setup/Foundational/Polish carry no story label)
- Exact file paths included. All paths relative to repo root.

## Path Conventions

Single Go project. Source in `internal/ui/`, `internal/storage/`, `internal/cache/`. White-box tests live beside source in `internal/ui/*_test.go`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: test infrastructure needed before any test-first story work.

- [ ] T001 [P] Add a read-only list-call counter to `storage.Fake` (count `ListBuckets` + `ListLevel` invocations, expose a `Calls()`/`ListLevelCalls()` getter; read-only — `check-readonly` must stay green) in `internal/storage/fake.go`
- [ ] T002 [P] Add a width-parametrized render helper for tier tests (e.g. `viewAtWidth(m, w, h)` building on the existing `viewOf`/`deliver` so a test can drive `WindowSizeMsg{w,h}` and assert on `App.View().Content`) in `internal/ui/helpers_test.go` (confirm against existing helpers before adding)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: shared layout/state scaffolding every layout story builds on.

**⚠️ CRITICAL**: US1/US2/US3 cannot begin until this phase is complete. (US4 is independent and may start in parallel — see Dependencies.)

- [ ] T003 [P] Write failing unit test for `layoutTier(w)` boundaries — Full ≥130, Dual 100–129, Single ≤99 (assert 130/129/100/99) in `internal/ui/styles_test.go`
- [ ] T004 Add `layoutTier(w)` classifier returning Full/Dual/Single per the normative tiers in `internal/ui/styles.go` (reuse existing `paneSplitMin=100` for the Single boundary)
- [ ] T005 Add focus state to the `App` struct — `focusZone` (zoneBuckets|zoneObjects, default zoneBuckets) + `bucketLoadGen int` (bucket→objects reload debounce counter). The objects zone REUSES `m.level` (content) and `m.treeSel` (cursor) — NO new level/cursor field — per data-model.md / research R6 reconciliation — in `internal/ui/app.go`
- [ ] T006 [P] Add active/dim zone style tokens (focused zone border+title use the accent style, unfocused use the dim style) in `internal/ui/styles.go`

**Checkpoint**: layout tier + focus/objects state available — story phases can begin.

---

## Phase 3: User Story 1 — Peek a bucket's contents without opening it (Priority: P1) 🎯 MVP

**Goal**: highlighting a bucket lazily loads its first-level entries into a bordered objects zone beside the bucket list (Dual tier), with no Enter; startup lists only bucket names; loads are debounced and cached.

**Independent Test**: at ≥100-col width, move the bucket cursor — the objects zone shows the newly-highlighted bucket's first-level entries; startup issued zero object listings; fast scroll fetches only the settled selection; revisits hit cache. (SC-001/003/004/010)

### Tests for User Story 1 (write first — MUST fail) ⚠️

- [ ] T007 [P] [US1] Failing test: entering the browse screen with K buckets issues exactly one bucket-name listing and **zero** object-level listings (`Fake` counter) in `internal/ui/lazyload_test.go`
- [ ] T008 [P] [US1] Failing test: at Dual width, highlighting a bucket renders that bucket's first-level folders+objects in the objects zone (`viewAtWidth` asserts zone title + entries) in `internal/ui/twopane_test.go`
- [ ] T009 [P] [US1] Failing test: fast scroll across N buckets issues ≤1 `ListLevel` (settled-only; `Fake` counter) in `internal/ui/lazyload_test.go`
- [ ] T010 [P] [US1] Failing test: re-highlighting a previously-viewed bucket issues **zero** additional listings (cache hit) in `internal/ui/lazyload_test.go`
- [ ] T011 [P] [US1] Failing test: empty bucket → `(empty)`; in-flight → `loading…`; denied (`AccessDeniedBuckets`) → `error:` line AND a revisit re-attempts (not cached) in `internal/ui/lazyload_test.go`

### Implementation for User Story 1

- [ ] T012 [US1] Add the objects-zone debounce: bump `bucketLoadGen` on bucket-cursor move and schedule a settle tick (reuse `paneDebounce` 180 ms; mirror `afterSelectionMove`/`onPaneTick`) — `bucketTickCmd(gen,bucket)` + `bucketTickMsg` — in `internal/ui/commands.go` and `internal/ui/messages.go`
- [ ] T013 [US1] On settle (`bucketLoadGen`+bucket match), load the bucket's first level **into `m.level`** reusing `loadLevel` + `levelKeyFor(bucket,"")` + cache `Get`/`Put` (reset `treeSel=0` as `enterLevel`); reuse the existing `onLevel` (cache only on success `levelMsg`; failures arrive as `errMsg`, uncached, so revisit re-attempts) — in `internal/ui/tree.go` and `internal/ui/app.go`
- [ ] T014 [US1] Compose the Dual tier in `listWithPane`: buckets box │ objects box via `lipgloss.JoinHorizontal`, objects rendered from `m.level` windowed by `windowBounds`, both in `boxView` rounded borders — in `internal/ui/app.go`
- [ ] T015 [US1] Render the objects zone + its explicit states (`(empty)` / `loading…` / `error:`) in a new `objectsView` helper (reuses the `renderTable`/`windowBounds` level-render path on `m.level`), called from `listWithPane`, without disturbing the bucket list — in `internal/ui/app.go`
- [ ] T016 [US1] Wire bucket-cursor movement (`onBucketsKey` up/down/top/bottom) to bump `bucketLoadGen` and schedule the settle tick; ensure startup `loadBuckets` stays names-only (no eager object listing) — in `internal/ui/app.go`

**Checkpoint**: US1 independently testable — MVP deliverable.

---

## Phase 4: User Story 2 — Navigate into the objects pane and drill (Priority: P1)

**Goal**: focus crosses from buckets into the objects zone (`→`/`l`/`Tab`/Enter), navigates + drills folders there with the bucket list staying put, and returns via `←`/`h`/`Esc`/`Tab`.

**Independent Test**: with contents shown, `→`/`Tab` moves focus into objects (active-zone style switches); folder Enter descends the objects zone; object Enter opens full-screen; `←`/`Esc` ascends-or-returns per precedence; `Tab` toggles both ways preserving cursors. (SC-002)

### Tests for User Story 2 (write first — MUST fail) ⚠️

- [ ] T017 [P] [US2] Failing test: `→`/`Tab` from the bucket list moves focus into the objects zone (active-zone style asserted) and the objects cursor moves independently of `bucketSel` in `internal/ui/focus_test.go`
- [ ] T018 [P] [US2] Failing test: Enter/`→` on a folder descends the objects level (`m.level`, `treeSel`); the bucket list (and `bucketSel`) is unchanged in `internal/ui/focus_test.go`
- [ ] T019 [P] [US2] Failing test: Enter on an object opens `modeObject`; leaving it restores the objects-zone position + focus in `internal/ui/focus_test.go`
- [ ] T020 [P] [US2] Failing test: `←`/`Esc` precedence in objects zone — clears active search first, else ascends one level, else (at root) returns focus to buckets in `internal/ui/focus_test.go`
- [ ] T021 [P] [US2] Failing test: `Tab` is a symmetric toggle — from a deep objects level it jumps straight back to buckets, both zone cursors preserved in `internal/ui/focus_test.go`

### Implementation for User Story 2

- [ ] T022 [US2] Add focus-aware key dispatch in `Update`: branch on `focusZone`; `→`/`l`/`Tab`/Enter-on-bucket cross into objects; `←`/`h`/`Esc` follow FR-009 precedence; `Tab` toggles both ways — in `internal/ui/app.go`
- [ ] T023 [US2] Objects-zone cursor navigation + drill on `m.level`/`treeSel`, gated by `focusZone == zoneObjects`: reuse the existing level key logic (`onTreeKey` cursor, folder `enterLevel`, ascend `goBack`, paging via `fetchNextPage`). FR-011 search/marking/sorting carry over from this reuse; **marks/sort scoping is FR-011 derived consistency and MAY be deferred** per spec (cursor + drill + open are the must) — in `internal/ui/tree.go`
- [ ] T024 [US2] Apply active/dim zone styling per `focusZone` in `listWithPane` (focused zone border/title = accent) in `internal/ui/app.go`
- [ ] T025 [US2] Open `modeObject` from the objects zone on Enter-on-object and restore focus+position on return in `internal/ui/app.go`

**Checkpoint**: US1+US2 = fully navigable two-zone browse.

---

## Phase 5: User Story 3 — Preserve the details pane as an adaptive third zone (Priority: P2)

**Goal**: at the Full tier (≥130) a third details zone is shown, adapting to focus — bucket metadata when a bucket is focused, object metadata + bounded preview when an object is focused; it collapses first under width pressure.

**Independent Test**: at ≥130 cols all three zones render; details = bucket-meta on a focused bucket, object-meta+preview on a focused object; narrowing to Dual collapses details (object details still reachable via full-screen Enter). (US3 acceptance)

### Tests for User Story 3 (write first — MUST fail) ⚠️

- [ ] T026 [P] [US3] Failing test: at Full width three zone titles render and details shows bucket metadata while focus is on a bucket in `internal/ui/details_test.go`
- [ ] T027 [P] [US3] Failing test: with focus in the objects zone on an object, details shows object metadata + bounded preview in `internal/ui/details_test.go`
- [ ] T028 [P] [US3] Failing test: narrowing from Full to Dual collapses the details zone (buckets│objects remain) in `internal/ui/details_test.go`

### Implementation for User Story 3

- [ ] T029 [US3] Full-tier composition: append the details box as a third `JoinHorizontal` zone (reuse pane width math) in `internal/ui/app.go`
- [ ] T030 [US3] Make `paneView` adaptive to `focusZone`: bucket metadata when a bucket is focused, object metadata + preview when an object is focused (reuse existing field set) in `internal/ui/pane.go`
- [ ] T031 [US3] Drive the details object-metadata load off the objects-zone selection reusing the existing pane debounce/gen (no new load machinery) in `internal/ui/commands.go`

**Checkpoint**: full three-zone master-detail at wide widths; graceful collapse.

---

## Phase 6: User Story 4 — Hotkey clarity: mnemonic review + bold glyphs (Priority: P2)

**Goal**: remove the illogical global `AddConn` `n` (the "+ add connection" row stays the affordance), render every advertised key glyph bold in the hint bar + help, keep an NO_COLOR-safe cue, and keep the keymap single-sourced.

**Independent Test**: pressing `n` on the bucket list no longer opens the add-connection form (the row still does); every advertised key renders bold; under `$NO_COLOR` keys stay distinguishable; navigation/locked keys unchanged. (SC-005/006)

**NOTE**: independent of the layout track — touches `keys.go`/`hintbar.go`/`connections.go` only; may run in parallel with Phases 3–5.

### Tests for User Story 4 (write first — MUST fail) ⚠️

- [ ] T032 [P] [US4] Failing test: pressing `n` on the bucket list does NOT open the add-connection form, and the "+ add connection" row (connections list) still opens it in `internal/ui/keys_test.go`
- [ ] T033 [P] [US4] Failing test: every action key advertised in the hint bar and help is rendered with a bold ANSI attribute in `internal/ui/keys_test.go`
- [ ] T034 [P] [US4] Failing test: with `$NO_COLOR` set, each advertised key remains distinguishable from its label via the non-color cue in `internal/ui/keys_test.go`
- [ ] T035 [P] [US4] Failing test: navigation keys (arrows, `hjkl`, `gG`) and locked keys (Enter/Esc/`q`/`?`/`:`/Space) are unchanged in `defaultKeys` in `internal/ui/keys_test.go`

### Implementation for User Story 4

- [ ] T036 [US4] Remove `AddConn` from `defaultKeys` and delete its dispatch (the `matches(key, m.keys.AddConn)` branch ~`app.go:604`); the "+ add connection" row (`connections.go:103`) remains the sole affordance — in `internal/ui/keys.go` and `internal/ui/app.go`
- [ ] T037 [US4] Render key glyphs bold: add `.Bold(true)` to the key style used by `keyGlyph`/`formatKeys`, the hint bar key render, and `helpLines` (single keymap source preserved) — in `internal/ui/keys.go` and `internal/ui/hintbar.go`
- [ ] T038 [US4] Ensure an NO_COLOR-safe non-color cue for keys (bold attribute survives color stripping and/or a stable delimiter) in `internal/ui/keys.go`/`internal/ui/hintbar.go`
- [ ] T039 [US4] Record the migration in the help surface (note `n` removed → "+ add connection" row only) in `internal/ui/keys.go`

**Checkpoint**: keymap reviewed, `n` gone, all keys bold, NO_COLOR-safe.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T040 [P] Resize-reflow test: crossing Full↔Dual↔Single boundaries preserves the highlighted bucket, the objects cursor, and `focusZone`, and never breaks a border in `internal/ui/twopane_test.go` (SC-008)
- [ ] T041 [P] Footer-always-visible test: at the minimum row budget the identity + hint line (incl. help/quit) stay on screen across tiers in `internal/ui/footer_test.go` (FR-017)
- [ ] T042 Run `make check-readonly` — MUST stay green (no new write-S3 symbol, no new `storage.Storage` method) (SC-009)
- [ ] T043 [P] Run `make fmt vet lint` — zero issues (golangci-lint built with the module toolchain)
- [ ] T044 Manual smoke per quickstart.md at ≥130 / 100–129 / ≤99 cols (three-zone / two-zone / single-column parity); the ≤99 pass MUST exercise the full Single-tier flow buckets → Enter → objects → Enter → object (SC-007)
- [ ] T045 [P] Update any user-facing keybinding docs (README/help blurb) for the `n` removal + bold keys
- [ ] T046 Coverage check: `go test -cover ./internal/ui/` maintains or improves the package coverage baseline
- [ ] T047 [P] Single-tier parity test (SC-007): at ≤99 cols, assert the browse flow is behaviourally identical to today — Enter-on-a-bucket drills full-screen (`modeTree`), no objects zone rendered, `focusZone` inert — in `internal/ui/twopane_test.go`

---

## Dependencies & Execution Order

- **Setup (P1)** → **Foundational (P2)** → stories.
- **US1 (P3)** depends on Foundational (T003–T006). MVP.
- **US2 (P4)** depends on US1 (objects zone must render) + Foundational focus state.
- **US3 (P5)** depends on US1+US2 (focus drives the adaptive details zone).
- **US4 (P6)** depends only on Setup — **independent of the layout track**; can run in parallel with US1–US3 (disjoint files).
- **Polish (P7)** after all targeted stories; T042/T043 gate the PR.
- Within a story: all `[P]` test tasks first (they must fail), then implementation. Implementation tasks that touch the same file (`app.go`) are sequential; cross-file impl tasks may parallelize.

## Parallel Execution Examples

- **Foundational**: T003 (test) ∥ T006 (styles) while T004→T005 proceed (T004 before T005; both in different files than T006).
- **US1 tests**: T007 ∥ T008 ∥ T009 ∥ T010 ∥ T011 (independent test files/cases) — all authored together, all red.
- **Cross-story**: the entire US4 phase (T032–T039) runs in parallel with US1–US3 — different files (`keys.go`/`hintbar.go` vs `app.go` layout).
- **Polish**: T040 ∥ T041 ∥ T043 ∥ T045 (independent).

## Implementation Strategy

- **MVP = US1** (Phases 1→2→3): lazy two-zone browse with caching. Shippable alone; Enter still drills full-screen until US2 lands.
- **Increment 2 = US2**: focus crossing makes it a real navigable two-pane.
- **Increment 3 = US3**: adaptive details third zone at wide widths.
- **Parallel track = US4**: hotkey clarity — the cheap, decoupled win; can land first or alongside any increment.
- Every story is TDD: red tests → implement → green → refactor; `make check-readonly` + `fmt vet lint` green before each PR (Constitution III/V).
