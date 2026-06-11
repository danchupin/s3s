# Data Model: Budgeted Usage, Insights & Details UX (017)

Phase 1 output. Entities, fields, relationships, validation, and state transitions. Types live
in `internal/storage` (contract types), `internal/config` (knobs), `internal/share` (pure
artifacts), `internal/preview` (payload kinds), `internal/ui` (view state). No new persistence:
everything is session-scoped except config (YAML) and exported report files.

## 1. Scan Budget (config)

| Field | Type | Default | Rules |
|---|---|---|---|
| `Config.UsageScanBudget` | `*int` (YAML `usageScanBudget`) | `nil` → 20 000 | `nil` = default; `0` = ambient scanning disabled (explicit-only); `<0` = validation error |
| `Config.HealthSmallObjectKiB` | `int` (YAML `healthSmallObjectKiB`) | `0` → 128 | `<0` = validation error |
| `Config.HealthSmallObjectShare` | `float64` (YAML `healthSmallObjectShare`) | `0` → 0.5 | outside `(0,1]` (except 0=default) = validation error |

Resolved once at startup; passed into `ui.App` as plain values (UI never reads config files).

## 2. UsageReport (extended) — `internal/storage`

Existing (`storage.go:194-201`): `Bucket`, `Prefix`, `TotalSize`, `TotalCount`, `Children`,
`Complete`.

| New field | Type | Meaning |
|---|---|---|
| `Bounded` | `bool` | enumeration stopped at the `maxObjects` cap — totals are a lower bound |
| `ScanStart` | `time.Time` | age-histogram reference point (set by `usageAgg`; injectable in Fake) |
| `AgeDist` | `[6]DistBucket` | `<1d, 1–7d, 7–30d, 30–90d, 90–365d, >1y` |
| `SizeDist` | `[6]DistBucket` | `<128KiB, 128KiB–1MiB, 1–16MiB, 16–128MiB, 128MiB–1GiB, ≥1GiB` |
| `ClassDist` | `map[string]DistBucket` | key = reported storage class, `""` normalized to `STANDARD` |

```go
type DistBucket struct {
    Count int   // objects in the bucket
    Size  int64 // bytes in the bucket
}
```

**Completeness semantics** (drives every "≥" marker):

| `Complete` | `Bounded` | Meaning | Cacheable | Rendered as |
|---|---|---|---|---|
| `true` | `false` | exact result | yes | plain totals |
| `false` | `true` | stopped at budget | yes (NEW) | `≥` lower bound + full-scan affordance |
| `false` | `false` | cancelled mid-scan | yes (NEW) | `≥` lower bound + full-scan affordance |
| `true` | `true` | — | invalid state (never constructed) | — |

**Validation**: `TotalCount == Σ AgeDist.Count == Σ SizeDist.Count == Σ ClassDist.Count`;
`TotalSize` likewise — asserted in storage unit tests (single-pass aggregation invariant).

**Transitions** (per `(context,bucket,prefix)` cache key):
`absent → bounded/cancelled (partial)` → overwritten by → `exact`; any state → evicted by
manual refresh (`r`), context switch, or `usageResults.Clear()`. Exact is never overwritten by
a partial (UI only launches scans for absent-or-partial targets).

## 3. IncompleteUploads / IncompleteUpload — `internal/storage`

```go
type IncompleteUploads struct {
    Bucket, Prefix  string
    State           ConfigState // reuse 016 tri-state: configured(=present)/none/denied/unsupported
    Count           int         // total in-progress uploads found (all pages)
    OldestInitiated time.Time   // zero when Count == 0
    TotalSize       int64       // Σ part sizes over the first SizedCount uploads
    SizedCount      int         // how many uploads were size-enriched (≤ sizing cap 100)
}
```

- `State == ConfigNone` ⇔ `Count == 0` and the listing call succeeded — an HONEST zero (exact,
  never a fallback for denied/unsupported; spec FR-022).
- `SizedCount < Count` ⇒ card renders `TotalSize` as `≥` with "(N of M sized)".
- One sub-entity per upload is NOT exposed (card shows aggregates only); per-upload rows are a
  future iteration.

**Relationship**: cached in a dedicated `cache.Cache[*storage.IncompleteUploads]` keyed by the
same `(context,bucket,prefix)` `cache.Key` as `usageResults`; invalidated together with it.

## 4. HealthCard (view aggregate) — `internal/ui`

Not a stored type — composed at render time from:

| Source | Part |
|---|---|
| `UsageReport` (cache) | totals, exact/lower-bound state, AgeDist, SizeDist, ClassDist |
| `IncompleteUploads` (cache or in-flight probe) | MPU block (count/size/age or denied/unsupported/loading) |
| config knobs | small-object warning: fires when `share(Size < KiB·1024) > Share` over `SizeDist` |

View state on `App`: `healthGen int`, `healthCancel context.CancelFunc`, `healthTarget
cache.Key` — lifecycle identical to 016's `detailGen` (`internal/ui/analyze.go:236-249`):
bump+cancel on entry/exit/navigate; stale-gen messages dropped.

**Mode transition**: `modeBuckets/modeTree --H/:health (bucket|prefix focus)--> modeHealth
--Esc--> prevMode` (existing `prevMode` machinery, `app.go:130,863-868`). Entering does NOT
start a full scan (FR-003): absent data ⇒ budgeted scan only + affordance.

## 5. Share Artifact — `internal/share` (pure values)

| Builder | Input | Output |
|---|---|---|
| `S3URI(bucket, key)` | focus | `s3://bucket/key` (prefix keeps trailing `/`) |
| `HTTPURL(endpoint, pathStyle, bucket, key)` | resolved context info | path-style `https://host/bucket/esc(key)` or vhost `https://bucket.host/esc(key)` |
| `CLISnippet(endpoint, bucket, key, out)` | focus | `aws s3api get-object --endpoint-url … --bucket … --key … <out>` |
| `CurlSnippet(presignedURL)` | presign result | `curl -fLo <base(key)> '<url>'` |
| `ExportCSV/ExportJSON(report, uploads, meta)` | report state | serialized bytes; `bounded` field/column carries partial-ness |

Validation: key escaping (space, unicode, `+`, `?`) unit-tested; builders are total functions
(no error returns) over already-validated inputs.

**PresignRequest** (storage side): `(bucket, key, ttl)` where `ttl ∈ {15m, 1h, 24h, 7d}` —
enforced in the UI picker AND validated in `PresignGet` (defense in depth; 7d = SigV4 hard
max). Result: `(url string, warn string)`; `warn` non-empty when credentials expire before
`now+ttl`. The `url` value is a bearer secret: rendered + clipboard only, never logged.

## 6. Export Report file

`<DownloadDir>/s3s-report-<bucket>[-<prefix-slug>]-<YYYYMMDD-HHMMSS>.{csv,json}`

- `prefix-slug` = prefix with `/`→`-`, truncated to 40 chars, empty omitted.
- CSV layout: `section,label,count,bytes,bounded` rows (totals, each dist bucket, class rows,
  MPU line); JSON = the same tree as the in-memory report (stable field names — schema in
  `contracts/copy-share-menu.md`).
- Failure handling: write to temp name + rename on success; on failure remove temp, footer
  error (spec edge case "no half-written file presented as success").

## 7. Preview payload kinds (extended) — `internal/preview`

Existing `Kind`: text / image / binary (`text.go:29-36`).

| Addition | Meaning |
|---|---|
| `KindJSON` | single JSON value (object/array) — pretty-printable |
| `KindNDJSON` | ≥2 newline-delimited JSON values — per-line pretty |
| gzip wrapper | not a Kind: detected pre-classify (magic `1f 8b`), decompressed (capped at `Limit`), result re-classified; `Payload` gains `Compressed{From, Shown int64, Truncated bool}` metadata |
| hexdump | render form of `binary` Kind (offset+hex+ASCII), not a new Kind |

View state on `App`: `rawPreview bool` (toggle `p`, `modeObject` only; reset on each new
object load).

## 8. Details-pane groups (presentation contract) — `internal/ui`

`metaFieldRows` reorganized into ordered groups (omit-empty at GROUP granularity too):

| Group | Fields (016 names) |
|---|---|
| Identity & content | Key, Size, Modified (relative + exact), Type, Class, ETag (+multipart annotation), Version, Delete marker |
| Security & governance | Encryption, KMS key, Lock, Retain until, Legal hold, Replication, Restore |
| Delivery | Expires, Encoding, Cache, Disposition |
| User metadata | existing sorted KV block |

Field-state rendering (text + palette role, NO_COLOR-safe):

| State | Text | Style role |
|---|---|---|
| populated | value | `metaValStyle` |
| not set (core) | `—` | dim |
| omitted (optional) | row absent | — |
| unknown (permission-gated header absent) | `unknown` | warn |
| denied (explicit AccessDenied) | `denied` | warn |
| unsupported | `unsupported` | dim |

Per-field copy: field-select sub-state on the pane (`copyFieldSel int`) entered from the copy
menu ("copy a field…"); emits OSC52 with the full untruncated value.
