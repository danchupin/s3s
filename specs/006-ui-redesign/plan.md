# Implementation Plan: UI Redesign (k9s-style, menu-less actions, in-app connections)

**Branch**: `006-ui-redesign` | **Date**: 2026-06-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/006-ui-redesign/spec.md`

## Summary

Rework the TUI from a full-width list + modal action menu into a **k9s-style
single table with a persistent details/preview pane**, **menu-less single-key
actions** backed by an always-visible contextual hint bar, a **`:` command bar**
for jumping and discovery, and an **in-app connection manager** that writes a new
cluster connection to the config (secret to the OS keychain). All existing
capabilities (browse, contexts, object view, download, `du`, bulk, sort, write
toggle, confirmations) are preserved — only the interaction surface and layout
change.

The change is almost entirely in `internal/ui`. The connection writer and keychain
write reuse existing UI-agnostic packages (`internal/config` `Upsert`/`Save`,
`internal/secret` `StoreKeychain`, `internal/storage` `New`/`ListBuckets`) behind a
new closure seam injected by `cmd/s3s/main.go`, so Core/UI Separation (Constitution
I) holds and no S3/config logic enters the UI.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`).

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2
(`charm.land/lipgloss/v2`), `aws-sdk-go-v2/service/s3` (confined to
`internal/storage`), `zalando/go-keyring` (in `internal/secret`),
`golang.org/x/term`.

**Storage**: S3-compatible backends (Ceph RGW, MinIO) via `storage.Storage`;
local config YAML (`~/.config/s3s/config.yaml`) for connections; OS keychain for
secrets.

**Testing**: `go test` white-box UI tests (`package ui`, `deliver`/`press`
helpers, assert on `App.View().Content`); `storage.Fake` for units;
`//go:build integration` tests against MinIO via testcontainers; `go-keyring` mock
keyring for keychain unit tests.

**Target Platform**: macOS/Linux terminal (cross-platform CLI/TUI).

**Project Type**: Single-project Go CLI/TUI (`cmd/s3s` + `internal/*`).

**Performance Goals**: 60 fps render; no per-row backend call during fast scroll —
the details pane fetch is debounced ~150–250 ms and superseded loads are
cancelled (FR-009); every backend/keychain call off the event loop (Constitution
II).

**Constraints**: Read-only-by-default safety preserved; the structural read-only
guard (`scripts/check-readonly.sh`) stays green — no new S3 write symbols leave
`internal/storage`; secrets never written to config in plaintext (FR-022/FR-005);
all destructive direct-key actions keep the two-tier confirmation (Constitution
V); layout must not clip the hint bar/footer/write badge at 80×24 (FR-013).

**Scale/Scope**: One feature, 4 user stories, ~30 functional requirements. Touches
~10–14 files in `internal/ui`, adds a `config.AddConnection`-style writer and a
`main.go` connector seam; no new top-level package required.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution v1.0.0, five principles. No amendment needed (a UI/UX rework within
existing principles).

- **I. Core/UI Separation** — PASS. New behavior (connection persistence, keychain
  store, reachability test) lives in `internal/{config,secret,storage}`; the UI
  reaches it only through an injected `Connector` closure (mirrors the existing
  `Resolver` seam). No SDK or config-marshalling logic enters `internal/ui`.
- **II. Non-Blocking TUI** — PASS. The debounced pane load, the connection test,
  the keychain write, and the config save all run in `tea.Cmd`s and report back
  via messages carrying a generation id; superseded loads drop. The render loop
  never blocks.
- **III. Test-First** — PASS (process gate). Every new behavior gets a failing
  white-box UI test or a config/secret unit test first (hint-bar contents, direct
  keys, debounced pane, command bar parsing, connection-form validation, writer
  mapping, duplicate rejection).
- **IV. Integration Testing** — PASS. The in-app add-connection path is covered by
  an integration test that creates a connection against MinIO, runs the
  reachability test, stores/reads the secret from a (mock or real) keyring, and
  switches to the new context.
- **V. Observability & Safe Operations** — PASS. Direct-key destructive actions go
  straight into the *existing* confirmation flows (no bypass, FR-005); a new
  `config.write`/`connection.add` log line records the add before/after; secrets
  stay `logging.Secret`-redacted and never logged.

**Read-only guard (implementation invariant)** — UNCHANGED. This feature adds no
S3 write symbols outside `internal/storage`; `make check-readonly` stays green.

Result: **PASS, no violations. Complexity Tracking left empty.**

## Project Structure

### Documentation (this feature)

```text
specs/006-ui-redesign/
├── plan.md              # This file
├── research.md          # Phase 0 — decisions (R1–R8)
├── data-model.md        # Phase 1 — entities (Connection form, Action, pane state, command)
├── quickstart.md        # Phase 1 — how to exercise the redesigned UI
├── contracts/           # Phase 1 — behavior contracts
│   ├── layout-contract.md
│   ├── actions-keybindings-contract.md
│   ├── command-bar-contract.md
│   └── connection-manager-contract.md
└── checklists/
    └── requirements.md  # from /speckit-specify (validated)
```

### Source Code (repository root)

```text
cmd/s3s/
├── main.go              # MODIFY: build + inject a Connector closure (test→store→upsert→save→names)
└── cred.go              # reused (keychain account helpers, StoreKeychain)

internal/config/
├── config.go            # reused: Config/Cluster/User/Context, Validate, KeychainAccount
├── generate.go          # reused: Upsert(cl,u,cx,setCurrent), Marshal, Save; MODIFY if a
│                        #   single AddConnection helper is added here (UI-agnostic)
└── resolve.go           # reused: ClientConfig, ContextNames, WriteModeFor

internal/secret/
└── keychain.go          # reused: StoreKeychain(account, secret) (no change expected)

internal/storage/
└── s3client.go          # reused: New(ClientConfig), ListBuckets (reachability test)

internal/ui/
├── app.go               # MODIFY: View() splits body into list+pane; remove modeActionMenu;
│                        #   add modeCommand, modeConnections/modeConnForm; new state fields
├── keys.go              # MODIFY: rebind (a→analyze, d→download, x/X→delete/recursive), drop Menu;
│                        #   add Command(:), Download, Analyze, Connect bindings
├── pane.go              # NEW: persistent details/preview pane (debounced load, superseded-drop)
├── hintbar.go           # NEW: always-visible contextual action catalog (replaces footerHints+menu)
├── command.go           # NEW: `:` command bar (registry, parse, dispatch)
├── connections.go       # NEW: connection manager list + add-form modes, validation, save flow
├── actionmenu.go        # REMOVE (modal menu deleted, FR-001) — logic folded into hintbar/keys
├── tree.go              # MODIFY: treeView width-aware (leaves room for the pane)
├── messages.go          # MODIFY: add paneMetaMsg/panePreviewMsg, connTestedMsg/connSavedMsg
├── commands.go          # MODIFY: add debounced pane loaders, connection test/save cmds
├── styles.go            # MODIFY: pane border, hint-bar styles, disabled-action style
└── *_test.go            # NEW/MODIFY: white-box tests for every story
```

**Structure Decision**: Single Go project, unchanged layout. The feature is
concentrated in `internal/ui` with thin reuse of `config`/`secret`/`storage`
seams. `actionmenu.go` is deleted; its selection/capability logic is reused by the
new `hintbar.go` and the `:` command registry. The only cross-package addition is
an optional `config.AddConnection` convenience wrapper over the existing `Upsert` +
`Save`, kept in `internal/config` to honor Core/UI Separation.

## Complexity Tracking

> No constitution violations. No entries.
