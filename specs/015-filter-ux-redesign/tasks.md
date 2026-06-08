---
description: "Task list for feature 015 — filter UX redesign (always-visible, ergonomic)"
---

# Tasks: Filter UX Redesign (always-visible, ergonomic)

**Input**: Design documents from `specs/015-filter-ux-redesign/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: INCLUDED — TDD is non-negotiable (Constitution III). Failing tests precede implementation.

**Organization**: By user story. The always-visible strip + height-budget reserve is the shared
backbone (Foundational) for US1 (visible) and US2 (fits). The chip is reworked across US1
(term-gated, both visible) then US4 (counts) — sequential edits to the same render functions.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different file / independent, no incomplete dependency)
- **[Story]**: US1..US5 from spec.md
- Exact file paths included. All changes in `internal/ui` (white-box `package ui` tests).

## Path note

Go module `github.com/danchupin/s3s`. Files: `internal/ui/app.go`, `internal/ui/styles.go`,
`internal/ui/search.go`, plus tests (`app_test.go`, `search_test.go`, `footer_test.go`,
`spec013_test.go`). No new package; one new render function (`filterStripView`). UI tests use
`deliver`/`press`/`viewOf`/`stripANSI`; `assertWidthSweep` guards the footer/strip fit invariant.

---

## Phase 1: Setup

- [X] T001 Confirm baseline `make test` is green on branch `015-filter-ux-redesign`, and capture the `deliver`/`press`/`viewOf`/`stripANSI` helpers and `assertWidthSweep` (internal/ui/footer_test.go:92) for reuse in the new tests.

---

## Phase 2: Foundational — always-visible filter strip + height-budget reserve (BLOCKS US1/US2)

**Purpose**: extract the filter input from the transient footer line into a dedicated
always-visible strip, and reserve its row from the LIST (never the footer). The backbone the
visibility (US1) and fit (US2) stories build on.

- [X] T002 [P] Add a failing test `TestFilterStripAlwaysVisible` in `internal/ui/app_test.go`: in `modeBuckets`/`modeTree` with NO active filter and `searching == false`, `App.View().Content` contains the idle filter strip (a `filter`/`/ to filter` line) — RED until T003-T005.
- [X] T003 Add `filterStripView(w int) string` in `internal/ui/app.go` (next to `statusLine`/`footerBlock`): renders one line — active (`searching`) = `▌ filter <pane>: <input>` + caret + live hints; idle = the focused scope's committed term (`▌ filter <pane>: <term>`) or a dim `/ to filter <pane>` placeholder. `<pane>` from `filterIsBucketList()`. Reuse `warnStyle`/`accentStyle`/`dimCellStyle` (no new hue). Elide a long input/term to one line.
- [X] T004 In `internal/ui/app.go` `statusLine()` (1431): DELETE the `m.searching` filter-input case (1450-1459); `statusLine` keeps only loading / notice / error / op-prompt. The input now lives in the strip.
- [X] T005 In `internal/ui/app.go` `View()` (1124): compute `filterStripH := 1` when the (body) mode is `modeBuckets`/`modeTree`, else `0`; change `rows := m.height - footerH - 2` (1142) to subtract `filterStripH` too; render `body + "\n" + m.filterStripView(w) + "\n" + footer` (extend 1209) only in filterable modes. The list (`windowBounds`/`treeView`) absorbs the reserved row.
- [X] T006 [P] In `internal/ui/app_test.go`: migrate `TestStatusSearchPending` (273) to assert the input renders via the strip (not `statusLine`); add `TestStatusLineNeverHasFilterInput` (a notice/error and an active filter coexist — the input is not in `statusLine`).

**Checkpoint**: `go build ./...` green; the strip renders in browse modes; `go test ./internal/ui/` for the new strip tests passes.

---

## Phase 3: US2 — The filter and footer always fit on screen (Priority: P1)

**Goal**: at every supported width/height the filter strip, the chips, and the footer hints are all visible; only the LIST gives up rows.

**Independent test**: shrink to the narrowest/shortest supported size with a filter active — strip, indicator, and footer hints all readable; only the list shows fewer rows.

**Depends on**: Foundational (the strip + reserve).

- [X] T007 [P] [US2] In `internal/ui/footer_test.go`: extend `assertWidthSweep` (92) to also assert the always-visible filter strip is present and ≤1 line at every width; assert the editable input field retains a usable minimum (≥10 visible columns, FR-006 "usable minimum" — quantified) at the narrowest supported width; add an `assertHeightSweep` (decreasing heights) asserting the footer + strip stay fully rendered and only the list-row count decreases. Failing-first.
- [X] T008 [US2] In `internal/ui/app.go` `View()`: verify the `filterStripH` reserve makes the list shrink (not the footer) — `rows`/`dataRows` decrease by one in filterable modes; confirm the `rows < 3` / `dataRows < 1` floors still hold and the footer height is untouched.
- [X] T009 [P] [US2] In `internal/ui/app_test.go`: add `TestNoStripReserveInNonFilterableModes` — `modeObject`/`modeConnections`/`modeConnForm`/`modeUsage` reserve no strip row (full body), so the strip cost is only paid where filtering applies.

**Checkpoint**: width + height sweeps green; the footer never clips; the list is what shrinks. **Part of MVP.**

---

## Phase 4: US1 — The active filter is always visible per-pane (Priority: P1)

**Goal**: each scope's committed filter is shown as a chip on its pane, independent of focus; both chips visible at once.

**Independent test**: apply a bucket filter, move focus to objects — bucket chip stays; apply an object filter — both chips visible at once.

**Depends on**: Foundational.

- [X] T010 [P] [US1] In `internal/ui/spec013_test.go`: migrate `TestBucketFilterChipCommitted` (143) and `TestObjectsFilterChipCommitted` (106) to the always-visible model (the chip shows the committed term regardless of focus); add `TestBothChipsVisibleTogether` (both committed chips present at once, focus-agnostic). Failing-first.
- [X] T011 [US1] In `internal/ui/app.go`: make the chip TERM-GATED + zone-agnostic — `bucketFilterChip` (1317) renders whenever `m.bucketFilter != ""`, `objectsFilterChip` (1320) whenever `m.search != ""`, INDEPENDENT of `focusZone`; a scope's chip is hidden only while THAT scope is being actively edited (`m.searching` on that scope), so its live term shows only in the strip. Update `filterChipText` (1309)'s `m.searching` gate to be per-scope, not global.
- [X] T012 [US1] In `internal/ui/app.go` `listWithPane` (1251): the two-pane layout MUST chip each box independently — the bucket box passes `bucketFilterChip()`, the objects box passes `objectsFilterChip()` — so BOTH chips render simultaneously regardless of focus. `primaryFilterChip` (1324, the mode-dependent single chip) is retained ONLY for single-pane modes (e.g. full-screen `modeTree` without the split); it is NOT used to pick one chip in the two-pane layout.

**Checkpoint**: both committed chips visible at once, surviving focus changes; spec013 chip tests green. **Part of MVP.**

---

## Phase 5: US3 — Both filter scopes are preserved (Priority: P1)

**Goal**: the bucket instant-local filter and the object within-prefix filter both work and are independent.

**Independent test**: filter buckets (instant), switch to objects and filter (within prefix); each narrows; clearing one does not affect the other.

**Depends on**: Foundational (the lifecycle is shared).

- [X] T013 [P] [US3] In `internal/ui/search_test.go`: confirm `TestSearchNarrowsLevel` (11), `TestSearchClearRestores` (62), `TestSearchEnterConfirmsAndCloses` (87) stay green with the strip; add `TestDualScopeIndependent` — committing a bucket filter and an object filter leaves both committed and independent (clearing one does not change the other); add `TestObjectSearchSupersedes` — a fast keystroke burst coalesces and a stale (older-gen) `searchFireMsg` does NOT replace a newer result (FR-013, the existing `searchGen` guard in `onSearchFire`).
- [X] T014 [US3] In `internal/ui/search.go`: confirm `filterIsBucketList`/`committedFilterTerm`/`runSearch` scope logic is unchanged and drives the strip + both chips correctly (no scope merged or dropped); the object search stays debounced/non-blocking (`scheduleSearch`/`onSearchFire`).

**Checkpoint**: both scopes functional + independent; existing filter tests green. **Completes the P1 MVP (Foundational + US2 + US1 + US3).**

---

## Phase 6: US4 — Live narrowing with a match count (Priority: P2)

**Goal**: typing narrows live and a match count shows — bucket `matched/total`, object `N matched`.

**Independent test**: type a bucket filter and watch the count update; type an object filter and confirm it narrows without freezing.

**Depends on**: US1 (chip rendering).

- [X] T015 [P] [US4] In `internal/ui/spec013_test.go` (or `search_test.go`): add `TestChipShowsMatchCount` — a committed bucket filter chip reads `filter: <term> · M/T`, a committed object chip reads `filter: <term> · N`; add a live-count-while-typing assertion. Failing-first.
- [X] T016 [US4] In `internal/ui/app.go`: extend `filterChipText` (1309) to `(term string, matched, total int, hasTotal bool)` → `filter: <term> · M/T` when `hasTotal`, else `filter: <term> · N`. `bucketFilterChip` passes `len(filteredBuckets()), len(m.buckets), true`; `objectsFilterChip` passes `m.level.count(), 0, false`. No new network call (FR-013).
- [X] T017 [US4] In `internal/ui/styles.go` / `internal/ui/app.go`: ensure `filterChipTermMax` (1304) budgets for the ` · M/T` suffix (elide the TERM first); the `boxViewWith` (335) degrade order is unchanged (center → filter → mode; mode survives; the filter chip drops WHOLE under width pressure — the strip still shows the active filter).

**Checkpoint**: counts render correctly + live; chips elide gracefully; degrade order intact.

---

## Phase 7: US5 — Refine and clear are obvious (Priority: P2)

**Goal**: re-open pre-filled, cancel reverts, clear removes the chip; the always-visible strip reflects each state.

**Independent test**: apply → re-open (pre-filled) → cancel (revert) → clear (chip + count gone, full view).

**Depends on**: Foundational (the strip renders the lifecycle states).

- [X] T018 [P] [US5] In `internal/ui/search_test.go`: add `TestFilterStripLifecycle` — idle strip shows the committed term (or placeholder); `/` re-opens pre-filled (`TestSearchClearRestores` already covers Esc-revert — extend to assert the strip reflects the revert); clearing the term removes the chip and the strip returns to the idle placeholder; add `TestNavigateAwayClearsFilter` — going up a level / switching context clears that level's filter and removes its chip + resets the strip (FR-016, existing `goBack`/`objectsBack`/ctx-switch behavior, now also resetting the strip). Failing-first where new.
- [X] T019 [US5] In `internal/ui/app.go` `filterStripView`: confirm the idle appearance (committed term vs `/ to filter <pane>` placeholder), the pre-filled `startSearch` (search.go:24) input, and the Esc-revert / clear transitions all render correctly in the strip (no caret when idle; caret when `searching`).

**Checkpoint**: the full set/refine/revert/clear loop reads cleanly in the always-visible strip.

---

## Phase 8: Polish & Cross-Cutting

- [X] T020 Run `make fmt vet lint`; resolve any issues (alignment, unused).
- [X] T021 Run `make check-readonly` — confirm green (no S3 symbol introduced).
- [X] T022 Run `go test -race ./...` and the full `internal/ui` suite — all green; confirm `TestFooterWidthSweepNoOverflow` and the new height sweep pass.
- [X] T023 [P] In `internal/ui/spec013_test.go`: extend `TestChipsTextNoColor` (218) to assert the match-count text is readable under `NO_COLOR` (text, not color alone — SC-007); final sweep against SC-001..007.

---

## Dependencies & Execution Order

```
Setup (T001)
  └─ Foundational: always-visible strip + reserve (T002-T006)   # backbone for US1/US2
       ├─ US2 (T007-T009)   # P1 — fits; list shrinks not footer        ┐
       ├─ US1 (T010-T012)   # P1 — both chips visible, focus-agnostic    ├─ MVP (P1)
       └─ US3 (T013-T014)   # P1 — both scopes preserved + independent   ┘
            └─ US4 (T015-T017)   # P2 — match count on chips (after US1 chip rework)
                 └─ US5 (T018-T019)  # P2 — refine/clear lifecycle in the strip
                      └─ Polish (T020-T023)
```

- **Foundational → US1/US2/US3**: all build on the strip + reserve.
- **US1 → US4**: the count edits the same chip functions US1 reworked (sequential, same file).
- US2/US1/US3 are independently testable P1 increments → the MVP.

## Parallel Opportunities

- Foundational: T002 + T006 (test files) parallel to the impl edits (T003-T005, same app.go → sequential).
- US2: T007 + T009 (different test files).
- US1: T010 (test) parallel to T011/T012 (app.go impl, sequential).
- US4: T015 (test) parallel to T016/T017 (impl, sequential within app.go/styles.go).
- US5: T018 (test) parallel to T019 (impl).
- Polish: T023 parallel to the gate runs.

## Implementation Strategy

**MVP = Foundational + US2 + US1 + US3** (the three P1 stories): an always-visible filter strip
that always fits (list shrinks, footer never), both committed-filter chips visible at once, and
both scopes preserved and independent — directly resolving the user's "always visible + doesn't
fit" complaint. Layer US4 (match count) then US5 (lifecycle polish) on top. Note US1 and US4 both
edit the chip render functions, so land US1's term-gating before US4's count to avoid rework.
