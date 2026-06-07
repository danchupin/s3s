# Tasks: Blocked command bar (info · read · write), capability-visible in read-only

**Feature**: 007-command-bar-blocks | **Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

TDD is non-negotiable (Constitution III): each story's tests are written first and MUST fail
before implementation. White-box UI tests live in `package ui` (`deliver`/`press` helpers,
assert on `App.View().Content`); storage units use `storage.Fake`; config units use a temp
config. `[P]` = parallelizable (distinct files, no incomplete dep).

## Phase 1: Setup

- [x] T001 Confirm the baseline is green before any change: run `make test vet lint check-readonly` from repo root and record the pass state (no edits).

## Phase 2: Foundational (blocking prerequisites)

- [x] T002 Add dangerous-action + connection keys to `internal/ui/keys.go`: `DeleteChord []string{"ctrl+x"}`, `MoveChord []string{"ctrl+o"}`, `AddConn []string` (a visible add-connection key), set them in `defaultKeys()`, and add `keyGlyph` entries so `ctrl+x`→`^x`, `ctrl+o`→`^o` render compactly (research R1).

**Checkpoint**: keys exist for US2/US4/US5; build still green (additive struct fields).

---

## Phase 3: User Story 1 — three blocks, write visible-but-dimmed in read-only (P1) 🎯 MVP

**Goal**: Footer shows `info · read · write` columns; the write block is always visible and
dimmed in read-only, active (caution) when armed.

**Independent test**: Open a read-only context — the write block lists all six write actions
dimmed; arm `w` — it switches to active; read/info always present; collapse < ~100 cols still
shows the dimmed write block + badge.

### Tests (write first — MUST fail)

- [x] T003 [P] [US1] In `internal/ui/commandbar_test.go`, assert `App.View().Content` in `modeBuckets` and `modeTree` renders three labelled blocks — info, read, write — as side-by-side columns (FR-001/FR-002).
- [x] T004 [P] [US1] In `internal/ui/commandbar_test.go`, assert: read-only render lists all six write actions (delete, copy, move, rm, upload, new folder) in a dimmed style (FR-007); armed render shows the write block active and distinct (FR-008); pressing a dimmed write key mutates nothing and shows the read-only nudge, no auto-arm (FR-009); `w` flips dimmed↔active (FR-010).
- [x] T005 [P] [US1] In `internal/ui/commandbar_test.go`, assert responsive collapse: at width < `blockColMin` the bar is a compact row that still lists the write entries (dimmed) and keeps the `[RW]/[RO]` badge (FR-016); 80×24 renders without clipping the list (SC-005).
- [x] T006 [P] [US1] In `internal/ui/commandbar_test.go`, table-driven test over the read+write action catalog asserting every label obeys FR-005a (single imperative verb, ≤2 words, lowercase, no articles, no trailing punctuation) (SC-014).
- [x] T006a [P] [US1] In `internal/ui/commandbar_test.go`, mark ≥2 objects (multi-select) and assert the read/write blocks render the BULK variants with a count (e.g. `download 2`, `delete 2`) rather than the single-item labels (FR-017).
- [x] T006b [P] [US1] In `internal/ui/commandbar_test.go`, assert a selection-inapplicable write action (e.g. recursive delete with an object — not a folder — selected) renders in the `entryInapplicable` style and that this style is visually DISTINCT from the read-only `entryDimmed` style (FR-018).

### Implementation

- [x] T007 [US1] Create `internal/ui/commandbar.go`: `blockKind` (info/read/write), `barEntry` (key, label, role, state), `entryState` (active/dimmed/inapplicable); builders that produce info entries (context/cluster/user/region/version) and read/write entries from the existing `actionCatalog()`, emitting write entries ALWAYS with `entryDimmed` when `!m.writable()`, `entryInapplicable` for wrong-selection (rendered in a style DISTINCT from `entryDimmed` — satisfies T006b/FR-018), else `entryActive`. Reuse `actionLabel()` so a `bulk` action under an active multi-select renders its bulk label + count (satisfies T006a/FR-017) (data-model §Command bar).
- [x] T008 [US1] In `internal/ui/styles.go`, add `styleRole` + a role→`lipgloss.Style` map reusing existing tokens only (info = `seg*Style` hues, read keys = `accentStyle`, write-armed = `warnStyle` amber, write-dimmed = `emptyStyle` faint, write-inapplicable = a third existing token distinct from both dimmed and armed — e.g. `dimCellStyle` with a non-color cue — for FR-018); no new hue (FR-013).
- [x] T009 [US1] Rework `internal/ui/hintbar.go` `hintBarView` (and `app.go` `footerBlock`) to render the three blocks as columns via `lipgloss.JoinHorizontal`; below `blockColMin` collapse to a single compact wrapped row that still lists write (dimmed) and keeps the badge (FR-002/FR-006/FR-016). Remove the old undifferentiated single-strip path.
- [x] T010 [US1] Update labels to satisfy FR-005a in `internal/ui/hintbar.go` catalog: `rm -r`→`delete` (recursive conveyed by folder selection + chord), `mkdir`→`new folder`; keep bulk count suffix (`delete 3`).
- [x] T011 [US1] Ensure the dimmed-write-key no-op + nudge path (`dispatchActionKey` `writeOnly && !writable()` → `ErrReadOnly`) still fires from the new block bar, and the read/info blocks render in every write state (FR-005/FR-006).

**Checkpoint**: US1 independently shippable — blocks render, write dimmed in RO, MVP done.

---

## Phase 4: User Story 2 — add-connection affordance in the info block (P1)

**Goal**: A visible add-connection key in the info block opens the existing connection manager.

**Independent test**: From the bucket list the info block shows an add-connection key; pressing
it opens `modeConnections`/form; the 006 add flow is unchanged.

**Depends on**: US1 (info block), Foundational (AddConn key).

### Tests (write first — MUST fail)

- [x] T012 [P] [US2] In `internal/ui/commandbar_test.go`, assert the info block shows the add-connection key+label (FR-011) and that pressing `AddConn` in `modeBuckets`/`modeTree` opens the connection manager (FR-012).

### Implementation

- [x] T013 [US2] In `internal/ui/commandbar.go`, add the add-connection affordance entry to the info block whenever `m.connect != nil`; wire the `AddConn` key in `onBucketsKey`/`onTreeKey` (via `app.go`) to `openConnections` (no change to add/test/save flow).

**Checkpoint**: add-connection discoverable from the bar.

---

## Phase 5: User Story 4 — dangerous chords + tier surfaces + bucket delete (P1)

**Goal**: Dangerous actions require a Ctrl chord; binary tier confirms in a centered popup,
typed tier in a prominent inline form; whole-bucket delete (empty-only) added.

**Independent test**: bare `x`/`X`/`m` do nothing; `Ctrl+x` on an object → binary popup, on a
folder → typed-path inline form, on the bucket list → typed-name (empty bucket only); `Ctrl+o`
moves; read-only chord → nudge only.

**Depends on**: Foundational (chords).

### Storage — bucket delete (TDD)

- [x] T014 [P] [US4] In `internal/storage/writer_test.go` (or `fake_test.go`), assert `Fake.RemoveBucket` removes an empty bucket and returns `ErrBucketNotEmpty` for a non-empty one; assert `readOnlyGuard.RemoveBucket` returns `ErrReadOnly` in `internal/storage/guard_test.go` (SC-015).
- [x] T015 [US4] In `internal/storage/storage.go` add `ErrBucketNotEmpty` sentinel and `RemoveBucket(ctx, bucket) error` to `Mutator`; implement `readOnlyGuard.RemoveBucket → ErrReadOnly` in `guard.go` and `Fake.RemoveBucket` (empty-only) in `fake.go`.
- [x] T016 [US4] In `internal/storage/writer.go` implement `(*s3Client).RemoveBucket`: `ListObjectsV2(maxKeys=1)`; if any content → `ErrBucketNotEmpty` (no delete); else `DeleteBucket` (FR-024b). The `DeleteBucket` S3 symbol stays in this file only.
- [x] T017 [US4] In `internal/storage/s3client_integration_test.go` (`//go:build integration`) add `RemoveBucket` cases: empty bucket deleted; non-empty bucket refused with `ErrBucketNotEmpty` (MinIO; `t.Skip` without Docker) (Constitution IV).

### Confirm surfaces + chord gating (TDD)

- [x] T018 [P] [US4] In `internal/ui/dangerous_test.go`, assert bare `x`/`X`/`m` trigger NO mutation and open NO confirmation surface (SC-008); `ctrl+x` (object/folder/bucket-list) and `ctrl+o` (object) do trigger (FR-021/FR-022).
- [x] T019 [P] [US4] In `internal/ui/dangerous_test.go`, assert the surface per tier: delete-object/bulk/move/overwrite → centered binary popup (y deletes, n/Esc aborts); recursive delete → inline form requiring the exact path; bucket delete → inline form requiring the exact bucket name; a wrong identifier aborts with no mutation; both surfaces show the write badge and cancel on Esc (FR-023/FR-024/FR-024a/FR-025/FR-027a).
- [x] T020 [P] [US4] In `internal/ui/dangerous_test.go`, assert a dangerous chord in a read-only context opens no surface and shows the read-only nudge (FR-028).

### Implementation

- [x] T021 [US4] In `internal/ui/operation.go`: add `delete_bucket` op kind + `startRemoveBucket` (typed tier, `expect = bucket`); change `startRemoveObject` and `startMove` from `confirmTyped` to `confirmSimple` (binary tier), keep `startRecursiveDelete` typed; route `delete_bucket` in `dispatchOp` to `RemoveBucket` with `logMutationStart("delete_bucket", …)` before execution (data-model; Constitution V).
- [x] T022 [US4] In `internal/ui/app.go` (+ `onTreeKey`/`onBucketsKey`): gate dangerous actions behind the chord — bare `x`/`X`/`m` become inert + read-only/chord nudge; `DeleteChord` routes by selection context (object→delete, folder→recursive, bucket-list→bucket), `MoveChord`→move (FR-021).
- [x] T023 [US4] Create `internal/ui/confirmview.go`: `confirmSurface(op) surface` (binary popup vs inline typed); a `centerOverlay()` popup renderer for the binary tier and a prominent inline typed-form renderer (real editable field, horizontal scroll for long identifiers); both reuse one shared style + `writeBadge` (FR-023a/FR-027a).
- [x] T024 [US4] In `internal/ui/confirm.go` route `onConfirmKey` by surface (binary y/N for the popup, typed byte-exact for the inline form) and in `internal/ui/app.go` `View()` overlay the centered popup over the dimmed body for the binary tier; the inline typed form renders in the footer status zone.
- [x] T025 [US4] In `internal/ui/commandbar.go`, render dangerous write entries with their chord (`^x delete`, `^o move`) so the gate is discoverable (FR-026).

**Checkpoint**: no bare keystroke destroys data; tiered surfaces work; bucket delete (empty-only) lands.

---

## Phase 6: User Story 3 — palette distinctness & calm (P2)

**Goal**: info/read/write-active/write-dimmed are visually distinct using only the existing
palette; meaning survives NO_COLOR; the screen stays calm.

**Independent test**: render read-only and armed; an observer distinguishes all four; under
`NO_COLOR` the active-vs-inactive write distinction is still legible.

**Depends on**: US1.

### Tests (write first — MUST fail)

- [x] T026 [P] [US3] In `internal/ui/commandbar_test.go`, assert the four block roles map to distinct existing-palette styles and that under `NO_COLOR` the inactive write block carries a redundant text cue (`(w)`/`^`/dim glyph) so active-vs-inactive survives (FR-013/FR-014/FR-015, SC-004/SC-007).

### Implementation

- [x] T027 [US3] In `internal/ui/styles.go`, finalize the role→style mapping (no new hue; armed-write amber distinct from read coral and from dimmed faint) and add the NO_COLOR text cues; keep contrast calm (one extra role only).

**Checkpoint**: palette grouping legible and calm.

---

## Phase 7: User Story 5 — delete a connection on the contexts screen (P2)

**Goal**: Remove a saved connection (config triple + keychain) from the contexts screen behind
a typed-name confirmation; active context non-deletable; last connection → empty state.

**Independent test**: contexts screen, `Ctrl+x` on a non-active connection → typed-name confirm
→ removed live; active → refused; last → empty state, no crash.

**Depends on**: US4 (typed inline surface + chord), Foundational (keys).

### Tests (write first — MUST fail)

- [x] T028 [P] [US5] In `internal/config/connection_test.go`, assert `(*Config).RemoveConnection` drops the cluster+user+context triple and persists, refuses `name == CurrentContext`, tolerates a missing keychain secret (best-effort), and returns the updated context names (FR-031/FR-032/FR-033).
- [x] T029 [P] [US5] In `internal/ui/connections_test.go`, assert the contexts screen exposes the delete key, `Ctrl+x` on a non-active row opens the typed-name confirm and on success refreshes `m.contexts` live; the active context is refused with a nudge; deleting the last connection renders the empty state without crashing; Esc cancels (FR-029/FR-030/FR-032, SC-010/SC-011).

### Implementation

- [x] T030 [US5] In `internal/config/connection.go`, add `(*Config).RemoveConnection(name string) ([]string, error)`: refuse `CurrentContext`; trial-validate the triple-removed copy; `secret.RemoveKeychain(name)` best-effort; persist; commit live; `slog.Info("connection.delete", …)` non-secret fields; return `ContextNames()`.
- [x] T031 [US5] In `cmd/s3s/connection.go` add `connSeam.Delete` calling `RemoveConnection`; add `Delete(ctx, name) ([]string, error)` to the `Connector` interface in `internal/ui/connections.go`.
- [x] T032 [US5] In `internal/ui/connections.go` (+ `messages.go`): bind `DeleteChord` on a non-active context row → the shared typed-name inline confirm (US4) → `connDeleteCmd` → `connDeletedMsg{names, err}`; on success set `m.contexts` live and notice; refuse the active context with a nudge.

**Checkpoint**: connection lifecycle complete (add from 006 + delete here).

---

## Phase 8: User Story 6 — progress bar for long operations (P2)

**Goal**: A Claude-Code-style determinate bar (percent + elapsed) inline in the footer for long
ops; indeterminate fallback; threshold-gated; non-blocking.

**Independent test**: a long op shows a bar that advances and is cancellable; a fast op shows no
bar; an unknown-total op shows an activity indicator (no percent).

**Depends on**: Foundational.

### Tests (write first — MUST fail)

- [x] T033 [P] [US6] In `internal/ui/progress_test.go`, assert: a determinate op (known total) renders a bar with a monotonic percent and a growing filled track; an indeterminate op (unknown total) renders an activity indicator with no percent (FR-037); an op finishing under the threshold renders no bar (SC-013); completion/cancel clears the bar; the bar fits 80×24 without clipping the list (SC-012).

### Implementation

- [x] T034 [US6] Create `internal/ui/progress.go`: `progressBar(frac float64, width int) string` (filled `colAccent` / empty `colDim` + ` NN%`), `opProgress.determinate() (float64, bool)`, and the "taking a while" threshold gate (using the spinner tick count) (research R3).
- [x] T035 [US6] In `internal/ui/operation.go` `opProgressLine`, use `progressBar` when determinate and past the threshold, else the indeterminate activity indicator; render inline in the footer status zone, keeping the op cancellable (`x`/Esc) (FR-034/FR-036/FR-038).

**Checkpoint**: long ops show progress; fast ops do not flash.

---

## Phase 9: Polish & cross-cutting

- [x] T036 [P] Update `internal/ui/keys.go` `helpLines` to advertise the chords (`^x delete`, `^o move`), the add-connection key, and delete-connection on the contexts screen.
- [x] T037 [P] Run `make fmt vet lint check-readonly` and fix findings; confirm no fused mutation-verb+entity S3 symbol leaks outside `internal/storage` (read-only guard intact).
- [x] T038 Run the full `go test ./...` suite and walk `quickstart.md` US1–US6; update `README.md` for any user-facing key changes (chords, delete-connection).
- [x] T039 [P] Cross-check SC-001..SC-015 each have a covering test; add any missing assertion.

---

## Dependencies & execution order

- **Setup (T001)** → **Foundational (T002)** → stories.
- **US1 (T003–T011)** — MVP, no story deps (only Foundational not strictly needed for US1).
- **US2 (T012–T013)** — after US1 (info block) + Foundational (AddConn).
- **US4 (T014–T025)** — after Foundational (chords). Storage subset (T014–T017) is independent
  of the UI subset and can run in parallel with it.
- **US3 (T026–T027)** — after US1.
- **US5 (T028–T032)** — after US4 (typed inline surface) + Foundational.
- **US6 (T033–T035)** — after Foundational; independent of US1–US5 (parallelizable).
- **Polish (T036–T039)** — last.

## Parallel opportunities

- T003, T004, T005, T006, T006a, T006b (US1 tests) together; T014 ∥ T018/T019/T020 (US4 storage vs UI tests).
- Storage cluster T014–T017 can proceed while UI cluster T018–T025 is built (different packages).
- US6 (T033–T035) can be built in parallel with US2/US3/US5 once Foundational lands.
- Polish T036, T037, T039 are `[P]`.

## Implementation strategy

- **MVP = US1** (three blocks + RO-dimmed write) — the core request, independently shippable.
- Then US2 (discoverability) and US4 (safety) — both P1 — before the P2 polish (US3 color, US5
  delete-connection, US6 progress).
- Keep each story green (tests → impl → checkpoint) before starting the next; the read-only
  guard (`make check-readonly`) and `go test` must stay green at every checkpoint.
