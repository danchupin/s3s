# Implementation Plan: UI/UX Refinement — Action Menu, Footer Redesign & Key Discoverability

**Branch**: `004-ui-ux-refinement` | **Date**: 2026-06-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-ui-ux-refinement/spec.md`

## Summary

Reduce the TUI keymap and declutter the footer. The six top-level write keys
(`+ d u y m D`) and `r` refresh collapse into a single **contextual action menu** opened
by `a`; `x` cancel folds into `Esc` (contextual: cancels an in-flight load). The menu is a
new modal overlay in `internal/ui` that lists only the actions valid for the current
selection/context and, when chosen, dispatches the **existing** `start*` operation
functions unchanged — so name/destination entry and the two-tier confirmation are
preserved exactly (Constitution V). The footer becomes ≤ 3 rows: a compact identity row,
a single contextual hint row (capped at 6, advertising `a actions` instead of six write
keys, arrow glyphs as primary nav), and an optional status row. Connection metadata and
the full keymap (including the menu's contents and the vim aliases) move into a
redesigned, categorized help surface. Status feedback names what is loading, shows
search-pending, and distinguishes success (green) from error (red). All changes are
confined to `internal/ui`; no `internal/storage`, SDK, or write-semantics change.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod`).

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2
(`charm.land/lipgloss/v2`). No new dependency.

**Storage**: N/A — `internal/storage` untouched; no S3 calls change.

**Testing**: `go test` white-box UI tests in `package ui` (`press`/`deliver` helpers,
assert on `App.View().Content`); in-memory `storage.Fake`. No new integration tests.

**Target Platform**: Terminal (xterm-256color cell renderer; Bubble Tea v2 alt-screen).

**Project Type**: Single-project Go TUI. Existing `internal/ui` package.

**Performance Goals**: Render stays non-blocking (Constitution II); menu/footer/help are
pure string assembly; operations still dispatch via `tea.Cmd`.

**Constraints**: Footer ≤ 3 rows; hint row one line, ≤ 6 hints; zero overflow 40–200 cols
(below 40 best-effort); top-level interactive actions ≤ 12 (aliases/`1-9` count once); arrows
primary, Top/Bottom unadvertised (vim + Home/End help-only); single status row by precedence
(op-prompt > running > loading > search-pending > notice > error, FR-018a); Esc modal
precedence (open overlay closes first, no background-load cancel, FR-029); operation semantics
& confirmation tiers preserved; secrets redacted.

**Scale/Scope**: ~7 files in `internal/ui` (1 new source + 1 new test + edits to
`keys.go`, `app.go`, `tree.go`, `styles.go`, plus test files). No new packages.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Verdict |
|-----------|------------|---------|
| **I. Core/UI Separation** | All work in `internal/ui`; no S3 SDK import; `internal/storage` untouched. | PASS |
| **II. Non-Blocking TUI** | Menu/footer/help are synchronous string work; operations still run in `tea.Cmd`; no new render-path I/O. | PASS |
| **III. Test-First (NON-NEGOTIABLE)** | Each slice starts with failing white-box tests (menu contextual items, removed-keys-inert, footer rows/cap, help Actions/Connection, named loading). | PASS |
| **IV. Integration Testing** | No storage-client contract change → no new real-backend tests; existing suite unaffected. | PASS (N/A) |
| **V. Observability & Safe Operations** | Menu only re-enters the existing `start*` flows; two-tier confirmation + pre-execution logging preserved (FR-026/FR-020); secret redaction preserved (FR-021). No new destructive path. | PASS |

**Result**: No violations. Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/004-ui-ux-refinement/
├── plan.md              # This file
├── research.md          # Phase 0
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/           # Phase 1
│   ├── action-menu-contract.md     # NEW
│   ├── footer-hints-contract.md    # UPDATED (a actions, arrows, no write/refresh/cancel keys)
│   └── help-surface-contract.md    # UPDATED (Actions section, vim aliases, arrow-primary)
├── checklists/
│   ├── requirements.md
│   └── ux.md
└── tasks.md             # /speckit-tasks output (regenerated for new scope)
```

### Source Code (repository root)

```text
internal/ui/
├── actionmenu.go        # NEW: modeActionMenu state; contextual item builder
│                        #   (buckets→[Refresh]; tree writable→selection-gated set; RO→[Refresh]);
│                        #   menu overlay view; onMenuKey (↑/↓ + vim move, Enter→invoke start*,
│                        #   Esc/Back→close). Items dispatch existing startCreateFolder/
│                        #   startRemoveObject/startUpload/startCopy/startMove/
│                        #   startRecursiveDelete/refresh — NO new op logic.
├── actionmenu_test.go   # NEW: US1 white-box tests (contextual items, RO, removed-keys inert,
│                        #   invoke enters existing op flow, Esc cancels load).
├── keys.go              # CHANGE: keyMap adds Menu []string{"a"}; remove Cancel "x" (fold into
│                        #   Back-when-loading); keep all write/refresh fields BOUND-but-not-top-
│                        #   level-routed (invoked from menu). helpLines→ m.helpLines() categorized
│                        #   (Navigation/Search & View/Actions/Context/Global + Connection),
│                        #   listing vim aliases + the menu contents; arrow-primary.
├── tree.go              # CHANGE onTreeKey: remove cases for NewFolder/Delete/Upload/Copy/Move/
│                        #   DeleteAll/Refresh; add case Menu → openActionMenu(); Back unchanged.
├── app.go               # CHANGE: onKey routes Menu (buckets+tree) and modeActionMenu; replace
│                        #   `Cancel && loading` + phaseRunning Cancel with Back-key cancel (FR-029);
│                        #   View() renders the menu overlay; footerBlock → 3 rows (identity+hints+
│                        #   status), drop separator+endpoint; statusLine names load + "Esc to
│                        #   cancel" + search-pending + notice hue. onBucketsKey adds Menu case.
├── styles.go            # CHANGE: hint catalog (a actions, arrow glyphs, no d/u/y/m/D/+/r/x;
│                        #   cap 6 + "? more"); compact identity builder; noticeStyle (green);
│                        #   menu overlay render helper.
├── footer_test.go       # CHANGE/ADD: footer ≤3 rows, one-line hint, a actions present & write
│                        #   keys absent, cap 6, "? more", esc clear/back, width 40–200.
├── keys_test.go         # NEW: categorized help lists all actions+aliases incl vim + Actions
│                        #   (menu) section + Connection; writable reflection; close hint.
└── app_test.go          # CHANGE/ADD: named loading + "Esc to cancel"; search-pending; notice
                         #   vs error hue; Back cancels in-flight load; removed top-level keys inert.
```

**Structure Decision**: Single Go project; all changes in `internal/ui`. One new source
file (`actionmenu.go`) for the menu modal; the rest are edits. `internal/storage`,
`cmd/s3s`, and `scripts/check-readonly.sh` untouched.

## Complexity Tracking

> No Constitution violations — section intentionally empty.
