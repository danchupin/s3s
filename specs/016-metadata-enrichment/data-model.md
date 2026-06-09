# Phase 1 Data Model: Metadata Enrichment & Inline Usage

Entities are split into **storage-layer types** (in `internal/storage`, the SDK
boundary) and **UI view-state** (in `internal/ui`, never SDK-aware). Field types and
SDK sources are grounded against `s3client.go:151-168` (the existing `HeadObject`
mapping) and the AWS SDK `HeadObjectOutput` / `GetObjectTaggingOutput` /
per-bucket-config outputs.

## 1. Enriched ObjectMetadata (storage.go:187-195) — SHARED render path

Existing fields retained: `Key string`, `Size int64`, `LastModified time.Time`,
`ContentType string`, `StorageClass string`, `ETag string`,
`UserMetadata map[string]string`.

New fields (all populated from the **existing** `HeadObject`, FR-001/FR-002):

| Field | Go type | SDK source (`HeadObjectOutput`) | Notes |
|---|---|---|---|
| `VersionId` | `string` | `out.VersionId` (`aws.ToString`) | optional |
| `DeleteMarker` | `bool` | `out.DeleteMarker` (`aws.ToBool`) | optional |
| `SSEAlgorithm` | `string` | `string(out.ServerSideEncryption)` enum `types.ServerSideEncryption` (`"AES256"`, `"aws:kms"`) | optional |
| `SSEKMSKeyId` | `string` | `out.SSEKMSKeyId` (`aws.ToString`) | optional; long ARN — revealable |
| `ReplicationStatus` | `string` | `string(out.ReplicationStatus)` enum `types.ReplicationStatus` (`COMPLETE/PENDING/FAILED/REPLICA`) | optional |
| `RestoreStatus` | `string` | parsed from `out.Restore` (`ongoing-request="…", expiry-date="…"`) | optional; malformed → "" (omit), never crash |
| `ObjectLockMode` | `string` | `string(out.ObjectLockMode)` enum `types.ObjectLockMode` | **permission-gated** |
| `ObjectLockRetainUntil` | `time.Time` | `out.ObjectLockRetainUntilDate` (`aws.ToTime`) | optional |
| `ObjectLockLegalHold` | `string` | `string(out.ObjectLockLegalHoldStatus)` enum | **permission-gated** |
| `LifecycleExpiration` | `string` | `out.Expiration` (`aws.ToString`) | optional |
| `ContentEncoding` | `string` | `out.ContentEncoding` (`aws.ToString`) | optional |
| `CacheControl` | `string` | `out.CacheControl` (`aws.ToString`) | optional |
| `ContentDisposition` | `string` | `out.ContentDisposition` (`aws.ToString`) | optional |

**Where the rows are emitted (FR-003)**: ALL field rows — the unconditional core block
AND the omit-empty optional block — are emitted inside the **shared** `metaFieldRows`
(`internal/ui/metadata.go:28-37`). This single function is consumed by BOTH render paths:
- the Enter object view (`metaPane`, `metadata.go:54`, `modeObject`);
- the focus details pane (`paneTree`, `pane.go:79`), reached on the wide layout via
  `browseDetailsView` (`pane.go:38-43`, dispatches to `paneTree` when
  `m.focusZone == zoneObjects`).
Anchoring the optional block in `metaFieldRows` (not in `metaPane` only) is what makes
US1's "on focus" path render the enriched fields too.

**Validation / render rules (FR-003)**:
- Core fields (Key/Size/Modified/Type/Class/ETag) ALWAYS render (keep the existing
  `orDash` for these — `metadata.go:30-35`).
- Optional fields render only when non-empty (`omitEmpty(label, value, false)` → no
  line when empty).
- Permission-gated fields (`ObjectLockMode`, `ObjectLockLegalHold`) ALWAYS render:
  `omitEmpty(label, value, true)` → "unknown" when empty (absence is information,
  `spec.md:249-250`).
- Multipart ETag (`"<hash>-<n>"`) presented as-is, not labeled MD5 (edge case
  `spec.md:254`).

## 2. ObjectTags (storage.go, new)

```
type ObjectTags struct {
    ObjectKey string
    Tags      map[string]string  // zero or more key/value pairs (values, not just count)
}
```

Returned by `GetObjectTagging(ctx, bucket, key)`. Errors: `ErrNotFound` (object absent),
`ErrAccessDenied` (denied), `ErrUnreachable`. An empty `Tags` (200 with no tag set, or
the `NoSuchTagSet` code) = "no tags" (none), distinct from `ErrAccessDenied`. Validation:
keys/values rendered via `sanitizeLabel` (legibility); no count-only fallback.

## 3. BucketConfig + ConfigItem (storage.go, new)

```
type ConfigState string // "configured" | "none" | "denied" | "unsupported"

type ConfigItem struct {
    State  ConfigState
    Detail string  // human summary when configured (e.g. "Enabled", "SSE-KMS …", "3 rules")
    Reason error   // nil | ErrAccessDenied | ErrUnsupported (codes only, never secrets)
}

type BucketConfig struct {
    Bucket            string
    Versioning        ConfigItem
    Encryption        ConfigItem
    Lifecycle         ConfigItem
    Replication       ConfigItem
    PublicAccessBlock ConfigItem
    Location          ConfigItem
}
```

Returned by `GetBucketConfiguration(ctx, bucket)`. (Bucket *policy* / policy-public
status — `GetBucketPolicy`/`GetBucketPolicyStatus` — is intentionally absent: out of
scope per spec.md Assumptions; the panel surfaces `PublicAccessBlock` only.) Each
sub-resource is fetched and classified **independently** so one failure does not fail
the whole call (FR-012/FR-013
graceful degradation; edge cases `spec.md:251-253, 269-270`):

| `State` | Meaning (FR-013) | `Reason` | Backend signal that maps here |
|---|---|---|---|
| `configured` | sub-resource is set; `Detail` summarizes it | nil | a 200 with content |
| `none` | call succeeded, nothing configured | nil | the `*NotFound`/`*NotConfiguration` family — `NoSuchTagSet`, `ServerSideEncryptionConfigurationNotFoundError`, `NoSuchLifecycleConfiguration`, `ReplicationConfigurationNotFoundError`, `NoSuchPublicAccessBlockConfiguration` (also a 200-with-empty for tagging) |
| `denied` | caller lacks read permission | `ErrAccessDenied` | 401/403, `AccessDenied`, `Forbidden` |
| `unsupported` | backend does not implement this call | `ErrUnsupported` | `NotImplemented`, `MethodNotAllowed`, HTTP 501, HTTP 405 |

**This is the FR-013 three-way distinction, made unambiguous**: the `*NotFound` family
means "nothing configured" (→ `none`), NEVER "unsupported". `unsupported` is reserved for
a genuinely different signal (`NotImplemented`/`501`/`405`). The earlier
"ErrUnsupported/none" wording is removed — conflating them would make "a bucket with no
lifecycle rule" indistinguishable from "a backend that can't do lifecycle", defeating
FR-013/SC-004.

**Validation**: `Detail` and `Reason` text are codes/summaries only — never SDK response
bodies or secrets (constitution V; `classify` discipline). `State` is a closed enum; the
UI maps each to a text label distinguishable under `NO_COLOR` (SC-004).

## 4. Error sentinel + classify extension (storage.go:19-44, s3client.go:231-283)

`ErrUnsupported = errors.New("storage: backend does not support this operation")` — new
sentinel joining `ErrNotFound`/`ErrAccessDenied`/`ErrUnreachable`/`ErrInvalidConfig`/
`ErrReadOnly`/`ErrInvalidName`/`ErrMovePartial`/`ErrBucketNotEmpty`.

`classify` (`s3client.go:231-283`) gains the three-bucket split (ordered, before the
`ErrUnreachable` fallback):
1. existing cancellation/`NotFound`/`AccessDenied` mapping is unchanged
   (`s3client.go:238-267`);
2. NEW: `smithy.APIError` code `NotImplemented` or `MethodNotAllowed`, OR
   `awshttp.ResponseError` HTTP status `501`/`405` → `ErrUnsupported`;
3. the per-sub-resource caller in `GetBucketConfiguration` additionally treats the
   `*NotFound`/`*NotConfiguration` family (`NoSuchTagSet`,
   `ServerSideEncryptionConfigurationNotFoundError`, `NoSuchLifecycleConfiguration`,
   `ReplicationConfigurationNotFoundError`, `NoSuchPublicAccessBlockConfiguration`) as
   `ConfigState "none"` (these would otherwise fall through to `ErrUnreachable`); this
   "not-configured → none" classification lives in the config caller, NOT a generic
   `classify` change, so the generic sentinel mapping stays narrow.

`Reason`/`Detail` carry codes/summaries only, never secrets (constitution V;
`classify` logs only `code`/`status`/`message`, `s3client.go:275-281`).

## 5. UsageReport / UsageChild / UsageProgress (storage.go:130-154, existing — now inline)

Unchanged structures, reused verbatim:

```
type UsageChild struct { Name string; IsDir bool; Size int64; Count int }
type UsageReport struct { Bucket, Prefix string; TotalSize int64; TotalCount int;
                          Children []UsageChild; Complete bool }
type UsageProgress struct { ScannedCount int; ScannedSize int64 }
```

`Children` ranked Size-desc (ties by Name); `Complete=false` ⇒ partial (cancelled).
Now presented in the details pane (`pane.go`) instead of the deleted `usageView`.

## 6. UI view-state (internal/ui, app.go App struct)

**Removed** (with `modeUsage`): `usage *storage.UsageReport`, `usageSel int`,
`usageBucket string`, `usagePrefix string`, `usageReturn mode`,
`usageProg storage.UsageProgress`, `usageCh chan usageEvent` (`app.go:219-227`); the
`modeUsage` constant (`app.go:30`).

**Added**:

| State | Type | Purpose |
|---|---|---|
| `usageResults` | `*cache.Cache[*storage.UsageReport]` | session cache keyed by `cache.Key{Context, Bucket, Prefix, Search:""}` (FR-007); `Clear()`ed on context switch, `Invalidate`/`InvalidateBucket`d on refresh |
| `usageProg` | `storage.UsageProgress` | running partial during the active scan (re-added, scoped to the inline scan; drives the running line, R6) |
| `usageScanKey` | `cache.Key` | the target of the in-flight scan (for the generation/target guard) |
| `usageGen` | `int` | scan generation, ISOLATED from `m.gen`; bumped TOGETHER with `usageCancel()` on focus move + in `beginLoad` (FR-016) |
| `usageCancel` | `context.CancelFunc` | cancels the in-flight scan's OWN context (NOT `loadCancel`); called together with the `usageGen` bump so cancel + drop key off the same generation |
| `usageCh` | `chan usageEvent` | scan progress channel (re-targeted from the analyze flow); pump drains ungated to `close` (no leak) |
| `detailSection` | `enum {sectNone, sectBreakdown, sectTags, sectConfig}` | which SINGLE expandable detail section is shown (mutually exclusive — budget gate, see layout-budget contract); toggled by the `MoreDetail` key |
| `objectTags` | `*storage.ObjectTags` | last-loaded tags for the focused object (US4), nil until requested |
| `bucketCfg` | `*storage.BucketConfig` | last-loaded config for the focused bucket (US4), nil until requested |
| `detailKey` | `string` | the object/bucket the tags/config were loaded for (drop stale results) |
| `detailGen` | `int` | generation for tag/config loads; stale-gen `objectTagsMsg`/`bucketConfigMsg` dropped |

**Reused unchanged**: `gen`, `loadCtx`, `loadCancel` (`app.go:194-197` region) — for the
MAIN load only, NOT usage; `paneGen`, `bucketLoadGen` dwell generations; `keys.MoreDetail`
(renamed from `keys.Analyze`).

## 7. Usage-scan state machine (per focused target) — the corrected triple

The **(gen bump, ctx cancel, drop-check)** triple all key off `usageGen` + `usageCancel`,
never `m.gen`/`loadCancel`.

```
                 focus moves onto bucket/folder/level
   idle ───────────────────────────────────────────► dwelling
     ▲    (afterBucketMove OR afterSelectionMove-extended:                │
     │     m.usageCancel() + usageGen++; schedule usageTickCmd{gen,b,pfx}) │
     │                                                                     │
     │ cached hit (show immediately, no scan, FR-005, SC-007)              │ dwell tick fires
     │                                                                     │ (msg.gen==usageGen &&
     │                                                                     │  focusedUsageTarget()==(b,pfx)
     │                                                                     │  && not cached)
     │                                                                     ▼
     │                                                                  scanning ──progress──► scanning
     │                                                                     │  (usageProgressMsg gen-matched →
     │                                                                     │   usageProg; pump re-arms UNGATED)
     │   navigate away / beginLoad: m.usageCancel() + usageGen++           │
     ├───────────────────────────◄────────────────────────────────────────┤
     │   ctx cancelled → UsageOf returns Complete=false (partial, FR-006)  │
     │   late usageDoneMsg stamped OLD usageGen ⇒ report NOT applied;       │ usageDoneMsg gen matches
     │   its channel still drains to close (no goroutine leak)             │ (err nil/canceled)
     │                                                                     ▼
     │                                                                 complete ─► usageResults.Put(key,rep)
     └────────────────────◄────────────────────────────────────────────────┘
              re-focus same target → cached → idle→show
```

Transitions:
- **idle → dwelling**: focus move bumps `usageGen` + calls `usageCancel()` (together),
  schedules `usageTickCmd{gen, bucket, prefix}`. In `modeBuckets` this is in
  `afterBucketMove`; in `modeTree` `afterSelectionMove` is EXTENDED to schedule the tick
  for dir/level selections too (today it returns early for non-objects).
- **dwelling → scanning**: `onUsageTick` with `msg.gen==usageGen` AND
  `focusedUsageTarget()==(msg.bucket,msg.prefix)` AND no cache entry → `loadUsage` (a
  `tea.Cmd` with a FRESH ctx whose cancel is stored in `usageCancel`).
- **dwelling/scanning → idle (cached)**: a target already in `usageResults` shows
  immediately, bypassing the dwell (FR-005, SC-007).
- **scanning → scanning**: `usageProgressMsg` (gen-matched) updates `usageProg`; the pump
  (`waitForUsage` re-arm in `onUsageProgress`) re-arms as long as `m.usageCh != nil`
  REGARDLESS of gen (drain discipline — no leak).
- **scanning → complete**: `usageDoneMsg` (gen-matched, err nil/canceled) → store report,
  `usageResults.Put`. Cancelled (`Complete=false`) stored as partial, rendered partial.
- **any → cancelled/dropped**: navigating away calls `usageCancel()` (scan ctx ends →
  `UsageOf` returns → goroutine `close(ch)`) AND bumps `usageGen` (the superseded
  `usageProgressMsg`/`usageDoneMsg` is not APPLIED); the superseded channel's pump keeps
  draining until `done`/`close`.
- **cache lifecycle**: tree `r` → `m.usageResults.Invalidate(usageKey)` beside
  `m.cache.Invalidate` (`tree.go:144`); bucket-list `r` →
  `m.usageResults.InvalidateBucket(ctxName, highlightedBucket)` in `refreshBuckets`
  (`hintbar.go:175`, which today invalidates nothing); context switch →
  `m.usageResults.Clear()` beside `m.cache.Clear()` (`app.go:~1060`) for memory parity
  (cross-context bleed is already impossible — `Context` is in the key).

## 8. Tag/config load state + the budget gate (per "more detail" key press, US4)

`idle → loading (tea.Cmd, detailGen-tagged) → loaded | error`. A result whose `detailKey`
no longer matches the focused object/bucket, OR whose `detailGen` is stale, is dropped
(FR-016). Loads are **only** triggered by the explicit `MoreDetail` key / `:detail`
command (FR-011/FR-012 "on demand"), never on focus.

**Budget gate (constitution VI)**: `detailSection` is a single mutually-exclusive
selector — at most ONE of `{breakdown, tags, config}` renders in the detail zone at a
time. The `MoreDetail` key toggles the section appropriate to the focus (bucket/prefix →
breakdown + config; object → tags), and a second press collapses it back to `sectNone`.
This keeps the enriched-metadata + one-section stack within the `rows-2` body budget; when
even that overflows, the pane emits a trailing `… +N more (i to reveal)` line and the
clipped rows are recoverable via `keys.Reveal` (verified by the height sweep, layout-budget
contract). The new `objectTagsMsg`/`bucketConfigMsg` types carry `detailGen`; their drop
wiring mirrors `paneTickMsg` (`onPaneTick`, `app.go:344-357`).

