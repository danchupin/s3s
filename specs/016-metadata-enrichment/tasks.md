---
description: "Task list for feature 016 — Metadata Enrichment & Inline Usage"
---

# Tasks: Metadata Enrichment & Inline Usage

**Input**: Design documents from `/specs/016-metadata-enrichment/`
**Prerequisites**: plan.md ✔, spec.md ✔, research.md ✔, data-model.md ✔, contracts/ ✔ (6)

**Tests**: REQUIRED — constitution III (Test-First) is NON-NEGOTIABLE for this repo. Every
phase writes its failing tests (RED) before production code. UI tests are white-box
(`package ui`, `App.View().Content` via `deliver`/`press`/`viewOf`); storage units use
`storage.Fake`; the US4 read-contract change adds `//go:build integration` MinIO tests
(constitution IV).

**Organization**: Tasks grouped by user story. US1 (MVP) and US5 are INDEPENDENT of the
Phase-2 migration and may proceed in parallel with it; US2/US3/US4 depend on Phase 2.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no incomplete-task dependency)
- **[Story]**: US1–US5 (Setup/Foundational/Polish carry no story label)

## Path Conventions

Single Go module `github.com/danchupin/s3s`. UI in `internal/ui/`, SDK-bound storage in
`internal/storage/`, reused cache in `internal/cache/`. Paths below are repo-relative and
exact (from plan.md §Project Structure).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish a clean baseline and the shared test helper before any change.

- [X] T001 Confirm green baseline on branch `016-metadata-enrichment`: run `make test && make check-readonly` and note the pass state; do not start edits until green.
- [ ] T002 [P] Ensure an `assertHeightSweep(t, app, widths, heights)` helper exists in `internal/ui/footer_test.go` (mirror `assertWidthSweep` at `footer_test.go:92`): asserts the footer/command-hint bar is present AND every seeded identifier is present-or-revealable across heights. Add it if missing (shared by US1 + Polish).

---

## Phase 2: Foundational (Blocking Migration) ⚠️

**Purpose**: The build-breaking shared migration — delete the separate `analyze`/`modeUsage`
screen and repurpose the freed `a` key into the context-aware `MoreDetail` dispatcher
(FR-008, FR-019). US2/US3/US4 all sit on this.

**⚠️ CRITICAL**: US2, US3, US4 cannot begin until this phase completes (build won't compile
mid-rename). **US1 (Phase 3) and US5 (Phase 7) do NOT depend on this phase** and may run in
parallel — but note US1 and Phase 2 both touch `internal/ui/pane.go` (different regions:
US1 the metadata block, Phase 2 the `keyHint` lines 54/67/71), so do not edit the same
hunk concurrently.

### Tests for Foundational (write RED first)

- [X] T003 [P] Migrate `internal/ui/analyze_test.go` (and any usage-mode test) RED: drop assertions on the removed full-screen `usageView`/`modeUsage`; retain `usageTarget` + `UsageOf` ranking assertions that survive the inline fold.
- [X] T004 [P] Update `internal/ui/footer_test.go:194,249` RED: replace the `hintCtx{mode: modeUsage}` references (test-only — `footerHints` at `styles.go:511` has no `c.mode` branch, so no production footer change).
- [X] T005 Add RED test `internal/ui/more_detail_test.go`: pressing `a` (now `MoreDetail`) and running `:detail`/`:info` BOTH dispatch the SAME `startMoreDetail` target (assert one action, no drift — FR-019, `contracts/more-detail-key.md`).

### Implementation for Foundational

- [X] T006 Delete `modeUsage` and usage view-state in `internal/ui/app.go`: const (`:30`), usage fields (`:219-227`), `onKey` case (`:881`), `View` case (`:1190-1191`).
- [X] T007 In `internal/ui/analyze.go` delete `runAnalyze`/`onUsageKey`/`usageView`/`usageTitle`; KEEP `usageTarget` and `analyzeCmd`/`waitForUsage` for reuse by US2.
- [X] T008 In `internal/ui/command.go`: drop `modeUsage` from `canOpenCommand` (`:57`); change the `analyze`/`du` command entry (`:33`) to `detail`/`info` with `invoke: App.startMoreDetail`.
- [X] T009 Rename `keys.Analyze` → `keys.MoreDetail` (binding stays `"a"`) in `internal/ui/keys.go:21,54` + help row; propagate to `internal/ui/hintbar.go:52,70` and `internal/ui/pane.go:54,67,71` (`keyHint(m.keys.Analyze,…)` → `k.MoreDetail`, label "detail"/"info").
- [X] T010 Add `startMoreDetail` dispatcher + `detailSection` enum state (`sectNone/sectBreakdown/sectTags/sectConfig`) to `internal/ui/app.go` (data-model §6): context-aware (bucket/prefix vs object), toggles ONE mutually-exclusive section, second press collapses to `sectNone` (`contracts/more-detail-key.md`, `contracts/layout-budget.md`).
- [X] T011 Add the shared `… +N more (i to reveal)` overflow affordance helper to the pane render in `internal/ui/pane.go` (trailing line when body exceeds the `rows-2` budget; clipped rows recoverable via `keys.Reveal`) — used by US1/US3/US4 (`contracts/layout-budget.md`).

**Checkpoint**: build compiles; `a` = MoreDetail; no full-screen analyze; `:detail`/`:info` share the dispatcher; existing suite green.

---

## Phase 3: User Story 1 — Rich object metadata in-context (Priority: P1) 🎯 MVP

**Goal**: Surface the version/encryption/replication/restore/object-lock/legal-hold/
lifecycle/content-handling fields the existing `HeadObject` already returns, omit-empty, in
the shared `metaFieldRows` — on BOTH the Enter object view and the focus pane.

**Independent Test**: Open/focus an SSE-KMS + versioned object → pane shows encryption type,
KMS key, version id, storage class beside the core fields; an object with no optionals shows
only the core block (no placeholder lines); object-lock/legal-hold unreadable → "unknown".

**Independent of Phase 2** — may run in parallel (shares `pane.go` region-wise only).

### Tests for User Story 1 (RED first)

- [X] T012 [P] [US1] RED storage unit in `internal/storage/fake_test.go`: `HeadObject` populates the 13 new `ObjectMetadata` fields; per-field deny flag drives empty `ObjectLockMode`/`ObjectLockLegalHold`.
- [X] T013 [P] [US1] RED UI test `internal/ui/object_metadata_test.go`: `metaFieldRows` always renders core fields; omit-empty optional rows; permission-gated `ObjectLockMode`/`ObjectLockLegalHold` render "unknown" when empty; multipart ETag (`-N`) shown as-is. Assert on BOTH `metaPane` (Enter, `modeObject`) and `paneTree` (focus) via `viewOf` (single shared source).

### Implementation for User Story 1

- [X] T014 [US1] Enrich `storage.ObjectMetadata` in `internal/storage/storage.go:187-195` with the 13 new fields (data-model §1: VersionId, DeleteMarker, SSEAlgorithm, SSEKMSKeyId, ReplicationStatus, RestoreStatus, ObjectLockMode, ObjectLockRetainUntil, ObjectLockLegalHold, LifecycleExpiration, ContentEncoding, CacheControl, ContentDisposition).
- [X] T015 [US1] Extend the `HeadObject` mapping in `internal/storage/s3client.go:151-168` to populate each field from `HeadObjectOutput` (enum→string, `aws.ToString/ToBool/ToTime`; parse `out.Restore`; malformed restore → "" never panic).
- [X] T016 [US1] In `internal/ui/metadata.go:28-37` add `omitEmpty(label, value string, gated bool)` and the omit-empty optional block to the shared `metaFieldRows` (core block keeps `orDash`; `gated && empty` → "unknown"). Reuse `metaRow`/palette roles.
- [ ] T017 [US1] Extend the existing `HeadObject` assertion in `internal/storage/s3client_integration_test.go` (`//go:build integration`) to verify the enriched fields populate against MinIO.

**Checkpoint**: US1 fully functional and independently testable — the zero-backend-cost MVP.

---

## Phase 4: User Story 2 — Inline bucket/prefix totals (Priority: P1)

**Goal**: Show total size + object count in the details pane on focus, via a dwell-gated,
generation-guarded, session-cached, cancelable `UsageOf` scan — no separate screen.

**Independent Test**: Focus a non-empty bucket and rest → "scanning… N, <size>" → `total
<size> · N objects` inline; navigate away cancels (partial marked partial); re-focus cached
target is instant; `modeUsage` is gone.

**Depends on**: Phase 2 (migration + dispatcher).

### Tests for User Story 2 (RED first)

- [X] T018 [P] [US2] RED `internal/ui/inline_usage_test.go`: deliver focus → `usageTickMsg` → `loadUsage`; delivered `usageProgressMsg` shows running line; `usageDoneMsg` renders `total … · N objects`; `Complete=false` renders as partial.
- [X] T019 [P] [US2] RED generation-isolation in `inline_usage_test.go`: scan bucket A, deliver focus move to B (bumps `usageGen` + `usageCancel`), deliver stale `usageDoneMsg` for A → pane does NOT show A's totals (quickstart).
- [X] T020 [P] [US2] RED resilience in `inline_usage_test.go`: (a) no producer leak — superseded scan channel drains to `close` (pump re-arms ungated); (b) rapid transit over N folders + one stale tick → zero `loadUsage` for passed-over targets; (c) both `r` paths (tree + bucket) cause a `usageResults` miss/rescan; (d) context switch → `usageResults.Len()==0` and same-named bucket rescans.

### Implementation for User Story 2

- [X] T021 [US2] Add usage view-state to the `App` struct in `internal/ui/app.go` (data-model §6): `usageResults *cache.Cache[*storage.UsageReport]`, `usageProg`, `usageScanKey`, `usageGen int`, `usageCancel context.CancelFunc`, `usageCh`; instantiate `usageResults` in `New()` (constructor, NOT `Init` — v2 gotcha).
- [X] T022 [US2] In `internal/ui/messages.go` add `usageTickMsg{gen,bucket,prefix}` and set `usageProgressMsg`/`usageDoneMsg` gen field to `usageGen`; in `internal/ui/commands.go` add `usageTickCmd` dwell helper (`tea.Tick`, mirror `spinnerTick`) and `loadUsage(gen, ctx)` (fresh ctx → `usageCancel`) reusing `analyzeCmd`/`waitForUsage`.
- [X] T023 [US2] Wire the dwell gate: in `internal/ui/app.go` `afterBucketMove` and the EXTENDED `afterSelectionMove` (`:328-338`, now also arming dir/level selections) call `usageCancel()`+`usageGen++` together and schedule `usageTickCmd{gen,b,pfx}`; add `onUsageTick` firing `loadUsage` only when `msg.gen==usageGen` AND `focusedUsageTarget()==(b,pfx)` AND not cached.
- [X] T024 [US2] In `internal/ui/analyze.go` make `onUsageProgress` drain `usageCh` UNGATED (re-arm `waitForUsage` while `usageCh!=nil`), gating ONLY result application (`usageProg`) on `usageGen`; `onUsageDone` (gen-matched) stores into `usageResults.Put`; cancel usage inside `beginLoad` (`usageCancel()`+`usageGen++`).
- [X] T025 [US2] Render inline totals + running/partial line in `paneBucket` (`internal/ui/pane.go:45-56`) and `paneTree` (`:58-96`), reusing `accentStyle`/`dimCellStyle`/`warnStyle`.
- [X] T026 [US2] Cache lifecycle: tree `r` → `usageResults.Invalidate` beside `m.cache.Invalidate` in `internal/ui/tree.go:144`; bucket `r` → `usageResults.InvalidateBucket` in `internal/ui/hintbar.go:175` (`refreshBuckets`); context switch → `usageResults.Clear()` beside `m.cache.Clear()` in `internal/ui/app.go` (`onContextResolved`, ~`:1060`).

**Checkpoint**: US2 testable; analyze screen gone; totals inline, dwell-gated, cached, cancelable.

---

## Phase 5: User Story 3 — Expandable breakdown inline (Priority: P2)

**Goal**: From a focused target's totals, expand the ranked largest-first child breakdown
(size + share) in the same pane as the single active detail section; collapse; drill into a child.

**Independent Test**: With totals shown, press `a` → children listed largest-first with
size + share bar; second press collapses; only ONE section visible; Enter on a child
sub-prefix steps into it.

**Depends on**: Phase 2 (dispatcher) + US2 (totals + `UsageOf` result).

### Tests for User Story 3 (RED first)

- [X] T027 [P] [US3] RED `internal/ui/inline_breakdown_test.go`: `MoreDetail` on bucket/prefix sets `detailSection=sectBreakdown`, renders children Size-desc (ties by Name) with `usageBar` share; re-press collapses to `sectNone`; mutually exclusive with tags/config; step-into child re-targets navigation + usage.

### Implementation for User Story 3

- [X] T028 [US3] In `internal/ui/pane.go` render the `sectBreakdown` section (reuse `usageBar`/share + `renderTable` from the old `usageView`) bounded by the budget gate + overflow affordance (T011); wire collapse and child drill-down (Enter on a `UsageChild` sub-prefix → navigate into it) via `startMoreDetail`/the pane key handler.

**Checkpoint**: US3 testable; breakdown + drill-down inline, one section at a time.

---

## Phase 6: User Story 4 — Tags & bucket configuration on demand (Priority: P2)

**Goal**: Lazily load object tag VALUES and bucket configuration sub-resources
(versioning/encryption/lifecycle/replication/public-access/location) on the `MoreDetail`
key, each with a tri-state status (configured / none / denied / unsupported).

**Independent Test**: `a` on a tagged object → KV pairs; `a` on a bucket → each config item
labeled, distinguishing none vs unknown/denied vs unsupported under `NO_COLOR`.

**Depends on**: Phase 2 (dispatcher). Storage extension is otherwise self-contained.

### Tests for User Story 4 (RED first)

- [X] T029 [P] [US4] RED storage unit `internal/storage/fake_test.go`: `GetObjectTagging` → KV / none / denied; `GetBucketConfiguration` per-sub-resource tri-state incl. `FakeBucket.UnsupportedGetConfigs[sub]=true` → `State=="unsupported"`, `Reason==ErrUnsupported`; per-sub-resource deny → `denied`; one failing sub-resource leaves the rest loaded (partial).
- [X] T030 [P] [US4] RED `internal/storage/classify_unit_test.go`: synthetic `smithy.APIError` codes `NotImplemented`/`MethodNotAllowed` and HTTP 501/405 → `errors.Is(classify(err), ErrUnsupported)`; `NoSuchLifecycleConfiguration` / `ServerSideEncryptionConfigurationNotFoundError` / `NoSuchTagSet` / `ReplicationConfigurationNotFoundError` / `NoSuchPublicAccessBlockConfiguration` do NOT map to `ErrUnsupported` (→ none).
- [ ] T031 [P] [US4] RED integration `internal/storage/s3client_integration_test.go` (`//go:build integration`): seed via raw SDK; assert tag KV; `configured` versioning/encryption; `none` for an unconfigured sub-resource (MinIO returns `*NotFound` family); `denied` for a policy-denied read; partial when one sub-resource fails. (Unsupported is covered by T029/T030 — MinIO can't yield it.)
- [X] T032 [P] [US4] RED UI `internal/ui/object_tags_test.go` + `internal/ui/bucket_config_test.go`: `MoreDetail` loads tags (object) / config (bucket) lazily via `tea.Cmd`; tri-state text labels (none / unknown-denied / unsupported) distinguishable under `NO_COLOR`; stale `detailGen`/`detailKey` results dropped.

### Implementation for User Story 4 — storage (read-view extension)

- [X] T033 [US4] In `internal/storage/storage.go` add `ObjectTags`, `ConfigState`/`ConfigItem`/`BucketConfig` (data-model §2–3), the `ErrUnsupported` sentinel (§4), and `GetObjectTagging`/`GetBucketConfiguration` on the `Storage` read-view interface (`:100-128`).
- [X] T034 [US4] In `internal/storage/s3client.go` add the `s3API` members (`GetObjectTagging`/`GetBucketVersioning`/`GetBucketEncryption`/`GetBucketLifecycleConfiguration`/`GetBucketReplication`/`GetPublicAccessBlock`/`GetBucketLocation`, near `:23-32`) and implement `GetObjectTagging`/`GetBucketConfiguration` (each sub-resource fetched + classified independently); extend `classify` (`:231-283`): `NotImplemented`/`MethodNotAllowed`/501/405 → `ErrUnsupported`; the `*NotFound`/`*NotConfiguration` family → `none` in the config caller (NOT generic classify). Ensure both new reads emit the storage layer's structured operation/target/outcome log (constitution V), consistent with the existing methods.
- [X] T035 [US4] In `internal/storage/fake.go` add `FakeBucket.BucketConfig`, `FailGetTags`, `UnsupportedGetConfigs`, per-field deny flags, and implement `Fake.GetObjectTagging`/`GetBucketConfiguration`.

### Implementation for User Story 4 — UI

- [X] T036 [US4] In `internal/ui/app.go` add `objectTags`/`bucketCfg`/`detailKey`/`detailGen` state (data-model §6/§8); in `internal/ui/messages.go` add `objectTagsMsg`/`bucketConfigMsg` carrying `detailGen`; in `internal/ui/commands.go` add `loadObjectTags`/`loadBucketConfig` cmds. `startMoreDetail` sets `sectTags` (object) / `sectConfig` (bucket) and dispatches the load; stale `detailGen`/`detailKey` dropped in `Update` (mirror `onPaneTick`, `:344-357`).
- [X] T037 [US4] In `internal/ui/pane.go` render the `sectTags` (KV via `sanitizeLabel`) and `sectConfig` (per-item tri-state text labels) sections — one at a time, within the budget gate + overflow affordance.
- [X] T038 [US4] Run `make check-readonly` and confirm `PASS` — every new symbol is `Get*` (no `Put|Delete|Create|Copy|Upload|Restore|Write`); SDK stays in `internal/storage`.

**Checkpoint**: US4 testable; tags + config on demand, tri-state honest, guard green.

---

## Phase 7: User Story 5 — Storage class visible in listing (Priority: P3)

**Goal**: Mark non-standard storage classes in the listing `type` column without column
bloat; full class revealable.

**Independent Test**: A level with a STANDARD and a GLACIER object → GLACIER row shows a
compact marker, STANDARD adds no noise, `i` reveals the full `GLACIER` string.

**Independent of all other phases** (touches `tree.go` only).

### Tests for User Story 5 (RED first)

- [X] T039 [P] [US5] RED `internal/ui/storage_class_marker_test.go`: GLACIER row shows the compact marker in the `type` cell; STANDARD adds no per-row noise; `i` (reveal) shows the full class; column widths stay legible at 80/120/160 cols.

### Implementation for User Story 5

- [X] T040 [US5] In `internal/ui/tree.go:224-240` render a non-standard storage-class marker in the `type` column using the CLOSED token table in `contracts/listing-storage-class.md` (STANDARD/"" → `obj` no marker, GLACIER → `glac`, GLACIER_IR → `gir`, DEEP_ARCHIVE → `arch`, INTELLIGENT_TIERING → `int`, STANDARD_IA → `ia`, ONEZONE_IA → `1zia`, REDUCED_REDUNDANCY → `rr`, any other non-standard → `cls*`) within the 5-char budget; full class recoverable via `keys.Reveal` on the row.

**Checkpoint**: US5 testable.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T041 [P] FR-017 budget sweep: add `internal/ui/metadata_legibility_test.go` — at 130×24 and 80×24 seed ALL enriched optional fields + one detail section (breakdown OR tags) and assert every seeded value is present in `View().Content` OR represented by `… +N more (i to reveal)`, AND the footer/command-hint bar is present (NOT just footer-present — body is hard-capped at `styles.go:348-350`). Uses `assertHeightSweep` (T002).
- [X] T042 [P] Extend `assertWidthSweep`/height coverage in `internal/ui/footer_test.go` to the new pane content; migrate `spec013` / `app_test` references touched by the rename; keep `search_test` green. Add a design-system conformance assertion (FR-018): new rows reuse `metaRow`/`usageBar`/`colHeadStyle` + palette roles (`accentStyle`/`dimCellStyle`/`warnStyle`), no ad-hoc styling.
- [X] T043 [P] Update `README.md` and any in-app help: remove `analyze`/`du`; document `a` = "more detail" (+ `:detail`/`:info`), the enriched object metadata, inline usage totals/breakdown, tags/config, and the storage-class marker.
- [ ] T044 Run the `quickstart.md` manual validation walkthrough (SC-001..SC-007 + FR-015) against a MinIO/RGW context; record results in the PR.
- [X] T045 Final gate: `make fmt vet lint && make check-readonly && make test && make test-integration` (Lima `DOCKER_HOST` + `TESTCONTAINERS_RYUK_DISABLED=true` for integration) — all green.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (P1)**: no dependencies.
- **Foundational (P2)**: after Setup. BLOCKS US2/US3/US4. Does NOT block US1/US5.
- **US1 (P3)**: after Setup; independent of Foundational (parallelizable with P2, mind the shared `pane.go` regions).
- **US2 (P4)**: after Foundational.
- **US3 (P5)**: after Foundational + US2 (needs totals + dispatcher).
- **US4 (P6)**: after Foundational (dispatcher); storage sub-block self-contained.
- **US5 (P7)**: after Setup; independent of everything else.
- **Polish (P8)**: after all desired stories.

### User Story Dependencies

- **US1 (P1)** — independent (zero-backend MVP). No dependency on the migration.
- **US2 (P1)** — depends on Phase 2 migration.
- **US3 (P2)** — depends on Phase 2 + US2.
- **US4 (P2)** — depends on Phase 2; storage extension independent.
- **US5 (P3)** — independent.

### Within Each Story

- RED tests before implementation (constitution III); verify they fail.
- Storage types/mapping before UI render; messages/commands before the handlers that consume them; core render before the budget/overflow polish.

### Parallel Opportunities

- Setup T002 ∥ baseline.
- Foundational RED: T003 ∥ T004 (T005 after the rename target exists conceptually).
- US1 tests T012 ∥ T013; US1 can run wholesale in parallel with Phase 2.
- US5 (T039→T040) can run any time after Setup, fully in parallel.
- US2 tests T018 ∥ T019 ∥ T020. US4 tests T029 ∥ T030 ∥ T031 ∥ T032; storage impl T033→T034→T035 then UI T036→T037.
- With staff: after Phase 2, Dev A=US2→US3, Dev B=US4, Dev C=US1, Dev D=US5 — converge at Polish.

---

## Parallel Example: User Story 4 (RED batch)

```bash
# Launch the US4 failing tests together (different files):
Task: "RED Fake tri-state in internal/storage/fake_test.go"            # T029
Task: "RED classify unsupported in internal/storage/classify_unit_test.go"  # T030
Task: "RED MinIO integration in internal/storage/s3client_integration_test.go"  # T031
Task: "RED UI tags/config in internal/ui/object_tags_test.go + bucket_config_test.go"  # T032
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Phase 1 Setup → 2. Phase 3 US1 (independent of the migration) → 3. **STOP & VALIDATE**:
open SSE-KMS/versioned objects, confirm enriched omit-empty metadata on Enter + focus,
"unknown" for permission-gated, footer intact. Ship the MVP — it adds zero backend cost.

### Incremental Delivery

1. Setup + US1 → ship (rich object metadata).
2. Foundational migration → US2 (inline totals, analyze screen gone) → ship.
3. US3 (breakdown) → ship. 4. US4 (tags/config) → ship (requires integration tests).
5. US5 (storage-class marker) → ship. 6. Polish (budget sweep + docs + final gate).

Each story is independently testable; stop at any checkpoint to validate.

---

## Notes

- [P] = different files, no incomplete dependency. [Story] traces task → user story.
- Verify every RED test fails before implementing (constitution III).
- US4 is the only storage-contract change → MinIO integration is REQUIRED (constitution IV);
  the `unsupported` branch is covered by Fake + classify units (MinIO can't produce it).
- `make check-readonly` MUST stay green throughout — new symbols are all `Get*`.
- Commit after each task or logical group; keep the footer/command-hint bar on-screen at
  every supported width (constitution VI) — assert it, don't assume it.
