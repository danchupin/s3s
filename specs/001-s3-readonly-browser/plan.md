# Implementation Plan: Read-Only S3 Browser (TUI)

**Branch**: `001-s3-readonly-browser` | **Date**: 2026-06-04 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-s3-readonly-browser/spec.md`

## Summary

An interactive, keyboard-driven terminal UI to browse S3-compatible object storage (Ceph RGW and
MinIO) **read-only**: select a cluster via a kubectl-style context, list buckets, walk the object
key namespace as a tree by the `/` delimiter with on-demand pagination, view object metadata,
preview content (text + images), and narrow a level with a server-side prefix search.

Technical approach: a Go application split into a UI-agnostic core (storage, config, preview) and
a Bubble Tea (v2) TUI layer. All S3 access goes through a read-only storage **interface** backed
by `aws-sdk-go-v2`; the UI runs every S3 call off the event loop via `tea.Cmd` returning a
`tea.Msg`, with `context` cancellation for superseded loads. Levels are cached for the session
with explicit manual refresh. Image preview defaults to ANSI half-block (works in any 24-bit
terminal) with an optional graphics-protocol path (kitty/iTerm2/sixel) when detected.

## Technical Context

**Language/Version**: Go 1.24

**Primary Dependencies**:
- S3: `github.com/aws/aws-sdk-go-v2` (+ `config`, `credentials`, `service/s3`)
- TUI: `github.com/charmbracelet/bubbletea/v2`, `github.com/charmbracelet/bubbles/v2`,
  `github.com/charmbracelet/lipgloss/v2`
- Image preview: `github.com/eliukblau/pixterm/pkg/ansimage` (half-block default);
  `github.com/blacktop/go-termimg` (optional kitty/iTerm2/sixel)
- Config: `go.yaml.in/yaml/v3` (maintained successor to the archived `gopkg.in/yaml.v3`)
- Logging: stdlib `log/slog` (JSON handler → file)

**Storage**: No local datastore. Remote S3-compatible backends (Ceph RGW, MinIO). Config file at
`~/.config/s3s/config.yaml` (XDG); per-session in-memory level cache only.

**Testing**: `go test`; unit tests against a hand-written fake of the storage interface;
integration tests against a real MinIO via `github.com/testcontainers/testcontainers-go/modules/minio`
(build-tagged `integration`, skipped when Docker is unavailable).

**Target Platform**: Cross-platform terminal (Linux/macOS), 24-bit color terminal.

**Project Type**: Single Go project (desktop CLI/TUI).

**Performance Goals** (from Success Criteria):
- Bucket list visible ≤ 2s after launch (SC-001)
- First page of a level ≤ 1s (SC-002); search first page ≤ 1s (SC-004)
- Context switch ≤ 2s without restart (SC-005)
- Text preview ≤ 1s, image preview ≤ 2s on capable terminal (SC-006)
- No frozen frames during in-flight loads (SC-007)

**Constraints**:
- ≤ 1 listing request per page shown; never load a whole bucket up front (SC-003, FR-010)
- Preview fetch bounded to first 5 MiB via ranged read (FR-014/016)
- Secrets never logged/displayed (FR-005, FR-021)
- Strictly read-only: zero create/update/delete requests (FR-019, SC-009)

**Scale/Scope**: Buckets up to millions of keys must stay navigable via on-demand paging
(S3 page size 1000 + UI windowing). 5 user stories, ~23 functional requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution v1.0.0 — five principles:

| # | Principle | How this plan satisfies it | Gate |
|---|-----------|----------------------------|------|
| I | Core/UI Separation | S3/config/preview logic lives in `internal/{storage,config,preview}`; only `internal/storage` imports `service/s3`; `internal/ui` depends on the storage **interface**, never the SDK. Core compiles/tests with no TUI dependency. | PASS |
| II | Non-Blocking TUI | Every S3 call wrapped in `tea.Cmd` → goroutine → `tea.Msg`; UI loop never blocks. Per-load `context.CancelFunc` + generation IDs cancel/drop superseded loads; spinner + cancel during loads. | PASS |
| III | Test-First (NON-NEGOTIABLE) | Tasks ordered tests-first (Red→Green→Refactor). Storage interface fake enables unit TDD; bug fixes start with a regression test. | PASS (enforced in tasks) |
| IV | Integration Testing | testcontainers-go MinIO exercises real auth, pagination boundaries, ranged reads, error paths, and the storage-client contract. | PASS |
| V | Observability & Safe Operations | `log/slog` JSON to a file (never the TUI frame), secrets excluded. Read-only is **structural**: interface exposes only read methods; CI guard forbids Put/Delete/Create/Copy symbols outside `internal/storage`. No destructive actions exist, so no confirm prompts needed. | PASS |

**Result**: All gates PASS. No violations → Complexity Tracking left empty.

## Project Structure

### Documentation (this feature)

```text
specs/001-s3-readonly-browser/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── storage-interface.md
│   ├── config-schema.md
│   └── tui-contract.md
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
cmd/
└── s3s/
    └── main.go              # entrypoint: load config → build storage → run tea.Program

internal/
├── config/                  # kubectl-style YAML: clusters/users/contexts/current-context
│   ├── config.go            # structs + loader + current-context resolver
│   └── config_test.go
├── storage/                 # read-only S3 interface + aws-sdk-go-v2 impl (ONLY s3 importer)
│   ├── storage.go           # interface: ListBuckets, ListLevel, HeadObject, GetObjectRange
│   ├── s3client.go          # aws-sdk-go-v2 implementation (endpoint/path-style/creds/anon)
│   ├── fake.go              # in-memory fake for unit tests
│   ├── storage_test.go      # unit tests (fake)
│   └── s3client_integration_test.go  // +build integration (MinIO testcontainer)
├── preview/                 # text + image rendering; no S3 dependency
│   ├── text.go
│   ├── image.go             # half-block default + protocol detection/enhanced
│   └── preview_test.go
├── cache/                   # per-session level cache (no TTL, manual invalidate)
│   ├── cache.go
│   └── cache_test.go
└── ui/                      # bubbletea models/messages/commands/styles
    ├── app.go               # root model, context switcher
    ├── tree.go              # bucket/prefix/object tree navigation + paging
    ├── metadata.go          # metadata pane
    ├── preview_view.go      # preview pane
    ├── search.go            # prefix search input
    ├── commands.go          # tea.Cmd wrappers around storage (async + cancellation)
    ├── messages.go          # tea.Msg types
    ├── keys.go              # keymap
    └── ui_test.go

logging/ or internal/logging/log.go   # slog file handler setup
```

**Structure Decision**: Single Go module with strict core/UI separation per Constitution I.
`internal/storage` is the sole importer of `aws-sdk-go-v2/service/s3`; `internal/ui` consumes the
storage interface. `internal/{config,preview,cache}` are UI-agnostic and unit-tested without Bubble
Tea. `cmd/s3s` wires them together. Module path assumed `github.com/dochupin/s3s` (adjust at
`go mod init` if different).

## Complexity Tracking

> No constitution violations. Section intentionally empty.
