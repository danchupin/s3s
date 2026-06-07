---
description: "Task list — 010 pinned buckets"
---

# Tasks: Pinned Buckets for Scoped Connections

**Input**: Design documents from `specs/010-pinned-buckets/`

**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: INCLUDED and TEST-FIRST — Constitution III (Test-First) is NON-NEGOTIABLE for this repo.
Write each test, confirm it FAILS (Red), then implement to pass (Green).

> **Go TDD compile-stub rule (remediation U1)**: a Go test referencing a not-yet-existing symbol
> fails to **compile** — that is not a meaningful "Red". For every test-first task below, introduce
> the **minimal stubs** the test names (struct field, interface method, `mode`/`fld` const,
> `fakeConnector` method) **together with the test**, with bodies that return zero values / are
> unimplemented, so the test COMPILES and fails on **behavior** (assertion), not on a missing
> symbol. The behavior is then filled by the implementation tasks in the same phase. Examples:
> T005 needs `App.pinnedBuckets` (stub the field with T005), T012 needs `fakeConnector.AddBucket` +
> `modeAddBucket` (stub with T012), T019 needs `fldBuckets` + `ConnDraft.Buckets` (stub with T019).

**Organization**: by user story (US1 P1, US2 P1, US3 P2, US4 P3). US1 is the MVP.

## Format: `[ID] [P?] [Story] Description with file path`

- **[P]** = parallelizable (different files, no incomplete deps)
- **[USx]** = user-story label (story phases only)

## Path Conventions

Single Go project at repo root: `internal/config/`, `internal/ui/`, `internal/storage/`, `cmd/s3s/`.
White-box UI tests live in `package ui` (`internal/ui/*_test.go`).

---

## Phase 1: Setup

**Purpose**: confirm clean baseline before any change (establish Red later against a green tree).

- [x] T001 Confirm branch `010-pinned-buckets` checked out and baseline gates pass unchanged: run `make test fmt vet lint check-readonly` and record they are green before edits.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: shared config schema + test infrastructure that multiple stories depend on.

**⚠️ CRITICAL**: complete before US1–US4.

- [x] T002 [P] Write FAILING config test for the pinned-bucket schema in `internal/config/config_test.go`: loading YAML with `buckets: [a, b]` on a cluster yields `Cluster.Buckets == ["a","b"]`; a config WITHOUT `buckets:` loads with `Buckets == nil` and re-`Marshal`s byte-identical (omitempty). (Contract: contracts/config-schema.md §round-trip.)
- [x] T003 Add `Buckets []string \`yaml:"buckets,omitempty"\`` to `config.Cluster` in `internal/config/config.go` (after `TLSSkipVerify`); no `Validate()` change. Make T002 pass.
- [x] T004 [P] Extend `storage.Fake` (test-only, read-only knobs — check-readonly stays green) in `internal/storage/fake.go`: add `FailListBuckets bool` (→ `ErrAccessDenied` from `ListBuckets`), `AccessDeniedBuckets map[string]bool` (→ `ErrAccessDenied` from `ListLevel` for that bucket), and `ListBucketsCalls`/`ListLevelCalls` counters; add `internal/storage/fake_test.go` asserting each knob/counter behaves. (Contract: contracts/pinned-bucket-list.md, conn-test-and-error.md need these.)

**Checkpoint**: config carries pins; Fake can simulate list-all-denied + count calls.

---

## Phase 3: User Story 1 - Browse and switch between named buckets (Priority: P1) 🎯 MVP

**Goal**: a connection with pinned buckets renders exactly those names and browses/switches them with
**zero** `ListBuckets` calls.

**Independent Test**: `pinnedBuckets=["alpha","beta"]` + Fake `FailListBuckets=true` → view shows both,
`Fake.ListBucketsCalls == 0`, Enter opens `alpha` (modeTree), switching to `beta` keeps calls at 0.

### Tests for User Story 1 (write first, must FAIL)

- [x] T005 [P] [US1] FAILING white-box UI test in `internal/ui/pinned_test.go`: build App with `pinnedBuckets=["alpha","beta"]` + Fake `FailListBuckets=true`; after initial load assert `viewOf(m)` contains `alpha`+`beta` and `Fake.ListBucketsCalls == 0`; `press(m,"enter")` → `m.mode==modeTree`, `m.bucket=="alpha"`; back + open `beta` → still 0 ListBuckets calls. Add a control case: no pins + Fake with 2 real buckets → list-all renders both, `ListBucketsCalls==1`. (Contract: contracts/pinned-bucket-list.md.)

### Implementation for User Story 1

- [x] T006 [US1] Add `PinnedBuckets []string` to `ui.Backend` in `internal/ui/app.go` (struct ~77-87).
- [x] T007 [US1] Add `pinnedBuckets []string` to `App`; seed it in `New()` from `initial.PinnedBuckets`; refresh it from `contextResolvedMsg.be.PinnedBuckets` on context switch — `internal/ui/app.go`.
- [x] T008 [US1] Populate `Backend.PinnedBuckets` from the resolved `Cluster.Buckets` in `backendFrom` — `cmd/s3s/main.go` (~105-128). (`cfg.Resolve(name)` already returns the cluster.)
- [x] T009 [US1] Change `loadBuckets` to accept the pinned list and, when non-empty, synthesize `[]storage.Bucket{{Name:n}}` (zero `CreationDate`) and return `bucketsMsg{gen,buckets}` WITHOUT calling `st.ListBuckets`; else unchanged. Update all call sites (`Init`, `beginLoad`-armed load, `refresh`, `applyContext`) to pass `m.pinnedBuckets` — `internal/ui/commands.go`, `internal/ui/app.go`.
- [x] T010 [US1] Render synthesized pinned buckets in `bucketsView` with a blank/`—` date column — `internal/ui/app.go` (~1095-1114). Make T005 pass.

**Checkpoint**: scoped connections are browsable; SC-005 (0 list-all) holds. MVP complete.

---

## Phase 4: User Story 2 - Reach different buckets from one connection, in the UI (Priority: P1)

**Goal**: a `+ add bucket` row (scoped lists only) lets the user add/open a bucket by name at runtime,
persisted to the connection.

**Independent Test**: on a scoped list, select `+ add bucket`, type a name, Enter → name persists
(fakeConnector.AddBucket called), appears in the list, opens; Esc cancels; the row is hidden on a
working list-all connection.

### Tests for User Story 2 (write first, must FAIL)

- [x] T011 [P] [US2] FAILING config test for `AppendBucket` in `internal/config/connection_test.go`: append a name → on-disk + live list grows; append duplicate → idempotent no-op (nil err); empty/whitespace → `ErrInvalid`, config untouched; unknown context → `ErrNotFound`; reload persists. (Contract: contracts/config-schema.md §AppendBucket.)
- [x] T012 [P] [US2] FAILING white-box UI test in `internal/ui/pinned_test.go`: `+ add bucket` row visible when scoped (pins set OR Fake `FailListBuckets=true`) and HIDDEN on list-all success with results; Enter on the row → `m.mode==modeAddBucket`; `typeStr(m,"gamma")`+Enter with `fakeConnector{...}` → `fakeConnector.addedBucket=="gamma"`, mode back to `modeBuckets`, `viewOf` contains `gamma`; Esc → list unchanged; adding an existing name → no duplicate row. (Contracts: contracts/add-bucket.md.)

### Implementation for User Story 2

- [x] T013 [US2] Add `AppendBucket(ctxName, bucket string) ([]string, error)` to `internal/config/connection.go`: trial-copy → normalize (trim/dedupe/order-stable, reject empty) → `Validate` → `Marshal`/`Save` → commit live; `slog.Info("connection.bucket-add", "context", …, "bucket", …, "outcome", "ok")`; return updated cluster bucket list. Make T011 pass.
- [x] T014 [US2] Add `AddBucket(ctx, ctxName, bucket string) ([]string, error)` to the `ui.Connector` interface in `internal/ui/connections.go`; add `addedBucket`/`buckets`/`addErr` fields + `AddBucket` to `fakeConnector` in `internal/ui/connections_test.go`.
- [x] T015 [US2] Implement `connSeam.AddBucket` via `cfg.AppendBucket` in `cmd/s3s/connection.go`.
- [x] T016 [US2] Add `addBucketMsg{gen int; buckets []string; err error}` to `internal/ui/messages.go`; add `addBucketCmd(c Connector, ctxName, bucket string, gen int) tea.Cmd` in `internal/ui/connections.go` (mirror `saveConnCmd`).
- [x] T017 [US2] Add `modeAddBucket` + `bucketAddForm{name textField; err string}`; `onAddBucketKey` (rune/backspace/delete/caret edit, Enter submit→`addBucketCmd`, Esc cancel, empty-name inline err); `onAddBucket(msg)` handler (drop stale gen; on success set `m.pinnedBuckets`+`m.info.PinnedBuckets`, clear form, `beginLoad`+`loadBuckets` to reflect; on err set `m.err`); dispatch `addBucketMsg` in `Update`; route `tea.PasteMsg` to the add field — `internal/ui/app.go`, `internal/ui/connections.go`.
- [x] T018 [US2] In `bucketsView` inject the `+ add bucket` row (blank date) when the list is scoped; track `m.bucketsScoped` (set in the `modeBuckets` `bucketsMsg`/`errMsg` handlers: scoped = pins present OR load errored OR zero buckets); in `onBucketsKey` detect Enter on the add-row index (`bucketSel == len(filteredBuckets())`) → open `modeAddBucket` — `internal/ui/app.go`. Make T012 pass.

**Checkpoint**: one connection reaches many buckets; additions persist (SC-006).

---

## Phase 5: User Story 3 - Add a scoped connection through the form (Priority: P2)

**Goal**: the add-connection form has a `buckets` field; entered names persist with the connection.

**Independent Test**: type names into the `buckets` field → saving maps a normalized list onto the
connection; the field is optional; the boolean toggles still work after the index shift.

### Tests for User Story 3 (write first, must FAIL)

- [x] T019 [P] [US3] FAILING white-box UI test in `internal/ui/connections_test.go`: navigate to `fldBuckets`, type `"a, b  c,,a"` → `draft().Buckets == ["a","b","c"]`; `space` at `fldBuckets` inserts a space (no toggle); `space` at `fldPathStyle`/`fldReadOnly` still toggles (index-shift regression); empty buckets → still validates/saves; save → `fakeConnector` draft has normalized `Buckets`; paste into the field inserts. (Contract: contracts/conn-form-buckets-field.md.)
- [x] T020 [P] [US3] FAILING config test in `internal/config/connection_test.go`: `AddConnection(NewConnection{…, Buckets:["a","b"]}, secret)` writes both into `Cluster.Buckets` on disk (no plaintext secret); reload → present.

### Implementation for User Story 3

- [x] T021 [US3] Add `Buckets []string` to `config.NewConnection`; map `nc.Buckets` into the trial `Cluster` (`cl.Buckets = nc.Buckets`) in `AddConnection` — `internal/config/connection.go`. Make T020 pass.
- [x] T022 [US3] Add `Buckets []string` to `ui.ConnDraft` (`internal/ui/connections.go`); map `d.Buckets` → `NewConnection.Buckets` in `connSeam.Save` — `cmd/s3s/connection.go`.
- [x] T023 [US3] Add `buckets textField` to `connForm`; add `fldBuckets` const after `fldSecret` (shift `fldPathStyle`→6/`fldReadOnly`→7/`connFieldCount`→8); insert `"buckets"` into `connFieldLabels`; add `case fldBuckets: return &f.buckets` to `focusField` — `internal/ui/connections.go`.
- [x] T024 [US3] Render the `buckets` row as a text field in `connFormView` (extend the `fields` slice to include `f.buckets`); add a `connFieldHint` case for `fldBuckets`; confirm the space-toggle switch does NOT add a `fldBuckets` case (text append) — `internal/ui/connections.go`.
- [x] T025 [US3] Add a shared `parseBuckets(string) []string` normalize helper (split on comma/space, trim, drop empty, dedupe order-stable); use it in `connForm.draft()` to set `ConnDraft.Buckets` — `internal/ui/connections.go`. Make T019 pass.

**Checkpoint**: scoped connections can be created entirely in-app with their bucket set.

---

## Phase 6: User Story 4 - Honest connection-test error + scoped probe (Priority: P3)

**Goal**: test failures show the real classified cause (not blanket "unreachable"); `AccessDenied`
counts as reachable; the probe targets a pinned bucket when present.

**Independent Test**: `fakeConnector` returning each sentinel → correct message / save behavior.

### Tests for User Story 4 (write first, must FAIL)

- [x] T026 [P] [US4] FAILING white-box UI test in `internal/ui/connections_test.go`: `fakeConnector{testErr: storage.ErrAccessDenied}` + submit → connection SAVES; `ErrUnreachable` → `m.form.err` contains "Backend unreachable" + "press Enter again to save anyway", second Enter saves; `ErrNotFound` → contains "Not found" (NOT "unreachable"); after cancel/save `m.err == nil` (no footer bleed). (Contract: contracts/conn-test-and-error.md.)

### Implementation for User Story 4

- [x] T027 [US4] Rewrite `onConnTested`: treat `msg.err == nil` OR `errors.Is(msg.err, storage.ErrAccessDenied)` as success→`saveConnCmd`; else set `m.err = msg.err` and `m.form.err = m.errorText() + " — press Enter again to save anyway"`; ensure `Update` sets `m.err` from `connTestedMsg` before the call; clear `m.err` on form `esc` and in `onConnSaved` — `internal/ui/connections.go`, `internal/ui/app.go`. Make T026 pass.
- [x] T028 [US4] Change `connSeam.Test`: when `d.Buckets` is non-empty, probe `st.ListLevel(ctx, storage.LevelQuery{Bucket: d.Buckets[0], MaxKeys: 1})`; else keep `st.ListBuckets`; return the classified error verbatim — `cmd/s3s/connection.go`. (Optional Fake-driven assertion: `ListLevelCalls≥1 && ListBucketsCalls==0` for a pinned draft.)

**Checkpoint**: add-connection no longer dead-ends or misleads scoped-credential users.

---

## Phase 7: Polish & Cross-Cutting

- [x] T029 [P] Document the `buckets` config key + pinned/scoped connections + the domain-style note in `README.md` (near the existing `pathStyle` example, ~line 165).
- [x] T030 [P] Add edge-case UI tests if coverage of `internal/ui` dropped below the prior baseline (~77%): pinned-list filter (`/`), refresh on a pinned list keeps `ListBucketsCalls==0`, scoped-empty (list-all returns 0) shows the add row — `internal/ui/pinned_test.go`.
- [x] T031 Run full gates green and the quickstart flow: `make test fmt vet lint check-readonly`, then walk `specs/010-pinned-buckets/quickstart.md` against a Fake-backed run. **(G1)** Include the FR-011 domain-style check: with a real path-style-off connection (e.g. endpoint `https://bucket.avito-sd`) + a pinned real bucket, confirm opening the pinned bucket succeeds (the SDK addresses `<bucket>.<host>`) and `~/.local/state/s3s/s3s.log` shows `list level` for the bucket and NO `list buckets` against the unlistable apex. If no live backend is available, record FR-011 as verified-by-existing-addressing (no code path added by 010 changes endpoint construction).
- [x] T032 [P] Regression guard: assert a real-world config WITHOUT `buckets:` round-trips byte-identical and behaves exactly as before (no `+ add bucket` row, list-all path) — `internal/config/config_test.go` / `internal/ui/pinned_test.go`.
- [x] T033 [P] **(C1, OPTIONAL — Constitution IV credential-flow)** Add an integration test (`//go:build integration`) near `internal/storage/cred_auth_integration_test.go` against MinIO: provision a user that is DENIED `s3:ListAllMyBuckets` but ALLOWED on a named bucket; assert `ListBuckets` returns `ErrAccessDenied` while `ListLevel(MaxKeys:1)` on that bucket succeeds — the exact scoped-creds shape the pinned-buckets probe relies on. Skips automatically without a Docker provider (existing testcontainers pattern). Not required (no storage-contract change), but hardens the credential-flow path.

---

## Dependencies & Execution Order

### Phase dependencies
- **Setup (P1)** → no deps.
- **Foundational (P2)** → after Setup; BLOCKS all stories (T003 config field + T004 Fake knobs feed every story's tests).
- **US1 (P3 phase)** → after Foundational. MVP.
- **US2 (P4 phase)** → after US1 (reuses pinned render + bucketsView + loadBuckets reload) and Foundational.
- **US3 (P5 phase)** → after Foundational (config field). Independent of US1/US2 at runtime.
- **US4 (P6 phase)** → T027 (honest error) independent; T028 (probe switch) depends on `ConnDraft.Buckets` from US3 (T022) — order US3 before US4 (matches P2<P3).
- **Polish (P7)** → after the stories you intend to ship.

### Within each story
- Test task(s) FIRST and FAILING, then implementation (Constitution III).
- Per the Go compile-stub rule (U1): the test task lands the minimal symbol stubs it references so it
  compiles and fails on behavior; implementation tasks then supply the real bodies.
- config field/method before the seam; seam before UI wiring; model/data before render.

### Parallel opportunities
- T002 ∥ T004 (different files).
- Test-authoring tasks T005, T011/T012, T019/T020, T026 are each [P] within their phase.
- US3 (T019–T025) and US4-T027 touch mostly distinct symbols from US1/US2 and could be staffed in parallel after Foundational, but US2 and US3 both edit `internal/ui/connections.go` — serialize edits to that file.

### File-contention note
`internal/ui/connections.go` is touched by US2 (T014/T016/T017), US3 (T022–T025), and US4 (T027/T028 cmd side). `internal/ui/app.go` by US1/US2/US4. Do NOT run [P] tasks that edit the same file concurrently — the [P] marks cross-file independence only.

---

## Parallel Example: Foundational

```bash
# T002 and T004 edit different packages — safe to author together:
Task: "FAILING config round-trip test for Cluster.Buckets (internal/config/config_test.go)"
Task: "storage.Fake test knobs + counters + fake_test.go (internal/storage/fake.go)"
```

## Implementation Strategy

### MVP first (US1 only)
1. Setup (T001) → Foundational (T002–T004) → US1 (T005–T010).
2. STOP & VALIDATE: scoped connection browses its pinned buckets with 0 list-all calls.
3. Ship/demo — already solves the reported dead-end (manual config `buckets:` list).

### Incremental delivery
- + US2 → add buckets in-app, persisted (the "one connection, many buckets" ask).
- + US3 → create scoped connections fully in the form.
- + US4 → honest test errors + scoped probe (removes the "unreachable" lie).
- + Polish → README, coverage, gates, regression.

## Notes
- check-readonly MUST stay green: the only `internal/storage` edit is the read-only `Fake` knob (T004); no new write-S3 symbols anywhere.
- No integration tests: the `storage.Storage` contract is unchanged (probe reuses `ListLevel`).
- Commit after each green task or logical group (commit only when the user asks).
