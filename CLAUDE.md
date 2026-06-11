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

## Code style (constitution v1.3.0, Development Workflow)

**Comments must NOT reference spec-kit artifacts** — no US numbers, FR-/SC-/T-identifiers,
feature numbers (016/017/…), or research/contract/data-model pointers in code, tests, or any
product-visible text. Write the constraint itself in plain language. Pre-rule comments are
scrubbed opportunistically when a file is touched (ROADMAP backlog item).

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
v1.2.0). Its seven principles are **I. Core/UI Separation**, **II. Non-Blocking
TUI**, **III. Test-First**, **IV. Integration Testing**, **V. Observability & Safe
Operations**, **VI. UI Legibility** (every resource identifier fully visible or
revealable to read/copy; footer/command bar never scrolled off), and **VII. UI
Consistency & Design System** (shared prompt/label patterns; color as a consistent
accent via palette roles, never ad-hoc). Note that principle V already anticipates
*writes*: it mandates explicit confirmation + logging for destructive actions (delete
object/bucket, overwrite, recursive remove). Adding mutations therefore does **not**
require a constitution amendment. Principles VI–VII were added by feature 012
(`specs/012-ui-visibility-write-clarity`).

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
Active feature: 018-plugin-system — PLANNED. Plan: specs/018-plugin-system/plan.md. External plugins =
user-declared executables (explicit opt-in, `plugins:` config section), shlex argv exec (NEVER sh -c) +
owner-only config gate (secret/command.go precedent), one JSON request on stdin → one JSON response on stdout,
contractVersion=1, capped read 1MiB, default timeout 5s. v1 capabilities: bucket-discovery (US1 P1; merge ALWAYS
additive pinned ∪ listed ∪ discovered, dedup; invalid names discarded+counted; cap 5000) and object-metadata
(US2 P2; match rule connection+bucket-glob+keyPattern RE2; details-pane group "From <plugin>", 017 tri-state +
per-field copy reuse; caps 64 fields/4096B). US3 P3 status surface: NEW modePlugins full-screen 'P'/:plugins
(P free per keys.go), Esc→prevMode, space=toggle persisted via NEW Connector.SetPluginEnabled (AddBucket
pattern), Enter=error reveal. NEW UI-agnostic pkg internal/plugin (runner/sanitize/envelope; UI depends on
plugin.Runner iface, fake in tests). Request ctx: name/endpoint/userLabel/accessKeyId — SECRET NEVER (clarified).
Sanitize ALL plugin strings (CSI/OSC/C0/C1 strip) before render. Session cache, refresh-invalidated; gen-guard
drops stale. Logging: invocation facts only, never payload/argv. Actions + MCP = OUT of v1 (envelope
channel-agnostic for future MCP bridge). Zero plugins declared ⇒ zero UI change. No S3 SDK in internal/plugin;
check-readonly + MinIO suite untouched. Constitution v1.3.0 — no amendment. Artifacts: spec.md (3 US,
FR-001..019, SC-001..007, 4 clarifications), research.md D1-D15, data-model.md, quickstart.md (RED sets 0-3),
contracts/ (plugin-exec-contract, config-plugins, ui-plugin-surfaces).

017-usage-insights-ux — IMPLEMENTED+MERGED (#24, #25); manual validation (130×24 human pass + RGW prefix-wide
MPU check) still pending. MinIO QUIRK: ListMultipartUploads returns uploads ONLY for prefix==exact key —
bucket/prefix-wide listing EMPTY on MinIO (RGW/AWS honor prefixes); integration tests use exact-key path,
prefix semantics covered by Fake units. Keys taken by 017: Y/A/H/p.

016-metadata-enrichment — IMPLEMENTED+MERGED (#23); MinIO integration (T017/T031) + manual validation (T044)
still pending. 015 — #22. 014 — #20. conn-form cmd source — #21. 013 — PLANNED. 012 — #18. 011/010 — IMPLEMENTED.
008..001 — complete.
<!-- SPECKIT END -->
