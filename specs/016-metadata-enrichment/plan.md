# Implementation Plan: Metadata Enrichment & Inline Usage

**Branch**: `016-metadata-enrichment` | **Date**: 2026-06-09 | **Spec**: `specs/016-metadata-enrichment/spec.md`

**Input**: Feature specification from `/specs/016-metadata-enrichment/spec.md`

## Summary

Surface the rich object metadata the browser already fetches but discards (`HeadObject` at `internal/storage/s3client.go:151-168` returns version id, encryption, replication, archival/restore, object-lock, legal-hold, lifecycle expiration, content-handling headers — none reach the screen) and fold the separate full-screen `analyze` mode (`modeUsage`, `internal/ui/app.go:30`) into the main browse screen's details area. Concretely:

- **US1 (P1)** — Enrich `storage.ObjectMetadata` (`storage.go:187-195`) with the free `HeadObject` fields; render them omit-empty inside the **shared** `metaFieldRows` (`internal/ui/metadata.go:28-37`) — the single source consumed by BOTH the Enter object view (`metaPane`, `metadata.go:42-68`) and the focus pane (`paneTree`, `pane.go:79`, reached on wide layouts via `browseDetailsView`, `pane.go:38-43`) — so a compact pane keeps the footer on-screen; always render core fields and always render permission-gated fields (object-lock, legal-hold) as "unknown".
- **US2 (P1)** — Delete `modeUsage` and its full-screen `usageView`; show bucket/prefix totals inline in the details pane (`internal/ui/pane.go` `paneBucket`/`paneTree`) via a **dwell-gated, generation-guarded, session-cached** background `UsageOf` scan, reusing the existing usage backend (`storage.UsageOf`, `storage.go:123-127`) and the cache pattern (`internal/cache`).
- **US3 (P2)** — Keep the ranked largest-first child breakdown as an expandable section in the same pane (collapse/drill-down), driven by the repurposed key. Because the detail zone's row budget is tight at the supported minimum, the breakdown and the US4 tag/config detail are **mutually exclusive sections** (one detail section visible at a time) — see the layout-budget contract.
- **US4 (P2)** — Add read-only storage methods for object **tag values** and **bucket configuration** sub-resources (versioning, encryption, lifecycle, replication, public-access/policy), each carrying a tri-state status (configured / none / denied / unsupported); loaded lazily on the "more detail" key.
- **US5 (P3)** — Show a non-standard storage-class marker in the listing (`internal/ui/tree.go:224-240`) within the column legibility budget, with the full class recoverable via reveal (`i`).
- **FR-019** — Repurpose the `a` key (freed by deleting `analyze`) from `keys.Analyze` to a context-aware "more detail" trigger: on bucket/prefix focus it expands the breakdown + loads bucket config; on object focus it loads tags + governance. Both the key and the renamed `:detail`/`:info` command-bar entry invoke the SAME `startMoreDetail` dispatcher (one target — they cannot drift).

Technical approach: presentation-only US1/US2/US3/US5 stay entirely in `internal/ui`; US4 adds read methods to the `storage.Storage` read-view interface (no mutating verb, so `check-readonly.sh` stays green) and therefore requires MinIO integration coverage (constitution IV). Every new load runs in a `tea.Cmd` carrying a generation id and is cancellable (constitution II); the inline usage scan owns its own `usageGen` + `usageCancel` (NOT routed through `beginLoad`/`m.gen`) and preserves the existing channel-drain discipline so no producer goroutine leaks. See `research.md`, `data-model.md`, `quickstart.md`, and `contracts/`.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`).

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`) and Lip Gloss v2 (`charm.land/lipgloss/v2`) for the TUI; `github.com/aws/aws-sdk-go-v2/service/s3` (+ `.../service/s3/types`) confined to `internal/storage` only (constitution I; enforced by `scripts/check-readonly.sh`). The UI depends on the `storage.Storage` interface, never the SDK.

**Storage**: S3-compatible object backends (Ceph RGW, MinIO) reached through the `storage.Storage` interface (`internal/storage/storage.go:100-128`). No local database; `internal/cache` is an in-memory, TTL-free, per-session level cache keyed by `(Context, Bucket, Prefix, Search)` (`cache.go:9-14`).

**Testing**: `go test` white-box UI tests (`package ui`) driven by `deliver`/`press`/`newApp`/`treeApp`/`dualApp`/`viewOf` helpers asserting on `App.View().Content`; storage unit tests against the in-memory `storage.Fake`; testcontainers MinIO integration tests (`//go:build integration`, `internal/storage/s3client_integration_test.go`) for the new read-contract methods. Test-first (constitution III).

**Target Platform**: Terminal / TUI (cross-platform; the alt-screen cell renderer).

**Project Type**: Single Go module CLI/TUI (`github.com/danchupin/s3s`), one binary `bin/s3s`.

**Performance Goals**: Non-blocking, 60fps-feel render — the event loop never blocks on I/O (constitution II). The usage scan is O(objects) (a full paginated listing — no cheap native size/count on RGW/MinIO over S3, spec research note) and runs entirely off the UI loop with running-partial progress, cancel-on-navigate, and a dwell gate so rapid list transit spawns no scan.

**Constraints**: The footer/command-hint bar MUST never be scrolled off at any supported terminal width (constitution VI). The actual failure mode of the height budget is NOT footer loss (the footer is composed AFTER the body and the body is hard-capped to `minRows` by `boxViewWith`, `styles.go:348-350`), but **silent truncation** of pane content — clipped rows vanish with no reveal. The plan therefore budgets concretely (below) and gates the detail sections so no identifier is silently clipped. The read-only structural guard (`scripts/check-readonly.sh`) MUST stay green — every new symbol is `Get*`, never matched by the `(Put|Delete|Create|Copy|Upload|Restore|Write)(entity)` ban (`scripts/check-readonly.sh:43-45`); the SDK import stays inside `internal/storage`. All added identifiers fully visible or revealable (`i` reveal, `keys.Reveal`).

**Scale/Scope**: Arbitrary bucket sizes (millions of objects) — the scan stays cancelable and surfaces a running partial total; cached per session keyed by target, invalidated by manual refresh (`r`) and cleared on context switch.

### Concrete height budget (the blocker the draft mis-modelled)

At the supported minimum the layout subtracts (from `View()`, `app.go:1138-1168`):

```
footerH        = strings.Count(m.footerBlock(w), "\n") + 1   (app.go:1138-1139; multi-row in browse modes — NOT 1)
filterFieldH   = 3   in modeBuckets/modeTree   (app.go:1151-1154)
rows           = m.height - footerH - filterFieldH - 2       (app.go:1159, floored at 3)
dataRows       = rows - 2                                     (app.go:1165, floored at 1)
```

The Full-tier details zone (≥130 cols) receives `rows-2` as its body budget (`browseDetailsView(detW-2, rows-2)`, `app.go:1298`); the Dual/Single pane receives `rows-2` (`paneView(paneW-2, rows-2)`, `app.go:1310`). `boxViewWith` HARD-CAPS the rendered body to `minRows` (`styles.go:348-350`), so anything past the budget is **clipped, not reflowed**. A realistic SSE-KMS + versioned + object-lock object renders 6 core rows + 6–9 omit-empty optional rows; stacking US4 tags AND the US3 breakdown into the same zone overflows a 24-row terminal's detail budget. **Design response (enforced, tested):** (a) the detail zone shows AT MOST ONE expandable section at a time — usage breakdown XOR object tags XOR bucket config — toggled by the `MoreDetail` key, never all at once; (b) when even the single section + enriched metadata exceeds the budget, the pane appends a visible `… +N more (i to reveal)` affordance on its last visible line and the clipped fields are recoverable via `keys.Reveal`, so no identifier is *silently* lost (constitution VI); (c) a height-sweep test seeds ALL enriched optional fields + one detail section at 130×24 and asserts every seeded value is either present in `View().Content` OR represented by the reveal affordance.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

- **I. Core/UI Separation** — PASS. US1/US2/US3/US5/FR-019 are presentation-only and stay in `internal/ui`, consuming `storage` types. US4's new read methods live in `internal/storage` (the only SDK-importing package, `s3client.go:16-17`); the UI calls `store.GetObjectTagging`/`store.GetBucketConfiguration` through the interface and consumes the plain `ObjectTags`/`BucketConfig` types. The tri-state classification (configured/none/denied/unsupported) is done in `internal/storage` (via `classify`), never in the render layer.
- **II. Non-Blocking TUI** — PASS, with the producer-drain discipline preserved. Every new retrieval runs in a `tea.Cmd`. The inline usage scan reuses the off-loop pattern (`analyzeCmd`/`waitForUsage` streaming on a buffered(8) channel, `analyze.go:73-98`), but is retargeted to a dedicated `usageGen int` AND a dedicated `usageCancel context.CancelFunc` (NOT `m.gen`/`loadCancel`). Cancellation is `m.usageCancel()` + `usageGen++` performed TOGETHER on every focus move and inside `beginLoad`; messages are stamped with `usageGen` and dropped on `msg.gen != m.usageGen`. CRITICAL: the channel pump (`waitForUsage` re-arm in `onUsageProgress`) keeps DRAINING a live channel REGARDLESS of generation (mirroring the existing `analyze.go:100-108` guard that explicitly is "NOT gated on mode … can never strand the producer on a full channel") — only the *result application* (`usageProg`/cache store) is gated on `usageGen`. Combined with `usageCancel()` (which makes `UsageOf` return promptly and the goroutine `close(ch)`), no producer goroutine leaks under rapid navigation. The dwell gate uses `tea.Tick` (mirroring `bucketTickCmd`/`paneTickCmd`, `app.go:374-400`/`328-357`). Tag/config loads are `tea.Cmd`s carrying a `detailGen`; stale-gen messages are dropped in `Update`.
- **III. Test-First (NON-NEGOTIABLE)** — PASS. A RED list precedes every change. The migration RED set is COMPLETE — it includes every dangling reference the deletion/rename touches, so the build breaks intentionally first: `modeUsage` at `app.go:30`/`219-227`/`881`/`1190-1191`, `analyze.go` (whole `runAnalyze`/`onUsageKey`/`usageView`/`usageTitle`), `command.go:33` (`analyze`/`du`), `command.go:57` (`canOpenCommand` lists `modeUsage`), and the `footer_test.go:194,249` `hintCtx{mode: modeUsage}` test references (no production `footer` change is needed — `footerHints`, `styles.go:511`, does NOT branch on `c.mode`); `keys.Analyze` at `keys.go:21`/`54`, help row, `hintbar.go:52,70`, and `pane.go:54`/`67`/`71`. RED tests: object-metadata render/omit-empty/permission-gated/core; inline-usage dwell/cancel/cache/partial/generation + producer-drain-no-leak; breakdown expand/collapse/drill; tag/config tri-state (incl. an explicit `unsupported` Fake-unit + a `classify`-unit over a synthetic `NotImplemented` code); storage-class marker + reveal-recovery; height/width sweep. See `quickstart.md`.
- **IV. Integration Testing** — PASS (REQUIRED), with the `unsupported` branch honestly scoped. US4 adds `GetObjectTagging`/`GetBucketConfiguration` to the storage-client contract, which constitution IV mandates covering against a real backend. New `//go:build integration` tests in `internal/storage/s3client_integration_test.go` seed via the raw SDK seed client and assert: tag KV pairs; a `configured` versioning/encryption state; a `none` for an unconfigured sub-resource (MinIO returns the `*NotFound`/`*NotConfiguration` family → mapped to `none`); a `denied` for a policy-denied read; and partial success when one sub-resource fails while the rest load. The `unsupported` state CANNOT be produced against MinIO (it implements every sub-resource and returns not-configured codes, never a method-not-implemented error). It is therefore covered by (a) a `Fake` unit test (`UnsupportedGetConfigs` map → `State=="unsupported"`) and (b) a `classify`-unit test feeding a synthetic `smithy.APIError` with code `NotImplemented` (and HTTP 501 / `MethodNotAllowed`) asserting `errors.Is(classify(err), ErrUnsupported)` — documented in `quickstart.md`/`contracts/storage-read-extension.md` as the explicit home for the riskiest branch. US1's enriched `HeadObject` mapping changes no contract shape (adds fields populated from the existing call) and is covered by extending the existing `HeadObject` integration assertion.
- **V. Observability & Safe Operations** — PASS. No destructive action (read-only feature, FR-014). The new `ErrUnsupported` sentinel and any `ConfigItem.Reason`/`Detail` carry only sentinel/code/summary text, never SDK response bodies (mirroring `classify`, which logs only `code`/`status`/`message`, `s3client.go:275-281`). Structured logging stays file-only; no new secret surfaces.
- **VI. UI Legibility** — PASS, on a corrected budget. The object pane omits absent optional fields (FR-003); `metaFieldRows` is the single omit-empty source so BOTH the Enter view and the focus pane stay compact. The concrete budget (above) is honored by the one-section-at-a-time gate plus the `… +N more (i to reveal)` affordance, verified by a height sweep at 130×24 that asserts every seeded value is present OR revealable (NOT merely that the footer is present — that always passes because the body is hard-capped). The non-standard storage-class marker fits the 5-char `type` column with a fixed lossy token; the FULL class is recoverable via `keys.Reveal` on the row (the listing-storage-class contract ties the marker to reveal). Long values (KMS key ARNs, lifecycle dates) remain revealable. State cues (none / unknown-denied / unsupported, partial) are text labels, distinguishable under `NO_COLOR` (`styles.go` text-carries-state convention).
- **VII. UI Consistency & Design System** — PASS. All new rows reuse `metaRow`/`metaFieldRows`/`colHeadStyle` and the established palette roles (`accentStyle`, `dimCellStyle`, `warnStyle`); the breakdown reuses `usageBar`/share + `renderTable`. The `a` key is repurposed (rebind `keys.Analyze` → `keys.MoreDetail`, binding stays `"a"`) — no new hue, no new keymap entry, no parallel prompt/label vocabulary. The rename propagates to every on-screen hint automatically via `keyHint`/`firstBind` (`keys.go:101-113`) once `pane.go:54/67/71` + `hintbar.go:52/70` are migrated to `k.MoreDetail` with the `detail`/`info` label. The `:detail`/`:info` command-bar entry and the `a` key share ONE invoke target (`startMoreDetail`).
- **Read-only structural posture (implementation invariant, NOT a principle)** — PASS, with corrected framing. `internal/storage` is NOT a write-free package — it already carries the `Mutator` write surface (`storage.go:54-98`: CreateFolder/RemoveObject/UploadFile/CopyKey/MoveObject/DeleteRecursive/RemoveBucket) and the private `s3API` interface already lists `PutObject/DeleteObject/CopyObject/DeleteBucket` (`s3client.go:28-31`). Read-only is a POSTURE enforced by `check-readonly.sh`'s verb+entity regex, which the `Mutator` names deliberately dodge. This feature extends only the **read-view** of the `Storage` interface; every new symbol is `Get*` (no `Put|Delete|Create|Copy|Upload|Restore|Write`), so the guard regex (`scripts/check-readonly.sh:43`) never matches even where UI code references the methods, and the SDK import stays inside `internal/storage` (excluded at `check-readonly.sh:21`). `make check-readonly` stays green. No constitution amendment is required (additive read methods + UI consolidation).

**Initial gate: PASS. Post-design re-check: PASS. No violations to track.**

## Project Structure

### Documentation (this feature)

```text
specs/016-metadata-enrichment/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── object-metadata-pane.md
│   ├── storage-read-extension.md
│   ├── inline-usage.md
│   ├── more-detail-key.md
│   ├── listing-storage-class.md
│   └── layout-budget.md
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/storage/                     # the ONLY package importing aws-sdk-go-v2/service/s3
├── storage.go                        # +enriched ObjectMetadata fields (187-195); +ObjectTags,
│                                     #  BucketConfig, ConfigItem/ConfigState tri-state types;
│                                     #  +ErrUnsupported sentinel (near 19-44);
│                                     #  +GetObjectTagging/GetBucketConfiguration on the Storage
│                                     #  read-view interface (100-128)
├── s3client.go                       # +s3API members (GetObjectTagging/GetBucketVersioning/
│                                     #  GetBucketEncryption/GetBucketLifecycleConfiguration/
│                                     #  GetBucketReplication/GetPublicAccessBlock/GetBucketLocation,
│                                     #  near 23-32); extend HeadObject mapping (151-168);
│                                     #  +GetObjectTagging/GetBucketConfiguration impls (each
│                                     #  sub-resource classified independently); extend classify —
│                                     #  *NotFound/*NotConfiguration → "none", NotImplemented/501/
│                                     #  MethodNotAllowed → ErrUnsupported (231-283)
├── fake.go                           # +FakeObject optional-metadata + per-field deny flags;
│                                     #  +FakeBucket.BucketConfig + FailGetTags + UnsupportedGetConfigs;
│                                     #  +Fake.GetObjectTagging/GetBucketConfiguration
├── fake_test.go                      # unit tests for the new Fake methods + each tri-state incl.
│                                     #  the unsupported branch (UnsupportedGetConfigs)
├── classify_unit_test.go             # ErrUnsupported mapping unit over synthetic NotImplemented/
│                                     #  501/MethodNotAllowed; *NotFound family → none (NOT unsupported)
├── usage_test.go                     # reused (UsageOf totals/ranking) for inline assertions
└── s3client_integration_test.go      # +tag/config integration: configured/none/denied/partial
│                                     #  (unsupported is Fake+classify units only — MinIO can't yield it)

internal/ui/                          # depends on storage interface only (constitution I)
├── app.go                            # DELETE modeUsage (30) + usage fields (219-227) + Update
│                                     #  usageProgress/usageDone cases + onKey modeUsage (881) +
│                                     #  View modeUsage case (1190-1191); ADD usageExpanded,
│                                     #  usageResults cache, usageGen, usageCancel, usageScanKey,
│                                     #  usageProg, usageCh, detailSection, objectTags, bucketCfg,
│                                     #  detailKey, detailGen; cancel usage in beginLoad
│                                     #  (m.usageCancel + usageGen++); usage tick handler near
│                                     #  onBucketTick (392-400)/onPaneTick (344-357); refresh +
│                                     #  onContextResolved clear/invalidate usageResults
├── analyze.go                        # REPURPOSE: drop modeUsage entry/runAnalyze/onUsageKey/
│                                     #  usageView/usageTitle; keep usageTarget + analyzeCmd/
│                                     #  waitForUsage; add loadUsage(usageGen, usageCancel-ctx);
│                                     #  onUsageProgress keeps DRAINING ungated (gate result only)
├── pane.go                           # paneBucket (45-56) + paneTree (58-96) + browseDetailsView
│                                     #  (38-43): inline totals + omit-empty enriched metaFieldRows +
│                                     #  ONE expandable detail section (breakdown XOR tags XOR config)
│                                     #  + "… +N more (i to reveal)" affordance; rename keys.Analyze
│                                     #  hints (54/67/71) → keys.MoreDetail "detail"
├── metadata.go                       # metaFieldRows (28-37): core block + omit-empty optional block
│                                     #  (SHARED by metaPane + paneTree); +omitEmpty(label,value,gated)
├── tree.go                           # listing storage-class marker in the type column (224-240);
│                                     #  refresh() (142-149) also Invalidate(usageResults key)
├── keys.go                           # rename Analyze→MoreDetail (21, 54); help row
├── command.go                        # `analyze`/`du` (33) → `detail`/`info`, invoke=startMoreDetail;
│                                     #  drop modeUsage from canOpenCommand (57)
├── hintbar.go                        # k.Analyze→k.MoreDetail, "detail" label/avail per FR-019
│                                     #  (52, 70); refreshBuckets (175) also Invalidate(usageResults)
├── styles.go                         # footerHints unchanged (no mode branch); action catalog
│                                     #  membership follows the rename (no new role)
├── messages.go                       # usageProgressMsg/usageDoneMsg gen field = usageGen;
│                                     #  +usageTickMsg{gen,bucket,prefix}; +objectTagsMsg/bucketConfigMsg
│                                     #  carrying detailGen
├── commands.go                       # +usageTickCmd dwell helper (tea.Tick like spinnerTick);
│                                     #  +loadObjectTags/loadBucketConfig cmds
└── *_test.go                         # object_metadata_test, inline_usage_test, inline_breakdown_test,
                                      #  object_tags_test/bucket_config_test, storage_class_marker_test,
                                      #  metadata_legibility_test; migrate analyze_test + spec013/app_test

internal/cache/                       # generic Cache[V]; instantiate Cache[*storage.UsageReport]
└── cache.go                          # NO change — reuse Key + Get/Put/Invalidate/InvalidateBucket/Clear
```

**Structure Decision**: Single Go module, no new packages. All UI work lands in `internal/ui`; the only cross-package surface is the read-view extension of `internal/storage` (interface + s3client + fake). `internal/cache` is reused unchanged via a second `Cache[*storage.UsageReport]` instance. This keeps the SDK boundary (constitution I) and the read-only guard intact.

## Complexity Tracking

> No constitution violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    | —          | —                                    |

