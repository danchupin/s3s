---
description: "Task list for 005-storage-ops-analytics"
---

# Tasks: Storage Operations & Analytics

**Input**: Design documents from `specs/005-storage-ops-analytics/`

**Prerequisites**: plan.md, spec.md, research.md (R1–R13), data-model.md, contracts/ (×4)

**Tests**: INCLUDED — TDD is non-negotiable for this project (Constitution III). Per the project's
white-box convention, UI tests are `package ui` driving the model with `deliver`/`press` helpers
and asserting on `App.View().Content`; storage uses the in-memory `Fake`; integration tests carry
`//go:build integration` and run against MinIO.

**Organization**: by user story. Phase order follows the plan's 3 slices (backbone → reads →
scale): US5 + US6 (P1 safety/credential backbone) → US1 (P1) + US2 (P2) reads → US3 (P3) + US4 (P4).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[Story]**: US1–US6 for story-phase tasks (none for Setup/Foundational/Polish)

## Path Conventions

Single Go project. Source under `cmd/s3s/`, `internal/{storage,secret,config,ui,logging,localfs}`;
tests live beside the code (`*_test.go`, `*_integration_test.go`). Spec docs under
`specs/005-storage-ops-analytics/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: dependencies and file scaffolding so later phases compile incrementally.

- [X] T001 Add dependencies `github.com/zalando/go-keyring` and `golang.org/x/term` via `go get`; verify `go.mod`/`go.sum` updated and `make build` still succeeds.
- [X] T002 [P] Create `internal/secret/` package with `doc.go` (package comment: UI- and SDK-agnostic credential-source resolution per research R8–R13).
- [X] T003 [P] Create empty UI files with package decl + file-level comment: `internal/ui/writemode.go`, `internal/ui/download.go`, `internal/ui/analyze.go`, `internal/ui/selection.go`, `internal/ui/bulk.go`, `internal/ui/sort.go`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: storage interface additions + the dynamic-guard model refactor that multiple stories
build on. **No story phase may start until this is complete.**

**⚠️ CRITICAL**: T004–T009 block US1/US2/US3/US5.

- [X] T004 Add read methods `GetObject(ctx, bucket, key) (io.ReadCloser, error)` and `UsageOf(ctx, bucket, prefix, onProgress) (UsageReport, error)` to the `Storage` interface, plus types `UsageReport`/`UsageChild`/`UsageProgress`, in `internal/storage/storage.go` (per data-model.md + storage-read-ops-contract C1/C2).
- [X] T005 [P] Implement `GetObject` + `UsageOf` in `internal/storage/fake.go` (deterministic in-memory: full bytes for GetObject; recursive aggregate + immediate-child bucketing + ranking for UsageOf) so unit tests can drive both.
- [X] T006 [P] Guard parity test in `internal/storage/guard_test.go`: assert `readOnlyGuard` exposes `GetObject`/`UsageOf` (reads pass through, no `ErrReadOnly`) — storage-read-ops-contract C3.
- [X] T007 Refactor `ui.Backend`/`ui.Resolver` to carry the **raw** (unguarded) store + `ReadOnly bool`; update `cmd/s3s/main.go` to stop calling `storage.Guard` at construction and pass raw store + ReadOnly + initial armed state into `ui.New` (write-toggle-contract C1).
- [X] T008 In `internal/ui/app.go`: replace `writable bool` with `raw storage.Storage`, `ctxReadOnly bool`, `armed bool`; add `writable()` derived (`armed && !ctxReadOnly`) and `activeStore() storage.Storage` (`storage.Guard(raw, writable())`); re-derive on `applyContext` (write-toggle-contract C4).
- [X] T009 Route every existing mutating start (`startRemoveObject`/`startCopy`/`startMove`/`startUpload`/`startCreateFolder`/`startRecursiveDelete`) and `dispatchOp` through `activeStore()` instead of `m.store`; update the read paths (loadBuckets/level/metadata/preview) to use `activeStore()` (a read, harmless) or `raw`. Confirm `make build` + existing tests green.

**Checkpoint**: storage reads available; dynamic guard in place; existing behavior unchanged.

---

## Phase 3: User Story 5 - Runtime read-only↔write toggle + loud signalling (Priority: P1)

**Goal**: arm/disarm write at runtime on a hotkey, with a high-contrast always-on WRITE indicator;
`readonly:true` never armable; transitions logged.

**Independent Test**: launch RO → mutating actions absent; toggle → confirm → loud WRITE badge +
actions appear; toggle → instant RO; `readonly:true` context refuses to arm.

### Tests for User Story 5 (write first, must FAIL)

- [X] T010 [P] [US5] Test in `internal/ui/writemode_test.go`: RO + WriteToggle → confirm prompt; confirm → `writable()` true; second toggle → instant RO (no confirm) — write-toggle-contract C2.
- [X] T011 [P] [US5] Test in `internal/ui/writemode_test.go`: `ctxReadOnly` context + WriteToggle → refused, stays RO with reason (FR-028); `--write` initial → starts armed (FR-031).
- [X] T012 [P] [US5] Test in `internal/ui/writemode_test.go`: armed → `applyContext` to a `readonly:true` context forces RO; to a writable context preserves armed (FR-029).
- [X] T013 [P] [US5] View test in `internal/ui/writemode_test.go`: `App.View().Content` contains the loud WRITE marker on each mode + the action-menu/help/object overlays while armed, and the calm RO marker while disarmed, including a narrow-width render (FR-027, write-toggle-contract C3).
- [X] T014 [P] [US5] Log test in `internal/ui/writemode_test.go`: each RO↔write transition emits a slog record with new state + context (FR-032, contract C5).

### Implementation for User Story 5

- [X] T015 [US5] Add `WriteToggle` binding to `keyMap`/`defaultKeys()` in `internal/ui/keys.go` (pick a free key, e.g. `w`); add it to the help surface (`helpLines`).
- [X] T016 [US5] Implement arm/disarm flow in `internal/ui/writemode.go`: WriteToggle → if `ctxReadOnly` refuse with notice; else if disarmed open a simple confirm then set `armed=true`; if armed set `armed=false` instantly; log the transition (FR-025/026/028/032).
- [X] T017 [US5] Add `writeBadgeStyle` (high-contrast inverse/red, bold) in `internal/ui/styles.go`; render the WRITE/RO badge in `footerIdentityCompact` and inject it into the alt-screen overlay renderers (`actionMenuView`, `helpView`, `objectView`) so it shows on every screen and is never dropped first when narrow (FR-027).
- [X] T018 [US5] In `cmd/s3s/main.go`, set the initial `armed` from the `--write` flag and pass it into `ui.New`; keep `--write` help text updated to "start in write mode (toggle at runtime with the write key)".

**Checkpoint**: US5 fully testable; mutating actions are gated by the runtime toggle.

---

## Phase 4: User Story 6 - Secure credential sources (Priority: P1)

**Goal**: a context resolves its secret from exactly one of keychain / command / AWS profile /
`${ENV}`, with a secure prompt fallback; secrets never on disk in plaintext or required in env.

**Independent Test**: store a secret in the keystore via `s3s cred set`; in a fresh terminal with
nothing exported, `s3s --context <ctx>` connects with no prompt; no plaintext secret on disk; the
secret never appears in logs.

### Tests for User Story 6 (write first, must FAIL)

- [X] T019 [P] [US6] Config validation tests in `internal/config/config_test.go`: exactly-one-source passes; two sources → one-source error (FR-041); `${ENV}` inline still validates + resolves (FR-042); anonymous unaffected (credential-source-contract C1).
- [X] T020 [P] [US6] Config perms test in `internal/config/config_test.go`: a group/world-readable config triggers the insecure-permissions warning (FR-040, R13).
- [X] T021 [P] [US6] Keychain resolver test in `internal/secret/keychain_test.go` against a fake keystore: store/fetch/remove round-trip; absent keystore → clear error (FR-043, contract C2).
- [X] T022 [P] [US6] Command resolver test in `internal/secret/command_test.go`: argv command stdout (newline-trimmed) becomes the secret; 0600 owner config runs it; 0666/group-writable config → refused with the insecure-perms reason (FR-036, contract C3).
- [X] T023 [P] [US6] AWS-profile resolver test in `internal/secret/awsprofile_test.go` against fixture credential files: static keys (+ optional token) parsed; missing profile / static-less profile → clear error (R11).
- [X] T024 [P] [US6] Redaction test in `internal/secret/source_test.go`: a resolved secret stays `logging.Secret`-redacted in String()/log output for every source (FR-039, SC-014).
- [X] T025 [P] [US6] `cred` subcommand test in `cmd/s3s/cred_test.go`: `set`/`rotate`/`rm` write/remove in the (fake) keystore only — never the config file (FR-037, contract C4).

### Implementation for User Story 6

- [X] T026 [P] [US6] Extend `config.User` in `internal/config/config.go` with `keychain bool`, `cmd string`, `awsProfile string`; add `Validate` rule for exactly-one credential source (FR-041); keep `${ENV}` resolution (FR-042).
- [X] T027 [US6] Add the config-file permissions check + warning in `internal/config/config.go` `Load` (FR-040, R13).
- [X] T028 [P] [US6] Implement `Source`/`SourceKind`/`ResolvedCredential` + `ResolveSecret(ctx, src, accessKeyID)` dispatch in `internal/secret/source.go` (credential-source-contract C2).
- [X] T029 [P] [US6] Implement keychain store/fetch/remove (`zalando/go-keyring`, service "s3s", account=context) in `internal/secret/keychain.go` (R9).
- [X] T030 [P] [US6] Implement the command resolver in `internal/secret/command.go`: owner-only perms gate, POSIX shell-words argv split (`google/shlex`), `exec.Command` (not `sh -c`), ctx timeout (R10, FR-036).
- [X] T031 [P] [US6] Implement the AWS shared-credentials INI parser (honor `AWS_SHARED_CREDENTIALS_FILE`) in `internal/secret/awsprofile.go` (R11).
- [X] T032 [P] [US6] Implement the no-echo prompt (`x/term`) + optional save-to-keystore in `internal/secret/prompt.go` (R12, FR-038).
- [X] T033 [US6] Rewire `internal/config/resolve.go` `ClientConfig` to resolve the single source via `internal/secret` (non-interactive) and surface a clear error when unavailable (FR-043); keep `storage.ClientConfig` shape unchanged.
- [X] T034 [US6] In `cmd/s3s/main.go`: resolve the active context's secret at startup (prompt allowed pre-TUI per R12); on in-TUI context switch resolve non-interactively and show a "relaunch to enter this context's secret" notice when a prompt would be needed.
- [X] T035 [US6] Add the `s3s cred set|rotate|rm <context>` subcommand in `cmd/s3s/cred.go` (keystore-only writes; FR-037).
- [X] T036 [US6] Extend the `config init` wizard in `internal/config/generate.go` to ask the credential source and, for keychain, store the secret in the keystore instead of printing an `export` line (FR-041a).
- [X] T036a [US6] Integration test in `internal/secret/auth_integration_test.go` (`//go:build integration`): a context whose secret is resolved from a keychain (fake-backed) / command / awsProfile source builds a `ClientConfig` that successfully authenticates against MinIO (lists a bucket) — closes Constitution IV's credential/auth-flow focus for non-env sources.

**Checkpoint**: US6 fully testable; no plaintext secret on disk, none required in env.

---

## Phase 5: User Story 1 - Download an object to local disk (Priority: P1)

**Goal**: stream a full object to a local file with progress + cancel; works read-only.

**Independent Test**: RO context, select object, download → byte-identical local file with progress;
existing local file prompts before overwrite; cancel leaves no partial.

### Tests for User Story 1 (write first, must FAIL)

- [X] T037 [P] [US1] Unit test in `internal/storage/fake_test.go` (or `storage_test.go`): `GetObject` returns seeded bytes byte-for-byte; missing key → `ErrNotFound` (storage-read-ops-contract C1).
- [X] T038 [P] [US1] Integration test in `internal/storage/s3client_integration_test.go` (`//go:build integration`): download a large (multipart-sized) object from MinIO and verify full length + content; cancellation mid-stream surfaces `ctx.Err()`.
- [X] T039 [P] [US1] UI test in `internal/ui/download_test.go`: download writes a byte-identical file to the default dir; runs in a RO context (no `--write`) — FR-002.
- [X] T040 [P] [US1] UI test in `internal/ui/download_test.go`: an existing local file triggers a simple overwrite confirm before writing (FR-005); cancel mid-download leaves no `.partial` file (FR-004); failure leaves no complete-looking file (FR-006).

### Implementation for User Story 1

- [X] T041 [US1] Implement `GetObject` (full-object stream, no range) in `internal/storage/s3client.go` with sentinel classification (storage-read-ops-contract C1).
- [X] T042 [US1] Implement the download operation in `internal/ui/download.go`: stream `GetObject` → `dest+".partial"` → atomic rename on success; remove partial on cancel/failure; progress via the existing `progressEvent`/`opCh`; HeadObject for the progress total (data-model download transfer).
- [X] T043 [US1] Add a non-`Mutator` (read) dispatch path for the `download` op kind in `internal/ui/operation.go` (download must not require write).
- [X] T044 [US1] Wire the overwrite confirm (simple tier) + the default destination (resolve `S3S_DOWNLOAD_DIR` env > `downloadDir` config key > cwd) with per-download override via the existing file browser (`phaseBrowse`) in `internal/ui/download.go` (FR-005/007); add the `downloadDir` field to `internal/config/config.go`.
- [X] T045 [US1] Add the `download` item to the action menu in `internal/ui/actionmenu.go` for an object selection (read — available in RO). Menu-only — no dedicated top-level key (FR-023, keep the footer uncluttered) (action-menu-selection-contract C4).

**Checkpoint**: US1 fully functional; download works against production read-only.

---

## Phase 6: User Story 2 - `du` storage analytics (Priority: P2)

**Goal**: recursive size/count + ranked immediate-child breakdown with live progress, cancel, and
drill-down; works read-only.

**Independent Test**: seed a known size distribution, analyze → totals match, children ranked
largest-first; cancel a long scan shows partial totals; empty prefix shows zero.

### Tests for User Story 2 (write first, must FAIL)

- [X] T046 [P] [US2] Unit test in `internal/storage/fake_test.go`: `UsageOf` totals + child ranking (size desc, name tiebreak) + share for a known distribution; empty prefix → zero report, no error (storage-read-ops-contract C2, FR-012).
- [X] T047 [P] [US2] Integration test in `internal/storage/s3client_integration_test.go` (`//go:build integration`): `UsageOf` across a >1000-key pagination boundary and a deep nested prefix; cancellation returns `Complete=false` with partial counts.
- [X] T048 [P] [US2] UI test in `internal/ui/analyze_test.go`: `modeUsage` renders totals + ranked children with size + %; Enter on a child sub-prefix drills down (re-analyze); empty prefix shows zero (FR-008/009/012/013, action-menu-selection-contract C5).

### Implementation for User Story 2

- [X] T049 [US2] Implement `UsageOf` in `internal/storage/s3client.go`: paginated delimiter-less list under prefix, immediate-child bucketing, ranking, periodic `onProgress`, cancellable partial result (storage-read-ops-contract C2). Share the paginating helper with the existing recursive-delete enumerator where practical.
- [X] T050 [US2] Add `modeUsage` + the analyze view (ranked children, size bars, totals, human-readable units) in `internal/ui/analyze.go`; render live progress + Esc-to-cancel during the scan (FR-011/012).
- [X] T051 [US2] Implement analyze dispatch + drill-down (Enter on a child re-runs `UsageOf` under it; back returns) in `internal/ui/analyze.go`, under the existing `gen`/`loadCtx`/progress machinery (FR-013, non-blocking).
- [X] T052 [US2] Add the `analyze` item to the action menu in `internal/ui/actionmenu.go` for a bucket (bucket list) or folder/level selection (read — available in RO). Menu-only — no dedicated top-level key (FR-023).

**Checkpoint**: US2 fully functional; "what's eating space" answerable in-TUI.

---

## Phase 7: User Story 3 - Multi-select & bulk operations (Priority: P3)

**Goal**: mark multiple objects; bulk download (read, hierarchy-preserving) / delete / copy
(write-gated) with truthful per-item results.

**Independent Test**: mark several objects (count + combined size shown), bulk download → mirrored
subdirs + per-item summary; navigating away clears the selection.

**Depends on**: US1 (download op), US5 (armed state for bulk delete/copy), Foundational guard.

### Tests for User Story 3 (write first, must FAIL)

- [X] T053 [P] [US3] Selection test in `internal/ui/selection_test.go`: Mark toggles an object into `sel` and updates count/combined size; marking a folder is a no-op; navigating away clears `sel` (FR-014/019, action-menu-selection-contract C2).
- [X] T054 [P] [US3] Bulk download test in `internal/ui/bulk_test.go`: N marked keys → N files in mirrored local subdirs + truthful per-item summary; one failing item does not abort the rest (FR-015a/018, contract C3); runs in RO.
- [X] T055 [P] [US3] Bulk mutate test in `internal/ui/bulk_test.go`: bulk delete requires armed write + typed count confirm + logs each op (FR-017); bulk delete/copy refused while RO (FR-016); recursive delete is not reachable via multi-select.

### Implementation for User Story 3

- [X] T056 [US3] Implement per-level selection in `internal/ui/selection.go`: `sel map[string]bool` (objects only), derived count + combined size; clear on enter-level / back / bucket-entry / context-switch (FR-019); render a marker glyph + the `<n> selected · <size>` header.
- [X] T057 [US3] Add the `Mark` (space) binding in `internal/ui/keys.go` + help; wire it in `onTreeKey` to toggle the current object row.
- [X] T058 [US3] Implement bulk execution in `internal/ui/bulk.go`: iterate marked keys applying the per-item backend call (download via `GetObject`→local mirrored path; delete via `RemoveObject`; copy via `CopyKey`), report per-item `progressEvent`, continue past failures, end with `succeeded/failed` summary (FR-015/018, contract C3).
- [X] T059 [US3] Add bulk menu items (download always; delete/copy only when `writable()` and a selection exists) in `internal/ui/actionmenu.go`; bulk delete uses the typed-count confirm and logs each op before execution (FR-016/017).

**Checkpoint**: US3 fully functional; batch operations with truthful results.

---

## Phase 8: User Story 4 - Sortable lists (Priority: P4)

**Goal**: sort the current list by name/size/modified, toggle direction; session-persistent.

**Independent Test**: size-desc puts the largest object first; toggling reverses; the sort persists
into a newly entered level; sort composes with active search.

### Tests for User Story 4 (write first, must FAIL)

- [X] T060 [P] [US4] Sort test in `internal/ui/sort_test.go`: size-desc orders largest object first; direction toggle reverses; dirs ordered consistently when sorting by size/modified (FR-020/021, action-menu-selection-contract C6).
- [X] T061 [P] [US4] Persistence test in `internal/ui/sort_test.go`: the chosen sort persists across navigation into a newly entered level; sort + an active search/filter both apply (FR-020).

### Implementation for User Story 4

- [X] T062 [US4] Implement render-time sort in `internal/ui/sort.go`: `sortBy`/`sortAsc` session fields; sort a copy of `level.dirs`+`level.objects` at render time; dirs grouped consistently for size/modified (FR-021).
- [X] T063 [US4] Add `Sort` (cycle column) + direction-toggle bindings in `internal/ui/keys.go` + help; show the active column + direction in the box header/footer; apply in `treeView`/`bucketsView` render (FR-020).

**Checkpoint**: all six user stories independently functional.

---

## Phase 9: Polish & Cross-Cutting Concerns

- [X] T064 [P] Update `README.md`: download / `du` analytics / multi-select / sort / runtime write toggle / credential sources (keychain/cmd/awsProfile/env/prompt) + `s3s cred` subcommand.
- [X] T065 [P] Update `ROADMAP.md`: move delivered items out of the backlog; note remaining out-of-scope (presigned URLs, bucket admin, versioning, multipart cleanup, clipboard, tags).
- [X] T066 [P] Refresh the help overlay (`internal/ui/keys.go` `helpLines`) so the new primitive keys (Mark, Sort, WriteToggle), the menu-only operations (download/analyze/bulk), and the credential note are categorized correctly (FR-031).
- [X] T067 Run `make check-readonly` and confirm it stays green (no write SDK symbols outside `internal/storage`; UI SDK-free).
- [X] T068 Run `make fmt vet lint` and `make test`; fix any failures.
- [X] T069 Run `make test-integration` (Lima `DOCKER_HOST` + `TESTCONTAINERS_RYUK_DISABLED=true`) to exercise `GetObject`/`UsageOf` against MinIO.
- [X] T070 Execute `specs/005-storage-ops-analytics/quickstart.md` end-to-end and confirm SC-001…SC-015.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: depends on Setup; **blocks all stories** (storage methods + dynamic guard).
- **US5 (Phase 3)**: after Foundational. Backbone — provides the armed state US3 mutations need.
- **US6 (Phase 4)**: after Foundational. Independent of US5; both are the P1 backbone slice.
- **US1 (Phase 5)**: after Foundational (needs `GetObject`).
- **US2 (Phase 6)**: after Foundational (needs `UsageOf`).
- **US3 (Phase 7)**: after Foundational + US1 (download op) + US5 (armed state for delete/copy).
- **US4 (Phase 8)**: after Foundational.
- **Polish (Phase 9)**: after all desired stories.

### Slice checkpoints (recommended delivery)

1. **Slice 1 (backbone)**: Setup + Foundational + US5 + US6 → safe runtime toggle + secret hygiene.
2. **Slice 2 (reads)**: US1 + US2 → download + `du`, production-safe.
3. **Slice 3 (scale)**: US3 + US4 → bulk + sort.

### Parallel Opportunities

- Setup: T002, T003 in parallel after T001.
- Foundational: T005, T006 in parallel after T004; T007–T009 sequential (shared files: app.go/main.go).
- Each story's test tasks ([P]) run in parallel and must FAIL before its implementation.
- US6 resolvers T028–T032 are largely independent files → parallel after T026.
- US5 and US6 can be staffed in parallel (disjoint files) once Foundational is done.
- US1 and US2 can proceed in parallel (download.go vs analyze.go; both need Foundational).

---

## Parallel Example: User Story 6 (after T026/T027)

```bash
# Resolvers — independent files, run in parallel:
Task: "Implement keychain store/fetch/remove in internal/secret/keychain.go"
Task: "Implement command resolver + perms gate in internal/secret/command.go"
Task: "Implement AWS shared-credentials INI parser in internal/secret/awsprofile.go"
Task: "Implement no-echo prompt + save-to-keystore in internal/secret/prompt.go"
```

---

## Implementation Strategy

### MVP (Slice 1 — backbone)

1. Phase 1 Setup → 2. Phase 2 Foundational → 3. Phase 3 US5 → 4. Phase 4 US6.
5. **STOP & VALIDATE**: safe-by-default launch, runtime toggle with loud indicator, secret resolved
   from a secure source with no env export. Deploy/demo.

### Incremental Delivery

- Slice 1 (US5+US6) → Slice 2 (US1 download, US2 `du`) → Slice 3 (US3 bulk, US4 sort).
- Each slice is independently valuable and testable; ship between slices.

---

## Notes

- TDD: every implementation task is preceded by a failing test in the same phase (Constitution III).
- `[P]` = different files, no incomplete-task dependency.
- `check-readonly.sh` MUST stay green — download/`du` are reads; UI never imports the SDK.
- Secrets stay `logging.Secret`-redacted everywhere; never written to an s3s file; never required in env.
- Commit after each task or logical group; stop at any slice checkpoint to validate.
