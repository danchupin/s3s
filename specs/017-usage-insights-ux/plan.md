# Implementation Plan: Budgeted Usage, Insights & Details UX

**Branch**: `017-usage-insights-ux` | **Date**: 2026-06-11 | **Spec**: `specs/017-usage-insights-ux/spec.md`

**Input**: Feature specification from `/specs/017-usage-insights-ux/spec.md`

## Summary

Five user stories on one theme — give maximum information about buckets/objects while taking the
cluster-hostile behaviour OUT of the ambient path:

- **US1 (P1) Budgeted scan** — the dwell-triggered inline usage scan (`armUsageScan`,
  `internal/ui/analyze.go:74-95` → `storage.UsageOf`, `internal/storage/s3client.go:242-272`)
  becomes **bounded**: `UsageOf` gains a `maxObjects` cap (default budget 20 000 enumerated
  objects, configurable, `0` config value = ambient off). Hitting the cap yields a
  lower-bound report (`Bounded=true`), which IS cached (today `onUsageDone`,
  `analyze.go:179-191`, discards anything incomplete). The full scan moves behind its own
  explicit action (`A` key / `:scan`), streams progress, stays cancellable, and a cancelled
  full scan also caches its progress as a lower bound. `a`/`:detail` (breakdown) NEVER starts
  unbounded work anymore (today `startMoreDetail`, `analyze.go:224-231`, silently launches a
  full scan — that branch is re-pointed at the budgeted scan).
- **US2 (P1) Details-pane readability** — `metaFieldRows` (`internal/ui/metadata.go:35-60`)
  reorganized into named groups (identity & content / security & governance / delivery / user
  metadata) with `colHeadStyle` headers; dates render relative + exact; denied/none/unknown
  visually distinct via palette roles + text (NO_COLOR-safe); multipart ETag (`-N` suffix)
  explained; per-field copy via a field-select reveal flow reusing the OSC52 path
  (`internal/ui/reveal.go`).
- **US3 (P2) Copy & share** — one `Y` copy menu (focus-aware): S3 URI, style-aware HTTPS URL,
  download-command snippet, presigned GET (TTL presets 15m/1h/24h/7d via
  `s3.NewPresignClient` — client-side, no network, never logged), report export CSV/JSON into
  the existing `DownloadDir`. Pure string builders live in a new UI-agnostic `internal/share`
  package (constitution I); presign lives in `internal/storage` (SDK boundary).
- **US4 (P2) Health card** — new full-screen `modeHealth` (`H` / `:health`) for a
  bucket/prefix: age/size/storage-class distributions accumulated **during the same
  enumeration** (extend `usageAgg`) plus an incomplete-multipart-uploads probe
  (`ListMultipartUploads`, lazily loaded, tri-state semantics reused from 016's
  `ConfigState`). Partial data ⇒ every figure labelled as lower bound.
- **US5 (P3) Preview upgrades** — `internal/preview` gains JSON/NDJSON pretty-print (raw
  toggle `p`), transparent gzip decompression (magic-bytes detection, decompressed output
  capped at the existing `preview.Limit` = 5 MiB, `internal/preview/text.go:15`), and a hex
  dump for binary payloads.

Storage-contract changes (constitution IV ⇒ MinIO integration tests): `UsageOf` signature gains
the cap + distribution output; new `ListIncompleteUploads`; new `PresignGet`. All new symbols
use `List*`/`Presign*` verbs — the read-only guard regex (`scripts/check-readonly.sh:43-45`:
`(Put|Delete|Create|Copy|Upload|Restore|Write)(entity)`) never matches, and the SDK stays inside
`internal/storage`. See `research.md` (decisions), `data-model.md`, `contracts/`, `quickstart.md`.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`).

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2
(`charm.land/lipgloss/v2`); `github.com/aws/aws-sdk-go-v2/service/s3` (+ `types`, + the
`s3.NewPresignClient` presigner from the same module — no new dependency) confined to
`internal/storage`. `compress/gzip`, `encoding/json`, `encoding/hex`, `encoding/csv` from the
standard library for US3/US5 — no new third-party modules.

**Storage**: S3-compatible backends (Ceph RGW, MinIO) via the `storage.Storage` read-view
interface (`internal/storage/storage.go:106-143`). `internal/cache` reused: the existing
`Cache[*storage.UsageReport]` instance now also stores bounded (partial) reports;
`Cache[*storage.IncompleteUploads]` is a new instance keyed by the same `(ctx,bucket,prefix)`.

**Testing**: White-box `package ui` tests (`deliver`/`press`/`viewOf` helpers, assertions on
`App.View().Content`); `storage.Fake` units for budget/caps/tri-states; `//go:build
integration` MinIO tests for every storage-contract change (cap honored, distributions,
incomplete-MPU listing, presigned URL actually fetchable). Test-first (constitution III).

**Target Platform**: Terminal/TUI, cross-platform (alt-screen cell renderer).

**Project Type**: Single Go module CLI/TUI (`github.com/danchupin/s3s`), one binary `bin/s3s`.

**Performance Goals**: Ambient background work per hover is hard-capped at the budget
(default 20 000 objects ≈ 20 listing pages ≈ ~2 s on a healthy backend) — SC-001/SC-002.
Full scans remain O(objects), off the UI loop, progress-streaming, cancellable
(constitution II). Distributions add O(1) work per enumerated object inside the existing
aggregation loop — no extra requests (FR-020). Presign is client-side only (no request).

**Constraints**: Footer/hint bar never scrolled off at any tier (constitution VI) — the health
card is a NEW full-screen mode precisely so the inline pane's height budget
(`boxViewWith` hard-cap, `internal/ui/styles.go:348-350`; ~11 detail rows at 130×24) is not
re-fought; the card composes its own body under the same `View()` budget arithmetic
(`app.go:1138-1168`) and collapses sections before clipping any value. The read-only guard
stays green (verb analysis above). Presigned URLs are bearer capabilities: never logged
(constitution V) — log only `{op:"presign", bucket, key, ttl}`.

**Scale/Scope**: Buckets of arbitrary size (100M+ objects): ambient cost bounded by budget;
full scan explicit-only. Incomplete-MPU probe paginates; per-upload size enrichment via
`ListParts` is capped at the first 100 uploads (beyond: count + oldest age only, sizes marked
unknown) — see research.md D6.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

- **I. Core/UI Separation** — PASS. New domain logic lands in UI-agnostic packages:
  budget/distribution aggregation in `internal/storage` (`usageAgg`); link/snippet builders +
  CSV/JSON report serialization in the new pure `internal/share` package (no SDK, no Bubble
  Tea — unit-tested headless); payload transforms (pretty JSON, gunzip, hexdump) in
  `internal/preview`. Presign requires SDK credentials machinery ⇒ `internal/storage`
  (`PresignGet` on the read view). The UI only dispatches intents and renders returned
  strings/structs.
- **II. Non-Blocking TUI** — PASS. Budgeted and full scans reuse the 016 channel discipline
  unchanged (`analyzeCmd`/`waitForUsage`, `internal/ui/analyze.go:134-160`: producer always
  reaches its terminal send; consumer drains regardless of generation; results gated on
  `usageGen`). The cap changes only WHEN `UsageOf` returns, not the concurrency shape. The
  MPU probe is a new `tea.Cmd` under `healthGen` + its own cancel (mirror of `detailGen`,
  `analyze.go:236-249`); cancel-on-navigate preserved. `PresignGet` runs in a `tea.Cmd` too:
  the external credential command (014) may block, so even "client-side" work stays off the
  loop. Export file write runs in a `tea.Cmd` (mirrors download, 005).
- **III. Test-First (NON-NEGOTIABLE)** — PASS. RED sets enumerated per story in
  `quickstart.md`; the behaviour-change RED set is explicit: `onUsageDone` MUST cache
  partial reports (inverts `analyze.go:185-187` — existing
  `inline_usage_resilience_test.go` assertions on discard flip), `startMoreDetail` MUST NOT
  launch an unbounded scan (`analyze.go:224-231`), `armUsageScan` budget plumb-through, and
  the `UsageOf` signature change breaks `storage.Fake`/tests intentionally first.
- **IV. Integration Testing** — PASS (REQUIRED). Storage-contract deltas and their MinIO
  coverage: (1) `UsageOf` cap — seed > budget objects, assert enumeration stops at cap,
  `Bounded=true`, totals = lower bound; (2) distributions — seed known age/size/class mix,
  assert buckets (MinIO supports STANDARD + REDUCED_REDUNDANCY classes for the class axis);
  (3) `ListIncompleteUploads` — seed via raw SDK `CreateMultipartUpload`+`UploadPart`
  WITHOUT complete (writes allowed only there: seed client inside
  `internal/storage/s3client_integration_test.go`), assert count/oldest/size-enrichment and
  pagination; (4) `PresignGet` — generate URL, plain `http.Get` MUST return the object bytes
  (no SDK on the fetch side), expired-TTL URL MUST be rejected by the backend. Denied
  variants reuse the 016 policy-denial harness. `unsupported` for MPU listing is untestable
  on MinIO ⇒ `Fake` unit + `classify` unit (same split 016 used for bucket config).
- **V. Observability & Safe Operations** — PASS. No destructive ops. Presigned URL = bearer
  secret: the URL string is NEVER logged (log records `op/bucket/key/ttl` only) and is not
  echoed into the slog file by any error path (the storage error wraps sentinel classes
  only, `storage.classify`). Export logs `{op:"export", path, format, rows}`. Clipboard
  writes (OSC52) remain best-effort and unlogged. No new secret sources.
- **VI. UI Legibility** — PASS. Lower-bound state is a TEXT marker ("≥", "partial"), not a
  colour; denied/none/unknown/unsupported keep text labels (NO_COLOR-safe). Health-card
  figures and histogram labels render full values or reveal via `i` (reused `keys.Reveal`).
  The card keeps the footer visible at 130×24 (height-sweep test mirrors 016's
  `metadata_legibility_test.go`). Per-field copy makes long values (KMS ARNs, ETags) usable,
  strengthening VI.
- **VII. UI Consistency & Design System** — PASS. Copy menu reuses the list-overlay pattern
  (connections manager) + shared prompt vocabulary; TTL picker is the same overlay one level
  deeper. New keys (`Y`, `A`, `H`, `p`) registered in `keyMap`/`defaultKeys`
  (`internal/ui/keys.go:43-74`), surfaced via `keyHint`/`formatKeys` (no hardcoded
  literals), added to help + hintbar. Histograms reuse `usageBar` (`analyze.go:279-287`) and
  palette roles; no new hues. Commands `:scan`/`:health`/`:copy` registered in the
  command-bar catalog (`command.go`) and invoke the SAME dispatchers as the keys (FR-019
  pattern from 016).
- **Read-only structural posture (invariant, NOT a principle)** — PASS. New interface
  symbols: `PresignGet`, `ListIncompleteUploads` (+ internal `ListParts`,
  `ListMultipartUploads` SDK calls inside `internal/storage`). Guard regex requires verb ∈
  {Put,Delete,Create,Copy,Upload,Restore,Write} fused to an entity: `Presign*`/`List*` never
  match; `IncompleteUpload`/`IncompleteUploads` contains no `\b`-anchored banned verb
  (`Upload` sits mid-word). UI never references SDK symbol names
  (`CreateMultipartUpload` appears ONLY in `internal/storage`, excluded at
  `check-readonly.sh:21-27`). `make check-readonly` stays green.

**Initial gate: PASS. Post-design re-check (after Phase 1 artifacts): PASS. No violations to
track.**

## Project Structure

### Documentation (this feature)

```text
specs/017-usage-insights-ux/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── budgeted-usage-scan.md
│   ├── details-pane-groups.md
│   ├── copy-share-menu.md
│   ├── health-card-view.md
│   ├── storage-read-extension.md
│   └── preview-rendering.md
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/storage/                  # the ONLY package importing aws-sdk-go-v2/service/s3
├── storage.go                     # UsageOf signature: +maxObjects cap; UsageReport: +Bounded,
│                                  #  +AgeDist/SizeDist/ClassDist; +IncompleteUploads/IncompleteUpload
│                                  #  types (tri-state via ConfigState reuse); +ListIncompleteUploads,
│                                  #  +PresignGet on the Storage read view
├── s3client.go                    # UsageOf: cap enforcement + distribution accumulation in usageAgg;
│                                  #  ListIncompleteUploads (ListMultipartUploads pagination +
│                                  #  ListParts size enrichment, first-100 cap); PresignGet via
│                                  #  s3.NewPresignClient (client-side; URL never logged)
├── usage.go / usage_agg           # usageAgg: histogram buckets (age 6 / size 6 / class map) —
│                                  #  O(1) per object, single pass (file split if usageAgg grows)
├── fake.go                        # Fake: cap honoring + distributions; FakeIncompleteUploads per
│                                  #  bucket + deny/unsupported toggles; PresignGet fake (deterministic
│                                  #  URL embedding bucket/key/ttl for assertions)
├── *_test.go                      # budget-cap unit, distribution unit, MPU tri-state unit,
│                                  #  presign-shape unit (+ classify reuse)
└── s3client_integration_test.go   # +cap honored; +distributions; +incomplete-MPU (seed CreateMPU/
                                   #  UploadPart w/o complete); +presign fetch-with-plain-http

internal/share/                    # NEW pure package (constitution I): no SDK, no Bubble Tea
├── share.go                       # S3URI, HTTPURL(endpoint, pathStyle, bucket, key),
│                                  #  CLISnippet/CurlSnippet builders (escaping!)
├── export.go                      # UsageReport/HealthCard → CSV bytes / JSON bytes
└── share_test.go, export_test.go  # table-driven units (escaping, path-vs-vhost, partial labels)

internal/preview/
├── text.go                        # Kind: +KindJSON/KindNDJSON discrimination (Classify);
│                                  #  PrettyJSON/PrettyNDJSON (raw fallback on parse error)
├── gzip.go                        # NEW: magic-bytes detect (1f 8b) + capped gunzip (Limit out)
├── hex.go                         # NEW: hexdump payload builder (offset+hex+ASCII)
└── *_test.go                      # pretty/raw, bomb-cap, hexdump goldens

internal/config/
└── config.go                      # Config: +UsageScanBudget *int (nil→20000 default; 0→ambient off)
                                   #  +HealthSmallObjectKiB (default 128), +HealthSmallObjectShare
                                   #  (default 0.5); validation + tests

internal/ui/
├── analyze.go                     # armUsageScan→budgeted loadUsage(maxObjects=budget);
│                                  #  onUsageDone CACHES partial (Bounded/cancelled) reports;
│                                  #  startMoreDetail: budgeted scan only + full-scan affordance;
│                                  #  startFullScan (A/:scan) = maxObjects 0 under usageGen
├── health.go                      # NEW: modeHealth view (histogram rows via usageBar), healthGen +
│                                  #  MPU-probe cmd/msg handlers, small-object warning line,
│                                  #  partial labelling, Esc-return via prevMode pattern (app.go:130)
├── copymenu.go                    # NEW: Y overlay (focus-aware items), TTL sub-pick, OSC52 emit +
│                                  #  reveal-popup fallback, footer confirmation strings
├── fieldcopy.go                   # NEW: field-select copy state machine (US2 machinery;
│                                  #  user-facing menu entry wired in US3 copymenu.go)
├── metadata.go                    # metaFieldRows → grouped sections w/ colHeadStyle headers;
│                                  #  relTime(now,t) dual dates; multipart ETag annotation;
│                                  #  field-select copy hook
├── pane.go                        # partial "≥" rendering + full-scan affordance line in details pane
├── app.go                         # +modeHealth (mode iota, app.go:25-33); key routing Y/A/H/p;
│                                  #  budget from config through App constructor; usageResults now
│                                  #  stores partial; refresh/context-switch invalidation extended to
│                                  #  the MPU cache
├── keys.go                        # +CopyMenu("Y"), +FullScan("A"), +Health("H"), +RawToggle("p");
│                                  #  help rows
├── command.go                     # +:scan/:health/:copy → same dispatchers as keys
├── hintbar.go                     # hints for the new actions per mode/focus
├── messages.go / commands.go      # +incompleteUploadsMsg(healthGen), +presignDoneMsg, +exportDoneMsg,
│                                  #  +clipboard cmd reuse; full-scan progress reuses usage msgs
└── *_test.go                      # budget/partial-cache/affordance, grouped-pane, copy-menu,
                                   #  health-card (incl. 130×24 sweep + NO_COLOR), preview toggles

scripts/check-readonly.sh          # NO change — verb analysis in contracts/storage-read-extension.md
```

**Structure Decision**: Single Go module; ONE new package `internal/share` (pure builders +
export serialization — keeps domain strings out of the TUI layer per constitution I, reusable
by a future plain CLI). Everything else extends existing packages along the 016 seams:
storage read-view + Fake + integration tests; preview transforms; UI files split by surface
(`health.go`, `copymenu.go`) mirroring the existing per-surface layout (`analyze.go`,
`reveal.go`).

## Complexity Tracking

> No constitution violations. Table intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    | —          | —                                    |
