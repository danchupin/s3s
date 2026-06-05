# Implementation Plan: Object Write Operations

**Branch**: `003-object-write-ops` | **Date**: 2026-06-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/003-object-write-ops/spec.md`

## Summary

Deliver the day-to-day object-level mutations on top of the 002 write foundation:
**delete** a single object, **upload** a local file, **copy** an object to a new
key, **move/rename** an object, and **recursively delete** a folder/prefix. Every
operation reuses the foundation unchanged — the `--write` switch, the per-context
`readonly` override, the read-only guard, the two-tier confirmation framework
(simple + typed), non-blocking `tea.Cmd` execution with ≤100 ms progress, and
pre-execution + outcome logging. Destructive operations (single delete, overwrite
via upload/copy, move, recursive delete) use the typed-confirmation tier.

Technical approach: extend the `storage.Mutator` interface with the new mutating
methods (`RemoveObject`, `UploadFile`, `CopyKey`, `MoveObject`, `DeleteRecursive`
— names deliberately chosen to avoid the `check-readonly.sh` verb+entity pattern so
UI code may call them while the SDK symbols stay inside `internal/storage`),
implemented only inside `internal/storage`; extend
`readOnlyGuard` to refuse **each** new method (the single runtime enforcement
point), and `storage.Fake` to implement them for unit tests. Composite operations
keep their logic in the core: `MoveObject` is copy-then-delete with a no-data-loss
guarantee (FR-007), and `DeleteRecursive` is best-effort enumerate-and-delete
returning deleted/failed counts (FR-009/FR-011). Two streaming operations (upload,
recursive delete) introduce a **progress channel + `waitForProgress` command**
pattern so the interface shows live progress without blocking. The UI gains a
keyboard-driven **local file browser** (new `internal/localfs` reader + a UI view)
for choosing the upload source, and a **destination-key entry** phase for
copy/move. `scripts/check-readonly.sh` is **retained unchanged** — the new SDK
mutations (`DeleteObject`, `PutObject`, `CopyObject`, `DeleteObjects`) all live in
`internal/storage`, exactly where the guard already confines them. No config or
constitution change.

## Technical Context

**Language/Version**: Go 1.25 (`go 1.25.0` in go.mod)

**Primary Dependencies**:
- S3: `github.com/aws/aws-sdk-go-v2` (+ `service/s3`, `feature/s3/manager` for
  multipart upload of large files) — confined to `internal/storage`
- TUI: `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`
  (Bubble Tea v2 import paths are `charm.land/...`, NOT `github.com/charmbracelet/...`)
- Config: `go.yaml.in/yaml/v3` (no change this feature)
- Logging: stdlib `log/slog` (JSON → file) + existing `logging.Secret` redaction

**Storage**: No local datastore. Remote S3-compatible backends (MinIO, Ceph RGW).
New operations map to: delete = `DeleteObject`; upload = `PutObject` (or multipart
via `manager.Uploader` for large files); copy = server-side `CopyObject`
(same-bucket); move = `CopyObject` + `DeleteObject`; recursive delete =
`ListObjectsV2` (paginated) + batched `DeleteObjects`. Per-session in-memory level
cache is invalidated on the affected level(s) after a successful/partial write.

**Testing**: `go test`; unit tests against the in-memory `storage.Fake` (extended
with the new mutating methods + guard refusal); white-box UI tests via
`deliver`/`press` helpers asserting on `App.View().Content`; integration tests
against real MinIO (`//go:build integration`) for each real operation, guard
refusal, overwrite detection, partial-move, and recursive partial-failure.

**Target Platform**: Cross-platform terminal (Linux/macOS), 24-bit color.

**Project Type**: Single Go module (desktop CLI/TUI).

**Performance Goals** (from Success Criteria):
- Progress feedback visible ≤ 100 ms after an operation starts; refreshed live for
  long uploads and recursive deletes (SC-007)
- No frozen frames for the duration of any backend call (SC-007)

**Constraints**:
- Inherited from 002: writes off unless `--write` (FR-012); per-context `readonly`
  overrides (read-only refuses 100% of these ops, SC-008); mutations confined to
  `internal/storage` behind the read-only guard.
- Destructive ops require the typed tier (SC-001); overwrite of an existing key
  (upload/copy) escalates to the typed tier (SC-004).
- Move never loses data (FR-007/SC-005); recursive delete is best-effort with
  truthful deleted/failed counts (FR-009/FR-011/SC-006).
- Every op logged before execution + on outcome, secrets never logged (FR-014/SC-009).
- Copy/move destinations are within the **current bucket** only (cross-bucket out
  of scope, per clarification).

**Scale/Scope**: 5 mutating operations. 5 user stories, 16 functional requirements,
9 success criteria. Recursive delete must handle multi-page prefixes (1000+ keys).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution v1.0.0 — five principles:

| # | Principle | How this plan satisfies it | Gate |
|---|-----------|----------------------------|------|
| I | Core/UI Separation | All new mutating methods + the guard refusals + the move (copy+delete) and recursive (enumerate+delete) orchestration live in `internal/storage`; the UI calls them only through `storage.Mutator`, never the SDK. The local file browser's filesystem reading lives in a UI-agnostic `internal/localfs` package (unit-tested without Bubble Tea); the UI view is a thin renderer. | PASS |
| II | Non-Blocking TUI | Every operation runs in a `tea.Cmd` → goroutine → `tea.Msg` under the existing generation/`context.CancelFunc` model. Streaming ops (upload, recursive delete) push progress through a channel consumed by a `waitForProgress` command, so the frame never blocks; superseded/cancelled results are dropped by generation (FR-010, SC-007). | PASS |
| III | Test-First (NON-NEGOTIABLE) | Tasks ordered tests-first: guard refusal per method, each mutator method, overwrite detection, partial-move, recursive partial-failure, and each UI flow start with a failing test (fake + integration). | PASS (enforced in tasks) |
| IV | Integration Testing | testcontainers MinIO exercises every real op (delete, upload incl. large/multipart, copy, move incl. partial, recursive incl. multi-page + partial-failure), the guard refusing each without a network call, overwrite/collision, auth, and error paths — the new storage-contract surface. | PASS |
| V | Observability & Safe Operations | This feature extends V's write-path contract: destructive ops (single delete, overwrite, move, recursive delete) require the typed tier and are logged before execution; non-colliding copy uses simple confirm; all ops log action/source/destination/context + outcome (incl. recursive counts) to the file log; secrets redacted via `logging.Secret`. `check-readonly.sh` retained — the new SDK mutations stay inside `internal/storage`. | PASS |

**Result**: All gates PASS. Read-only posture relaxes exactly as the constitution
and 002 plan anticipated (more mutating methods behind the same guard). No
violations → Complexity Tracking left empty.

## Project Structure

### Documentation (this feature)

```text
specs/003-object-write-ops/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── object-mutator-interface.md  # storage Mutator extension + guard refusals
│   └── ui-write-flows-contract.md   # operation kinds/phases, file browser, progress
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/
├── localfs/                 # NEW: UI-agnostic local filesystem reader (testable)
│   ├── localfs.go           # ReadDir(path) → sorted entries (dirs first); IsReadableFile
│   └── localfs_test.go      # unit: ordering, hidden files, errors, file-vs-dir
├── storage/                 # ONLY package importing service/s3 (guard-enforced)
│   ├── storage.go           # + Mutator methods; ErrMovePartial sentinel; DeleteSummary
│   ├── writer.go            # + RemoveObject, UploadFile, CopyKey, MoveObject, DeleteRecursive
│   ├── guard.go             # readOnlyGuard: refuse EACH new mutating method
│   ├── fake.go              # + new mutating methods on the in-memory map; guard support
│   └── *_test.go            # unit + //go:build integration (MinIO) for each op
└── ui/
    ├── operation.go         # + op kinds (delete/upload/copy/move/recursive); new phases
    ├── filebrowser.go       # NEW: local file browser view + key handling (upload source)
    ├── confirm.go           # reuse two-tier overlay; overwrite escalates to typed
    ├── commands.go          # + delete/upload/copy/move/recursive cmds + waitForProgress
    ├── messages.go          # + operationProgressMsg; operationDoneMsg gains summary/partial
    ├── keys.go              # + delete/upload/copy/move/recursive-delete keybindings
    ├── app.go               # route the new intents; refresh affected level(s) on outcome
    └── *_test.go            # white-box: each flow, overwrite, partial-move, recursive partial

scripts/
└── check-readonly.sh        # RETAINED unchanged (confines new SDK mutations to storage)

cmd/s3s/main.go              # unchanged (no new flag; --write/readonly already wired in 002)
```

**Structure Decision**: Single Go module, strict core/UI separation per Constitution
I. All write capability — every `Mutator` method, its SDK call, the read-only guard,
and the composite move/recursive orchestration — is confined to `internal/storage`;
the UI consumes it through the `storage.Mutator` interface and never imports the SDK.
The only new non-storage core is `internal/localfs`, a tiny UI-agnostic reader for
the upload file browser (kept out of `internal/ui` so its logic is unit-testable
without Bubble Tea). `check-readonly.sh` is kept as-is. Module path:
`github.com/danchupin/s3s`.

## Complexity Tracking

> No constitution violations. Section intentionally empty.
