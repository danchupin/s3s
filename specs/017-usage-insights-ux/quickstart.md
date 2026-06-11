# Quickstart: Budgeted Usage, Insights & Details UX (017)

TDD entry point. Per-story RED sets (write failing first — constitution III), then the
verification path.

## Commands

```bash
make test                 # unit (Fake storage, white-box ui)
make test-integration     # MinIO (Lima: DOCKER_HOST=… TESTCONTAINERS_RYUK_DISABLED=true)
make fmt vet lint check-readonly
go test ./internal/ui/ -run TestBudget        # focused
go test ./internal/share/ ./internal/preview/ # new/extended pure packages
```

## US1 — Budgeted scan (P1)

RED (the behaviour inversions are the heart of this feature):

1. `internal/storage`: `UsageOf` cap unit — Fake seeded budget+1 → `Bounded=true`, pages ≤
   cap; boundary (count==budget) → exact. The signature change (`maxObjects`) breaks
   `fake.go`, `analyze.go`, existing usage tests — that compile break IS the first RED.
2. `internal/ui`: `onUsageDone` partial-caching test — cancel mid-scan → cache hit with `≥`
   on revisit (inverts `inline_usage_resilience_test.go` discard expectations — update them
   deliberately).
3. `startMoreDetail` no-unbounded-scan test (Fake page counter).
4. `A`/`:scan` single-dispatcher test (mirrors 016 `a`/`:detail` pattern).
5. `budget=0` → no ambient arm.
6. Config: `usageScanBudget` parse/validate/default unit (`internal/config`).

GREEN order: storage cap → config knob → ui plumb (armUsageScan budget, startFullScan,
onUsageDone caching, pane `≥`+affordance).

## US2 — Details-pane groups (P1)

RED: grouped-render test (headers/order/empty-group-omission); 6-state matrix incl. NO_COLOR;
multipart-ETag annotation; dual-date with injected `now`; per-field copy payload; 130×24
height sweep (extend `metadata_legibility_test.go` pattern).

GREEN: `metaFieldRows` regroup → `relTime` → states → annotation → field-select copy.

## US3 — Copy & share (P2)

RED: `internal/share` builder table units (URI/URL escaping, path-vs-vhost, snippets,
CSV/JSON goldens incl. `bounded`); menu item matrix per focus; TTL picker presets; cred-expiry
warn; export file naming + failure path; presign log-redaction unit; storage `PresignGet`
shape unit (Fake, zero backend calls).

GREEN: `internal/share` pure pkg → `storage.PresignGet` → `copymenu.go` overlay → export cmd.

## US4 — Health card (P2)

RED: card render from seeded distributions; partial `≥` labelling; MPU 6-state matrix
(loading/none/present/denied/unsupported/error — denied NEVER renders as zero); small-object
warning threshold tests; 130×24 sweep + collapse order; Esc-restore; `H` budgeted-only scan;
storage `ListIncompleteUploads` units (cap-100 sizing, honest zero, tri-state).

GREEN: storage MPU method+Fake → usageAgg distributions → `modeHealth` view → probe wiring →
warnings.

## US5 — Preview upgrades (P3)

RED: pretty/raw goldens; NDJSON; invalid→silent raw; gzip golden + bomb cap + re-classify
(gzipped JSON); hexdump golden; `p` toggle reset semantics.

GREEN: `preview/gzip.go` → Kind split + pretty → `hex.go` → modeObject toggle.

## Integration (constitution IV — after units green)

`internal/storage/s3client_integration_test.go`: cap honored; distributions exact;
MPU seed (`CreateMultipartUpload`+`UploadPart`, NO complete — seeder stays inside
`internal/storage`); presign fetched by plain `http.Get`. Matrix in
`contracts/storage-read-extension.md`.

## Validation status (2026-06-11)

- Unit suites: ALL GREEN (`make test` — storage/share/preview/config/ui incl. budget,
  partial-cache, groups, copy-menu, presign-redaction, health-card sweeps, preview
  toggles). Gates: fmt/vet/lint/check-readonly GREEN.
- Integration (MinIO via Lima Docker): ALL GREEN in 13s — cap honored, distributions,
  incomplete-MPU (exact-key — see the MinIO quirk note in
  contracts/storage-read-extension.md), presign plain-http fetch + tamper rejection,
  plus the 016 backlog (tags/config).
- Manual validation below: PENDING — needs a human at a 130×24 terminal + an RGW
  endpoint for the prefix-wide MPU check.

## Manual validation (release gate)

1. Hover a huge bucket → pane shows `≥` within ~2 s, network quiet after; `A` runs full scan
   with live totals; navigate away mid-scan → return shows `≥` (nothing discarded).
2. 130×24 + NO_COLOR: pane groups, card, all states readable; footer always present.
3. `Y` on an object → all artifacts paste correctly (URL opens, presigned link works in
   curl, snippet runs); export file lands in DownloadDir and opens.
4. `H` on a real bucket with dangling MPUs (seed via `mc`/aws cli) → counts/age/size match.
5. Preview a gzipped NDJSON log → pretty; `p` → raw; binary → hexdump.
6. Log file: no presigned URL/signature anywhere.
