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
v1.1.0). Its seven principles are **I. Core/UI Separation**, **II. Non-Blocking
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
Active feature: 013-ui-mode-footer-filter (mode chip dedup, footer breathing room, applied-filter state;
3 user stories) — PLANNED (spec + clarify + plan done; tasks/impl pending). 012-ui-visibility-write-clarity
— IMPLEMENTED (#18). 011-two-pane-hotkeys — IMPLEMENTED. 010-pinned-buckets — IMPLEMENTED.
008/007/006/005/004/003/002/001 — complete.

Plan: specs/013-ui-mode-footer-filter/plan.md. Artifacts: spec.md (US1 single read/write mode indicator P1,
US2 applied-filter state visible P1, US3 footer/menu breathing room P2; FR-001..018, SC-001..008,
2 clarifications), research.md (R1..R7, grounded file:line), data-model.md, quickstart.md, contracts/
(border-chip, mode-indicator, applied-filter, footer-spacing, layout-visibility), checklists/requirements.md
(16/16). Constitution v1.1.0 (VI UI Legibility + VII UI Consistency/Design System drive this feature; no
amendment). check-readonly STAYS green (no new write-S3 symbol, no storage method). NO integration (no
storage-contract change; IV N/A justified). All changes in internal/ui; no new file, no new package, no new
keymap field, no new hue.

GOAL: small presentation iteration continuing 012. (1) ONE read/write mode indicator: keep the border chip,
remove the duplicate footer [RW]/[RO] tag. (2) Applied-filter state visible as a persistent border chip on the
filtered pane. (3) Wider footer/command-bar spacing without breaking the no-wrap/no-scroll invariant. Two
clarifications: filter indicator = border chip on the FILTERED pane (NOT footer, NOT breadcrumb title);
write-state in non-list modes = a UNIVERSAL mode chip on every browse box (one render path).

Key approach (grounded research.md R1-R7): (R1/US1) the chip already rides buckets/tree boxes (app.go:1256/
1270/1286); the ONLY browse box missing it is modeObject (app.go:1178 plain boxView) → swap to boxViewChip +
m.modeChip(). (R2/US1) strip [RW]/[RO] from footerIdentityCompact (styles.go:512-524) → identity = ● ctx ·
cluster; update 3 callers (app.go:1382, commandbar.go:185/254). Modal writeBadge (confirmview.go:36/69,
writemode.go:56) + help badge (app.go:1130) STAY (safety, not the dup). (R3/US2) applied-filter chip per-pane:
buckets box when m.bucketFilter!='' && !m.searching; objects box when m.search!='' && !m.searching (per-pane
field, NOT focus-relative committedFilterTerm search.go:14). warnStyle (no new hue), "filter: term" capped
with … (boxViewWith drops chip whole — does not elide; full term via re-opening / pre-filled). MOVE breadcrumb-embedded
markers off the title: drop objectsZoneTitle ' (term*)' (app.go:1354) + resourceTitle '/term*' (app.go:1478).
Clear is automatic (goBack tree.go:154 / objectsBack app.go:525 / ctx switch app.go:1063). (R4) extend
boxViewWith (styles.go:334-406) with a 2nd INBOARD chip slot; degrade order: center → filter chip → mode chip
LAST (mode safety-critical); objects pane gains a chip-bearing variant (today boxViewFocus, no slot). (R5/US3)
one separator token ' · '→'  ·  '; entryStyled key↔label 1→2 (commandbar.go:159); inter-column gap 2→3 via a
colGap const with natural := …+2*len(colGap) (commandbar.go:175/179 — kills the +4 double-count). Other
fitters (fitEntries, renderHintRow, footerIdentityCompact cluster-append) re-measure via lipgloss.Width →
self-accounting. (R7) TDD white-box package ui (deliver/press/viewOf/stripANSI; dualApp/treeApp/buildApp/
crossToObjects) + Fake.ListLevelCalls; failing-first per US; MIGRATE existing [RW]/[RO] asserts
(operation_test/visual_test/writemode_test/spec012_test) to the chip + keep width guards (footer_test
assertWidthSweep 40..200 ≤9 rows) green. Layout invariant: chips border-only (0 body rows) + boxViewWith
minRows cap → footer never scrolls (FR-016) at every tier.
<!-- SPECKIT END -->
