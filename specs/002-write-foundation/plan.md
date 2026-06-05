# Implementation Plan: Write Foundation & Safety

**Branch**: `002-write-foundation` | **Date**: 2026-06-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/002-write-foundation/spec.md`

## Summary

Give s3s the ability to *mutate* storage — safely. This feature is the foundation
the later write features (003+ upload/delete/copy) build on, so it ships only one
low-risk mutating operation (**create an empty folder**) and invests the rest in
safety scaffolding: a global `--write` switch (off by default), a per-context
`readonly: true` override (read-only always wins), a two-tier confirmation
framework (simple confirm for reversible actions, typed confirm for destructive
ones), non-blocking execution with ≤100 ms progress feedback, and pre-execution +
outcome logging for every mutation.

Technical approach: extend the `storage` boundary with a small **mutating
interface** (`CreateFolder`) implemented only inside `internal/storage`; wrap every
backend in a **read-only guard** that short-circuits mutations (returning
`ErrReadOnly`, no network call) whenever the context is read-only or `--write` is
off — this preserves the structural read-only guarantee (FR-012) at runtime. The
existing `scripts/check-readonly.sh` is **retained unchanged**: it already confines
SDK mutation symbols to `internal/storage`, which remains exactly the desired
invariant. The UI gains a confirmation overlay and an operation/progress message
flow reusing the existing `tea.Cmd` + generation/cancellation model.

## Technical Context

**Language/Version**: Go 1.25 (`go 1.25.0` in go.mod)

**Primary Dependencies**:
- S3: `github.com/aws/aws-sdk-go-v2` (+ `service/s3`) — confined to `internal/storage`
- TUI: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`
  (note: Bubble Tea v2 import paths are `charm.land/...`, NOT `github.com/charmbracelet/...`)
- Config: `go.yaml.in/yaml/v3`
- Logging: stdlib `log/slog` (JSON handler → file) + existing `logging.Secret` redaction

**Storage**: No local datastore. Remote S3-compatible backends (MinIO, Ceph RGW).
Create-folder = `PutObject` of a zero-length key ending in `/`. Per-session
in-memory level cache (invalidated on the affected level after a successful write).

**Testing**: `go test`; unit tests against the in-memory `storage.Fake` (extended
with the mutating method + guard); white-box UI tests via `deliver`/`press` helpers
asserting on `App.View().Content`; integration tests against a real MinIO
(`//go:build integration`) for create-folder + guard behaviour.

**Target Platform**: Cross-platform terminal (Linux/macOS), 24-bit color.

**Project Type**: Single Go module (desktop CLI/TUI).

**Performance Goals** (from Success Criteria):
- Progress feedback visible ≤ 100 ms after a mutation starts (SC-004)
- No frozen frames for the duration of the backend call (SC-004)

**Constraints**:
- Writes off unless `--write` is passed (FR-001); per-context `readonly: true`
  overrides it (FR-002). Read-only contexts refuse 100% of mutations (SC-002).
- Mutations confined to `internal/storage`; the read-only guard wrapper lives there
  and is the single runtime enforcement point (FR-003, FR-012).
- Every mutation confirmed before execution (SC-001) and logged before execution +
  on outcome, secrets never logged (FR-008, SC-005).
- Failed/denied mutation leaves storage unchanged (FR-011, SC-007).

**Scale/Scope**: One mutating operation (create folder). 3 user stories, 12
functional requirements, 7 success criteria.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution v1.0.0 — five principles:

| # | Principle | How this plan satisfies it | Gate |
|---|-----------|----------------------------|------|
| I | Core/UI Separation | The mutating method + read-only guard live in `internal/storage`; the UI calls them only through the `storage` interface, never the SDK. Confirmation/operation state lives in `internal/ui`; gating logic (policy resolution) is UI-agnostic and unit-tested without Bubble Tea. | PASS |
| II | Non-Blocking TUI | Create-folder runs in a `tea.Cmd` → goroutine → `tea.Msg` under the existing generation/`context.CancelFunc` model; progress ≤100 ms, cancellable, superseded results dropped (FR-006, FR-007). | PASS |
| III | Test-First (NON-NEGOTIABLE) | Tasks ordered tests-first: guard refusal, policy resolution, confirmation tiers, and create-folder each start with a failing test (fake + integration). | PASS (enforced in tasks) |
| IV | Integration Testing | testcontainers MinIO exercises real create-folder (`PutObject` of `prefix/`), the read-only guard (mutation refused without network), auth, and error paths — the new storage-contract surface. | PASS |
| V | Observability & Safe Operations | This feature *implements* V for the write path: destructive actions require explicit confirmation (typed tier) and are logged before execution; create-folder (reversible) uses simple confirm; all mutations log action/target/context/outcome to the file log; secrets redacted via `logging.Secret`. `check-readonly.sh` retained to keep SDK mutations inside `internal/storage`. | PASS |

**Result**: All gates PASS. No violations → Complexity Tracking left empty.

## Project Structure

### Documentation (this feature)

```text
specs/002-write-foundation/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── writer-interface.md     # storage mutating interface + read-only guard
│   ├── confirmation-contract.md# TUI two-tier confirmation framework
│   └── config-flag-delta.md    # `readonly` context field + `--write` flag
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/
└── s3s/
    └── main.go              # + `--write` flag → threads write-enabled into ui.App

internal/
├── config/
│   ├── config.go            # + Context.ReadOnly field; ClientConfig unchanged
│   └── resolve.go           # + WriteModeFor(name, writeFlag) (Resolve/ClientConfig unchanged)
├── storage/                 # ONLY package importing service/s3 (guard-enforced)
│   ├── storage.go           # + Mutator interface (CreateFolder); ErrReadOnly sentinel
│   ├── writer.go            # CreateFolder impl: PutObject of "<prefix>/" (zero bytes)
│   ├── guard.go             # readOnlyGuard wrapper: short-circuits mutations
│   ├── fake.go              # + CreateFolder + guard support for unit tests
│   └── *_test.go            # unit + //go:build integration (MinIO)
└── ui/
    ├── app.go               # + write-enabled state; route mutation intents
    ├── confirm.go           # confirmation overlay: simple + typed tiers
    ├── operation.go         # operation lifecycle state + progress rendering
    ├── commands.go          # + createFolderCmd (tea.Cmd, generation/cancel)
    ├── messages.go          # + operationStarted/Progress/Done msgs
    ├── keys.go              # + create-folder + confirm keybindings
    └── *_test.go            # white-box: guard refusal, tiers, create-folder flow

scripts/
└── check-readonly.sh        # RETAINED unchanged (confines SDK mutations to storage)
```

**Structure Decision**: Single Go module, strict core/UI separation per Constitution
I. All write capability (the `Mutator` method, its `PutObject` call, and the
read-only guard) is confined to `internal/storage`; the UI consumes it through the
`storage` interface and never imports the SDK. `check-readonly.sh` is kept as-is —
it already forbids SDK mutation symbols outside `internal/storage`, which is the
invariant we still want. Module path: `github.com/danchupin/s3s`.

## Complexity Tracking

> No constitution violations. Section intentionally empty.
