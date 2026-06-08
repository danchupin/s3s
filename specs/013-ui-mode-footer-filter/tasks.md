---
description: "Task list for feature 013 — UI mode chip dedup, footer breathing room, applied-filter state"
---

# Tasks: UI mode chip dedup, footer breathing room, applied-filter state

**Input**: Design documents from `specs/013-ui-mode-footer-filter/`

**Prerequisites**: plan.md, spec.md, research.md (R1–R7), data-model.md (E1–E6), contracts/ (border-chip,
mode-indicator, applied-filter, footer-spacing, layout-visibility)

**Tests**: REQUIRED. TDD is non-negotiable (constitution III) — every behavior change starts with a
failing white-box test (`package ui`, `deliver`/`press`/`viewOf`/`stripANSI`; builders `dualApp`/`treeApp`/
`buildApp`/`withBuckets`/`crossToObjects`/`selectObject`). All new tests live in
`internal/ui/spec013_test.go` (one file → tests within it are sequential, NOT `[P]` among themselves).

**Organization**: by user story. All paths are repo-relative; all changes are in `internal/ui`.
`internal/{storage,cache,preview,config,logging}` are UNCHANGED. `make check-readonly` MUST stay green.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different file, no dependency on an incomplete task)
- **[Story]**: US1 / US2 / US3 (setup, foundational, polish carry no story label)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: confirm a clean, green baseline before the TDD cycle begins.

- [x] T001 Confirm baseline green — run `make test fmt vet lint check-readonly` from repo root and record the
  current passing state (so later red tests are unambiguously the new failing-first tests, not pre-existing
  breakage).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: extend the shared border-chip composer so both US1 (object-view mode chip via the wrappers) and
US2 (a filter chip alongside the mode chip) build on a STABLE two-chip wrapper signature — avoiding mid-story
signature rework. Governs `contracts/border-chip-contract.md` (C1–C8). US3 does not depend on this phase.

**⚠️ CRITICAL**: no user-story work begins until this phase is complete and existing tests are green again.

- [x] T002 [P] Failing-first test for the two-chip border slot in `internal/ui/spec013_test.go`: assert a box
  rendered with BOTH a filter chip and a mode chip shows both on the first border line with the mode chip
  right-most (C1/C2); assert the degrade order at a too-narrow width — center label dropped first, then the
  filter chip, the mode chip surviving last (C3/C7); assert the first border line width ≤ `width` at every
  width swept (C4/C8). Tests MUST fail (single-slot today).
- [x] T003 Extend `boxViewWith` (`internal/ui/styles.go:334-406`) to accept a second, INBOARD chip in
  addition to the existing right-most chip: render order `… dashes …  ‹filterChip›  ‹modeChip› ╮`; subtract
  the sum of both rendered chip widths from `avail`/dash budget (C4); degrade center → filter chip → mode
  chip-last (C3). Mode chip text/style unchanged.
- [x] T004 Thread the new slot through the wrappers in `internal/ui/styles.go:299-320`
  (`boxViewChip`/`boxViewFocusChip`, and a chip-bearing focus variant usable by the objects pane), and update
  the THREE existing chip call sites to pass the mode chip + an EMPTY filter chip (no behavior change yet):
  `internal/ui/app.go:1256` (single/collapsed), `:1270` (buckets zone), `:1286` (tree list).
- [x] T005 Re-run `make test` — confirm the existing suite is green again (T002 may still assert the new
  capability; the EMPTY-filter-chip wiring must not alter any existing rendered output).

**Checkpoint**: two-chip border plumbing in place, existing behavior unchanged.

---

## Phase 3: User Story 1 — One read/write mode indicator, never duplicated (Priority: P1) 🎯 MVP

**Goal**: keep the border mode chip on EVERY browse box; remove the duplicate footer `[RW]/[RO]` tag; modal
& help badges stay. Governs `contracts/mode-indicator-contract.md`.

**Independent Test**: open RO and armed-write contexts; the mode shows once (the chip) on the bucket list,
object level, AND opened object; the footer carries no `[RW]/[RO]` tag; modal confirmations still show their
badge.

### Tests for User Story 1 (write FIRST, ensure they FAIL)

- [x] T006 [US1] Failing test in `internal/ui/spec013_test.go`: open the object view (`treeApp`/`dualApp`,
  `selectObject`, `press "enter"`, `m.loading=false`); assert the first border line of `viewOf(m)` contains
  `RO` (read-only) and `WRITE` (armed via `buildApp`/arm flow). Fails today (app.go:1178 plain `boxView`,
  M1).
- [x] T007 [US1] Failing test in `internal/ui/spec013_test.go`: on a chip-bearing browse view, assert the
  footer (`m.footerBlock(w)` and `footerIdentityCompact(...)`) contains neither `[RW]` nor `[RO]` (M3); and
  assert the mode indicator appears exactly once in `viewOf(m)` (SC-001).

### Implementation for User Story 1

- [x] T008 [US1] In `internal/ui/app.go:1177-1178` (`modeObject` render) switch `boxView(...)` →
  `boxViewChip(m.resourceTitle(), m.objectKind(), <empty filter chip>, m.modeChip(), m.objectView(w-2,rows),
  w, rows)` so the opened-object box carries the mode chip (M1).
- [x] T009 [US1] In `internal/ui/styles.go:512-524` remove the `[RW]/[RO]` tag from `footerIdentityCompact`;
  identity row becomes `● ctx · cluster`; drop the now-unused `writable bool` param (M3).
- [x] T010 [US1] Update the three `footerIdentityCompact` callers for the dropped param:
  `internal/ui/app.go:1382` (`footerBlock` non-list branch), `internal/ui/commandbar.go:185` (`infoColumn`),
  `internal/ui/commandbar.go:254` (`collapsedBarView`).
- [x] T011 [US1] Migrate the existing footer `[RW]/[RO]` assertions to the chip (`WRITE`/`RO`) — NOT
  regressions, expected churn: `internal/ui/operation_test.go:168/171` (TestFooterWriteTag),
  `internal/ui/visual_test.go:29/39`, `internal/ui/writemode_test.go:132/135/138/144/149`
  (TestWriteBadgeOnEveryScreen — KEEP its help branch on `writeBadge`), `internal/ui/spec012_test.go:145/148`
  (move the badge space-color invariant onto the chip). Confirm modal-badge tests (confirmview/arm) still
  assert `writeBadge` (M5).

**Checkpoint**: mode shown once on every browse box; footer tag gone; modal/help badges intact. MVP testable.

---

## Phase 4: User Story 2 — Applied filter state stays visible (Priority: P1)

**Goal**: a persistent `filter: <term>` chip on the FILTERED pane's box border when committed + input closed;
distinct from the typing input; cleared with the filter. Governs `contracts/applied-filter-contract.md`.
Depends on Phase 2 (two-chip slot).

**Independent Test**: commit a filter on the objects level and on the bucket list; the term shows as a chip
on the matching pane's border, hidden while typing, gone after clear; no extra backend load.

### Tests for User Story 2 (write FIRST, ensure they FAIL)

- [x] T012 [US2] Failing test in `internal/ui/spec013_test.go` (objects level): `dualApp` + `f.Seed`;
  `crossToObjects`; `press "/"`, type, `press "enter"`; assert the objects-box border line contains
  `filter:`+term (F2/F4); assert ABSENT while `m.searching` (F8); assert GONE after clear (`press "esc"`/back,
  F9); bracket with `f.ListLevelCalls` to prove zero new backend calls (F12). Long-term case (FR-012): commit
  a long term → assert the chip term is capped with a trailing `…` AND every footer/border line ≤ w (no wrap);
  then re-open the filter (`press "/"`) and assert the input is pre-filled with the FULL committed term
  (`startSearch`, search.go:22-27 — the term's reveal path; NOT the `i` reveal popup, which reveals the
  selected row, not the filter).
- [x] T013 [US2] Failing test in `internal/ui/spec013_test.go` (bucket list): focus buckets, `press "/"`,
  type, `press "enter"`; assert the buckets-box border contains `filter:`+term backed by `m.bucketFilter`
  (F1); assert the objects box is unaffected; assert the chip is distinct from the transient `statusLine`
  input.

### Implementation for User Story 2

- [x] T014 [US2] Add a per-pane filter-chip helper (in `internal/ui/search.go` or `internal/ui/app.go`):
  `bucketFilterChip()` shown iff `m.bucketFilter != "" && !m.searching`; `objectsFilterChip()` shown iff
  `m.search != "" && !m.searching`; format `filter: <term>` styled `warnStyle`, term capped with a trailing
  `…` BEFORE return (F1/F2/F4/F5, C-term). Use each pane's OWN field — NOT focus-relative
  `committedFilterTerm()` (F-predicate / R3 risk).
- [x] T015 [US2] Thread the filter chip into `listWithPane` (`internal/ui/app.go:1251-1289`): pass
  `bucketFilterChip()` to the buckets box (`boxViewFocusChip` Dual/Full app.go:1270; single `boxViewChip`
  app.go:1256); switch the objects pane (`boxViewFocus` app.go:1277/1282) to the chip-bearing variant and pass
  `objectsFilterChip()`; pass `objectsFilterChip()` to the tree/single primary box (app.go:1286/1256). Mode
  chip stays the right-most slot (C2).
- [x] T016 [US2] Remove the breadcrumb-embedded filter markers (now owned by the chip, F11): drop the
  ` (term*)` suffix in `objectsZoneTitle` (`internal/ui/app.go:1354-1356`) and the `/term*` suffix in
  `resourceTitle` (`internal/ui/app.go:1478-1479`); KEEP the `[count]` suffix.

**Checkpoint**: committed filter visible as a per-pane border chip; auto-cleared; presentation-only.

---

## Phase 5: User Story 3 — Breathing room in the footer / command bar (Priority: P2)

**Goal**: widen separators / key↔label / inter-column gaps consistently across wide + collapsed paths without
breaking the no-wrap/no-scroll invariant. Governs `contracts/footer-spacing-contract.md`. Independent of
Phases 2/3/4 (but shares `styles.go`/`commandbar.go` with US1 — see Dependencies).

**Independent Test**: at wide and narrow widths the footer/command-bar gaps are visibly larger and no two
elements merge; resizing 40→200 and shrinking height never wraps or scrolls a footer line off.

### Tests for User Story 3 (write FIRST, ensure they FAIL)

- [x] T017 [US3] Failing test in `internal/ui/spec013_test.go`: `treeApp(f,true)`, `m.width=140`; on
  `stripANSI(m.commandBarView(140))` assert the widened separator (`  ·  `) and the widened inter-column gap
  are present (S1/S3); add a boundary test at a width where the OLD natural width fit but the NEW one does not
  → the bar now renders the collapsed 3-row form (proves S4 was updated, not just the literal).
- [x] T018 [P] [US3] Failing/guard test in `internal/ui/spec013_test.go` reusing the width-sweep pattern:
  assert every footer line ≤ w and ≤ 9 rows across 40..200 AFTER widening (mirrors `assertWidthSweep`,
  footer_test.go:150) — pins S5/S7/L3.

### Implementation for User Story 3

- [x] T019 [US3] Introduce ONE package-level separator token in `internal/ui/styles.go` (or `commandbar.go`),
  ` · ` (w3) → `  ·  ` (w5), and replace the hardcoded literals at `internal/ui/commandbar.go:63` (barGlobals),
  `:262` (collapsed globals), `:276` (fitEntries), `internal/ui/styles.go:469/472` (renderHintRow),
  `:518/521` (footerIdentityCompact), `internal/ui/pane.go:54/71` (details hints) (S1).
- [x] T020 [US3] Widen the key↔label gap in `entryStyled` (`internal/ui/commandbar.go:159`) from 1 to 2
  spaces (S2).
- [x] T021 [US3] Introduce `colGap` const (2→3 spaces) in `internal/ui/commandbar.go`; replace the inter-
  column gap at `:179` (`JoinHorizontal(Top, info, colGap, read, colGap, write)`) AND the natural-width term
  at `:175` with `2*len(colGap)` (removes the `+4` double-count) (S3/S4).

**Checkpoint**: gaps visibly larger on every tier; footer never wraps/scrolls.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: verify the cross-cutting invariants (`contracts/layout-visibility-contract.md`) and run the
final gates.

- [x] T022 [P] Confirm the width/visibility guards stay green with both chips mounted and spacing widened:
  `internal/ui/footer_test.go` (`assertWidthSweep` 40..200 ≤9 rows, TestFooterFitsWidthAndShowsHints w=60,
  narrow-drop w=30, TestBoxLongTitleNoOverflow), `internal/ui/spec012_test.go` TestFooterVisibleAcrossTiers,
  `internal/ui/polish_test.go` TestFooterVisibleMinHeight (L1–L3).
- [x] T023 [P] NO_COLOR pass (`internal/ui/spec013_test.go` or extend us4_test patterns): assert the mode
  chip (`RO`/`WRITE`) and the filter chip (`filter:`+term) carry their state as text under `stripANSI`
  (SC-008, FR-006).
- [x] T024 Run `make fmt vet lint check-readonly` — all green; confirm no new write-capable S3 symbol and no
  new storage method (FR-018 / SC-007).
- [ ] T025 Run the `specs/013-ui-mode-footer-filter/quickstart.md` walkthrough manually against a real
  context (RO and write-capable) to confirm all acceptance scenarios.

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (T001)** — no deps.
- **Foundational (T002–T005)** — depends on Setup; BLOCKS US1 & US2 (the two-chip wrapper signature). US3
  does NOT depend on it.
- **US1 (T006–T011)** — depends on Foundational.
- **US2 (T012–T016)** — depends on Foundational.
- **US3 (T017–T021)** — depends only on Setup (independent of the chip plumbing).
- **Polish (T022–T025)** — depends on all desired stories.

### Cross-story file coupling (NOT independent files — coordinate / sequence)

- `internal/ui/styles.go`: Foundational (T003/T004), US1 (T009 footerIdentityCompact), US3 (T019 separators
  in footerIdentityCompact/renderHintRow). → run Foundational → US1 → US3 in order on this file.
- `internal/ui/commandbar.go`: US1 (T010), US3 (T019/T020/T021). → US1 before US3 on this file.
- `internal/ui/app.go`: Foundational (T004), US1 (T008/T010), US2 (T015/T016). → US1 before US2 on this file.
- `internal/ui/spec013_test.go`: every test task appends here → tests are sequential within the file (this is
  why the test tasks are NOT marked `[P]` against each other).

### Recommended order

Setup → Foundational → US1 (MVP) → US2 → US3 → Polish.

### Parallel opportunities

- T018 and T022/T023 are `[P]` (distinct concerns; T022 reads existing test files, T023 a new test).
- With multiple developers: US3 (T017–T021) can proceed in parallel with Foundational+US1+US2, IF the shared
  `styles.go`/`commandbar.go` edits are merged carefully (US1's footerIdentityCompact change lands before
  US3's separator change in that function). Otherwise keep sequential.

---

## Implementation Strategy

### MVP first (US1 only)

1. T001 (baseline green).
2. T002–T005 (two-chip plumbing — also unblocks US2 later).
3. T006–T011 (US1).
4. **STOP & VALIDATE**: mode shown once on every browse box, footer tag gone, modal badges intact.

### Incremental delivery

US1 (dedup) → US2 (filter chip) → US3 (spacing) — each an independently testable, shippable increment.
Run `make test fmt vet lint check-readonly` after each phase; commit per task or logical group.

---

## Notes

- `[P]` = different file, no incomplete-task dependency.
- Verify each failing-first test is RED before implementing the matching task.
- Assert on `stripANSI` output or the structured `readEntries`/`writeEntries` fields to avoid ANSI brittleness
  (R7 risk); pin width ≥ `blockColMin=100` (use 140) for command-bar column assertions, ≥ tier for chip
  assertions (`dualApp`=120).
- The `[RW]/[RO]` test churn in T011 is migration, NOT regression — those tests move to assert the chip.
- No new package, no new file (except `spec013_test.go`), no new keymap field, no new hue, no storage change.
