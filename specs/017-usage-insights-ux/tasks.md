# Tasks: Budgeted Usage, Insights & Details UX

**Input**: Design documents from `/specs/017-usage-insights-ux/`

**Prerequisites**: plan.md, spec.md, research.md (D1–D15), data-model.md, contracts/ (6), quickstart.md

**Tests**: INCLUDED — TDD is non-negotiable (constitution III). Every story phase lists RED
(failing tests first) before GREEN (implementation). Do not write production code ahead of its
failing test.

**Organization**: Phases 3–7 map 1:1 to spec user stories US1–US5 (priority order). Each story
phase is an independently testable increment.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different files, no dependency on an incomplete task)
- **[Story]**: US1–US5 (story phases only)

---

## Phase 1: Setup

**Purpose**: clean baseline; no project scaffolding needed (existing module).

- [ ] T001 Verify baseline green on branch: `make test fmt vet lint check-readonly` all pass; record `go test ./... -count=1` time as reference (no file changes)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the `UsageOf` contract change (cap + distributions) — blocks US1 (cap/Bounded) AND
US4 (distributions). One atomic signature change, compile-break first (quickstart US1 RED-1).

- [ ] T002 RED: budget-cap unit tests in `internal/storage/usage_budget_test.go` — Fake seeded budget+1 objects → `Bounded=true`, listed pages ≤ ⌈(budget+999)/1000⌉ (Fake page counter); seeded == budget exactly → `Complete=true, Bounded=false`; `maxObjects=0` → unlimited; cancelled ctx → partial + ctx.Err() (unchanged)
- [ ] T003 [P] RED: distribution unit tests in `internal/storage/usage_dist_test.go` — seeded known age/size/class mix → exact `AgeDist`/`SizeDist`/`ClassDist` buckets (boundaries per data-model §2); Σ dist counts == `TotalCount`, Σ sizes == `TotalSize`; empty class → `STANDARD`; `ScanStart` injectable in Fake
- [ ] T004 GREEN: change `Storage.UsageOf` signature (+`maxObjects int`) and extend `UsageReport` (+`Bounded`, `ScanStart`, `AgeDist`, `SizeDist`, `ClassDist`, new `DistBucket`) in `internal/storage/storage.go`
- [ ] T005 GREEN: cap enforcement (stop within one page of `maxObjects`) + single-pass histogram accumulation in `usageAgg` in `internal/storage/s3client.go`
- [ ] T006 GREEN: `Fake.UsageOf` cap + distributions + pages-listed counter + seedable `ScanStart` in `internal/storage/fake.go`
- [ ] T007 Restore compile of existing callers with NO behavior change: `startUsageScan` passes `maxObjects=0` temporarily in `internal/ui/analyze.go`; adjust existing usage tests' call sites (`internal/ui/analyze_test.go`, `internal/storage` tests)

**Checkpoint**: `make test check-readonly` green; T002/T003 pass; UI behaves exactly as 016.

---

## Phase 3: User Story 1 — Budgeted ambient scan + explicit full scan (P1) 🎯 MVP

**Goal**: ambient hover work hard-capped at the configured budget; partials cached as lower
bounds; full scan only via `A`/`:scan`; `a` never starts unbounded work.

**Independent Test**: Fake bucket > budget → dwell shows `≥` totals + affordance within the
page cap; `A` full-scans with progress; cancel keeps the lower bound (contract
budgeted-usage-scan.md, test obligations 1–7).

- [ ] T008 [US1] RED: `usageScanBudget` config tests in `internal/config/config_test.go` — absent → nil → default 20000 at resolve; `0` accepted (ambient off); negative → validation error
- [ ] T009 [US1] GREEN: `Config.UsageScanBudget *int` (YAML `usageScanBudget`) + validation in `internal/config/config.go`
- [ ] T010 [US1] GREEN: plumb resolved budget into the UI — `App` field set in `New()` via `cmd/s3s/main.go` wiring (UI never reads config files)
- [ ] T011 [P] [US1] RED: budgeted ambient-scan UI tests in `internal/ui/budget_scan_test.go` — dwell on uncached big target → `≥`+`partial` in `View().Content`, Fake page counter ≤ cap; boundary (count==budget) → no `≥`; `budget=0` → dwell arms nothing, cached results still render; dwell on a target with a cached PARTIAL → `≥` renders instantly, zero new pages (partial is a cache HIT for ambient purposes — only `A`/`:scan` upgrades it)
- [ ] T012 [P] [US1] RED: partial-caching tests — cancel mid-scan → cache holds lower bound, revisit renders `≥` instantly with zero new pages; exact never overwritten by partial; full-scan completion replaces partial; superseded-gen report still cached under its own `usageScanKey`; update discard expectations in `internal/ui/inline_usage_resilience_test.go`
- [ ] T013 [P] [US1] RED: full-scan dispatcher tests in `internal/ui/full_scan_test.go` — `A` and `:scan` invoke ONE dispatcher; progress totals stream; cancellable; `a`/`:detail` on uncached target runs BUDGETED scan only (page counter) + affordance line present
- [ ] T014 [US1] GREEN: budget plumb through `armUsageScan`/`onUsageTick`/`startUsageScan`; `startFullScan` (maxObjects=0) under `usageGen`; `onUsageDone` caches exact AND `Bounded` AND cancelled reports (drop the discard at the 016 `analyze.go:185-187` branch) in `internal/ui/analyze.go`
- [ ] T015 [US1] GREEN: `keys.FullScan = "A"` in `internal/ui/keys.go` + help row; `:scan` entry → `startFullScan` in `internal/ui/command.go`; affordance hint in `internal/ui/hintbar.go`
- [ ] T016 [US1] GREEN: pane lower-bound rendering (`≥` totals, `partial` text marker, `A full scan` affordance via `keyHint`) in `internal/ui/pane.go`
- [ ] T017 [US1] Integration test: cap honored against real MinIO pagination (seed > budget, assert `Bounded` + request count) in `internal/storage/s3client_integration_test.go`

**Checkpoint**: US1 fully functional — cluster-safety fix delivered. MVP shippable.

---

## Phase 4: User Story 2 — Details pane people can read (P1)

**Goal**: grouped metadata, dual dates, text-distinct field states, multipart-ETag
explanation, per-field copy machinery.

**Independent Test**: render a fully-enriched object at 130×24 — group headers, states,
annotation, dual dates all present/revealable (contract details-pane-groups.md, obligations 1–6).
Caveat: per-field copy (spec US2 scenario 4) is verified via direct dispatcher invocation in
white-box tests; the user-facing entry ("copy a field…" menu item) ships in US3/T032. US2 alone
delivers machinery + tests, not the menu entry — acceptable because phases execute US2→US3.

- [ ] T018 [P] [US2] RED: grouped-render + state-matrix + multipart-ETag tests in `internal/ui/metadata_groups_test.go` — 4 group headers in stable order; empty groups omitted; `unknown` ≠ `—` ≠ `denied` ≠ `unsupported` by TEXT; same assertions under NO_COLOR; ETag `32hex-N` → `(multipart, N parts — not a content hash)`, plain ETag unannotated
- [ ] T019 [P] [US2] RED: `relTime` unit + dual-date render tests in `internal/ui/reltime_test.go` — fixed `now` → `just now/Nm/Nh/Nd/Nmo/Ny ago` table; Modified row carries BOTH relative and exact `formatDate` forms
- [ ] T020 [P] [US2] RED: per-field copy tests in `internal/ui/field_copy_test.go` — field-select over visible rows; Enter emits OSC52 cmd with FULL untruncated value (long KMS ARN); footer confirms label; Esc exits without copy
- [ ] T021 [P] [US2] RED: extend the 130×24 height sweep for grouped layout + all states seeded in `internal/ui/metadata_legibility_test.go` — every value present in `View().Content` OR revealable
- [ ] T022 [US2] GREEN: regroup `metaFieldRows` into 4 ordered groups with `colHeadStyle` headers + state rendering table in `internal/ui/metadata.go`
- [ ] T023 [US2] GREEN: `relTime(now, t)` helper + `App.now func() time.Time` injection (default `time.Now`, fixed in tests) in `internal/ui/metadata.go` + `internal/ui/app.go`
- [ ] T024 [US2] GREEN: multipart-ETag annotation (`^"?[0-9a-f]{32}-(\d+)"?$`) in `internal/ui/metadata.go`
- [ ] T025 [US2] GREEN: field-select copy machinery (state machine + OSC52 emit via the reveal path; directly invokable — menu entry wired in US3/T032) in `internal/ui/fieldcopy.go`

**Checkpoint**: US1+US2 = both P1 stories done; pane readable, identifiers copyable.

---

## Phase 5: User Story 3 — Copy & share affordances (P2)

**Goal**: `Y` menu → URI / style-aware URL / CLI snippet / presigned GET (4 TTL presets) /
field copy / report export to DownloadDir.

**Independent Test**: each menu item on object/bucket/prefix/health focus produces the exact
artifact; presigned link never logged (contract copy-share-menu.md, obligations 1–7).

- [ ] T026 [P] [US3] RED: builder units in `internal/share/share_test.go` — `S3URI` (trailing `/` for prefixes); `HTTPURL` path-style vs vhost from `pathStyle`; key escaping table (space, `+`, unicode, `?`); `CLISnippet`/`CurlSnippet` exact strings
- [ ] T027 [P] [US3] RED: export units in `internal/share/export_test.go` — CSV golden (`section,label,count,bytes,bounded` rows); JSON round-trip; `bounded:true` carried for partial reports; filename helper `s3s-report-<bucket>[-<prefix-slug>]-<ts>.{csv,json}` incl. 40-char slug truncation
- [ ] T028 [US3] GREEN: new pure package `internal/share` — `share.go` (builders) + `export.go` (serializers + filename helper); no SDK, no Bubble Tea imports
- [ ] T029 [P] [US3] RED: `PresignGet` storage units in `internal/storage/presign_test.go` — only 15m/1h/24h/7d accepted (else `ErrInvalidConfig`-classified); `warn` non-empty when Fake creds `CanExpire && Expires < now+ttl`; Fake records ZERO backend calls; deterministic Fake URL embeds bucket/key/ttl
- [ ] T030 [US3] GREEN: `PresignGet` on the read view in `internal/storage/storage.go` + `s3.NewPresignClient` implementation logging only `{op:"presign", bucket, key, ttl}` in `internal/storage/s3client.go` + Fake in `internal/storage/fake.go`
- [ ] T031 [P] [US3] RED: copy-menu UI tests in `internal/ui/copymenu_test.go` — item matrix per focus (object / bucket / prefix / details-pane with usage report visible → export items — exact items, no more; health-card focus deferred to US4/T039); TTL picker shows exactly 4 presets, Enter default 1h; OSC52 payload per artifact; "show value" fallback popup; footer confirmation names artifact+target; presign flow leaves NO URL/signature in a hooked log buffer
- [ ] T032 [US3] GREEN: `internal/ui/copymenu.go` overlay (connections-list pattern) + `keys.CopyMenu = "Y"` in `internal/ui/keys.go` + `:copy` in `internal/ui/command.go` + help/hintbar rows + "copy a field…" wired to the US2 machinery (`fieldcopy.go`)
- [ ] T033 [US3] GREEN: export `tea.Cmd` — write to `DownloadDir` via temp+rename, remove temp on failure, footer absolute path / error in `internal/ui/commands.go` + `internal/ui/messages.go`; log `{op:"export", path, format, rows}` — never file content
- [ ] T034 [US3] Integration test: presigned URL fetched via plain `http.Get` returns object bytes; URL carries correct `X-Amz-Expires` in `internal/storage/s3client_integration_test.go`

**Checkpoint**: every browse hit convertible to a shareable artifact in ≤3 keystrokes (SC-004).

---

## Phase 6: User Story 4 — Operator health card (P2)

**Goal**: full-screen `modeHealth` (`H`/`:health`): age/size/class histograms (zero extra
requests), incomplete-MPU block (tri-state), small-object warning.

**Independent Test**: seeded mix renders exact card; partial data `≥`-labelled; denied MPU
never reads as zero (contract health-card-view.md, obligations 1–7).

- [ ] T035 [P] [US4] RED: `ListIncompleteUploads` storage units in `internal/storage/incomplete_uploads_test.go` — pagination across seeded `FakeIncompleteUpload`s; sizing capped at first 100 (`SizedCount`, `TotalSize`); honest zero → `State==ConfigNone, Count==0`; denied → `ConfigDenied`; unsupported via `UnsupportedListUploads` Fake toggle + `classify` unit (NotImplemented/501); cancelled ctx → partial + ctx.Err()
- [ ] T036 [US4] GREEN: `IncompleteUploads` type + `ListIncompleteUploads` on the read view in `internal/storage/storage.go`; `ListMultipartUploads`+`ListParts` (cap 100) implementation + `s3API` members in `internal/storage/s3client.go`; Fake seeds/toggles in `internal/storage/fake.go`
- [ ] T037 [P] [US4] RED: health knob config tests (`healthSmallObjectKiB` default 128, `healthSmallObjectShare` default 0.5, validation ranges) in `internal/config/config_test.go`
- [ ] T038 [US4] GREEN: health knobs in `internal/config/config.go` + plumb to `App` via `cmd/s3s/main.go`
- [ ] T039 [P] [US4] RED: health-card UI tests in `internal/ui/health_test.go` — card renders seeded distributions exactly (6+6 histogram rows via bars); partial report → every figure `≥`, header `partial`, affordance present; MPU 6-state matrix (loading/none/present/denied/unsupported/error — denied NEVER zero); small-object warning fires >50% under threshold with both numbers in text, silent at ≤50%; Esc restores exact selection/zone/scroll; `H` on uncached target runs BUDGETED scan only (page counter); object focus → no-op + footer note; card renders "scanning…" running totals while the budgeted scan it started is in flight; copy menu opened FROM the card offers export CSV/JSON (extends the T031 matrix once modeHealth exists)
- [ ] T040 [P] [US4] RED: card legibility sweep at 130×24 + 24-row collapse order (classes→size→age) + NO_COLOR in `internal/ui/health_legibility_test.go` — footer last line intact, every value present or revealable
- [ ] T041 [US4] GREEN: `modeHealth` + `internal/ui/health.go` view (histograms via `usageBar`, MPU block under `healthGen`+cancel, warning lines, collapse order) + mode routing/`prevMode` return in `internal/ui/app.go`
- [ ] T042 [US4] GREEN: `keys.Health = "H"` in `internal/ui/keys.go` + `:health` in `internal/ui/command.go` + help/hintbar rows; MPU cache (`cache.Cache[*storage.IncompleteUploads]`) invalidated with `usageResults` on refresh (`internal/ui/tree.go`, `internal/ui/hintbar.go`) and context switch (`internal/ui/app.go`); add `modeHealth` to `canOpenCommand` in `internal/ui/command.go`; route `Y` (copy menu w/ export items) and `A` (full scan) inside the card
- [ ] T043 [US4] Integration tests: MPU seed via `CreateMultipartUpload`+`UploadPart` WITHOUT complete (seeder stays inside `internal/storage`) → Count/OldestInitiated/TotalSize; distribution exactness on seeded mix in `internal/storage/s3client_integration_test.go`

**Checkpoint**: operator killer feature live; all storage-contract changes integration-covered.

---

## Phase 7: User Story 5 — Previews that understand the payload (P3)

**Goal**: pretty JSON/NDJSON with `p` raw toggle; transparent capped gunzip; hexdump for binary.

**Independent Test**: JSON/gzip/NDJSON/binary fixtures render per contract
preview-rendering.md (obligations 1–6).

- [ ] T044 [P] [US5] RED: gzip units in `internal/preview/gzip_test.go` — golden small payload; high-ratio bomb capped at `Limit` with `Truncated`; `.gz` hint + bad magic → silent raw; gzipped JSON re-classifies to pretty
- [ ] T045 [P] [US5] RED: pretty units in `internal/preview/pretty_test.go` — JSON golden (2-space indent); raw toggle returns byte-identical original; invalid JSON → raw, no error text; NDJSON 3 records in order; one bad line → whole payload raw
- [ ] T046 [P] [US5] RED: hexdump golden (offset+hex+ASCII, non-printables, header summary preserved) in `internal/preview/hex_test.go`
- [ ] T047 [US5] GREEN: `internal/preview/gzip.go` (magic-bytes detect + `io.LimitReader` cap + `Compressed` metadata), `KindJSON`/`KindNDJSON` split + pretty in `internal/preview/text.go`, `internal/preview/hex.go`
- [ ] T048 [P] [US5] RED: toggle UI tests in `internal/ui/preview_toggle_test.go` — `p` flips pretty↔raw in `modeObject`; state resets on next object; no-op for text/image kinds; hint advertised
- [ ] T049 [US5] GREEN: `keys.RawToggle = "p"` in `internal/ui/keys.go` + `modeObject` wiring (`rawPreview` reset on load) in `internal/ui/app.go` + hintbar row in `internal/ui/hintbar.go`

**Checkpoint**: all 5 stories complete.

---

## Phase 8: Polish & Cross-Cutting

- [ ] T050 [P] Update `README.md` — budget config, `Y`/`A`/`H`/`p` keys, health card, export, presigned links (read-only posture note)
- [ ] T051 Full gates: `make fmt vet lint check-readonly test` — zero findings; verify no `Y/A/H/p` conflicts in help output
- [ ] T052 `make test-integration` full matrix (Lima: `DOCKER_HOST` + `TESTCONTAINERS_RYUK_DISABLED=true`) — T017/T034/T043 plus the pending 016 T017/T031 backlog in one run
- [ ] T053 Manual validation per `specs/017-usage-insights-ux/quickstart.md` §Manual (huge-bucket hover quiet, 130×24+NO_COLOR, artifacts paste/run, MPU figures vs `mc ls -I`, gzip/NDJSON/hexdump, log free of presigned URLs) — record results in quickstart.md

---

## Dependencies & Execution Order

```
Phase 1 (T001)
  └─ Phase 2 (T002→T007)                      ← blocks everything
       ├─ Phase 3 US1 (T008→T017)  P1 🎯 MVP
       ├─ Phase 4 US2 (T018→T025)  P1         ← independent of US1
       ├─ Phase 5 US3 (T026→T034)  P2         ← T032 needs T025 (field-copy machinery)
       ├─ Phase 6 US4 (T035→T043)  P2         ← needs Phase 2 dists; T039 partial-labels need T014 (US1 caching);
       │                                         T041 age labels need T023 (relTime, US2)
       └─ Phase 7 US5 (T044→T049)  P3         ← fully independent
            └─ Phase 8 (T050→T053)            ← after all stories
```

Story-level: US2 ∥ US1 (different files). US3 after US2-T025 (menu wires field copy). US4
after US1-T014 (partial-cache semantics it renders) and US2-T023 (relTime for card age
labels). US5 anytime after Phase 2 (actually independent even of it — only shares the repo).

## Parallel Execution Examples

- **Phase 2**: T002 ∥ T003 (two new test files).
- **US1**: T011 ∥ T012 ∥ T013 after T010 (three test files); then T014→T016 sequential (shared `analyze.go`/`keys.go`/`pane.go`).
- **US1 ∥ US2**: one developer/agent on T008–T017, another on T018–T025 — zero file overlap.
- **US2 RED wave**: T018 ∥ T019 ∥ T020 ∥ T021 (four test files).
- **US3**: T026 ∥ T027 ∥ T029 (share tests vs storage tests); T031 after T028/T030.
- **US5**: T044 ∥ T045 ∥ T046; whole phase parallel to US3/US4.

## Implementation Strategy

**MVP = Phase 1 + Phase 2 + Phase 3 (US1)** — ships the cluster-safety fix alone: budgeted
ambient scans, cached partials, explicit `A` full scan. Independently valuable and the reason
this feature exists.

Then increments in priority order: US2 (readability, P1) → US3 (share, P2) → US4 (health
card, P2) → US5 (preview, P3). Each checkpoint leaves `main`-mergeable state: all gates green,
no dangling affordances (keys/hints land in the same phase as their feature).
