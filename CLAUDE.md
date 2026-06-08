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
Active feature: 015-filter-ux-redesign (always-visible filter; keep bucket+object scopes; fix "doesn't fit";
5 user stories) — PLANNED (spec + clarify + plan done; tasks/impl pending). 014-credentials-config-path —
IMPLEMENTED (#20). conn-form cmd source — IMPLEMENTED (#21). 013-ui-mode-footer-filter — PLANNED (spec+clarify+plan).
012-ui-visibility-write-clarity — IMPLEMENTED (#18). 011/010 — IMPLEMENTED. 008/007/006/005/004/003/002/001 — complete.

Plan: specs/015-filter-ux-redesign/plan.md. Artifacts: spec.md (US1 filter always visible per-pane P1, US2 filter+
footer always fit P1, US3 both scopes preserved P1, US4 live narrowing + match count P2, US5 refine/clear P2;
FR-001..016, SC-001..007, 3 clarifications), research.md (R1..R8 grounded file:line), data-model.md, quickstart.md,
contracts/ (filter-strip, applied-filter-chip-count, layout-budget, dual-scope-visibility), checklists/requirements.md
(16/16). Constitution v1.2.0 (VI UI Legibility + VII UI Consistency drive this; NO amendment). check-readonly STAYS
green (no S3 symbol). IV N/A (no storage-contract change). All changes in internal/ui; no new package/file/hue/keymap.

GOAL: make the filter ALWAYS visible + always fit (user: "не всегда влезает"). Research top-5 TUI (fzf/broot/k9s/
ranger-lf/yazi, adversarial-verified). DECISIONS (clarify): (1) filter input = ALWAYS-VISIBLE STRIP (fzf/broot), not
transient — reserved chrome, LIST absorbs the line, footer never sacrificed; (2) object match count = "N matched"
(no level total — paginated), bucket = "matched/total" (local); (3) count baked into the per-pane chip.

Key approach (research.md R1-R8, file:line verified): (R1) View()(app.go:1124) height budget 1138-1142 rows:=height-
footerH-2 → ADD filterStripH=1 in filterable modes (modeBuckets/modeTree only) → rows:=height-footerH-filterStripH-2;
render body+"\n"+filterStripView(w)+"\n"+footer (1209). windowBounds/treeView adapt → LIST shrinks. (R2) DELETE the
searching case from statusLine(app.go:1450-1459) — input now owned by the strip; statusLine keeps loading/notice/
error/op-prompt. (R3) NEW filterStripView: active=▌filter <pane>: <input>+caret+hints; idle=dim committed term or
"/ to filter <pane>"; one strip bound to focused scope (filterIsBucketList). (R4/R5) filterChipText(app.go:1309)
+(matched,total,hasTotal)→"filter: term · M/T"(bucket: filteredBuckets/buckets) | "term · N"(object: m.level.count(),
no total fetched — FR-013); TERM-GATED + zone-agnostic so BOTH chips show at once (listWithPane:1251 already chips
both boxes); a scope's chip hides only while THAT scope edits live. (R6) boxViewWith degrade order unchanged (center→
filter→mode, mode survives); chip drops WHOLE under width, strip still shows filter; filterChipTermMax(1304) budgets
the " · M/T" suffix, elide term first. (R8) TDD: extend assertWidthSweep(footer_test:92) + height-sweep (strip+footer
fit, LIST shrinks); add TestFilterStripAlwaysVisible + TestBothChipsVisibleTogether; migrate spec013_test
(TestBucketFilterChipCommitted/TestObjectsFilterChipCommitted) to always-visible + counts; migrate app_test
TestStatusSearchPending→filterStripView; keep search_test green (scopes independent).
<!-- SPECKIT END -->
