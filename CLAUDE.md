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
Active feature: 014-credentials-config-path (credential sources → keychain+cmd only; config-path override;
5 user stories) — PLANNED (spec + clarify + plan done; tasks/impl pending). 013-ui-mode-footer-filter —
PLANNED (spec+clarify+plan; tasks/impl pending). 012-ui-visibility-write-clarity — IMPLEMENTED (#18).
011-two-pane-hotkeys / 010-pinned-buckets — IMPLEMENTED. 008/007/006/005/004/003/002/001 — complete.

Plan: specs/014-credentials-config-path/plan.md. Artifacts: spec.md (US1 two sources only P1, US2 removed
sources gone P1, US3 config-path override P2, US4 cross-platform keychain + headless loud-fail P2, US5 docs P3;
FR-001..022 incl. 008a/018a/020a, SC-001..008, 4 clarifications), research.md (R1..R8 grounded file:line),
data-model.md, quickstart.md, contracts/ (config-path-resolution, credential-sources, keychain-account-
namespacing, cred-and-wizard), checklists/requirements.md (16/16). Constitution v1.2.0 (Tech&Security credential
bullet amended this iteration: env/AWS-profile/prompt → keychain/cmd/prompt, never plaintext-in-config, headless
loud-fail toward cmd). check-readonly STAYS green (only removes cred code; no write-S3 symbol). IV: kept keychain
auth flow covered by existing cred_auth_integration_test; no storage-contract change → no new integration test.

GOAL: cut 4 credential sources to 2 (keychain default + cmd hatch); add config-path override for multiple configs.
DECISIONS (clarify): NO migration (pre-release, no users — just delete inline/${ENV}/sessionToken/awsProfile from
schema+resolver+validation+wizard); keychain account NAMESPACED by config-identity; cmd = secret only (no STS
token); config switch is relaunch-only.

Key approach (research.md R1-R8, file:line verified): (R2) secret.Inline FULLY removable — prompt fallback uses
ClientConfigWithSecret(name,sec)(resolve.go:91) which injects the raw secret, never builds Kind=Inline. (R3) ONE
helper keychainAccount(configPath,userName)=base64url(sha256(filepath.Abs(path)))[:8]+":"+userName at FIVE keystore
sites (map found 3; 2 more by direct read): secretRequest Ref(resolve.go:77), KeychainAccount(resolve.go:110),
AddConnection(connection.go:70), RemoveConnection(connection.go:180), wizard(generate.go:162). Config.path already
set by Load/Empty → NO signature changes; secret.{Get,Store,Remove}Keychain keep account string (Constitution I).
(R4) config.ConfigPath(flag,env)=flag>env>DefaultPath + const EnvConfig="S3S_CONFIG"; --config already exists
(main.go:53/cred.go:20/runConfigInit:197), only env+precedence new; explicit(flag||env set) non-existent path =
hard error, default path = first-run empty (FR-017). (R5) headless: GetKeychain(keychain.go:26) unavailable-store
error → name cmd remedy; prompt stays TTY-gated(main.go:149). DELETE: User.{SecretAccessKey,SessionToken,AWSProfile}
(config.go:70/71/74), awsprofile.go + awsprofile_test.go, secret.{Inline,AWSProfile} + Resolve cases, Resolved.
SessionToken, EnvVarName + env/awsProfile wizard branches + export-hint; ClientConfig sessionToken block(148-154);
keep ${ENV} for accessKeyId only. (R7) drop now-unused logging imports (config.go/generate.go) — go build flags.
(R8) TDD failing-first per US; delete Inline/awsProfile/sessionToken/env tests; migrate validYAML→keychain (mock
keyring); add namespacing-isolation + ConfigPath-precedence + explicit-not-found tests; update connection_
integration_test asserted account to namespaced. README/ROADMAP: 4 sources→2, ROADMAP item→Done.
<!-- SPECKIT END -->
