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
Active feature: 016-metadata-enrichment (enrich HeadObject fields omit-empty in SHARED metaFieldRows; fold analyze/
modeUsage into inline usage totals+ONE-section breakdown w/ dedicated usageGen+usageCancel + ungated channel-drain;
new read-only GetObjectTagging/GetBucketConfiguration tri-state, NotFound→none vs NotImplemented/501→ErrUnsupported,
unsupported via Fake+classify units only; storage-class marker w/ reveal; 'a' Analyze→MoreDetail shared by :detail
cmd) — IMPLEMENTED unit (US1-US5 green: enriched metadata, inline usage totals+breakdown, tags+config
tri-state, storage-class marker; lint+vet+check-readonly green); MinIO integration (T017/T031) + manual
validation (T044) pending. 015-filter-ux-redesign — IMPLEMENTED (#22).
014-credentials-config-path — IMPLEMENTED (#20). conn-form cmd source — IMPLEMENTED (#21). 013-ui-mode-footer-filter
— PLANNED. 012-ui-visibility-write-clarity — IMPLEMENTED (#18). 011/010 — IMPLEMENTED. 008/007/006/005/004/003/002/001
— complete.

Plan: specs/016-metadata-enrichment/plan.md. Artifacts: spec.md (US1 rich object meta P1, US2 inline bucket/prefix
totals P1, US3 expandable breakdown P2, US4 tags+bucket-config tri-state P2, US5 storage-class in list P3; FR-001..019,
SC-001..007, 4 clarifications), research.md (Decision/Rationale/Alt, file:line), data-model.md, quickstart.md,
contracts/ (object-metadata-pane, storage-read-extension, inline-usage, more-detail-key, listing-storage-class,
layout-budget), checklists/requirements.md (16/16). Constitution v1.2.0 (VI UI Legibility + VII UI Consistency drive;
NO amendment). check-readonly STAYS green (Get* only, never write-verb regex; SDK stays in internal/storage). IV
REQUIRED: US4 adds GetObjectTagging/GetBucketConfiguration to storage-client contract → MinIO integration tests.

DECISIONS (clarify): (1) on-demand affordances = single freed 'a' = context-aware MoreDetail (bucket/prefix→expand
breakdown+load config; object→tags+governance), shared by :detail cmd; (2) object pane OMITS empty optional fields,
always shows core + permission-gated as "unknown/denied"; (3) usage scan = dwell-gated (tea.Tick), session-cached
(usageResults keyed (context,bucket,prefix)), cancel-on-navigate. version-history OUT of scope.

Key corrected design (adversarial-verified, file:line): (1) height-budget failure mode is NOT footer-loss (footer
composed after body; boxViewWith hard-caps body to minRows, styles.go:348-350) but SILENT TRUNCATION of pane content
— at 130×24 footerH≈6 + filterFieldH=3 → details body budget rows-2≈11; enriched obj (6 core + 6-9 optional) + tags +
breakdown overflow → FIX: ONE expandable detail section at a time (breakdown XOR tags XOR config) + "… +N more (i to
reveal)" affordance + 130×24 height-sweep test asserting every seeded value present OR revealable. (2) inline scan owns
dedicated usageGen + usageCancel, NOT m.gen/loadCancel (analyze.go:65 beginLoad coupling removed); cancel = usageCancel()
+ usageGen++ together on focus-move & in beginLoad; result-application gated on usageGen, channel pump (waitForUsage
re-arm) DRAINS regardless of gen (mirror analyze.go:100-108) so no producer leak. (3) dwell: extend afterSelectionMove
(app.go:328-338) to arm usageTick for dir/level too (not just objects); onUsageTick fires loadUsage only if
gen==usageGen AND focusedUsageTarget() unchanged AND not cached. (4) refresh: tree 'r' (tree.go:144 cache.Invalidate)
+ bucket 'r' (refreshBuckets, hintbar.go:175) both also Invalidate(usageResults). (5) classify: *NotFound/*NotConfigured
→ none; reserve ErrUnsupported for NotImplemented/501/405 (untestable on MinIO → Fake-unit + classify-unit). Delete
modeUsage RED set COMPLETE: app.go:30/219-227/881/1190-1191, analyze.go, command.go:33+57(canOpenCommand),
footer_test.go:194/249, keys.go:21/54, hintbar.go:52/70, pane.go:54/67/71.
<!-- SPECKIT END -->
