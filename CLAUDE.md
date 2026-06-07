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
Active feature: 008-connection-form-ux (connection + browser UX fixes; 9 user stories) —
IMPLEMENTED (40/40 tasks; go test + lint + vet + check-readonly green; ui coverage 77%).
007/006/005/004/003/002/001 — complete.

Plan: specs/008-connection-form-ux/plan.md. Artifacts (specs/008-connection-form-ux/):
research.md (R1 textField editor, R2 paste, R3 chord label, R4 delete-hint placement, R5 secret
guidance, R6 drop ALL block titles, R7 same-bucket post-mutation invalidation, R8 connections
relabel+collapse-order, R9 filter-reset, R10 no-dup-delete), data-model.md, quickstart.md, contracts/ (text-input,
connection-ui, command-bar, key-label, post-mutation-visibility), checklists/ (requirements 16/16,
ux 27). Constitution v1.0.0; no amendment. ALL changes in internal/ui (NO storage/config edits;
check-readonly stays green). Clarified: secret = hint-only, keychain stays sole save path
(env/cmd/awsProfile = config-file-only, form does NOT resolve ${ENV}); delete hint = inline in
connections view (NOT command-bar catalog); label format = "Ctrl+X" no-space (matches Ctrl+C);
US6 = SAME-BUCKET cross-prefix only (CopyKey/MoveObject single-bucket), precise src+dst key
invalidation (NOT cache.Clear); US7 = relabel "connections"; US9 = show-only-applicable delete.
Scope expanded (4 follow-up defects): US6 post-mutation visibility ALL actions incl same-bucket
cross-prefix copy/move/bulk_copy; US7 "connections" affordance; US8 filter-reset; US9 no dup delete.
/speckit-analyze remediation applied: same-bucket scope, INFO heading also removed (US5), FR-020
collapse-reorder, FR-014 defined surface (literal "w to arm"), FR-003 absent, bulk_copy covered.

Key approach (9 US): (1) NEW textfield.go — rune-aware single-line editor {Value,Caret(rune
idx)} Insert/Backspace/DeleteFwd/Left/Right/Home/End/Render(width,masked); shared by connForm 5
text fields AND op.input (typed-confirm). (2) PASTE: bracketed paste on by default in BT v2 →
tea.PasteMsg dropped today (Update only handles KeyPressMsg); add `case tea.PasteMsg` routing to
active text surface (search/command/connForm/op.input), strip trailing \n, interior \n→space.
(3) keys.go keyGlyph: "ctrl+x"→"Ctrl+X", "ctrl+o"→"Ctrl+O" (single source; all surfaces via
glyph()); trim redundant "(Ctrl chord required)" nudge tail in hintbar dispatchActionKey. (4)
connections.go connectionsView: inline delete hint (`Ctrl+X delete`) active for non-active conn,
absent on +add/empty, guard on active (FR-002 unchanged). (5) connFormView: per-field focus hint;
secret hint = "stored in OS keychain · env/cmd/awsProfile via config file". (6) commandbar.go:
drop ALL THREE blockTitle rows INFO(162)/READ(148)/WRITE(191), keep columns+gap grouping;
relocate "w to arm" literal text to write column lead row when !writable (FR-014).
confirmview.typedConfirmForm renders op.input via textField.Render (real caret, not tail-only).
US6: operation.go onOperationDone invalidates PRECISELY src+dst PREFIX keys SAME bucket for
copy/move/bulk_copy (NOT whole Clear) via new tree.go invalidateLevel(key)+parentPrefix; no
cross-bucket (single-bucket storage); same-level mutations already auto-show via refresh(). US7:
relabel "new conn"→"connections" at infoColumn:172 AND collapsedBarView:220 (bound to AddConn);
reorder ahead of droppable read entries on collapse (FR-020 — today appended last, fitEntries
drops trailing first). US8: readEntries appends `Esc clear` when searchActive()&&!searching (list
modes render command bar, NOT legacy footerHints which had the cue). US9: writeEntries shows ONLY
the selection-applicable delete (suppress inapplicable) — targeted exception to 007 all-write-
always-shown. Command-bar/title/^x test asserts live in hintbar_test.go (NOT commandbar_test.go;
T036 edits :166 INFO/READ/WRITE, T018 edits :199 ^x). Tests white-box package ui (deliver/press +
tea.PasteMsg helper); textfield_test unit; operation same-bucket cross-prefix visibility test
(storage.Fake); no integration (no storage contract change).
<!-- SPECKIT END -->
