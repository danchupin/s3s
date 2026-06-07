# Implementation Plan: UI Legibility, Hotkey Parity, Breadcrumbs & Write-Mode Clarity

**Branch**: `012-ui-visibility-write-clarity` | **Date**: 2026-06-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/012-ui-visibility-write-clarity/spec.md`

## Summary

A presentation/UX iteration on the two-pane browser. It (1) guarantees every resource identifier is fully
visible or revealable (bucket-column auto-grow into objects-zone slack, active-row wrap, a dedicated reveal
popup with best-effort OSC52 copy); (2) makes write mode legible and reversible (fix the `[RW]` whitespace
coloring, clear symmetric enable/disable labels, a prominent centered arm-confirmation popup, and a new
border-mounted mode chip modeled on an editor mode indicator); (3) adds a location breadcrumb
(context→bucket→prefix) with middle-elision; (4) fixes the **confirmed regressions** where focus in the
objects zone left marking, sorting, context-switch and all per-item actions dead, and where `/` filtered
buckets instead of the current objects level; (5) reworks filtering into a prominent Claude-Code-style
input that commits on Enter and hands focus to the filtered pane; (6) surfaces sort (incl. modification
date) in the command bar; and (7) consolidates every on-screen hint onto the single keymap (no stale
`^x`/`d/x/y` literals, `confirm.go` dispatch via `m.keys.Back`, `Tab` added to the keymap) and declutters
redundant interface annotations. Two constitution principles (VI UI Legibility, VII UI Consistency / Design
System) were added in v1.1.0 to govern this work. Approach is grounded in `research.md` (R1–R10) — reuse
existing components (`confirmPopupView`, `boxViewWith`, `barEntry`, `LevelQuery.Search`, the keymap+`glyph`
path); no storage contract change.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`)

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2 (`charm.land/lipgloss/v2`);
`tea.SetClipboard` (OSC52, `clipboard.go`) for copy. `internal/storage` wraps `aws-sdk-go-v2/service/s3`
(unchanged this feature).

**Storage**: S3-compatible (Ceph RGW / MinIO) via the `storage.Storage` interface — **no change**. The
objects-zone filter reuses the existing `LevelQuery.Search` server-side current-prefix narrowing.

**Testing**: `go test ./...` white-box `package ui` (deliver/press/viewOf helpers) + in-memory
`storage.Fake` (with the existing `ListLevelCalls`/`ListBucketsCalls` counters). No integration tests added
(no storage-contract change — constitution IV N/A, justified). `make fmt vet lint check-readonly`.

**Target Platform**: terminal TUI (macOS/Linux), truecolor + NO_COLOR; Bubble Tea v2 cell renderer.

**Project Type**: single Go project (CLI/TUI). All changes in `internal/ui`; `internal/storage`,
`internal/cache`, `internal/preview`, `internal/config`, `internal/logging` untouched.

**Performance Goals**: non-blocking TUI (constitution II) — every backend call already runs in a `tea.Cmd`;
this feature adds no synchronous I/O. Reveal/copy/breadcrumb/chip/filter-input are pure render or single/
debounced `tea.Cmd`.

**Constraints**: footer + command bar MUST stay fully visible at every width/tier (FR-022 / layout
invariant / `boxView` `minRows` hard-cap); every cue NO_COLOR-safe; single keymap source for all hints.

**Scale/Scope**: ~12–15 files in `internal/ui`; 9 user stories; new keymap fields (`Reveal`, `Tab`); one new
render slot (right-aligned border chip); a reworked filter input surface; no new package.

## Constitution Check

*GATE: must pass before Phase 0 and re-checked after Phase 1. Constitution v1.1.0.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Core/UI Separation | ✅ PASS | All work in `internal/ui`; no S3/SDK use outside `internal/storage`; UI stays render+intent. |
| II. Non-Blocking TUI | ✅ PASS | No synchronous I/O added. Filter reuses the debounced `tea.Cmd`+generation path; copy is one `tea.SetClipboard` Cmd; reveal/chip/breadcrumb are render-only. |
| III. Test-First (NON-NEGOTIABLE) | ✅ PASS (gate) | TDD per `research.md` R10 — failing white-box test first per user story; regressions (US6/US7) start from a failing parity/scope test. |
| IV. Integration Testing | ✅ N/A (justified) | No storage-client contract change (objects-zone filter reuses existing `LevelQuery.Search`; only a test-only counter is read on `Fake`). No new credential/pagination/transfer path. |
| V. Observability & Safe Operations | ✅ PASS | Write-arm keeps its `slog` event + deliberate confirm (now a prominent popup); disarm instant; no secrets logged; no new destructive op. |
| VI. UI Legibility (v1.1.0) | ✅ PASS (drives feature) | FR-001..004/010..013/021/038 implement it; `boxView` `minRows` cap preserves footer; reveal popup guarantees no identifier is permanently hidden. |
| VII. UI Consistency / Design System (v1.1.0) | ✅ PASS (drives feature) | Every new surface maps to an existing component (R9); all hints single-sourced from the keymap (FR-035..037); no new hue/`NewStyle` outside `styles.go`; filter input reuses the shared field/box style. |

**Read-only structural guard**: STAYS green — no new write-capable S3 symbol outside `internal/storage`,
no new storage method. **Initial gate: PASS. No violations → Complexity Tracking empty.**

Post-Phase-1 re-check: design adds no new package, no synthetic mode (objects-zone parity via a shared
`onLevelKey` + focus-aware `selKind`/`actionCatalog`), reuses popup/box/bar/field components — **PASS, no
new violations.**

## Project Structure

### Documentation (this feature)

```text
specs/012-ui-visibility-write-clarity/
├── plan.md              # This file
├── spec.md              # 9 user stories, FR-001..041, SC-001..015
├── research.md          # Phase 0 (R1–R10 incl. mode-chip R7)
├── data-model.md        # Phase 1 (UI state entities)
├── quickstart.md        # Phase 1 (manual verification walkthrough)
├── contracts/           # Phase 1 (keymap, reveal-popup, level-filter, layout-visibility, writemode)
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/ui/                     # ALL changes live here
├── app.go                       # onObjectsKey→shared onLevelKey; selKind/actionCatalog focus-aware;
│                                #   listWithPane col-grow + slack; breadcrumb()+ctx; primary (leftmost) list box gets the mode chip;
│                                #   statusLine yields to arm popup + filter input; reveal popup overlay in View()
├── keys.go                      # +Reveal field (default 'i'); +Tab field (default 'tab'); glyph coverage
├── tree.go                      # onTreeKey → delegates to shared onLevelKey
├── selection.go                 # marks level-scoped — clear m.sel in loadObjectsLevel (R3 fix)
├── search.go                    # filter input surface (commit-on-Enter → focus filtered pane); focus-aware
│                                #   scope (objects-zone server filter vs bucket local filter); cancel/clear lifecycle
├── sort.go                      # sortIndicator reused; reachability via parity (no algo change)
├── commandbar.go                # writeColumn symmetric labels; sort barEntry; entry glyphs from keymap
├── styles.go                    # badge space-color fix; boxViewWith right-slot (mode chip); elideMiddle();
│                                #   shared single-line input/field style for the filter surface
├── pane.go                      # details hints via glyph(m.keys.*) (kill '^x')
├── confirm.go                   # onConfirmKey dispatch via m.keys.Back (not literal "esc")
├── confirmview.go               # reuse popup base; hints via formatKeys(m.keys.*)
├── connections.go / operation.go / filebrowser.go  # hand-written hints → formatKeys; "tab" → m.keys.Tab
├── writemode.go                 # armConfirmPopupView (reuse confirmPopupView); disarm stays instant
├── reveal.go (NEW)              # reveal popup state + render + tea.SetClipboard copy cmd
└── *_test.go                    # failing-first white-box tests per user story (R10)
# internal/storage, internal/cache, internal/preview, internal/config, internal/logging — UNCHANGED
```

**Structure Decision**: Single project; the feature is entirely within `internal/ui`, extending existing
files plus one new `reveal.go`. Structural additions are refactors of existing code (a shared `onLevelKey`
handler, a right-aligned label slot in `boxViewWith`, a promoted filter-input surface), not new
subsystems — honouring Core/UI Separation (I) and avoiding a new package.

## Complexity Tracking

> No constitution violations — section intentionally empty.

## Phase 1 Artifacts

- [data-model.md](./data-model.md) — UI state entities (Reveal surface, Breadcrumb path, Mode chip,
  Write-state cue, Sort state, Focus/level context, Filter input, Keymap additions).
- [contracts/](./contracts/) — `keymap-contract.md`, `reveal-popup-contract.md`, `level-filter-contract.md`
  (incl. the commit-on-Enter input flow), `layout-visibility-contract.md`, `writemode-contract.md`.
- [quickstart.md](./quickstart.md) — manual verification walkthrough mapped to acceptance scenarios.
- CLAUDE.md `<!-- SPECKIT … -->` block updated to point at this plan.
