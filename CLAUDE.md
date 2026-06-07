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
Active feature: 011-two-pane-hotkeys (three-zone master-detail browse + hotkey mnemonic review; 4
user stories) — PLANNED (spec + plan + Phase 0/1 artifacts done; tasks not yet generated).
010-pinned-buckets — IMPLEMENTED. 008/007/006/005/004/003/002/001 — complete.

Plan: specs/011-two-pane-hotkeys/plan.md. Artifacts (specs/011-two-pane-hotkeys/): spec.md (US1 live
bucket-contents pane P1, US2 cross-pane focus nav P1, US3 preserve adaptive details pane P2, US4
hotkey mnemonic review + bold glyphs P2; FR-001..027 incl. FR-002a/006a-e, SC-001..010, 8
clarifications), research.md (R1..R10), data-model.md, quickstart.md, contracts/ (keymap-contract,
two-pane-layout, lazy-load-cache), checklists/ (requirements 16/16, lazy-load 28/28). Constitution
v1.0.0; NO amendment (touches I+II+III; IV N/A justified — no storage-contract change). check-readonly
STAYS green (no new write-S3 symbol, no new storage method; only a test-only read counter on Fake).

GOAL: split browse into Miller-columns — buckets │ objects │ details. Highlighting a bucket lazily
loads its first level into the OBJECTS zone (no Enter); details zone (feature 006) preserved as
adaptive 3rd zone (bucket-meta when bucket focused, object-meta+preview when object focused). Focus
crosses: Tab=symmetric toggle, →/l/Enter-on-bucket cross IN, ←/h/Esc ascend-or-return (FR-009 precedence:
clear-search → ascend → return-to-buckets). Tiers: Full ≥130 (3 zones) / Dual 100-129 (buckets│objects,
details collapses) / Single ≤99 (today's single-column stack UNCHANGED, Enter-on-bucket still drills).

Key approach (canonical Design B — reuse m.level/treeSel, see research.md R6 reconciliation): objects
zone REUSES m.level (content) + treeSel (cursor) — NO separate objLevel. Load REUSES loadLevel→levelMsg
→onLevel + cache.Key{ctx,bucket,Prefix:"",Search:""} via levelKeyFor (shared cache w/ tree view). NEW
state = ONLY App.focusZone (zoneBuckets|zoneObjects, default zoneBuckets) + App.bucketLoadGen (debounce
counter). Scrolling buckets → bucketLoadGen debounce tick (paneDebounce 180ms ≤200ms, mirror
afterSelectionMove/onPaneTick) → settle: beginLoad+loadLevel into m.level, reset treeSel=0 (as enterLevel);
superseded load dropped by m.gen guard in onLevel, superseded tick by bucketLoadGen. CROSSING NEVER LOADS
(m.level already loaded lazily); load only on folder-drill inside objects. Lazy: startup loadBuckets =
NAMES only, 0 object listings (FR-002a/SC-010). First page = DefaultMaxKeys 1000 (SAME as enterLevel →
shared-cache invariant); paging via fetchNextPage. Errors NOT cached (failure=errMsg, onLevel Puts only on
levelMsg) → revisit re-attempts (FR-006c). In-flight dedup via the gen guard (FR-006d). Layout: listWithPane
→ 3 boxes via JoinHorizontal, paneW math reused, boxView active(accent)/dim per focusZone; windowBounds
stateless reflow; footer never scrolls (boxView minRows cap). Focus: Update dispatch branches on focusZone
(zoneBuckets→bucketSel, zoneObjects→treeSel) for →/←/Esc/Tab/Enter; Tab=symmetric toggle.
Hotkeys: defaultKeys REMOVES AddConn 'n' (app.go:604 dispatch gone; "+ add connection" row connections.go:103
stays sole affordance); y/ctrl+o kept; ALL advertised keys rendered Bold (today accentStyle fg-only — add
.Bold(true)) in keyGlyph/formatKeys/hintbar/helpLines; NO_COLOR keeps non-color cue; single keymap source
→ dispatch+hintbar+help never drift. Tests: white-box package ui (deliver/press/viewOf) + storage.Fake
NEW read list-call counter (assert 0 obj-listings at startup, ≤1 per fast-scroll, 0 on revisit, re-attempt
after denied); NO integration (no storage contract change).
<!-- SPECKIT END -->
