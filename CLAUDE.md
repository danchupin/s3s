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
Active feature: 010-pinned-buckets (scoped connections; 4 user stories) — IMPLEMENTED (33/33 tasks;
go test + fmt + vet + lint(0) + check-readonly green; ui coverage 77.5%, config 79.1%).
008/007/006/005/004/003/002/001 — complete.

Plan: specs/010-pinned-buckets/plan.md. Artifacts (specs/010-pinned-buckets/): spec.md (US1 browse
pinned w/o ListBuckets P1, US2 in-UI +add bucket per-connection P1, US3 form buckets field P2, US4
honest test error P3; FR-001..016, SC-001..006), research.md (R1..R9), data-model.md, quickstart.md,
contracts/ (config-schema, pinned-bucket-list, add-bucket, conn-form-buckets-field, conn-test-and-error),
checklists/requirements.md 16/16. Constitution v1.0.0; no amendment. Changes span internal/config +
internal/ui + cmd/s3s (NOT just ui); check-readonly STAYS green (no new write-S3 symbols; only a
test-only read knob on storage.Fake).

PROBLEM: bucket-scoped creds (no s3:ListAllMyBuckets) → ListBuckets 403 dead-ends s3s 3 ways (test
probe, mislabeled "unreachable", browser can't reach a bucket). Verified Avito case: domain-style
`<bucket>.bucket.avito-sd` resolves per provisioned bucket, apex `bucket.avito-sd` is unlistable.

Clarified (2026-06-07): add-bucket UX = "+ add bucket" row (mirror "+ add connection"); affordance
SCOPED-ONLY — shown iff pins exist OR list-all failed/denied/empty, HIDDEN when list-all succeeds
(no footgun hiding buckets on a working connection).

Key approach (canonical names): config.Cluster.Buckets []string `yaml:"buckets,omitempty"` (R1 — on
Cluster not Context); NewConnection.Buckets + AddConnection maps it; NEW config.AppendBucket(ctx,
bucket) trial-validate-persist + logs connection.bucket-add (R8). ui.Backend.PinnedBuckets +
App.pinnedBuckets seeded in New()/contextResolvedMsg. loadBuckets(ctx,st,gen,pinned): pins non-empty
→ synthesize []storage.Bucket{{Name}} (zero date), NO ListBuckets call (R2). bucketsView injects
"+ add bucket" row at render when scoped (R3/R4); onBucketsKey Enter on it → modeAddBucket +
bucketAddForm{name textField,err}; submit → Connector.AddBucket → addBucketCmd/addBucketMsg{gen,
buckets,err}/onAddBucket (mirror saveConnCmd/connSavedMsg/onConnSaved) → update pins + reload.
connForm: NEW buckets textField at fldBuckets=5 (shifts fldPathStyle→6/fldReadOnly→7/count→8); label
"buckets"; focusField returns &f.buckets; draft() parseBuckets (split comma/space,trim,drop,dedupe
order-stable); validateForm optional. connSeam.Test: d.Buckets non-empty → ListLevel(d.Buckets[0],
MaxKeys:1) else ListBuckets (R6). onConnTested: nil OR ErrAccessDenied → save (R7/FR-009); else
m.err=msg.err + m.form.err = errorText()+" — press Enter again to save anyway" (drop hardcoded
"unreachable"); clear m.err on esc/save. storage.Fake test knobs: FailListBuckets bool +
AccessDeniedBuckets (R9, read-only, check-readonly green). Tests white-box package ui (deliver/press/
viewOf + fakeConnector; assert 0 ListBuckets calls via Fake counter); config temp-file round-trip +
AppendBucket; textfield_test for buckets parse; NO integration (no storage contract change).
<!-- SPECKIT END -->
