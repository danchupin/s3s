---
description: "Task list for 002-write-foundation"
---

# Tasks: Write Foundation & Safety

**Input**: Design documents from `/specs/002-write-foundation/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: INCLUDED — TDD is non-negotiable (Constitution III). Every behavior task
is preceded by a failing test.

**Organization**: Grouped by user story. US1 + US2 (both P1) are the MVP; US3 (P2)
ships the typed-confirm tier for future destructive features.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[Story]**: US1 / US2 / US3 (story-phase tasks only)
- File paths are repo-relative.

## Path Conventions

Single Go module: `cmd/s3s/`, `internal/{storage,config,ui,logging}/`. Per plan.md.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Clean baseline before changes.

- [ ] T001 Confirm branch `002-write-foundation` and a green baseline: run `make build test check-readonly` and record they pass before edits.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Type/scaffold additions every user story depends on. No behavior yet.

**⚠️ CRITICAL**: No user story work begins until this phase is complete.

- [ ] T002 [P] Add `Mutator` interface (`CreateFolder(ctx, bucket, prefix) error`) and the `ErrReadOnly` sentinel to `internal/storage/storage.go`.
- [ ] T003 [P] Add optional `ReadOnly bool` field (yaml `readonly`, default false) to the context struct plus load-time validation in `internal/config/config.go`.
- [ ] T004 [P] Extend `internal/storage/fake.go` so `Fake` implements `Mutator.CreateFolder` (mutates its in-memory map) for unit tests (depends on T002).

**Checkpoint**: interfaces + config field + fake compile; behavior added per story.

---

## Phase 3: User Story 1 - Writes are off unless explicitly enabled (Priority: P1) 🎯 MVP

**Goal**: Mutations are impossible unless `--write` is passed, and a context marked
`readonly: true` refuses mutations even then (read-only always wins).

**Independent Test**: Two contexts (one `readonly: true`). Without `--write`, any
mutation is refused; with `--write`, the writable context permits it and the
read-only one still refuses — all without mutating storage.

### Tests (write first — must fail)

- [ ] T005 [P] [US1] Failing unit test for the `WritePolicyFor` truth table (off/on × readonly absent/true → Writable) in `internal/config/resolve_test.go`.
- [ ] T006 [P] [US1] Failing unit test: a read-only-guarded backend returns `ErrReadOnly` from `CreateFolder` without invoking the underlying client (use a client stub that fails if called) in `internal/storage/guard_test.go`.

### Implementation

- [ ] T007 [US1] Add `WritePolicy{Writable bool}` and a NEW method `WritePolicyFor(name, writeFlag)` computing `Writable = writeFlag && !ctx.ReadOnly` in `internal/config/resolve.go`; leave existing `Resolve(name)` and `ClientConfig(name)` untouched so `cmd/s3s/main.go` callers don't break (depends on T003).
- [ ] T008 [US1] Implement `readOnlyGuard` (embeds read methods, mutations return `ErrReadOnly`) and `Guard(b, policy)` in `internal/storage/guard.go` (depends on T002).
- [ ] T009 [US1] Add the `--write` boolean flag in `cmd/s3s/main.go`, call `cfg.WritePolicyFor(name, writeFlag)` in the resolver closure, and wrap the backend with `storage.Guard(backend, policy)` (depends on T007, T008).
- [ ] T010 [US1] Surface a read-only hint ("context is read-only — start with --write") when a mutation is attempted on a non-writable context, mapping `ErrReadOnly`, in `internal/ui/app.go` (FR-003).
- [ ] T011 [US1] Integration test (`//go:build integration`): against real MinIO, a guarded (read-only) backend refuses `CreateFolder` without a network mutation and storage is unchanged, in `internal/storage/s3client_integration_test.go`.

**Checkpoint**: write gating works end-to-end; nothing can mutate a protected context.

---

## Phase 4: User Story 2 - Create a folder, confirmed, non-blocking, logged (Priority: P1) 🎯 MVP

**Goal**: On a writable context, create an empty folder with a simple confirmation,
non-blocking execution (progress ≤100 ms), success refresh, and full logging.

**Independent Test**: On a writable context, create `reports/` → simple confirm →
spinner appears immediately → folder visible after refresh → `mutation.start`/`done`
in the log with no secret; cancelling the confirm changes nothing.

**Depends on**: US1 (needs a writable backend + guard).

### Tests (write first — must fail)

- [ ] T012 [P] [US2] Failing unit test: `CreateFolder` normalizes the key to one trailing `/`, rejects empty/whitespace/control-char names, and reports an existing-name collision, in `internal/storage/writer_test.go`.
- [ ] T013 [P] [US2] Failing white-box UI test: create-folder intent → simple confirm overlay → on `y` a command is dispatched → `operationDone` makes the folder visible after refresh; `n`/`Esc` aborts with no command, in `internal/ui/operation_test.go`.
- [ ] T014 [P] [US2] Failing test: a mutation logs `mutation.start` before `mutation.done` with action/target/context and no secret value, in `internal/ui/operation_test.go` (log sink).

### Implementation

- [ ] T015 [US2] Implement `CreateFolder` (PutObject of `<prefix>/` with empty body) plus key normalization/validation in `internal/storage/writer.go` (depends on T002).
- [ ] T016 [US2] Implement the simple-tier confirmation overlay (state + `y`/`Enter`/`n`/`Esc`, rendered within the bordered layout) in `internal/ui/confirm.go`.
- [ ] T017 [US2] Implement operation lifecycle state and rendering — spinner within 100 ms of `operationStarted`, cancel on `x` — in `internal/ui/operation.go` (SC-004).
- [ ] T018 [US2] Add `createFolderCmd(gen, bucket, prefix)` (`tea.Cmd` with generation + `context.CancelFunc`) and `operationStarted/Progress/Done` messages in `internal/ui/commands.go` and `internal/ui/messages.go` (depends on T015, T017).
- [ ] T019 [US2] Add the create-folder keybinding and wire the intent in `internal/ui/keys.go` + `internal/ui/app.go`; invalidate the affected level cache and refresh on success (depends on T016, T018) (SC-006).
- [ ] T020 [US2] Emit `mutation.start` before dispatch and `mutation.done` on terminal state via `log/slog`, reusing `logging.Secret` redaction, in `internal/ui/operation.go`/`commands.go` (depends on T018) (FR-008, SC-005).
- [ ] T021 [US2] Integration test (`//go:build integration`): create `reports/` on a writable real-MinIO backend; re-list the level and assert it appears as a common prefix, in `internal/storage/s3client_integration_test.go` (depends on T015).

**Checkpoint**: the full vertical write slice works and is observable. MVP complete.

---

## Phase 5: User Story 3 - Guardrails for destructive actions (Priority: P2)

**Goal**: The confirmation framework provides a stronger typed-confirm tier so
future delete/overwrite features cannot fire on a casual keypress.

**Independent Test**: Drive a representative destructive intent → the typed tier
requires the exact target identifier; a mismatch aborts with no command; a
reversible action (create-folder) still needs only the simple confirm.

**Depends on**: US2 (extends the confirmation framework).

### Tests (write first — must fail)

- [ ] T022 [P] [US3] Failing white-box UI test driving a test-only `MutatingOperation{Tier: ConfirmTyped}` fixture (no production destructive action): typed tier confirms only on an exact target match, aborts on mismatch with no command, and leaves the simple tier unchanged, in `internal/ui/confirm_test.go`.

### Implementation

- [ ] T023 [US3] Extend `internal/ui/confirm.go` with the typed tier (`Expect`/`Input` exact match, echo input) and a `ConfirmTier` classification hook that destructive ops will set (depends on T016).
- [ ] T024 [US3] Expose a test-only constructor/fixture for a `ConfirmTyped` `MutatingOperation` (guarded so no production code path can trigger a destructive action in 002) so T022 drives the typed tier end-to-end, in `internal/ui/confirm.go` (test seam) + `internal/ui/confirm_test.go`.

**Checkpoint**: typed guardrail proven and ready for 003 delete/overwrite.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T025 [P] Update `README.md`: soften the "fully read-only" claim to "read-only by default; opt-in writes via `--write`", and add a short write/safety note.
- [ ] T026 [P] Mark write-foundation progress and link the spec in `ROADMAP.md`.
- [ ] T027 Run `make fmt vet lint check-readonly test test-integration` and confirm all gates pass (including the unchanged read-only guard).
- [ ] T028 [P] Verify success criteria manually per `quickstart.md`: progress ≤100 ms (SC-004), no secret in any log line (SC-005), refused mutations leave storage unchanged (SC-007).

---

## Dependencies

```text
Setup (T001)
  └─▶ Foundational (T002, T003, T004)
        └─▶ US1  (T005,T006 tests → T007,T008,T009,T010,T011)   [P1, MVP]
              └─▶ US2  (T012,T013,T014 tests → T015..T021)        [P1, MVP]
                    └─▶ US3  (T022 test → T023,T024)              [P2]
                          └─▶ Polish (T025..T028)
```

- US2 depends on US1 (writable backend + guard).
- US3 depends on US2 (extends the confirmation framework built there).

## Parallel Execution Examples

- **Foundational**: T002, T003, T004 touch different files → run in parallel
  (T004 needs T002's interface).
- **US1 tests**: T005 (config) and T006 (storage) → parallel.
- **US2 tests**: T012 (storage), T013 + T014 (ui) → parallel (T013/T014 same file,
  sequential between themselves).
- **Polish**: T025, T026, T028 → parallel; T027 runs after they land.

## Implementation Strategy

- **MVP = US1 + US2** (both P1): safe write mode + a working, confirmed,
  non-blocking, logged create-folder. Shippable increment.
- **US3 (P2)** adds the typed-confirm tier — no end-user destructive action yet, but
  unblocks 003 (delete/upload) cleanly.
- Ship US1 first (gating + guard), then US2 (the visible feature), then US3.

## Notes

- Naming caveat (from research §2): future 003 interface methods must avoid the
  `check-readonly.sh` verb+entity regex (e.g. name a delete `RemoveObject`, not
  `DeleteObject`) so the UI can call them. `CreateFolder` is already safe.
- Total: 28 tasks — Setup 1, Foundational 3, US1 7, US2 10, US3 3, Polish 4.
