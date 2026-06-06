# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`s3s` is a read-only, keyboard-driven TUI for browsing S3-compatible storage
(Ceph RGW, MinIO). See `README.md` for the user-facing overview and `ROADMAP.md`
for planned work.

## Commands

```bash
make test              # unit tests (in-memory fake storage; no Docker)
make test-integration  # integration tests against real MinIO (testcontainers)
make fmt vet lint      # gofmt, go vet, golangci-lint
make check-readonly    # structural read-only guard (scripts/check-readonly.sh)
make build             # -> bin/s3s

go test ./internal/ui/ -run TestBucketFilter   # a single test
go test -cover ./...                            # coverage
```

- **golangci-lint must be built with the module's Go toolchain** (`go 1.25` in
  go.mod). A lint binary built with an older Go refuses to run ("targeted Go
  version" error).
- **Integration tests** `t.Skip` automatically when no Docker provider is found.
  testcontainers doesn't auto-detect a Lima-based Docker; if `make
  test-integration` skips, run with `DOCKER_HOST` pointing at the Lima socket and
  `TESTCONTAINERS_RYUK_DISABLED=true`.

## Non-obvious gotchas

- **Bubble Tea v2 import path is `charm.land/bubbletea/v2`** (and
  `charm.land/lipgloss/v2`), NOT `github.com/charmbracelet/...`. `go get` resolves
  the github path via redirect, but imports must use `charm.land/...` or the build
  fails on the module-path mismatch.
- **`Init()` cannot mutate the model in v2** (it returns only a `tea.Cmd`). The
  initial load's generation/context is therefore armed in `New()`, not `Init()` —
  otherwise the first message is dropped as stale.
- **Terminal image protocols don't survive Bubble Tea v2's cell renderer.** It
  understands cells (incl. truecolor), so kitty/iTerm2 graphics escapes are
  stripped. Half-block (`internal/preview`) is the working default; the protocol
  code behind `S3S_IMAGE_PROTOCOL=kitty|iterm2|auto` is experimental.

## Architecture (the big picture)

The codebase is governed by a **constitution** (`.specify/memory/constitution.md`,
v1.0.0). Its five principles are **I. Core/UI Separation**, **II. Non-Blocking
TUI**, **III. Test-First**, **IV. Integration Testing**, **V. Observability & Safe
Operations**. Note that principle V already anticipates *writes*: it mandates
explicit confirmation + logging for destructive actions (delete object/bucket,
overwrite, recursive remove). Adding mutations therefore does **not** require a
constitution amendment.

**Read-only is a current implementation posture of the 001 feature, not a
constitution principle** — it is enforced structurally and is expected to relax
when write features land. How the principles + that posture show up in code:

1. **Core/UI separation (constitution I).** `internal/storage` is the *only*
   package that imports `aws-sdk-go-v2/service/s3`. `internal/ui` depends on the
   `storage.Storage` interface, never the SDK. `internal/{config,cache,preview,logging}`
   are UI-agnostic and unit-tested without Bubble Tea.
2. **Read-only guard (implementation invariant, NOT a constitution principle).**
   `storage.Storage` exposes only four read methods (`ListBuckets`, `ListLevel`,
   `HeadObject`, `GetObjectRange`). `scripts/check-readonly.sh` (run in `make
   check-readonly`) fails the build if a write-capable S3 symbol (`PutObject`,
   `DeleteObject`, `CreateBucket`, …) appears outside `internal/storage`. This is
   the structural read-only invariant of the 001 browser — slated to change for the
   write iteration (guard relaxed/inverted, interface gains write methods).
3. **Non-blocking TUI (constitution II).** Every backend call runs in a `tea.Cmd`.
   The model holds a monotonic generation id (`m.gen`) and a per-load
   `context.CancelFunc` (`beginLoad`); navigating/searching cancels the previous
   load and bumps the generation, so stale result messages are dropped (each
   message carries the gen it was issued under). This is how superseded loads never
   corrupt the view.
4. **Real-backend integration tests (constitution IV).**
   `internal/storage/s3client_integration_test.go` (`//go:build integration`)
   exercises the real client against a MinIO container; it seeds data via a raw
   write client (today the only place writes are allowed, in a test, inside
   `internal/storage`).
5. **Observability & safe operations (constitution V).** `log/slog` JSON to a file
   only (the TUI owns the terminal). Secrets are a `logging.Secret` type that
   redacts in `String()`, `%v/%s`, and slog; `storage.classify` maps SDK errors to
   sentinels (`ErrNotFound`/`ErrAccessDenied`/`ErrUnreachable`/`ErrInvalidConfig`)
   without leaking detail. When writes land, destructive ops must add the
   confirmation + pre-execution log this principle requires.

How the pieces connect at runtime (`cmd/s3s/main.go`): load + validate config →
resolve the active context (flag > `S3S_CONTEXT` env > `current-context`) → build
a `storage.Storage` → construct the `ui.App` with a `Resolver` closure (rebuilds
the backend on context switch) → run the Bubble Tea program.

### UI rendering model (`internal/ui`)

- `App.View()` composes a **bordered list box** on top and a **multi-line status
  footer** at the bottom. The visible window is computed *statelessly* at render
  time via `windowBounds(n, sel, rows)` — selection indices are the only state,
  which makes resize reflow trivial.
- Height budgeting matters: the box body must not exceed its budget or the footer
  (incl. the hints line) scrolls off. Table views reserve 2 rows for the column
  header; `boxView` hard-caps the body to `minRows`.
- Footer lines (`footerInfoLine`/`footerHintsLine` etc.) are assembled
  segment-by-segment and **wrap** rather than drop, so keybindings stay visible on
  narrow terminals. Each parameter has its own color.
- `modeObject` (opened with `Enter` on an object) loads `HeadObject` and the
  bounded ranged `GetObject` *concurrently under one generation* and shows
  metadata + content together.
- `internal/cache` is a per-session, TTL-free level cache keyed by
  `(context, bucket, prefix, search)`; only manual refresh (`r`) invalidates it.

## Testing conventions

TDD is non-negotiable (constitution III): write the failing test first. UI tests
are white-box (`package ui`); drive the model with `deliver`/`press` helpers and
assert on `App.View().Content`. Storage units use the in-memory `storage.Fake`.

<!-- SPECKIT START -->
Active feature: 006-ui-redesign (k9s-style UI rework; 4 user stories) — IMPLEMENTED
(34/34 tasks; only the MinIO integration test t.Skips without Docker). 005/004/003/002/001
— complete.

Implemented surfaces (internal/ui): hintbar.go (menu-less direct-action catalog + dispatch),
pane.go + paneTick debounce (persistent details/preview, paneGen supersede, NEVER flips
modeObject), listWithPane split (≥100 cols), command.go (`:` registry), connections.go
(modeConnections/modeConnForm + Connector seam). keys.go rebind: a=analyze d=download
x=delete X=rm-r, `:`=command. config/connection.go = (*Config).AddConnection (triple,
keychain-first, no plaintext, mutates live cfg for in-session switch). cmd/s3s/connection.go
= connSeam (Test=New+ListBuckets, Save=AddConnection). actionmenu.go DELETED.

Plan: specs/006-ui-redesign/plan.md. Design artifacts (specs/006-ui-redesign/):
research.md (R1–R8), data-model.md, quickstart.md, contracts/ (layout-contract,
actions-keybindings-contract, command-bar-contract, connection-manager-contract). Governed by
Constitution v1.0.0; no amendment needed. Clarified: pane load = debounced; new connection =
persist-to-config; secrets = keychain. Slices by priority: (1) US1 menu-less direct keys + hint
bar [P1]; (2) US2 list+pane layout [P1]; (3) US3 `:` command bar [P2]; (4) US4 in-app connection
manager [P2].

Key approach. ui (almost everything): DELETE actionmenu.go + `modeActionMenu`; reuse its
selection/capability gating to build hintbar.go (always-visible `key label` catalog) + a direct
dispatch table — each key calls the SAME `start*`/`refresh` entry the menu used, so confirmations
are untouched (FR-005). REBIND: `a`→analyze (frees menu key), `d`→download, delete `d`→`x`,
recursive `D`→`X`, `r`→refresh direct; add `:` command bar. NEW pane.go: persistent details/preview
pane via DEBOUNCED load (paneTick ~150–250ms + paneGen supersede; new paneMetaMsg/panePreviewMsg
that do NOT flip `modeObject`); reuse loadMetadata/loadPreview. app.go View() splits body into
list+pane (JoinHorizontal ≥100 cols, stack/collapse below). NEW command.go (`modeCommand`,
registry buckets/contexts/conn/analyze/refresh/help/quit). NEW connections.go (`modeConnections`,
`modeConnForm`): form → injected `Connector{Test,Save}` seam from main.go (UI-agnostic ConnDraft;
keeps S3/config out of UI per Constitution I). Save maps one draft → cluster+user+context triple
(schema unchanged) via config.Upsert+Save; secret → secret.StoreKeychain FIRST (abort config on
keychain fail), `keychain:true` only in config (no plaintext, FR-022). Test=storage.New+ListBuckets;
fail → "save anyway" (FR-025a). connSavedMsg returns new context names → live in-session switch.
Read-only guard untouched (no new S3 write symbols leave internal/storage).
<!-- SPECKIT END -->
