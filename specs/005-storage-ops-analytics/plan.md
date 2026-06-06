# Implementation Plan: Storage Operations & Analytics

**Branch**: `005-storage-ops-analytics` | **Date**: 2026-06-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/005-storage-ops-analytics/spec.md`

## Summary

Six user stories across three themes turn s3s from a browser into a daily-driver admin tool,
without weakening — and in fact strengthening — its read-only-by-default safety posture:

- **Get data out & see what's there** (US1 download, US2 `du` analytics) — both are *read*
  operations, so they work in read-only contexts and against production.
- **Operate at scale safely** (US3 multi-select bulk, US4 sortable lists) — bulk download is a
  read op; bulk delete/copy are write-gated and reuse the existing two-tier confirmation.
- **Safety & credential backbone** (US5 runtime read-only↔write toggle with a loud always-on
  WRITE indicator, US6 pluggable secure credential sources) — the safety/security foundation
  that must land before the bulk mutations are exposed.

Technical approach (all within the existing architecture): the **storage** package gains two
**read** methods (`GetObject` full-stream, `UsageOf` recursive aggregation) — no new SDK write
symbols, so `check-readonly.sh` is unaffected. The **ui** package gains download/analyze
operations (reusing the existing `operation`/progress/cancel machinery), per-level selection
state, a stateless render-time sort, and a runtime write-arm flag with a high-contrast
indicator. The read-only **guard** moves from construction-time (in `main`) to *dynamic* in the
UI, re-derived on every toggle and context switch. A new **secret** package adds credential
resolvers (keychain / command / AWS profile / env / prompt) behind a one-source-per-context
rule; `cmd/s3s` gains a `cred` subcommand, a startup secure prompt, and a wizard extension.

## Technical Context

**Language/Version**: Go 1.25.0

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2
(`charm.land/lipgloss/v2`), `aws-sdk-go-v2` (storage package only). **New**:
`github.com/zalando/go-keyring` (cross-platform OS keystore: macOS Keychain, Linux Secret
Service via D-Bus, Windows Credential Manager); `golang.org/x/term` (no-echo secret prompt);
`github.com/google/shlex` (POSIX-style argv split for the `cmd:` credential source).

**Storage**: S3-compatible backends (Ceph RGW, MinIO) via the `storage.Storage` interface;
local filesystem for downloads (`internal/localfs` + `os`); OS keystore + `~/.aws/credentials`
for secrets.

**Testing**: `go test` with in-memory `storage.Fake` (unit, white-box ui), testcontainers
MinIO (`//go:build integration`) for real-backend read methods; fakes for the keystore/command
secret resolvers.

**Target Platform**: macOS, Linux, Windows terminals (TUI). Headless Linux / SSH is a
first-class target for US6 (keystore may be absent → command/profile/env/prompt sources).

**Project Type**: Single Go project — keyboard-driven TUI (CLI binary `cmd/s3s`).

**Performance Goals**: 60 fps-feel TUI — the event loop never blocks (Constitution II). `du` and
downloads stream/aggregate incrementally with live progress; a multi-hundred-MB download and a
millions-of-keys recursive scan both stay cancellable with no frozen frame (SC-002, SC-006).

**Constraints**: No SDK write symbols outside `internal/storage` (`check-readonly.sh`). Secrets
never written to disk in plaintext, never required in env, redacted everywhere (FR-035/039,
SC-012/014). Read-only is default-safe; `readonly: true` is an absolute lock (FR-024/028).

**Scale/Scope**: Buckets with millions of keys (paginated reads, never a full up-front load);
bulk actions over 50+ objects (SC-003); per-session, per-level selection.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Compliance |
|-----------|------------|
| **I. Core/UI Separation** | Download/`du` are **read** methods on `storage.Storage`; the UI still never imports the SDK. Credential resolution lives in a new UI-agnostic `internal/secret` package (+ `internal/config`); the keystore/command/profile logic is unit-tested without Bubble Tea. ✅ |
| **II. Non-Blocking TUI** | Every new backend call (full download stream, recursive `du`, each bulk item) runs in a `tea.Cmd` under the monotonic `gen`, reports progress over a channel, and is cancellable; superseded results are dropped. The write toggle and sort are pure in-memory state. ✅ |
| **III. Test-First** | TDD throughout. New storage read methods get failing unit (Fake) + integration tests first; the dynamic guard, selection, sort, toggle, and each secret resolver get white-box/contract tests before code. ✅ |
| **IV. Integration Testing** | `GetObject` (full-object, large/multipart-sized) and `UsageOf` (pagination boundaries, deep prefixes, empty prefix) are exercised against MinIO. Credential-source resolvers are tested against a fake keystore/command; the AWS-profile parser against fixture files. ✅ |
| **V. Observability & Safe Operations** | Write-toggle transitions are logged as security events (FR-032); bulk deletes log each op before execution and keep typed confirmation (FR-017); the `cmd:` source is gated on owner-only config perms (FR-036); secrets stay `logging.Secret`-redacted. Download/`du` are reads — they cannot mutate. ✅ |
| **Read-only guard (impl invariant)** | `storage` exposes only read methods + the existing `Mutator`; `GetObject`/`UsageOf` are reads. The guard becomes *dynamic* (re-wrap on toggle) but remains the single runtime enforcement point. `check-readonly.sh` is unaffected — no new write SDK symbols, UI still SDK-free. ✅ |

**No constitution amendment required.** The `--write` semantics change (hard gate → initial
armed state) is strictly *more* protective at the moment of mutation (Principle V already
mandates confirmation + logging; the toggle adds a loud always-on indicator + deliberate
arming). **No Complexity Tracking entries needed.**

## Project Structure

### Documentation (this feature)

```text
specs/005-storage-ops-analytics/
├── plan.md              # This file
├── research.md          # Phase 0 — technical decisions
├── data-model.md        # Phase 1 — entities → Go types
├── quickstart.md        # Phase 1 — try the new capabilities
├── contracts/           # Phase 1 — storage / credential / write-toggle / action-menu contracts
│   ├── storage-read-ops-contract.md
│   ├── credential-source-contract.md
│   ├── write-toggle-contract.md
│   └── action-menu-selection-contract.md
├── checklists/
│   └── requirements.md  # spec quality (done)
└── tasks.md             # Phase 2 — /speckit-tasks (NOT created here)
```

### Source Code (repository root)

```text
cmd/s3s/
├── main.go              # CHANGED: stop pre-guarding (UI guards dynamically); resolve the active
│                        #   secret source at startup (prompt allowed pre-TUI); pass raw store +
│                        #   ctx-readonly + initial armed state into ui.New
├── cred.go              # NEW: `s3s cred set|rotate|rm <context>` keystore subcommand
└── (config init wiring) # extended for credential-source choice

internal/storage/
├── storage.go           # CHANGED: add read methods GetObject(full stream) + UsageOf(recursive
│                        #   aggregate w/ progress) to the Storage interface
├── s3client.go          # CHANGED: implement GetObject (no range) + UsageOf (paginated, no
│                        #   delimiter, client-side immediate-child bucketing)
├── fake.go              # CHANGED: implement the two new read methods for unit tests
└── guard.go             # UNCHANGED (read methods pass through; Guard() still wraps mutations)

internal/secret/         # NEW package — UI-agnostic credential-source resolution (Constitution I)
├── source.go            # Source kind + descriptor; ResolveSecret(); one-source validation
├── keychain.go          # go-keyring store/fetch/remove; service+account namespacing
├── command.go           # exec the cmd, capture stdout, owner-only-perms gate (FR-036)
├── awsprofile.go        # parse ~/.aws/credentials (ini) for a profile's static keys
└── prompt.go            # x/term no-echo prompt + optional save-to-keystore

internal/config/
├── config.go            # CHANGED: User gains source descriptors; Validate enforces exactly-one
│                        #   source (FR-041); config-perms warning (FR-040)
├── resolve.go           # CHANGED: ClientConfig resolves via internal/secret (single source)
└── generate.go          # CHANGED: wizard asks credential source; keychain → store in keystore

internal/ui/
├── app.go               # CHANGED: hold raw store + ctxReadOnly + armed; derive writable;
│                        #   activeStore() guards dynamically; clear selection on navigation
├── download.go          # NEW: full-object download op (read; temp-file + atomic rename + cancel)
├── analyze.go           # NEW: modeUsage view + UsageOf dispatch + drill-down
├── selection.go         # NEW: per-level object selection (mark/unmark, count, combined size)
├── bulk.go              # NEW: bulk download/delete/copy over a selection (per-item results)
├── sort.go              # NEW: stateless render-time sort (col + dir), session-persistent
├── writemode.go         # NEW: arm/disarm toggle, loud indicator, context-switch re-derive, log
├── operation.go         # CHANGED: allow a non-Mutator (read) download op kind
├── actionmenu.go        # CHANGED: add download / analyze / selection / bulk items (gated)
├── keys.go              # CHANGED: Mark (space), Sort (+dir), WriteToggle bindings (download/
│                        #   analyze/bulk are menu-only — FR-023 footer declutter)
└── styles.go            # CHANGED: loud WRITE indicator style (high-contrast)

scripts/check-readonly.sh # UNCHANGED — still green (no new write SDK symbols, UI SDK-free)
```

**Structure Decision**: Single Go project, existing `internal/` layout preserved. The one new
package is `internal/secret` (credential-source resolution), kept UI- and SDK-agnostic to honor
Core/UI Separation and to be unit-testable with fakes.

## Phasing (recommended slice order for /speckit-tasks)

This feature is large (6 stories, 3 themes). Implement and ship in dependency order so each slice
is independently valuable and the safety backbone lands before easier mass-mutation:

1. **Slice 1 — Safety & credential backbone (US5 + US6, both P1).** Dynamic guard + runtime
   toggle + loud indicator; credential sources + `cred` subcommand + wizard + perms gate. No new
   remote operations, but reshapes the write/credential model everything else builds on.
2. **Slice 2 — Read power tools (US1 + US2, P1/P2).** `GetObject`/download and `UsageOf`/analyze.
   Pure reads; high value; safe against production. Independent of bulk.
3. **Slice 3 — Scale & ergonomics (US3 + US4, P3/P4).** Multi-select bulk (delete/copy gated by
   Slice 1's armed state; download from Slice 2) and sortable lists.

Each slice is a viable MVP on its own; the spec's per-story Independent Tests define the gates.

## Complexity Tracking

No constitution violations — section intentionally empty.
