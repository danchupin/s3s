# Implementation Plan: Blocked command bar (info · read · write), capability-visible in read-only

**Branch**: `007-command-bar-blocks` | **Date**: 2026-06-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/007-command-bar-blocks/spec.md`

## Summary

Rework the 006 single-strip hint bar into a **three-block command bar** (`info · read ·
write`) laid out as side-by-side columns, with the **write block always visible but
dimmed in read-only** (reversing 006 FR-004). Add a **visible add-connection affordance**
and a **delete-connection** action on the contexts screen. Gate **dangerous actions
behind a Ctrl chord** and confirm them on a **tier-chosen surface**: a centered popup for
the binary (y/N) tier (single-object/group delete, move, overwrite) and a **prominent
inline form** for the typed-identifier tier (directory path, bucket name, connection
name). Add **whole-bucket delete** (empty-only) to the dangerous set. Show a
**Claude-Code-style determinate progress bar** inline in the footer for long operations.
Both confirm surfaces share **one consistent visual style** (FR-027a).

Technical approach: this is almost entirely an `internal/ui` re-skin + key-routing change
reusing existing `operation`/`confirm` machinery; the only core additions are
`Mutator.RemoveBucket` (storage) and `Connector.Delete` + `(*Config).RemoveConnection`
(config seam). No constitution amendment; read-only guard stays intact (the new bucket
method is `RemoveBucket`, whose verb is outside the guard's forbidden set).

## Technical Context

**Language/Version**: Go 1.25 (per go.mod)

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2
(`charm.land/lipgloss/v2`), `aws-sdk-go-v2/service/s3` (storage only), `zalando/go-keyring`.

**Storage**: S3-compatible (Ceph RGW / MinIO) via `internal/storage`; config YAML at
`~/.config/s3s/config.yaml`; secrets in OS keychain.

**Testing**: `go test` white-box UI tests (`package ui`, `deliver`/`press` helpers asserting
`App.View().Content`); `storage.Fake` for units; `//go:build integration` MinIO tests.

**Target Platform**: terminal (darwin/linux); alt-screen TUI.

**Project Type**: single Go module — CLI/TUI.

**Performance Goals**: non-blocking event loop (Constitution II); pane/progress never
block; 80×24 → large render without clip.

**Constraints**: read-only structural guard (`scripts/check-readonly.sh`) — no fused
mutation-verb+entity S3 symbol outside `internal/storage`; secrets never plaintext;
no new palette hue (FR-013); calm UI (no garish color).

**Scale/Scope**: ~6 UI files touched + 3 new; 1 storage method; 1 config method; 1 seam
method. 6 user stories (US1/US2/US4 = P1; US3/US5/US6 = P2).

## Constitution Check

*GATE: must pass before Phase 0 and re-checked after Phase 1.*

| Principle | Gate | Verdict |
|-----------|------|---------|
| I. Core/UI Separation | Bucket delete, connection delete, emptiness check live in `storage`/`config`; UI reaches them via `Mutator.RemoveBucket` + `Connector.Delete`. UI never imports the SDK. | PASS |
| II. Non-Blocking TUI | Bucket delete, connection delete, and every progress update run in a `tea.Cmd` under a generation; the progress bar reflects ticks, never blocks; ops stay cancellable. | PASS |
| III. Test-First | Every FR gets a failing white-box UI test (block layout, RO-dimmed write, chord gate, surface-per-tier, label rule) or storage/config unit test first. | PASS (enforced in tasks) |
| IV. Integration Testing | `RemoveBucket` (empty + non-empty refusal) gets a MinIO integration test alongside the existing write suite. | PASS |
| V. Observability & Safe Operations | Bucket delete + connection delete are dangerous: chord-gated, typed-confirm, logged before execution (`logMutationStart` / `connection.delete`); secrets never logged. | PASS |

**Read-only guard**: `RemoveBucket` verb ("Remove") is NOT in the guard's forbidden verb
set (`Put|Delete|Create|Copy|Upload|Restore|Write`); the S3 `DeleteBucket` call is confined
to `internal/storage/writer.go`. Guard adds a `RemoveBucket → ErrReadOnly` refusal. No
violation.

**Result**: PASS — no Complexity Tracking entries required.

## Project Structure

### Documentation (this feature)

```text
specs/007-command-bar-blocks/
├── plan.md              # This file
├── research.md          # Phase 0: chord scheme, confirm surfaces, progress, block layout, palette
├── data-model.md        # Phase 1: bar blocks, confirm surface/tier, progress state, conn-delete
├── quickstart.md        # Phase 1: manual exercise of all 6 stories
├── contracts/           # Phase 1: command-bar-blocks, dangerous-actions, progress, connection-mgmt
│   ├── command-bar-blocks-contract.md
│   ├── dangerous-actions-contract.md
│   ├── progress-contract.md
│   └── connection-delete-contract.md
└── checklists/
    ├── requirements.md  # spec-quality (16/16)
    └── actions.md       # action-label & confirm-tier quality (42 items)
```

### Source Code (repository root)

```text
internal/ui/
├── hintbar.go        # REWORK → three-block command bar (info|read|write); write block dimmed in RO
├── commandbar.go     # NEW (split from hintbar): block model + column layout + collapse
├── confirm.go        # REWORK → tier-routed surface: binary→centered popup, typed→inline form
├── confirmview.go    # NEW: centerOverlay() popup renderer + inline typed-form renderer (shared style)
├── operation.go      # EXTEND: bucket delete op kind; chord-aware start*; progress fraction
├── progress.go       # NEW: Claude-Code determinate bar renderer (fraction+percent+elapsed) + threshold
├── keys.go           # EXTEND: dangerous chords (Ctrl+key) + AddConn/DelConn keys; label glyphs
├── connections.go    # EXTEND: delete-connection on contexts screen via Connector.Delete
├── app.go            # EXTEND: footer composes 3 blocks; popup overlay in View(); chord routing
└── styles.go         # EXTEND: block palette roles (reuse tokens), progress-bar glyphs, popup box

internal/storage/
├── storage.go        # EXTEND: Mutator gains RemoveBucket; ErrBucketNotEmpty sentinel
├── writer.go         # EXTEND: (*s3Client).RemoveBucket (empty pre-check → DeleteBucket)
├── guard.go          # EXTEND: readOnlyGuard.RemoveBucket → ErrReadOnly
├── fake.go           # EXTEND: Fake.RemoveBucket (empty-only) for unit tests
└── s3client_integration_test.go  # EXTEND: RemoveBucket empty + non-empty-refusal cases

internal/config/
└── connection.go     # EXTEND: (*Config).RemoveConnection(name) → drop triple + RemoveKeychain

cmd/s3s/
└── connection.go     # EXTEND: connSeam.Delete implements ui.Connector.Delete
```

**Structure Decision**: single Go module, unchanged. The feature is concentrated in
`internal/ui` (presentation + key routing), with thin, guarded additions in
`internal/storage` (one bucket method) and `internal/config` (one delete method) reached
via the existing `Mutator` and `Connector` seams — preserving Core/UI separation.

## Phase 0 — Research (see research.md)

Resolves the spec's planning-deferred unknowns:
- **R1 Chord scheme**: which Ctrl+key per dangerous action, avoiding terminal-reserved
  combos (ctrl+c/q/s/z/d/h/i/m/[).
- **R2 Confirm surfaces**: centered popup overlay (binary) vs prominent inline form
  (typed), both reusing `operation`/`confirm`; one shared style (FR-027a).
- **R3 Progress bar**: Claude-Code determinate bar (fraction → filled/empty cells +
  percent + elapsed), indeterminate fallback, "taking a while" threshold.
- **R4 Block layout**: columns via `lipgloss.JoinHorizontal`; collapse order on narrow
  width; height budget (≤5–6 rows) vs the footer.
- **R5 Palette roles**: assign info/read/write/dimmed/caution to EXISTING tokens; NO_COLOR
  redundancy.
- **R6 Bucket delete**: `RemoveBucket` empty-only (pre-check `ListObjectsV2` maxkeys=1 →
  `ErrBucketNotEmpty`), guard refusal, method-name dodges the read-only scan.
- **R7 Connection delete**: `(*Config).RemoveConnection` (drop triple + keychain,
  refuse active context) via `Connector.Delete`; live context-list refresh.
- **R8 Label rule**: single imperative verb, ≤2 words, lowercase, no articles/punctuation.

## Phase 1 — Design & Contracts (see data-model.md, contracts/, quickstart.md)

- **data-model.md**: `bar` block model (`blockKind`, entry, dimmed/caution state),
  `confirmSurface` (popup|inline) × `confirmTier` (binary|typed), `opProgress` extended
  with a determinate `fraction()` + indeterminate flag, connection-delete intent.
- **contracts/**:
  - `command-bar-blocks-contract.md` — three blocks, RO-dimmed write, add-conn affordance,
    collapse behavior, palette roles, label rule.
  - `dangerous-actions-contract.md` — chord set, surface-per-tier, typed identifiers
    (path/bucket/connection), bucket empty-only, read-only fall-through.
  - `progress-contract.md` — determinate bar, percent, threshold, indeterminate, cancel.
  - `connection-delete-contract.md` — delete key, active-context refusal, config+keychain
    cleanup, last-connection empty-state.
- **quickstart.md**: manual walkthrough of US1–US6.
- **Agent context**: update the `<!-- SPECKIT START/END -->` block in `CLAUDE.md` to point
  at this plan.

**Post-design Constitution re-check**: PASS (no new violations; the design keeps all S3 +
config logic behind `storage`/`config` seams and every backend call in a `tea.Cmd`).
