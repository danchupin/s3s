# Implementation Plan: Two-Pane Browse + Hotkey Mnemonic Review

**Branch**: `011-two-pane-hotkeys` | **Date**: 2026-06-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/011-two-pane-hotkeys/spec.md`

## Summary

Two coupled UI scopes. **(A) Three-zone master-detail browse**: turn bucket browsing into a Miller-columns layout — `buckets │ objects │ details` — where the highlighted bucket's first-level objects load **lazily on selection** (no Enter) into a middle zone, and the existing details pane (feature 006) is preserved/relocated as an adaptive third zone (bucket metadata when a bucket is focused, object metadata + preview when an object is focused). Focus crosses zones (`Tab` toggle, `→`/`l`/Enter cross in, `←`/`h`/`Esc` ascend-or-return). Width-tiered: Full ≥130 (3 zones), Dual 100–129 (buckets│objects), Single ≤99 (today's single-column stack, unchanged). **(B) Hotkey clarity**: a full keymap review that removes only the illogical global `AddConn` `n` (the "+ add connection" row stays the affordance) and renders every advertised key **bold** in the hint bar + help, with an NO_COLOR-safe cue.

**Technical approach** (Phase 0 research): reuse the established machinery end to end — `loadLevel`/`onLevel`/`levelMsg` + the per-session `internal/cache` level cache keyed by `(context,bucket,prefix,search)` for the objects zone (shared with the full-screen tree view, so crossing in is a cache hit); the existing `paneDebounce` (180 ms ≤ 200 ms ceiling) + generation/`beginLoad` suppression for settled-selection-only fetches; `boxView` rounded borders + `windowBounds` + `lipgloss.JoinHorizontal` for the tiered layout; `defaultKeys()` as the single keymap source so dispatch + hint bar + help never drift. No new `storage.Storage` method, no write symbols → `check-readonly` stays green.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod`)

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), lipgloss v2 (`charm.land/lipgloss/v2`); `aws-sdk-go-v2/service/s3` confined to `internal/storage`.

**Storage**: S3-compatible (Ceph RGW / MinIO), **read-only** via the four-method `storage.Storage` interface (`ListBuckets`, `ListLevel`, `HeadObject`, `GetObjectRange`). Per-session, TTL-free level cache in `internal/cache`.

**Testing**: `go test` — white-box `package ui` (`deliver`/`press`/`viewOf` helpers, in-memory `storage.Fake`). testcontainers/MinIO integration exists but is **not exercised** by this feature (no storage-contract change).

**Target Platform**: terminal (darwin/linux), truecolor + NO_COLOR.

**Project Type**: single-project keyboard-driven CLI/TUI.

**Performance Goals**: non-blocking render loop (Constitution II); a settled bucket selection issues exactly one `ListLevel` within ≤ 200 ms; fast scroll across N buckets issues ≤ 1 listing; revisits are cache hits (0 calls).

**Constraints**: read-only posture preserved (`make check-readonly` green; no new storage method, no write S3 symbol outside `internal/storage`); footer (identity + hint line incl. help/quit) always visible; key emphasis must survive NO_COLOR via a non-color cue; layout reflows statelessly on resize.

**Scale/Scope**: changes confined to `internal/ui/*` (layout, focus, dispatch, keymap, hint bar, help) plus a read-only test counter knob on `storage.Fake`. ~10–14 files touched; no `cmd/` or `internal/config` change.

## Constitution Check

*GATE: evaluated before Phase 0 and re-checked after Phase 1 design. Constitution v1.0.0.*

| Principle | Status | Justification |
|-----------|--------|---------------|
| **I. Core/UI Separation** | ✅ PASS | All changes live in `internal/ui`; the objects zone consumes only the read-only `storage.Storage` interface (`ListLevel`). No `aws-sdk-go-v2` import enters `internal/ui`. |
| **II. Non-Blocking TUI** | ✅ PASS | Every objects-zone/details load runs in a `tea.Cmd` under `beginLoad`/`gen`; bucket-scroll loads are debounced (180 ms) and superseded loads are dropped. No I/O on the render path. |
| **III. Test-First (NON-NEGOTIABLE)** | ✅ PASS | White-box `package ui` tests written first per user story (Red→Green→refactor): layout-tier rendering, focus crossing/Tab, lazy-load counters, bold-glyph + `n`-removed assertions. |
| **IV. Integration Testing** | ✅ N/A (justified) | No storage-client contract change and no new S3 semantics (pagination/auth/transfer) are introduced — the feature only re-arranges existing read calls. Per the principle's scope ("any change to the storage-client contract"), no integration test is warranted; this is recorded explicitly rather than skipped silently. |
| **V. Observability & Safe Operations** | ✅ PASS | Browse-only; no destructive action added, so no new confirmation/logging surface is required. No secrets enter logs. |

**Gate result: PASS — no violations.** No constitution amendment required (the feature fits within v1.0.0). Post-Phase-1 re-check: still PASS (the design adds no SDK dependency to the UI, no write method, and no blocking call).

## Project Structure

### Documentation (this feature)

```text
specs/011-two-pane-hotkeys/
├── plan.md              # This file
├── spec.md              # Feature spec (27 FR + sub-IDs, 10 SC, 8 clarifications)
├── research.md          # Phase 0 — R1..R10 decisions (code-anchored)
├── data-model.md        # Phase 1 — App state model (focus, zones, cursors, cache)
├── quickstart.md        # Phase 1 — validation + TDD order
├── contracts/
│   ├── keymap-contract.md       # reviewed keymap, bold glyphs, focus keys
│   ├── two-pane-layout.md       # tiers, zones, focus transitions, reflow
│   └── lazy-load-cache.md       # lazy startup, settled-fetch, cache/error policy
├── checklists/
│   ├── requirements.md          # spec quality (16/16)
│   └── lazy-load.md             # lazy-load/cache requirements quality (28/28)
└── tasks.md             # Phase 2 — created by /speckit-tasks (NOT here)
```

### Source Code (repository root)

```text
internal/ui/
├── app.go            # View/listWithPane → 3-zone tiers; Update dispatch → focus-aware keys; new focus + objects-zone state
├── pane.go           # details pane → adaptive (bucket meta vs object meta+preview), reused as the Full-tier 3rd zone
├── styles.go         # boxView active/dim zone styling; windowBounds reuse
├── keys.go           # defaultKeys: remove AddConn 'n'; bold glyph render (formatKeys/keyGlyph/helpLines); NO_COLOR cue
├── commands.go       # actionCatalog/dispatch — focus-aware; objects-zone load reuses loadLevel; debounce tick
├── hintbar.go        # bold key glyphs in the always-visible hint bar
├── connections.go    # "+ add connection" row stays the sole add-connection affordance
├── messages.go       # reuse levelMsg/errMsg (+ objects-zone settle tick msg)
└── tree.go           # level navigation reused by the objects zone

internal/cache/       # per-session level cache — REUSED unchanged (shared key space)
internal/storage/     # Fake gains a read-only list-call counter knob for tests (no interface change)
```

**Structure Decision**: single project (existing layout). The feature is almost entirely an `internal/ui` re-composition + keymap edit; `internal/cache` and `internal/storage` are reused, the latter gaining only a test-only read counter. No new package, no `cmd/` or `config` change.

## Complexity Tracking

> No Constitution Check violations — this section is intentionally empty.

## Phase 0 — Research

Complete. See [research.md](./research.md): R1 objects-zone load wiring (reuse `loadLevel`/`onLevel`/cache), R2 debounce (`paneDebounce` 180 ms + gen suppression), R3 first-page size (`DefaultMaxKeys` 1000, shared-cache invariant), R4 error-not-cached (`onLevel` caches only on `levelMsg`; failures are `errMsg`), R5 three-zone composition (`listWithPane`/`boxView`/`JoinHorizontal`, tier math, height budget), R6 focus state (`focusZone` + per-zone cursors vs today's single `bucketSel`/tree cursor), R7 focus-aware key dispatch, R8 bold glyphs + NO_COLOR cue, R9 `AddConn` `n` removal (row path intact), R10 testing (white-box + `Fake` counter; no integration).

## Phase 1 — Design & Contracts

Complete. [data-model.md](./data-model.md) (BrowseFocus, zone cursors, ObjectsZoneState, LayoutTier, Keymap, cache entry + state transitions); [contracts/](./contracts/) three UI contracts with test assertions; [quickstart.md](./quickstart.md) (US/SC → validation mapping, manual smoke at ≥130 / 100–129 / ≤99 cols, TDD order). Agent context (`CLAUDE.md` SPECKIT block) updated to point here.

**Next**: `/speckit-tasks` to generate the dependency-ordered `tasks.md`.
