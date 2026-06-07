---
description: "Task list for 012 UI Legibility, Hotkey Parity, Breadcrumbs & Write-Mode Clarity"
---

# Tasks: UI Legibility, Hotkey Parity, Breadcrumbs & Write-Mode Clarity

**Input**: Design documents from `specs/012-ui-visibility-write-clarity/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ (keymap, reveal-popup,
level-filter, layout-visibility, writemode)

**Tests**: REQUIRED — constitution III (Test-First) is non-negotiable. Every user story leads with
failing white-box `package ui` tests (deliver/press/viewOf + `storage.Fake`), per `research.md` R10.

**Organization**: by user story (priority order). All changes are in `internal/ui/`; `internal/storage`,
`internal/cache`, `internal/preview`, `internal/config`, `internal/logging` are UNTOUCHED. `make
check-readonly` STAYS green (no new write-S3 symbol, no storage method).

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: parallelizable (different files, no incomplete-task dependency)
- **[Story]**: US1..US9 (Setup/Foundational/Polish carry no story label)

---

## Phase 1: Setup

- [ ] T001 Verify baseline green before changes: `go test ./...`, `make fmt vet lint check-readonly` — record current `internal/ui` coverage as the floor.
- [ ] T002 [P] Create the shared white-box test file `internal/ui/spec012_test.go` (`package ui`) with a header comment and imports; new failing tests below land here unless a more specific existing `_test.go` is named.

---

## Phase 2: Foundational (blocking prerequisites)

**Purpose**: shared plumbing several stories build on. No story label.

- [ ] T003 [P] Add `Reveal` (default `["i"]`) and `Tab` (default `["tab"]`) fields to `keyMap` + `defaultKeys`, and ensure `keyGlyph`/`glyph()` cover them, in `internal/ui/keys.go`. (Blocks US1 reveal, US5 single-source, US6 Tab.)
- [ ] T004 [P] Add a right-aligned label slot to `boxViewWith` (and the `boxView`/`boxViewFocus` callers) in `internal/ui/styles.go`; the centered-label width budget must subtract the right slot. (Blocks US2 mode chip, US3 breadcrumb center label.)

**Checkpoint**: keymap + border renderer ready; build still green.

---

## Phase 3: User Story 1 — Names never hidden (Priority: P1)

**Goal**: bucket column grows into objects-zone slack; active row auto-wraps when truncated; a reveal popup
shows + copies the full identifier. **Independent test**: long bucket/key fully visible or revealable;
`i` opens the popup and emits the clipboard cmd.

- [ ] T005 [P] [US1] Failing tests `TestBucketsColumnGrowsWithSlack`, `TestBucketsColumnCappedAtMax` in `internal/ui/lazyload_test.go`.
- [ ] T006 [P] [US1] Failing tests `TestActiveRowWrapsWhenTruncated`, `TestActiveRowFallsBackToRevealWhenTooTall`, `TestActiveRowWrapNoFooterClip` in `internal/ui/spec012_test.go`.
- [ ] T007 [P] [US1] Failing tests `TestRevealShowsFullValue`, `TestRevealEmitsClipboardCmd`, `TestRevealAllZones`, `TestRevealFooterNotClipped`, `TestRevealDismiss` in `internal/ui/spec012_test.go`.
- [ ] T008 [US1] Implement bucket-column auto-grow into measured objects-zone slack (per tier, bounded by max + slack; visible-window measurement only) in `listWithPane`, `internal/ui/app.go`.
- [ ] T009 [US1] Implement automatic active-row wrap (selected row only, multi-line within the `rows` budget; pre-measure → truncation+reveal fallback so `boxView` `minRows` cap holds) as a `renderTable` variant in `internal/ui/styles.go`; dispatch wrap only in Dual/Full tiers.
- [ ] T010 [US1] Create `internal/ui/reveal.go`: `revealState` + centered render (reuse `confirmPopupView`/`popupBoxStyle`/`metaRow`) + `tea.SetClipboard(value)` copy cmd; wire the `Reveal` key in the level/key dispatch and overlay the popup in `View()` (`internal/ui/app.go`), suppressed while `op != nil`/`armConfirm`.

**Checkpoint**: US1 tests green; no identifier permanently hidden; footer never clipped.

---

## Phase 4: User Story 2 — Write state legible + mode chip (Priority: P1)

**Goal**: clean badge, symmetric enable/disable labels, and a border-mounted RO/WRITE mode chip on the
primary list box. **Independent test**: badge space uncolored; chip flips accent↔neutral; labels symmetric.

- [ ] T011 [P] [US2] Failing tests `TestBadgeDoesNotColorAdjacentSpace`, `TestEnableWriteLabelWhenDisarmed`, `TestDisarmCueWhenArmed`, `TestReadonlyContextCue` in `internal/ui/writemode_test.go`.
- [ ] T012 [P] [US2] Failing tests `TestModeChipWriteAccent`, `TestModeChipReadonlyNeutral`, `TestModeChipOnPrimaryBox`, `TestModeChipNoColor` in `internal/ui/spec012_test.go`.
- [ ] T013 [US2] Fix the badge whitespace coloring in `footerIdentityCompact` (separator space `dimCellStyle`, tag text only carries state color) in `internal/ui/styles.go`.
- [ ] T014 [US2] Symmetric write affordance labels sourced from `m.keys.WriteToggle` ("enable write" disarmed / "→ read-only" armed / context-forbidden cue) in `writeColumn`, `internal/ui/commandbar.go`.
- [ ] T015 [US2] Render the `WRITE`/`RO` mode chip via the `boxViewWith` right slot on the PRIMARY list box (leftmost in multi-zone, sole box in Single); wire the box callers in `View()` (`internal/ui/app.go`); NO_COLOR-safe text.

**Checkpoint**: US2 tests green; write state glanceable at the frame edge.

---

## Phase 5: User Story 6 — Objects-zone hotkey parity (Priority: P1, REGRESSION)

**Goal**: every level-toolset key works when focus is in the objects zone, matching the full-screen view.
**Independent test**: in the objects zone, mark/sort/sortdir/context + all per-item actions + the delete
chord run identically; no silent dead key; marks clear on bucket/level change. **Depends on**: T003 (Tab).

- [ ] T016 [P] [US6] Failing tests `TestMarkObjectsInObjectsZone`, `TestSortCycleInObjectsZone`, `TestSortDirInObjectsZone`, `TestContextFromObjectsZone` in `internal/ui/focus_test.go`.
- [ ] T017 [P] [US6] Failing tests `TestObjectsZoneActionsParity` (download/analyze/delete+chord/copy/move/upload/newfolder/refresh, write-gated), `TestNoDeadKeyInObjectsZone`, `TestMarksClearOnBucketChange` in `internal/ui/focus_test.go`.
- [ ] T018 [US6] Factor a shared `onLevelKey(focusZone)` from `onTreeKey` (`internal/ui/tree.go`); have `onTreeKey` and `onObjectsKey` (`internal/ui/app.go`) delegate to it, adding the Mark/Sort/SortDir/Context branches and the default `dispatchChord`/`dispatchActionKey` fallthrough.
- [ ] T019 [US6] Make `selKind()` (`internal/ui/app.go`) and `actionCatalog()` (`internal/ui/hintbar.go`) treat `(mode==modeBuckets && focusZone==zoneObjects)` as a level context — return `selObject`/`selFolder` and the OBJECT catalog (not `selNone`/bucket catalog).
- [ ] T020 [US6] Clear `m.sel` in `loadObjectsLevel` (`internal/ui/app.go`) so marks are level-scoped; make context-switch from the objects zone restore focus to the bucket list via prevMode (not `objReturn`).

**Checkpoint**: US6 tests green; the two-pane browser is fully operable from the objects zone. Unblocks US7, US8.

---

## Phase 6: User Story 7 — Filter the current level + prominent input (Priority: P1)

**Goal**: `/` filters the focused pane (objects-zone = server-side current prefix); a prominent input
previews live, commits on Enter, and hands focus to the filtered pane. **Independent test**: objects-zone
`/` narrows the level (exactly one `ListLevel` call), bucket list unaffected; Enter commits + moves focus;
re-open pre-fills; Esc/clear lifecycle. **Depends on**: US6 (objects-zone dispatch).

- [ ] T021 [P] [US7] Failing tests `TestFilterScopesToObjectsLevel` (assert `Fake.ListLevelCalls == before+1`), `TestBucketFilterStillLocal` (no `ListLevel` call) in `internal/ui/lazyload_test.go`.
- [ ] T022 [P] [US7] Failing tests `TestFilterInputCommitMovesFocus`, `TestFilterReopenPrefilled`, `TestFilterEscRevertsToCommitted`, `TestFilterClearRestoresLevel`, `TestFilterLivePreviewDebounced` in `internal/ui/spec012_test.go`.
- [ ] T023 [US7] Make `afterFilterEdit`, the Esc/clear branch, and `searchActive()` focus-aware in `internal/ui/search.go` (+ `internal/ui/app.go`): objects-zone `/` runs the server `LevelQuery.Search` (current prefix); bucket-zone `/` stays the instant local filter.
- [ ] T024 [US7] Build the prominent filter input surface (reuse the shared field/box style in `internal/ui/styles.go`): live debounced preview; Enter commits → closes input → moves focus to the filtered pane + shows the `filter: <term>` indicator; re-open pre-filled; Esc cancels to last committed state — in `internal/ui/search.go` (+ `statusLine` yields in `internal/ui/app.go`).

**Checkpoint**: US7 tests green; filtering is fluid and correctly scoped.

---

## Phase 7: User Story 3 — Location breadcrumb (Priority: P2)

**Goal**: full path (ctx→bucket→prefix) in the objects-zone center label / Single box title, middle-elided
when long, revealable. **Independent test**: drill/ascend updates it; long path elides middle keeping
bucket+deepest. **Depends on**: T004 (box slot / center-label budget).

- [ ] T025 [P] [US3] Failing tests `TestBreadcrumbFullPath`, `TestBreadcrumbUpdatesOnDrill`, `TestBreadcrumbMiddleElision`, `TestBreadcrumbEmptyPrefix`, `TestBreadcrumbRevealable` in `internal/ui/spec012_test.go`.
- [ ] T026 [P] [US3] Add `elideMiddle(path, maxW)` (sibling of `truncate`) in `internal/ui/styles.go`.
- [ ] T027 [US3] Prepend `ctxName` in `breadcrumb()`; render it as the objects-zone center label (`objectsZoneTitle`) in Dual/Full and the box title (`resourceTitle`) in Single; append the `(search: …)` marker after elision — in `internal/ui/app.go`.

**Checkpoint**: US3 tests green; the user always sees where they are.

---

## Phase 8: User Story 4 — Prominent arm confirmation (Priority: P2)

**Goal**: arming opens a centered confirmation popup (not a faint status line); badge + chip stay visible;
disarm stays instant. **Independent test**: `w` opens a centered popup with consequence+keys; badge/chip
visible; disarm has no popup.

- [ ] T028 [P] [US4] Failing tests `TestArmConfirmationIsCenteredPopup`, `TestArmConfirmBadgeAndChipStayVisible`, `TestDisarmIsInstantNoPopup` in `internal/ui/writemode_test.go`.
- [ ] T029 [US4] Add `armConfirmPopupView` (reuse `confirmPopupView`) in `internal/ui/writemode.go`; overlay it in `View()` and make the `statusLine` `armConfirm` branch yield to it in `internal/ui/app.go`; keep disarm instant (`toggleWrite`/`onArmConfirmKey` unchanged; `slog` event preserved).

**Checkpoint**: US4 tests green; arming is unmissable.

---

## Phase 9: User Story 5 — Design-system consistency / keymap single-source (Priority: P2)

**Goal**: every on-screen hint renders via the keymap+glyph path; no stale literals; dispatch via keymap.
**Independent test**: rebind any action key → every surface shows the new key; zero `^x`/`d/x/y`/`"esc"`
literals.

- [ ] T030 [P] [US5] Failing tests `TestNoCaretLiteralsInAnyView`, `TestRebindPropagatesAllSurfaces` (rebind → assert across `View().Content`, `helpView`, `commandBarView`, `paneView`, `statusLine`) in `internal/ui/keys_bold_test.go`.
- [ ] T031 [US5] Replace hardcoded key literals with `glyph()`/`formatKeys()` + bold across `internal/ui/pane.go` (`^x`), `internal/ui/app.go` (`d/x/y`, filebrowser/search/spinner hints), `internal/ui/keys.go` (help `d/x/y`), `internal/ui/confirmview.go`, `internal/ui/connections.go`, `internal/ui/operation.go`, `internal/ui/filebrowser.go`, `internal/ui/commandbar.go` (`↵`).
- [ ] T032 [US5] Make `onConfirmKey` dispatch via `matches(key, m.keys.Back)` (both tiers) in `internal/ui/confirm.go`; replace literal `"tab"` with `matches(key, m.keys.Tab)` in `internal/ui/connections.go` field-nav and the focus toggle in `internal/ui/app.go`.

**Checkpoint**: US5 tests green; `grep -rn '\^x\|d/x/y\|"esc"\|"tab"' internal/ui/*.go | grep -v _test.go` returns only keymap/glyph definitions.

---

## Phase 10: User Story 8 — Sort surfaced + date sort reachable (Priority: P2)

**Goal**: the command bar advertises the sort key + current field/direction; modification-date sort is
reachable everywhere. **Independent test**: bar shows `s name↑`, cycles to `modified`, updates; works in
the objects zone. **Depends on**: US6 (sort reachable in objects zone).

- [ ] T033 [P] [US8] Failing tests `TestSortIndicatorInCommandBar`, `TestSortCycleReachesModified`, `TestSortBarWidthFitsNarrow` in `internal/ui/sort_test.go`.
- [ ] T034 [US8] Add the sort `barEntry` (`"s "+sortIndicator()`) as the first read-block entry in `readEntries` (`internal/ui/commandbar.go`); remove the duplicate sort indicator from the box title so the title carries only the breadcrumb (`internal/ui/app.go`).

**Checkpoint**: US8 tests green; sort is discoverable and reachable.

---

## Phase 11: User Story 9 — Declutter the interface (Priority: P2)

**Goal**: remove duplicate/redundant on-screen annotations (keep ≥1 advertisement per action) and strip
spec-citation noise from the code. **Independent test**: no duplicated hint on screen; every action still
advertised once; no actionable affordance lost.

- [ ] T035 [P] [US9] Failing tests `TestNoDuplicateOnScreenHints`, `TestEveryActionAdvertisedOnce` in `internal/ui/spec012_test.go`.
- [ ] T036 [US9] Remove redundant on-screen annotations (hints duplicating the command bar / restating the obvious) while keeping ≥1 advertisement per action, all keymap-sourced — in `internal/ui/pane.go`, `internal/ui/app.go`, `internal/ui/hintbar.go`.
- [ ] T037 [US9] Code-comment cleanup (non-behavioural): strip US/FR/review citation noise from `internal/ui/*.go`, keeping WHY/gotcha comments, machine directives (`//go:build`, `//nolint`, `//go:generate`) and minimal godoc on exported symbols; verify `make fmt vet lint check-readonly` stays green.

**Checkpoint**: US9 tests green; surface and source are decluttered without losing affordances.

---

## Phase 12: Polish & cross-cutting

- [ ] T038 [P] NO_COLOR edge tests for every new cue (badge, mode chip, arm popup, breadcrumb elision marker, reveal popup) in `internal/ui/spec012_test.go`.
- [ ] T039 [P] Footer/command-bar visibility tests across tiers (60×10, 120×8, 140×12) for all new surfaces in `internal/ui/tier_test.go`.
- [ ] T040 Run the full gate: `go test ./...`, `make fmt vet lint check-readonly`; confirm `internal/ui` coverage ≥ the T001 floor and `check-readonly` green. Design-system gate (FR-018/FR-020): assert no new `lipgloss.NewStyle()`/`lipgloss.Color()` was introduced outside `styles.go` (and the `commandbar.go` `roleStyle` map) — every new surface reuses an existing palette role/component.
- [ ] T041 Manual quickstart smoke (`specs/012-ui-visibility-write-clarity/quickstart.md`) on a real terminal: reveal+OSC52 paste, mode chip, filter input + focus handoff, objects-zone parity, breadcrumb elision.

---

## Dependencies

- **Setup (T001–T002)** → everything.
- **Foundational**: T003 (keymap) → US1(reveal), US6(Tab), US5; T004 (box slot) → US2(chip), US3(breadcrumb).
- **US6 (T016–T020)** → US7 (objects-zone filter dispatch), US8 (objects-zone sort).
- US1, US2, US3, US4, US5 are otherwise independent (given Foundational).
- **Polish (T038–T041)** → after all stories.

## Parallel execution examples

- Foundational: `T003` ∥ `T004` (keys.go vs styles.go).
- Per-story tests are `[P]` (independent files): e.g. `T005 ∥ T006 ∥ T007` (US1), `T016 ∥ T017` (US6),
  `T021 ∥ T022` (US7).
- Across stories once Foundational lands: US1, US2, US4 implementation tracks can proceed in parallel
  (different primary files), but tasks sharing `app.go` (T008/T010/T015/T020/T024/T027/T029/T034/T036) must
  serialize — sequence by phase.

## Implementation strategy

- **Regression-first MVP (recommended)**: Setup + Foundational + **US6 + US7** — restores the broken
  two-pane hotkeys + correct filter scope (the reported pain) and adds the prominent filter input. Highest
  user-felt value.
- **Legibility headline (spec P1)**: **US1 + US2** — names never hidden + write-state legible + mode chip.
- Then layer P2: US3 (breadcrumb), US4 (arm popup), US8 (sort), US5 (single-source), US9 (declutter) — US9
  last so it declutters stabilized surfaces.
- TDD throughout: land each story's `[P]` failing tests first, then its implementation, then checkpoint.
