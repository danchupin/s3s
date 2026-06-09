# Phase 0 Research: Metadata Enrichment & Inline Usage

All decisions are grounded in the existing code (file:line) and the spec's research
notes (`spec.md:36-51`). No open `NEEDS CLARIFICATION` remain — the plan-level
clarifications (metadata depth; fate of the analyze screen; lazy dwell-gated totals;
the single context-aware key; omit-empty pane policy) are resolved in `spec.md:53-83`.

## R1. Enriched object metadata: which fields, from where, and how to map them

**Decision**: Extend `storage.ObjectMetadata` (`internal/storage/storage.go:187-195`,
today `Key, Size, LastModified, ContentType, StorageClass, ETag, UserMetadata`) with
the fields the existing `HeadObject` already returns and discards: `VersionId string`,
`DeleteMarker bool`, `SSEAlgorithm string`, `SSEKMSKeyId string`,
`ReplicationStatus string`, `RestoreStatus string`, `ObjectLockMode string`,
`ObjectLockRetainUntil time.Time`, `ObjectLockLegalHold string`,
`LifecycleExpiration string`, `ContentEncoding string`, `CacheControl string`,
`ContentDisposition string`. Map each in `s3Client.HeadObject` (`s3client.go:151-168`,
which today maps only the seven existing fields from `HeadObjectOutput`):
`out.VersionId`, `out.DeleteMarker`, `string(out.ServerSideEncryption)` (enum
`types.ServerSideEncryption`), `out.SSEKMSKeyId`, `string(out.ReplicationStatus)`
(enum `types.ReplicationStatus`), parse `out.Restore` (a string like
`ongoing-request="true", expiry-date="..."`), `string(out.ObjectLockMode)`,
`out.ObjectLockRetainUntilDate`, `string(out.ObjectLockLegalHoldStatus)`,
`out.Expiration`, `out.ContentEncoding`, `out.CacheControl`, `out.ContentDisposition`.

**Rationale**: These cost **zero extra round-trips** — the browser already issues this
`HeadObject` on object open and on the debounced pane load (`pane.go:79`, fed by
`loadPaneMeta` via `onPaneTick`, `app.go:344-357`). The spec calls this "free metadata
fetched and discarded" (`spec.md:22-26, 38-44`). FR-002 mandates no new per-object
request for these.

**Alternatives considered**: (a) A second richer "describe" call — rejected: no such
single call exists on S3 (`spec.md:46-48`), and it would add a round-trip violating
FR-002. (b) Storing the raw SDK output type on `ObjectMetadata` — rejected: leaks the
SDK across the constitution-I boundary (`storage.go:1-7` declares this package the sole
SDK importer; UI consumes plain types).

## R2. Omit-empty render policy, the SHARED render path, and permission-gated fields

**Decision**: Put the omit-empty optional block inside the **shared** `metaFieldRows`
(`internal/ui/metadata.go:28-37`), NOT in `metaPane` only. `metaFieldRows` is the single
source of truth consumed by BOTH render paths: the Enter object view (`metaPane`,
`metadata.go:54`, reached in `modeObject`) and the focus details pane (`paneTree`,
`pane.go:79`, reached on the wide layout via `browseDetailsView`, `pane.go:38-43`, which
dispatches to `paneTree` when `m.focusZone == zoneObjects`). Split `metaFieldRows` into
(1) the core block (Key/Size/Modified/Type/Class/ETag) rendered unconditionally
(`metadata.go:30-35`, unchanged), then (2) an optional block emitted through a new helper
`omitEmpty(label, value string, gated bool)`: write nothing if `value=="" && !gated`;
write `metaRow(label, "unknown", w)` if `value=="" && gated`; otherwise
`metaRow(label, sanitizeLabel(value), w)`. Object-lock mode and legal-hold are the only
`gated=true` fields (their absence is meaningful, `spec.md:249-250`).

**Rationale**: FR-003 + the clarification (`spec.md:75-79`) require absent optional
fields to be **omitted entirely** (no placeholder line) so the pane stays compact, while
permission-gated fields must always render "unknown" — distinct from a configured-but-empty
"none". Anchoring the block in `metaFieldRows` (rather than `metaPane`) guarantees the
focus pane (`paneTree:79`) shows the new fields too; otherwise the wide-layout "on focus"
path (US1 acceptance) would silently miss them. The current `orDash` (`metadata.go:70-75`)
renders `—` for empty, which is the wrong policy for the new optional fields (it would
add ~10 placeholder lines and blow the height budget at 80×24 / 130×24); the core block
keeps `orDash`, the optional block uses `omitEmpty`.

**Alternatives considered**: (a) Put the optional block in `metaPane` only — rejected:
the focus pane would miss the enriched fields, breaking US1's "on focus" coverage. (b)
Always render all fields with `—` — rejected: blows the height budget and conflates
"none" with "denied" (SC-004). (c) A render-all toggle — rejected: adds a keybinding the
clarification explicitly avoids (`spec.md:69-74`).

## R3. Storage interface extension for tags + bucket config, and read-only safety

**Decision**: Add two read methods to the `storage.Storage` read-view interface
(`storage.go:100-128`): `GetObjectTagging(ctx, bucket, key) (ObjectTags, error)` and
`GetBucketConfiguration(ctx, bucket) (BucketConfig, error)`. `ObjectTags` is
`{ObjectKey string; Tags map[string]string}`. `BucketConfig` carries one `ConfigItem`
per sub-resource (Versioning/Encryption/Lifecycle/Replication/PublicAccessBlock/
Location), each a tri-state `{State ConfigState; Detail string; Reason error}` with
`ConfigState ∈ {"configured","none","denied","unsupported"}`. Add a new sentinel
`ErrUnsupported` near the existing sentinels (`storage.go:19-44`). Extend the private
`s3API` interface (`s3client.go:23-32`, which already lists the read ops ListBuckets/
ListObjectsV2/HeadObject/GetObject and the write ops Put/Delete/Copy/DeleteBucket) with
the additional read SDK ops `GetObjectTagging`, `GetBucketVersioning`,
`GetBucketEncryption`, `GetBucketLifecycleConfiguration`, `GetBucketReplication`,
`GetPublicAccessBlock`, `GetBucketLocation`. `GetBucketConfiguration` calls each
sub-resource **independently** and classifies its own error so one denied/unsupported
sub-resource does not fail the whole call.

**Classification split (this is the FR-013 three-way distinction — corrected)**:
`classify` (`s3client.go:231-283`) maps codes into THREE distinct buckets, in order:

- **→ `ConfigState "none"`** (call reached the backend; the sub-resource simply is not
  set): the `*NotFound`/`*NotConfiguration` family —
  `NoSuchTagSet`, `ServerSideEncryptionConfigurationNotFoundError`,
  `NoSuchLifecycleConfiguration`, `ReplicationConfigurationNotFoundError`,
  `NoSuchPublicAccessBlockConfiguration`. These mean "nothing configured", NOT
  "backend can't do it". (For `GetObjectTagging`, an empty tag set is a 200 with no
  tags → `none`, not an error.)
- **→ `ErrUnsupported` (`ConfigState "unsupported"`)**: a genuinely different signal —
  the backend does not implement the call: `smithy.APIError` code `NotImplemented` or
  `MethodNotAllowed`, or HTTP status `501 Not Implemented` / `405 Method Not Allowed`.
  This is added to `classify` BEFORE the `ErrUnreachable` fallback.
- **→ `ErrAccessDenied`**: the existing 401/403/`AccessDenied`/`Forbidden` mapping
  (`s3client.go:254-256, 265-266`).

Keeping the `*NotFound` family at `none` and reserving `ErrUnsupported` for the
`NotImplemented`/`501`/`405` family is what makes "a bucket with no lifecycle rule"
distinguishable from "a backend that can't do lifecycle" (FR-013, edge case
`spec.md:251-253, 269-270`). The earlier "ErrUnsupported/none" wording was ambiguous and
is removed.

**Read-only safety (FR-014)**: every new symbol is `Get*`. The structural guard bans only
identifiers matching `(Put|Delete|Create|Copy|Upload|Restore|Write)(Object|Bucket|…)`
(`scripts/check-readonly.sh:43-45`); `Get*` never matches even where UI code references
`store.GetObjectTagging`/`store.GetBucketConfiguration`, and the SDK import stays inside
`internal/storage` (the find at `check-readonly.sh:21` excludes that path). `make
check-readonly` stays green. (Framing note: this extends the package's READ view; the
package already holds the `Mutator` write surface, `storage.go:54-98`, and `s3API`
already lists `PutObject/DeleteObject/CopyObject/DeleteBucket`, `s3client.go:28-31` —
read-only is a guard-enforced posture, not an absence of write code.)

**Alternatives considered**: (a) Returning `error` per sub-resource to the UI and letting
it classify — rejected: pushes classification into the UI, violating constitution I.
(b) One bool per sub-resource instead of tri-state — rejected: cannot express the
FR-013 three-way distinction; SC-004 demands the user always know *why* a value is
absent. (c) Reusing `ErrAccessDenied` for unsupported, or mapping `*NotFound` to
`unsupported` — both rejected: they conflate distinct states the spec requires kept
apart.

## R4. Folding analyze into an inline pane (delete modeUsage), reusing UsageOf

**Decision**: Delete the full-screen `modeUsage` (`app.go:30`), its 7 usage fields
(`usage/usageSel/usageBucket/usagePrefix/usageReturn/usageProg/usageCh`, `app.go:219-227`),
the `Update` usageProgress/usageDone cases, the `onKey` case (`app.go:881`), and the
`View` case (`app.go:1190-1191`); drop `runAnalyze`/`onUsageKey`/`usageView`/`usageTitle`
from `analyze.go`. Also remove the now-dangling references the deletion exposes:
`modeUsage` in `canOpenCommand` (`command.go:57`) and the `hintCtx{mode: modeUsage}` test
references (`footer_test.go:194,249`) — note `footerHints` (`styles.go:511-527`) does NOT
branch on `c.mode`, so no production footer change is needed, only the test migration.
Keep `usageTarget` (`analyze.go:37-52`) and the off-loop scan plumbing
(`analyzeCmd`/`waitForUsage`, `analyze.go:73-98`). Render totals inline in `paneBucket`
(`pane.go:45-56`) and `paneTree` folder/level branches (`pane.go:58-96`) as
`total <size> · N objects` (reusing the `usageView` header style, deleted but its format
preserved), with `(partial)` when `Complete==false`. The ranked child breakdown becomes
an expandable detail section in the same pane gated by `usageExpanded`, reusing
`usageBar`/share (kept from `analyze.go:214-223`).

**Rationale**: The user's stated objection is the separate analyze interface
(`spec.md:9, 27-29`); FR-008 removes the destination and US2/US3 (SC-002, SC-006) require
totals + breakdown + drill-down on the main screen with zero mode switches.
`storage.UsageOf` (`storage.go:123-127`) already produces exactly
`{TotalSize, TotalCount, Children ranked largest-first, Complete}` (`storage.go:139-148`);
it is reused verbatim. The pane already renders metadata via `metaFieldRows` (`pane.go:79`)
and yields height to a preview (`pane.go:93`), so the breakdown slots into the same
height-budgeted area. Because that area is tight (see R-budget in `plan.md`), at most ONE
detail section renders at a time.

**Alternatives considered**: (a) Keep `modeUsage` as a fallback — rejected: SC-006
requires the destination to no longer exist; orphaned code violates the "no dead code"
posture. (b) A new dedicated usage cache type — rejected: `internal/cache` is generic
`Cache[V]` (`cache.go:18`); a second `Cache[*storage.UsageReport]` instance reuses the
proven Key + invalidation API with zero new code.

## R5. Non-blocking, dwell-gated, generation-guarded, session-cached usage load

This is the area three verifiers flagged hardest. The design below makes the
**(generation bump, ctx cancel, message drop-check) triple all key off the SAME
generation (`usageGen`)** and preserves the channel-drain discipline.

**Decision — the one coherent isolation model**: the inline usage scan owns BOTH a
dedicated `usageGen int` AND a dedicated `usageCancel context.CancelFunc`. It does NOT
reuse `m.gen`/`loadCancel`/`beginLoad` for cancellation (those are bumped by unrelated
level/pane loads). On EVERY event that should supersede a scan — a focus move onto a
new bucket/folder/level, AND inside `beginLoad` (so a navigation that starts a level load
also kills a running scan) — the model performs, together:

```
if m.usageCancel != nil { m.usageCancel() }   // make UsageOf return promptly → goroutine close(ch)
m.usageGen++                                    // any in-flight scan's messages are now stale
```

Each scan starts a fresh `ctx, cancel := context.WithCancel(context.Background())`, stores
`cancel` in `m.usageCancel`, and dispatches `loadUsage(ctx, store, bucket, prefix, ch,
m.usageGen)` (the retained `analyzeCmd` plumbing). `usageProgressMsg`/`usageDoneMsg` carry
`usageGen`. Because cancellation and the gen-bump happen together, a late `usageDoneMsg`
for the abandoned target is BOTH stamped with the old `usageGen` (so it is dropped) AND
its scan ctx is cancelled (so it does not run to completion on the wrong target). Neither
of the draft's two contradictory failure modes can occur.

**Drain discipline (no goroutine leak, constitution II)**: mirror the existing
`onUsageProgress` guard (`analyze.go:100-108`, whose comment is explicit: "NOT gated on
mode, so opening an overlay mid-scan can never strand the producer on a full channel
(goroutine leak)"). The channel **pump** (`waitForUsage` re-arm) keeps draining a live
channel REGARDLESS of `usageGen` — `onUsageProgress` returns `m, waitForUsage(m.usageCh,
gen)` as long as `m.usageCh != nil`; only the side effect (`m.usageProg = msg.p`) is
gated on `msg.gen == m.usageGen`. The producer (`analyzeCmd`, `analyze.go:75-86`) uses a
buffered(8) channel and a `default:`-drop on progress send (`analyze.go:78-81`), and
`close(ch)` in `defer`; combined with `usageCancel()` making `UsageOf` return promptly,
the producer can never block forever. (A subtlety the draft missed: with possibly two
scans briefly overlapping during rapid navigation, each scan has its OWN channel — the
old `m.usageCh` is replaced when the new scan starts, but its pump must drain to `close`.
The model keeps the SUPERSEDED channel pumping by NOT nil-ing it until its `done` event
arrives; `onUsageDone` for a stale gen still nil-checks and lets the channel close, it
just does not apply the report.)

**Dwell gate + cached path**: a `usageTickCmd(gen, bucket, prefix)` (`tea.Tick`, identical
pattern to `bucketTickCmd`/`paneTickCmd`, `commands.go` + `app.go:374-400`) is scheduled
on focus move. `onUsageTick` fires `loadUsage` only if `msg.gen == m.usageGen` AND the
focused usage target still equals `(msg.bucket, msg.prefix)` AND no cached result exists.
Cache results in `m.usageResults *cache.Cache[*storage.UsageReport]` keyed by
`cache.Key{Context: m.ctxName, Bucket: bucket, Prefix: prefix, Search: ""}`; a cache hit
shows immediately, bypassing the dwell (FR-005, SC-007).

**Scheduling the tick from BOTH focus paths (the draft's hole)**: the usage target is the
highlighted BUCKET (in `modeBuckets`), or the selected FOLDER / the current LEVEL prefix
(in `modeTree`, per `usageTarget`, `analyze.go:38-49`). But the existing
`afterSelectionMove` (`app.go:328-338`) only schedules a `paneTick` for OBJECT selections
and returns `m, nil` for folders/levels — exactly the usage targets. So:
- In `modeBuckets`: schedule the usage tick from `afterBucketMove` (`app.go:374-384`)
  alongside the existing `bucketTickCmd`; re-verify with `m.highlightedBucketName() ==
  msg.bucket` in `onUsageTick`.
- In `modeTree`: extend `afterSelectionMove` to ALSO schedule a `usageTickCmd` for
  dir/level selections (the branches that currently return early), carrying
  `(bucket, folderOrLevelPrefix)`; add a `focusedUsageTarget() (bucket, prefix)` accessor
  (a thin wrapper over `usageTarget`) and re-check `focusedUsageTarget() ==
  (msg.bucket, msg.prefix)` in `onUsageTick`. The tick message therefore carries the
  target identity (`usageTickMsg{gen, bucket, prefix}`), not just a key.

**Cancellation on navigate**: covered by the combined `usageCancel()` + `usageGen++` placed
in `beginLoad` AND in the focus-move handlers (above); a cancelled scan yields a report
with `Complete=false`, rendered `(partial)` (FR-006). The producer's `errorsIsCanceled`
path (`analyze.go:227-229`) keeps a cancelled scan from surfacing as a hard error.

**Cache lifecycle (corrected hook sites — the draft named the wrong one)**: there are TWO
refresh entry points and they behave differently today:
- Tree `r` = `refresh()` (`tree.go:142-149`) calls `m.cache.Invalidate(key)` DIRECTLY at
  `tree.go:144` (NOT the `invalidateLevel` helper, which is only called from `operation.go`
  after mutations). Add `m.usageResults.Invalidate(usageKey)` for the focused folder/level
  right beside it.
- Bucket-list `r` = `refreshBuckets()` (`hintbar.go:175-178`) does NOT touch any cache today
  (it only `beginLoad` + `loadBuckets`). Add `m.usageResults.InvalidateBucket(m.ctxName,
  m.highlightedBucketName())` so the highlighted bucket's totals rescan on the next focus
  (using `InvalidateBucket`, `cache.go:45`, since a bucket may have cached several prefix
  totals).
- Context switch = `onContextResolved` sets `m.ctxName = msg.target` then `m.cache.Clear()`
  (`app.go:~1060`); add `m.usageResults.Clear()` beside it for memory hygiene + parity. NOTE
  (corrected rationale): a cross-context value bleed is ALREADY impossible because
  `cache.Key` includes `Context` (`cache.go:9-14`) and `m.ctxName` is updated before any
  new key is built, so old-context entries are unreachable by key. `Clear()` is for memory
  reclamation/parity, NOT correctness of totals.

**Rationale**: Constitution II forbids blocking the loop; FR-016 requires
generation/cancellation guards; FR-005 requires the dwell gate (so rapid transit
`spec.md:263-265` spawns no scan) and an immediate cached path; FR-006 requires
cancel-on-navigate with a partial result; FR-007 requires session caching invalidated by
manual refresh; the edge case `spec.md:271-272` requires context-scoped caching.

**Alternatives considered**: (a) Reuse `m.gen` for usage (the draft's contradictory mix) —
rejected: `m.gen` is bumped by unrelated level/pane loads, so either the stale result is
NOT dropped (overwrites the pane with another target's totals) or the scan is never
cancelled (runs to completion on an abandoned target). A dedicated `usageGen` + paired
`usageCancel` is the only coherent isolation. (b) Gate the channel pump on `usageGen` —
rejected: strands the superseded producer on a full channel (goroutine leak), the exact
hazard `analyze.go:100-108` documents; the pump must drain ungated. (c) Scan eagerly on
every focus — rejected: O(objects) per transited entry on rapid scroll. (d) A wall-clock
timer goroutine — rejected: `tea.Tick` is the idiomatic, test-injectable Bubble Tea v2
debounce (tests `deliver` a synthetic `usageTickMsg`).

## R6. Inline running-indicator render driver (after runAnalyze deletion)

**Decision**: The inline "scanning…" running line is driven by the **`usageProgressMsg`
channel ticks** (`analyze.go:79`, the `onProgress` callback), NOT by `spinnerTick()`. The
deleted `runAnalyze` batched `spinnerTick()` (`analyze.go:68`) only to animate the
spinner glyph; the inline pane shows `scanning… N objects, <size> so far` whose numbers
are refreshed every time a `usageProgressMsg` arrives and re-renders the pane. No
`spinnerTick` is needed for the inline scan (the global spinner, `commands.go:285-287`,
remains for the main load, untouched). When `usageProg` is non-zero and no cached/complete
report exists for the focused target, the pane renders the running line; this is the
concrete driver for FR-005's "visible running indication".

**Rationale**: `usageProgressMsg` already re-arms via `waitForUsage` and already carries
the running totals (`analyze.go:96, 107`), so it is a sufficient and deterministic render
driver. Reintroducing `spinnerTick` for the pane would add a second, redundant tick source
and a second message to migrate. Tests `deliver` synthetic `usageProgressMsg`s to assert
the running line updates.

**Alternatives considered**: keep `spinnerTick` batched into `loadUsage` for a spinning
glyph — rejected as unnecessary (the changing object count already reads as live progress)
and as extra surface to keep in sync; if a glyph is wanted later it is additive.

## R7. Listing storage-class marker (FR-015)

**Decision**: In `treeView` (`tree.go:224-240`) keep the four columns
`{"name",0},{"type",5},{"size",11},{"modified",17}`. For objects with a **non-standard**
storage class (`e.obj.StorageClass != "" && != "STANDARD"`) render a fixed, lossy marker
in the `type` cell within its 5-char budget; the FULL class is recoverable via reveal
(`keys.Reveal` = `i`) on the row. Concrete marker map (≤5 chars, since real classes
exceed the budget — GLACIER=7, DEEP_ARCHIVE=12, INTELLIGENT_TIERING=19, GLACIER_IR=10):

| StorageClass | `type` cell marker |
|---|---|
| STANDARD (or "") | `obj` (no marker — zero noise) |
| GLACIER | `glac` |
| GLACIER_IR | `gir` |
| DEEP_ARCHIVE | `arch` |
| INTELLIGENT_TIERING | `int` |
| STANDARD_IA | `ia` |
| ONEZONE_IA | `1zia` |
| REDUCED_REDUNDANCY | `rr` |
| any other non-standard | `cls*` (asterisk = "see reveal") |

Directories always render `dir` (never marked). The full class string is what reveal shows.

**Rationale**: The storage class is already in the list response (`ListLevel` maps
`o.StorageClass`, carried on `ObjectRef`, `storage.go:179-184`), so this is free
(`spec.md:225-228`). FR-015/SC-005 require the non-standard class visible while STANDARD
adds zero noise. The 5-char `type` cell cannot hold the full class, so the cell is
deliberately lossy BUT the marker is a closed, documented set and the full value is
recoverable via reveal — satisfying constitution VI ("fully visible OR revealable"). The
draft's "an abbreviated token or single glyph" was under-defined; this pins the exact map
and ties it to the reveal affordance.

**Alternatives considered**: (a) A new dedicated class column — rejected: steals
flex-width from `name`, risking truncated keys at 80 cols (constitution VI). (b) A
multi-char word like "Glacier" prepended — rejected: overflows the 5-char `type` budget.
(c) Marking every row including STANDARD — rejected: FR-015 forbids redundant per-row
noise. (d) A lossless marker — impossible in 5 chars; reveal recovers the full value
instead.

## R8. Repurposed `a` "more detail" key (FR-019) — one dispatcher for key AND command

**Decision**: Rename the keymap field `Analyze` → `MoreDetail` (`keys.go:21, 54`, binding
stays `"a"`); update the help row, the `:` command (`command.go:33`, `analyze`/`du` →
`detail`/`info`), the hint catalog (`hintbar.go:52, 70`), AND the pane hint labels
(`pane.go:54, 67, 71`, which call `keyHint(m.keys.Analyze, "analyze"…)`) to
`keyHint(m.keys.MoreDetail, "detail")`. A new `startMoreDetail` dispatcher dispatches on
context: bucket-list/prefix focus → toggle the inline usage breakdown (`detailSection`)
and lazily load `GetBucketConfiguration`; object focus → load `GetObjectTagging` + render
governance detail. CRITICAL for FR-019's "one mental model": the renamed `:detail`/`:info`
command-bar entry (`command.go:33`) sets `invoke: App.startMoreDetail` — the SAME function
the `a` key calls — so the command bar and the key share one target and cannot drift.
`keyHint`/`firstBind` (`keys.go:101-113`) propagate the rebind to every hint automatically
once the field + labels are migrated.

**Rationale**: FR-008 frees the `a` key; FR-019 + the clarification (`spec.md:69-74`)
mandate one context-aware key, "no additional separate keybindings". Repurposing the
existing field keeps constitution VII (no new keymap, no new hue). Sharing one invoke
target between the key and the command bar is what guarantees the two never diverge (the
risk R8 itself warns against). The migration MUST include `pane.go:54/67/71` or the build
breaks (those still reference `m.keys.Analyze`).

**Alternatives considered**: (a) Two new keys (one for tags, one for breakdown) — rejected
by the clarification. (b) Keep `Analyze` and add `MoreDetail` — rejected: two fields bound
to overlapping intents drift apart (constitution VII). (c) Different invoke targets for
the key vs `:detail` command — rejected: command-bar/key drift, the exact failure FR-019
forbids.

## R9. Cost-tier grounding (why usage is a full scan; why HeadObject fields are free)

**Decision**: Treat the per-object enriched fields (US1) as zero-cost (already in the
issued `HeadObject`), object tag values and each bucket-config sub-resource (US4) as one
extra read each loaded **only** on the explicit "more detail" key, and bucket/prefix
totals (US2) as an O(objects) background scan cached per session.

**Rationale**: Grounded in the spec's research notes: a `ListObjectsV2` page returns only
key/size/modified/ETag/class (`spec.md:38-39`); `HeadObject` already returns the richer
fields at no extra round-trip (`spec.md:40-44`); tag **values** and bucket config require
separate reads with no single describe call (`spec.md:45-48`); and there is **no cheap
native size/count** on Ceph RGW or MinIO over the S3 API, so totals require paginating and
summing — exactly what `UsageOf` already does (`spec.md:49-51`). This cost shape dictates:
US1 on every open (free), US4 lazy on a key (one read each), US2 dwell-gated + cached
(amortizes the expensive scan).

**Alternatives considered**: (a) Auto-load tags/config on every focus — rejected: each is
a paid round-trip (FR-011/012 say "on demand"); the dwell-and-cache discipline is reserved
for the one expensive scan. (b) Admin-API metrics (CloudWatch / Storage Lens /
`radosgw-admin` / `mc admin`) for cheap totals — rejected: explicitly out of scope
(`spec.md:429-431`) and out-of-band from the S3 endpoint.

