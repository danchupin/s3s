---
description: "Task list for 003-object-write-ops"
---

# Tasks: Object Write Operations

**Input**: Design documents from `/specs/003-object-write-ops/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/
(object-mutator-interface, ui-write-flows-contract)

**Tests**: INCLUDED — TDD is non-negotiable (Constitution III). Every behavior task
is preceded by a failing test.

**Organization**: Grouped by user story. US1 (delete) + US2 (upload) are P1 = MVP.
US3 (copy) + US4 (move) are P2. US5 (recursive delete) is P3.

## ⚠️ Guard-safe naming (read before writing any UI code)

`scripts/check-readonly.sh` fails if an identifier matching
`\b(Put|Delete|Create|Copy|Upload|Restore|Write)(Object|Bucket|Part|…)[A-Za-z]*\b`
appears in a `.go` file outside `internal/storage/` (comments included). The UI calls
the `Mutator` methods, so use the guard-safe Go names everywhere outside storage:
`RemoveObject`, `UploadFile`, `CopyKey`, `MoveObject`, `DeleteRecursive`,
`DeleteSummary`. The raw SDK symbols (`DeleteObject`, `PutObject`, `CopyObject`,
`DeleteObjects`) appear ONLY inside `internal/storage`. Run `make check-readonly`
after touching UI files.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[Story]**: US1..US5 (story-phase tasks only)
- File paths are repo-relative.

## Path Conventions

Single Go module: `cmd/s3s/`, `internal/{storage,config,ui,localfs,logging}/`. Per plan.md.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Clean baseline before changes.

- [x] T001 Confirm branch `003-object-write-ops` and a green baseline: run `make build test check-readonly` and record they pass before edits.
- [x] T002 Add the AWS multipart upload helper dependency. **DEVIATION**: not added — `UploadFile` streams via a single `PutObject` (the body reads from the source file, not buffered), which keeps the narrow `s3API` mock interface clean and works for the MinIO/RGW targets. Multipart for very large files is deferred. No new dependency.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Interface/type/scaffold additions every user story depends on. No
behavior yet. **⚠️ No user story work begins until this phase is complete.**

- [x] T003 [P] Extend the `Mutator` interface in `internal/storage/storage.go` with `RemoveObject`, `UploadFile`, `CopyKey`, `MoveObject`, `DeleteRecursive(...) (DeleteSummary, error)`; add the `DeleteSummary` struct and the `ErrMovePartial` sentinel (reuse `ErrInvalidName`/`ErrReadOnly`/`ErrNotFound`).
- [x] T004 [P] Extend `readOnlyGuard` in `internal/storage/guard.go` to override EVERY new method to return `ErrReadOnly` (and `DeleteSummary{}` where applicable) without touching the wrapped backend (depends on T003).
- [x] T005 [P] Extend `internal/storage/fake.go` so `Fake` implements all five new `Mutator` methods against its in-memory map, including an injectable per-key failure hook for recursive/partial tests (depends on T003).
- [x] T006 [P] Create `internal/localfs/localfs.go` with `Entry`, `ReadDir(dir) ([]Entry, error)` (dirs-first, alphabetical) and `IsReadableFile(path) error` — no Bubble Tea imports.
- [x] T007 [P] Add the new UI operation scaffolding (no behavior): extend the `operation` struct with `srcKey/dstKey/prefix/localPath/localSize/progress`, add `opPhase` values `phaseBrowse`/`phaseDest`, add the `opProgress` struct, `operationProgressMsg`, and extend `operationDoneMsg` with `summary *storage.DeleteSummary` + `partial bool` in `internal/ui/operation.go` + `internal/ui/messages.go`.

**Checkpoint**: interfaces, guard, fake, localfs, and UI types compile; behavior added per story.

---

## Phase 3: User Story 1 - Delete a single object (Priority: P1) 🎯 MVP

**Goal**: On a writable context, delete the selected object after a typed
confirmation; refused on read-only; logged; gone after refresh.

**Independent Test**: Select an object, press `d` → typed confirm of the exact key →
match deletes it (gone after refresh, logged); mismatch/cancel changes nothing;
read-only context refuses with a hint.

### Tests (write first — must fail)

- [x] T008 [P] [US1] Failing unit test: `Fake.RemoveObject` deletes the key; a not-found key returns `ErrNotFound`; and `Guard(fake,false).RemoveObject(...)==ErrReadOnly` with the map unchanged, in `internal/storage/fake_test.go` + `internal/storage/guard_test.go`.
- [x] T009 [P] [US1] Failing white-box UI test: `d` on an object opens the typed overlay (expect = exact key); a wrong/trailing-space entry aborts with no command; an exact match dispatches; `operationDone` removes it after refresh; a read-only context refuses with the read-only hint and no command, in `internal/ui/operation_test.go`.

### Implementation

- [x] T010 [US1] Implement `RemoveObject` (SDK `DeleteObject`, classify errors, no secrets) in `internal/storage/writer.go` (depends on T003).
- [x] T011 [US1] Add the delete intent: `d` keybinding, `startRemoveObject` (read-only guard → `ErrReadOnly` hint; else typed-tier `operation{kind:"delete_object", expect:key}`), in `internal/ui/keys.go` + `internal/ui/operation.go`.
- [x] T012 [US1] Add `removeObjectCmd(ctx, mut, bucket, key, gen)` (`tea.Cmd`, generation + cancel) returning `operationDoneMsg`, and route dispatch from the typed-confirm path, in `internal/ui/commands.go` + `internal/ui/app.go`.
- [x] T013 [US1] On `operationDone` success, log `mutation.done`, invalidate the current level cache, and refresh; a not-found is reported benignly (object already gone), in `internal/ui/app.go`/`operation.go` (reuse 002 `logMutationStart/Done`).
- [x] T014 [US1] Integration test (`//go:build integration`): real MinIO delete of a seeded object; re-list shows it gone; access-denied leaves storage unchanged; guard refuses delete without a network call, in `internal/storage/s3client_integration_test.go` (depends on T010).

**Checkpoint**: single-object delete works end-to-end, typed-confirmed, logged, safe.

---

## Phase 4: User Story 2 - Upload a local file as an object (Priority: P1) 🎯 MVP

**Goal**: Pick a local file via an in-TUI browser, upload it to the current level
with live progress, warn (typed) on overwrite, cancel safely.

**Independent Test**: `u` → file browser → pick a fixture → non-colliding key uses
simple confirm; colliding key uses typed overwrite; progress updates while running;
cancel/missing-file is never a success; object appears after refresh.

**Depends on**: Foundational (localfs, progress types). Independent of US1.

### Tests (write first — must fail)

- [x] T015 [P] [US2] Failing unit test: `localfs.ReadDir` orders dirs-first/alphabetical and surfaces an error on an unreadable dir; `IsReadableFile` passes a regular file and rejects a dir/missing path, in `internal/localfs/localfs_test.go`.
- [x] T016 [P] [US2] Failing unit test: `Fake.UploadFile` stores the exact bytes at the key; `Guard(fake,false).UploadFile(...)==ErrReadOnly`, in `internal/storage/fake_test.go`/`guard_test.go`.
- [x] T017 [P] [US2] Failing white-box UI test: `u` opens the file browser; navigating + selecting a fixture advances to confirm; a non-existing target key → simple confirm, an existing key → typed overwrite (`expect=target key`); `operationProgressMsg` updates the rendered progress; a cancelled upload yields a non-success outcome, in `internal/ui/filebrowser_test.go` + `internal/ui/operation_test.go`.

### Implementation

- [x] T018 [US2] Implement `UploadFile` in `internal/storage/writer.go` (depends on T003). **DEVIATION**: streaming `PutObject` (reads from the file reader, honors ctx cancel) instead of `manager.Uploader` — see T002.
- [x] T019 [US2] Implement the local file browser view + key handling (navigate in/out, select file, cancel) over `internal/localfs` in `internal/ui/filebrowser.go`, active during `phaseBrowse` (depends on T006, T007).
- [x] T020 [US2] Implement the streaming-progress mechanism: `countingReader` (throttled ≤1/50 ms), `uploadCmd` (goroutine → `chan opProgress`), and `waitForProgress(ch, gen)` re-issuing until the terminal `operationDoneMsg`, in `internal/ui/commands.go` (depends on T018, T007).
- [x] T021 [US2] Wire the upload intent: `u` keybinding → read-only guard → `phaseBrowse`; on file pick compute target key (`parent+filename`), do the advisory overwrite check against the loaded level (escalate to typed), then confirm → dispatch; handle `operationProgressMsg` (store progress, re-issue wait) and `operationDoneMsg`, in `internal/ui/keys.go` + `internal/ui/app.go` + `internal/ui/operation.go`.
- [x] T022 [US2] Render upload progress (bytes/total) during `phaseRunning` within the bordered layout, first update ≤100 ms (SC-007), in `internal/ui/operation.go`.
- [x] T023 [US2] On done: log outcome; success invalidates + refreshes the level; a cancelled/failed upload is reported non-success and the level is reloaded to ground truth (FR-016), in `internal/ui/app.go`/`operation.go`.
- [x] T024 [US2] Integration test (`//go:build integration`): upload a small file and a large (multipart-triggering) file to real MinIO; readback is byte-identical (SC-003); a cancelled upload is not reported success; guard refuses upload without a network call, in `internal/storage/s3client_integration_test.go` (depends on T018).

**Checkpoint**: upload works end-to-end with a file browser, progress, overwrite guard, safe cancel. **MVP (US1+US2) complete.**

---

## Phase 5: User Story 3 - Copy an object to a new key (Priority: P2)

**Goal**: Server-side copy the selected object to a new key in the current bucket;
simple confirm for a free destination, typed overwrite when it exists.

**Independent Test**: `c` → destination field prefilled with the source key →
`dst==src` rejected → free dst uses simple confirm → existing dst uses typed
overwrite → object exists at both keys after refresh; source unchanged.

**Depends on**: Foundational. The `phaseDest` entry it adds is reused by US4.

### Tests (write first — must fail)

- [x] T025 [P] [US3] Failing unit test: `Fake.CopyKey` duplicates bytes to dst and leaves src; `dst==src` or invalid dst → `ErrInvalidName`; `Guard(fake,false).CopyKey(...)==ErrReadOnly`, in `internal/storage/fake_test.go`/`guard_test.go`.
- [x] T026 [P] [US3] Failing white-box UI test: `c` opens `phaseDest` prefilled with the source key; `dst==src`/empty rejected with guidance (stays in `phaseDest`); free dst → simple confirm; existing dst → typed overwrite; done shows both keys listed, in `internal/ui/operation_test.go`.

### Implementation

- [x] T027 [US3] Implement `CopyKey` (SDK `CopyObject`, same bucket; validate dst non-empty/no-control/`!=src` → `ErrInvalidName` before any call) in `internal/storage/writer.go` (depends on T003).
- [x] T028 [US3] Implement the `phaseDest` destination-key entry (reuse the name-input rendering; prefill with source key; validate on enter; esc cancels) in `internal/ui/operation.go` + `internal/ui/confirm.go`.
- [x] T029 [US3] Wire the copy intent: `c` keybinding → read-only guard → `phaseDest` → advisory overwrite check (escalate to typed) → confirm → `copyCmd` dispatch; on success invalidate + refresh, log outcome, in `internal/ui/keys.go` + `internal/ui/commands.go` + `internal/ui/app.go` (depends on T027, T028).
- [x] T030 [US3] Integration test (`//go:build integration`): real MinIO server-side copy to a new key; both keys present, source unchanged; guard refuses copy without a network call, in `internal/storage/s3client_integration_test.go` (depends on T027).

**Checkpoint**: copy works; destination-entry + overwrite path proven and reusable.

---

## Phase 6: User Story 4 - Move or rename an object (Priority: P2)

**Goal**: Move/rename as copy-then-delete with a no-data-loss guarantee; always
typed-confirmed (source removed); partial outcome reported truthfully.

**Independent Test**: `m` → destination entry → typed confirm → after refresh object
only at the destination; an induced copy-ok/delete-fail leaves data at the
destination (and source) and is reported partial, never a clean success.

**Depends on**: US3 (reuses `phaseDest`) and US1 (delete path within move).

### Tests (write first — must fail)

- [x] T031 [P] [US4] Failing unit test: `Fake.MoveObject` clean move leaves only dst; copy failure leaves src intact and no dst; copy-ok/delete-fail returns `ErrMovePartial` with BOTH keys present (inject the delete failure); `Guard(fake,false).MoveObject(...)==ErrReadOnly`, in `internal/storage/fake_test.go`/`guard_test.go`.
- [x] T032 [P] [US4] Failing white-box UI test: `m` → `phaseDest` → typed confirm (always); a returned `ErrMovePartial` renders a partial message (data safe at dst; source remains) and is NOT a clean success; clean move shows only the destination, in `internal/ui/operation_test.go`.

### Implementation

- [x] T033 [US4] Implement `MoveObject` = `CopyKey` then `RemoveObject(src)`, with the fixed ordering and `ErrMovePartial` on copy-ok/delete-fail (no data loss), in `internal/storage/writer.go` (depends on T027, T010).
- [x] T034 [US4] Wire the move intent: `m` keybinding → read-only guard → `phaseDest` → always-typed confirm → `moveCmd` dispatch; map `ErrMovePartial` to a partial outcome (`operationDoneMsg.partial=true`); on success/partial invalidate + refresh BOTH source and destination levels, log outcome, in `internal/ui/keys.go` + `internal/ui/commands.go` + `internal/ui/app.go` (depends on T033).
- [x] T035 [US4] Integration test (`//go:build integration`): real MinIO clean move (only dst remains) and an induced partial (data retrievable, reported partial), in `internal/storage/s3client_integration_test.go` (depends on T033).

**Checkpoint**: move/rename works with the no-data-loss guarantee and truthful partials.

---

## Phase 7: User Story 5 - Delete a folder/prefix recursively (Priority: P3)

**Goal**: Best-effort recursive delete of a prefix subtree, typed-confirmed on the
exact prefix, with live deleted/failed progress, cancellable, truthful partials.

**Independent Test**: `D` on a folder → typed confirm of the exact prefix →
progress shows running deleted/failed → completion removes the subtree (gone after
refresh); injected per-object failures yield a partial count, never a clean
success; cancel mid-run keeps deletions and reports partial/cancelled.

**Depends on**: Foundational (progress types) + US1 (delete semantics).

### Tests (write first — must fail)

- [x] T036 [P] [US5] Failing unit test: `Fake.DeleteRecursive` removes all keys under a prefix across multiple pages and reports `DeleteSummary{Deleted,Failed}`; an injected failing key increments `Failed` while others still delete (best-effort); a cancelled ctx returns partial counts + `ctx.Err()`; `Guard(fake,false).DeleteRecursive(...)==ErrReadOnly`, in `internal/storage/fake_test.go`/`guard_test.go`.
- [x] T037 [P] [US5] Failing white-box UI test: `D` on a folder opens the typed overlay (expect = exact prefix); progress messages render running deleted/failed counts; `Failed>0` → partial message (not clean success); a cancel yields a partial/cancelled outcome, in `internal/ui/operation_test.go`.

### Implementation

- [x] T038 [US5] Implement `DeleteRecursive`: paginate `ListObjectsV2` over the prefix, delete best-effort, count into `DeleteSummary`, call `onProgress`, honor ctx cancellation, in `internal/storage/writer.go` (depends on T003). **DEVIATION**: deletes **per-object** with `DeleteObject` rather than batched `DeleteObjects` — integration testing revealed batch `DeleteObjects` requires a `Content-MD5` header the current AWS SDK checksum defaults no longer emit, which older MinIO/Ceph RGW reject (`MissingContentMD5`). Per-object delete carries no such requirement and works on every S3 backend; counts stay exact.
- [x] T039 [US5] Add `recursiveDeleteCmd` reusing the `chan opProgress` + `waitForProgress` pattern (the `onProgress(DeleteSummary)` callback feeds the channel; terminal marker carries `summary` + `partial = Failed>0 || cancelled`), in `internal/ui/commands.go` (depends on T038, T020).
- [x] T040 [US5] Wire the recursive-delete intent: `D` (shift-d) keybinding on a folder/common-prefix → read-only guard → typed confirm (`expect=prefix`) → dispatch; render running deleted/failed progress; on done invalidate + refresh the parent level; report partial truthfully, log outcome incl. counts, in `internal/ui/keys.go` + `internal/ui/app.go` + `internal/ui/operation.go` (depends on T039).
- [x] T041 [US5] Integration test (`//go:build integration`): real MinIO recursive delete over a multi-page prefix (all gone after refresh) and a partial-failure case reporting accurate counts; guard refuses recursive delete without enumerating, in `internal/storage/s3client_integration_test.go` (depends on T038).

**Checkpoint**: recursive delete works best-effort with live progress, partial truth, safe cancel.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [x] T042 [P] Add footer hint bindings for `d`/`u`/`c`/`m`/`D` so the new operations are discoverable on narrow terminals, in `internal/ui/footer*`/`keys.go`.
- [x] T043 [P] Update `README.md` write/safety section to list the five object operations and their confirmation tiers.
- [x] T044 [P] Mark 003 progress and link the spec in `ROADMAP.md` (move upload/delete/copy/move/recursive out of the backlog).
- [x] T045 Run `make fmt vet lint check-readonly test test-integration` and confirm ALL gates pass — especially `check-readonly` (guard-safe names) and the read-only guard refusing every new method.
- [x] T046 [P] Verify success criteria manually per `quickstart.md`: typed confirm on every destructive op (SC-001), overwrite always gated (SC-004), byte-identical upload (SC-003), no-data-loss move (SC-005), accurate recursive counts (SC-006), progress ≤100 ms + cancellable (SC-007), read-only refuses 100% (SC-008), no secret in any log line (SC-009).

---

## Dependencies

```text
Setup (T001, T002)
  └─▶ Foundational (T003 → T004,T005; T006; T007)
        ├─▶ US1 delete   (T008,T009 → T010..T014)            [P1, MVP]
        ├─▶ US2 upload   (T015,T016,T017 → T018..T024)        [P1, MVP]
        ├─▶ US3 copy     (T025,T026 → T027..T030)             [P2]
        │     └─▶ US4 move (T031,T032 → T033..T035)           [P2]  (reuses phaseDest + delete)
        └─▶ US5 recursive (T036,T037 → T038..T041)            [P3]  (reuses progress + delete)
              └─▶ Polish (T042..T046)
```

- US1 and US2 are independent (different files/methods) and together form the MVP.
- US4 depends on US3 (`phaseDest`) and US1 (delete within move).
- US5 reuses the US2 progress mechanism and US1 delete semantics.

## Parallel Execution Examples

- **Foundational**: T004/T005 (after T003), T006, T007 → largely parallel (different files).
- **US1 tests**: T008 (storage) ∥ T009 (ui).
- **US2 tests**: T015 (localfs) ∥ T016 (storage) ∥ T017 (ui).
- **Cross-story storage units**: T008, T016, T025, T031, T036 touch fake/guard tests — coordinate (same files) but each story's UI test is independent.
- **Polish**: T042, T043, T044, T046 → parallel; T045 runs after they land.

## Implementation Strategy

- **MVP = US1 + US2** (both P1): delete a single object (typed-confirmed) and upload
  a local file (browser + progress + overwrite guard). Shippable two-way client.
- **US3 + US4 (P2)**: copy, then move/rename (no-data-loss). US4 builds on US3.
- **US5 (P3)**: recursive prefix delete — the highest-risk op, shipped last, after
  the single-delete destructive path is proven.
- Each storage method lands tests-first (fake + guard + integration); each UI flow
  lands tests-first (white-box). Run `make check-readonly` after every UI change.

## Notes

- **Guard-safe names are mandatory** (see the warning block above): `RemoveObject`,
  `UploadFile`, `CopyKey`, `MoveObject`, `DeleteRecursive`, `DeleteSummary`. A UI
  reference to `DeleteObject`/`UploadObject`/`CopyObject`/`PutObject` — even in a
  comment — breaks `make check-readonly`.
- Overwrite detection is advisory (from the loaded level), consistent with 002's
  create-folder collision check; confirmation, not detection, is the safety gate.
- No config or flag change: `--write` + per-context `readonly` from 002 are reused.
- Total: 46 tasks — Setup 2, Foundational 5, US1 7, US2 10, US3 6, US4 5, US5 6, Polish 5.
