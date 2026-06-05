# Implementation Plan: UI/UX Refinement — Footer Redesign & Key Discoverability

**Branch**: `004-ui-ux-refinement` | **Date**: 2026-06-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-ui-ux-refinement/spec.md`

## Summary

Declutter the TUI footer and relocate shortcut discovery. Today `footerBlock`
(`internal/ui/app.go:540`) stacks up to five rows — separator rule, identity line,
endpoint line, a hints line that lists ~13 shortcuts via `wrapSegs` (which *wraps*
rather than drops), and a status line — so on narrow terminals the footer eats 6+ rows.

The redesign caps the footer at **3 rows** (identity + hints + optional status): a single
compact identity row (context + `[RW]`/`[RO]`, cluster if it fits), a **contextual,
priority-ordered, single-row** hints line scoped to the current mode + selection +
context capability + width (dropping lowest-priority hints first and appending a
`? more` cue when anything is hidden), and the existing transient status row. The full
keymap plus the connection metadata removed from the footer (endpoint, region, user,
version) move into a **redesigned, categorized help surface**. Loading/search/confirm
status gets clearer wording. All changes are confined to `internal/ui` — no storage,
SDK, write-semantics, or confirmation-model changes.

## Technical Context

**Language/Version**: Go 1.25 (`go.mod`)

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2
(`charm.land/lipgloss/v2`). No new dependency introduced.

**Storage**: N/A — this feature does not touch `internal/storage`; no S3 calls change.

**Testing**: `go test` white-box UI tests in `package ui` (drive the model with
`press`/`deliver`, assert on `App.View().Content`); in-memory `storage.Fake` for any
backend the model needs. No new integration tests (no storage-contract change).

**Target Platform**: Terminal (xterm-256color cell renderer; Bubble Tea v2 alt-screen).

**Project Type**: Single-project Go TUI (CLI/desktop-app). Existing `internal/ui` package.

**Performance Goals**: Render stays non-blocking (Constitution II); footer/hints/help
assembly is pure string work with no I/O — no measurable render-time impact.

**Constraints**: Footer ≤ 3 rendered rows at any width; zero horizontal overflow for
terminal widths 40–200; hint row is exactly one row (degrades, never wraps); `? help`
and `q quit` affordances always reachable; secrets stay redacted; every existing
keybinding keeps working and is discoverable in help in one step.

**Scale/Scope**: ~5 source files touched in `internal/ui` (`styles.go`, `app.go`,
`keys.go`, plus tests; small helper additions). No new packages.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Verdict |
|-----------|------------|---------|
| **I. Core/UI Separation** | All work is in `internal/ui`; no import of the S3 SDK; `internal/storage` untouched. Pure presentation. | PASS |
| **II. Non-Blocking TUI** | No new I/O on the render path; footer/hints/help are synchronous string assembly. No `tea.Cmd` changes. | PASS |
| **III. Test-First (NON-NEGOTIABLE)** | Each slice starts with a failing white-box UI test (footer row-count/width, contextual-hint visibility, `? more` cue, categorized help content, named loading). | PASS |
| **IV. Integration Testing** | No storage-client contract change → no new real-backend tests required; existing integration suite unaffected. | PASS (N/A) |
| **V. Observability & Safe Operations** | Two-tier confirmation and pre-execution logging are preserved exactly (FR-017, FR-020); secret redaction preserved (FR-021). No destructive behavior added. | PASS |

**Result**: No violations. Complexity Tracking left empty.

## Project Structure

### Documentation (this feature)

```text
specs/004-ui-ux-refinement/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── footer-hints-contract.md
│   └── help-surface-contract.md
├── checklists/
│   └── requirements.md  # from /speckit-specify + /speckit-clarify
└── tasks.md             # /speckit-tasks output (NOT created here)
```

### Source Code (repository root)

```text
internal/ui/
├── styles.go        # CHANGE: replace footerHintsLine + footerEndpointLine usage;
│                    #   add hint catalog (hint{key,label,prio,predicate}), the
│                    #   single-row priority packer with "? more" cue, and a slimmer
│                    #   compact identity builder. footerEndpointLine retired from footer.
├── app.go           # CHANGE: footerBlock → identity + hints + status (drop separator
│                    #   rule + endpoint line; ≤3 rows). statusLine: name what is loading;
│                    #   add search-pending indicator. Pass mode/selection/contexts to hints.
├── keys.go          # CHANGE: helpView/helpLines → method on App; categorized sections
│                    #   (Navigation / Search & View / Context / Write / Global) + a
│                    #   Connection section (endpoint, region, user, version, context,
│                    #   cluster); reflect writable; explicit close hint.
├── footer_test.go   # CHANGE/ADD: row-count ≤3, hint-row single line, contextual
│                    #   visibility (RO hides writes; 1 context hides 1-9; selection-aware),
│                    #   "? more" cue on overflow, width 40–200 no overflow.
├── keys_test.go     # ADD: categorized help lists every action + aliases; connection
│                    #   section present; writable reflected; close hint present. (new file)
└── app_test.go      # CHANGE/ADD: named loading line; notice vs error distinct hue.
```

**Structure Decision**: Single Go project; all changes live in the existing
`internal/ui` package. No new packages, files limited to UI presentation plus their
white-box tests. `internal/storage`, `cmd/s3s`, and `scripts/check-readonly.sh` are
untouched.

## Complexity Tracking

> No Constitution violations — section intentionally empty.
