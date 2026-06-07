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
Active feature: 012-ui-visibility-write-clarity (UI legibility, hotkey parity, breadcrumbs, write-mode
clarity; 9 user stories) — PLANNED (spec + clarify + plan done; tasks/impl pending). 011-two-pane-hotkeys
— IMPLEMENTED. 010-pinned-buckets — IMPLEMENTED. 008/007/006/005/004/003/002/001 — complete.

Plan: specs/012-ui-visibility-write-clarity/plan.md. Artifacts: spec.md (US1 names-never-hidden P1, US2
write-state legible+reversible P1, US3 breadcrumb P2, US4 prominent arm-confirm P2, US5 design-system P2,
US6 objects-zone hotkey parity P1 REGRESSION, US7 filter current level + prominent input P1, US8 sort
reachable+advertised P2, US9 declutter P2; FR-001..041, SC-001..015, 4 clarifications), research.md
(R1..R10), data-model.md, quickstart.md, contracts/ (keymap, reveal-popup, level-filter, layout-visibility,
writemode), checklists/requirements.md (16/16). Constitution v1.1.0 (ADDED VI UI Legibility + VII UI
Consistency/Design System for this feature). check-readonly STAYS green (no new write-S3 symbol, no storage
method; only a test-only read counter on Fake). NO integration (no storage-contract change; IV N/A justified).

GOAL: presentation/UX iteration on the two-pane browser. Make every resource identifier fully visible or
revealable; make write mode legible+reversible; add a location breadcrumb; FIX the confirmed regressions
where the objects zone (focusZone==zoneObjects) had dead hotkeys + wrong filter scope; surface sort; and
single-source every hint. Border-mounted RO/WRITE mode chip (like Claude Code's ultracode chip) + a
Claude-Code-style filter input that commits on Enter and hands focus to the filtered pane.

Key approach (grounded research.md R1-R10): (R3, regression) onObjectsKey (app.go:445-486) shipped with nav
only — factor a shared onLevelKey from onTreeKey (tree.go:41-103) so both delegate; make selKind (app.go:60)
+ actionCatalog (hintbar.go:44) treat (modeBuckets && zoneObjects) as a LEVEL context (object catalog +
real selObject/selFolder, not bucket catalog/selNone); add the 5 missing branches (Mark/Sort/SortDir/Context
+ dispatchChord/dispatchActionKey); FIX marks leak — clear m.sel in loadObjectsLevel. (R2/US7) `/` in objects
zone reuses LevelQuery.Search server-side current-prefix filter (s3client.go:107 effPrefix=prefix+search,
Delimiter:/) — afterFilterEdit/Esc/searchActive branch on focusZone; NEW prominent filter input commits on
Enter → moves focus to filtered pane, re-open pre-fills, Esc cancels-to-committed, back/clear removes. (R1/US1)
reveal.go NEW: 'i' opens centered popup (reuse confirmPopupView) showing full id + tea.SetClipboard OSC52
copy; bucket col auto-grows into objects-zone slack + active-row wrap (renderTable variant, within minRows
cap → footer safe; fall back to popup). (R5/US3) breadcrumb ctx→bucket→prefix in objects-zone center label /
Single box title, elideMiddle keeps bucket+deepest. (R6+R7/US2/US4) badge space-color fix (styles.go:411);
symmetric enable/disable labels from m.keys.WriteToggle; armConfirmPopupView (prominent, reuse
confirmPopupView, badge+chip stay); NEW mode chip = right-aligned slot in boxViewWith top border
(WRITE accent / RO neutral, NO_COLOR-safe, safety-redundant exception to dedup). (R8/US8) sort = first
read-block barEntry "s name↑" (sortIndicator), drops via fitEntries; sortModified already exists. (R7-glyphs/
US5/US9) +Reveal +Tab in keyMap; replace ~22 hardcoded hint literals (pane.go:71 ^x, app.go:1341/keys.go:152
d/x/y, confirm.go literal "esc", connections/operation/filebrowser Enter/Esc/↑↓) with glyph()/formatKeys();
confirm.go dispatch via m.keys.Back. Reuse-only styling (styles.go palette/roles, no new hue). Tests: white-
box package ui (deliver/press/viewOf, dualApp/crossToObjects/treeApp/buildApp) + Fake.ListLevelCalls; failing-
first per US (R10). Layout invariant: boxView minRows cap → footer never scrolls (FR-022) at every tier.
<!-- SPECKIT END -->
