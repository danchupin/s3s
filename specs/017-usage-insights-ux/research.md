# Research: Budgeted Usage, Insights & Details UX (017)

Phase 0 output. Every plan-level unknown from the spec ("Outstanding" after `/speckit-clarify`)
plus every technology choice is resolved here as Decision / Rationale / Alternatives, grounded
in file:line evidence from the current tree.

---

## D1. Budget mechanics: cap inside `UsageOf`, not in the UI

**Decision**: Extend the storage contract — `UsageOf(ctx, bucket, prefix, maxObjects int,
onProgress)` (0 = unlimited). The enumeration loop (`internal/storage/s3client.go:245-270`)
stops once `agg.totalCount >= maxObjects` and returns `agg.report(false)` with a new
`Bounded=true` flag. The UI passes the configured budget for ambient/dwell scans and `0` for
the explicit full scan.

**Rationale**: The page loop and the aggregator live in `internal/storage`; capping there is
one comparison per page and keeps the UI free of enumeration logic (constitution I). A
`MaxKeys`-tuned final page is unnecessary — overshooting by part of one page (≤999 objects)
is immaterial against a 20 000 default and keeps the loop untouched.

**Alternatives considered**:
- *UI-side cancel after N progress ticks* — racy (progress is throttled/dropped on a full
  channel, `internal/ui/analyze.go:140-142`), puts policy in the render layer, cancels
  mid-page anyway.
- *Separate `UsageOfBounded` method* — two methods drift; one signature with `0=unlimited`
  matches `LevelQuery.MaxKeys` precedent (`storage.go:221`).
- *Server-side cheap size lookup* — does not exist in the S3 protocol; RGW/MinIO expose
  bucket stats only via admin APIs, which are out of scope (spec Assumptions).

## D2. Default budget = 20 000 objects; config key `usageScanBudget`

**Decision**: `Config.UsageScanBudget *int` (`internal/config/config.go:27-33` struct), YAML
key `usageScanBudget`. `nil` → default 20 000; `0` → ambient scanning disabled
(explicit-only); negative → validation error. Plumbed into `ui.App` at construction
(`cmd/s3s/main.go` wiring), not read from `internal/ui`.

**Rationale**: 20 000 objects ≈ 20 listing pages ≈ ~2 s of sequential requests on a healthy
backend — an ambient cost comparable to what a human dwell "asks for", and ~0.02% of a
100M-object bucket (SC-001). Pointer distinguishes "absent" from explicit `0` (the spec's
FR-006 disable mode). Config already carries per-user UX knobs (`DownloadDir`,
`config.go:33`), so this is the established home; constitution I keeps config parsing out of
the UI.

**Alternatives considered**: pages instead of objects (leaks pagination details into config);
env var (no precedent — config file + flags is the 014 pattern); per-context budget
(overkill; one global knob, revisit on demand).

## D3. Partial results are cached; full scan restarts from zero

**Decision**: `onUsageDone` (`internal/ui/analyze.go:179-191`) caches EVERY report that
carries data — exact, budget-bounded (`Bounded`), or cancelled (`Complete=false`) — tagged so
the pane renders `≥` lower-bound markers. A later full scan simply overwrites the cache entry
(spec FR-005). No continuation-token resume: a full scan restarts enumeration from the first
page.

**Rationale**: Caching partials is the fix for the today's worst case (scan 95% done →
navigate → all discarded, `analyze.go:185-187`). Resume-from-token was rejected because a
continuation token is only valid against a *stable listing*; objects created/deleted between
sessions of the scan silently corrupt totals, and the token's lifetime is
backend-discretionary. Restart keeps totals honest and the code identical to the existing
loop.

**Alternatives considered**: store `NextContinuationToken` in the cached partial and resume
(rejected: correctness as above + cache entry becomes a live resource); TTL the partials
(rejected: cache is session-scoped and manual-refresh-invalidated already, 016 pattern,
`internal/ui/tree.go:144`, `hintbar.go:175`).

## D4. Full-scan affordance: `A` key + `:scan`, never implicit

**Decision**: New `keys.FullScan = "A"` + command-bar `:scan` invoking ONE dispatcher
(`startFullScan`), mirroring the 016 `a`/`:detail` single-target pattern
(`internal/ui/analyze.go:197`, FR-019 there). `startMoreDetail`'s uncached branch
(`analyze.go:228-230`) is re-pointed at the *budgeted* scan. The details pane shows the
affordance line (`A full scan`) whenever the current target's report is absent or partial.

**Rationale**: Clarification session decision (spec `## Clarifications`). `A` is unbound today
(`internal/ui/keys.go:43-74` binds `a` but not `A`); shift-variant of "more detail" is the
established mnemonic family (`x`/`X` delete/recursive-delete, `s`/`S` sort/direction —
`keys.go:57-67`).

**Alternatives considered**: confirm-prompt on `a` (rejected by clarification — prompt noise
on small buckets); `:scan` only (fails SC-004-style keystroke economy and discoverability).

## D5. Histograms: fixed boundaries, computed in `usageAgg`, single pass

**Decision**: Distributions accumulate inside `usageAgg` (`internal/storage/s3client.go:243`,
`agg.add`) from fields already present in every `ListObjectsV2` entry (Key, Size,
LastModified, StorageClass — same source as `ObjectRef`, `storage.go:232-237`). Fixed
boundaries:
- **Age** (vs scan start time): `<1d`, `1–7d`, `7–30d`, `30–90d`, `90–365d`, `>1y`.
- **Size**: `<128 KiB`, `128 KiB–1 MiB`, `1–16 MiB`, `16–128 MiB`, `128 MiB–1 GiB`, `≥1 GiB`.
- **Storage class**: open map keyed by reported class (empty → `STANDARD`), carrying
  count + bytes each.

**Rationale**: O(1) per object inside the loop that already touches every object — FR-020's
"no additional requests" holds by construction. Fixed boundaries keep reports comparable
across buckets and render in exactly 6 rows each (health-card height budget); the 128 KiB
low edge intentionally equals the small-object threshold default (D8) so the warning is
readable straight off the first histogram row.

**Alternatives considered**: configurable boundaries (config bloat, breaks row-budget
guarantee, no user asked); exact percentile sketches (t-digest — dependency + overkill for a
6-bucket visual); log2 buckets (boundaries unfamiliar to operators).

## D6. Incomplete multipart uploads: `ListMultipartUploads` + capped `ListParts` enrichment

**Decision**: New read-view method `ListIncompleteUploads(ctx, bucket, prefix)` returning
`IncompleteUploads{State, Count, OldestInitiated, TotalSize, SizedCount}`. Implementation
paginates `ListMultipartUploads` (prefix-scoped); for the first **100** uploads it also calls
`ListParts` to sum part sizes (`TotalSize`, `SizedCount`); beyond 100, uploads are counted
and aged but not sized (the card renders `≥` and "N of M sized"). Tri-state reuses 016
semantics: denied → `ConfigDenied`-equivalent, `NotImplemented`/501 → unsupported via the
existing `classify` extension (`internal/storage/s3client.go` classify; sentinel
`ErrUnsupported` from 016), success-with-zero → honest zero (exact, spec edge case).

**Rationale**: `ListMultipartUploads` does not return sizes — only `ListParts` does, at one
request per upload. A dangling-MPU population is typically tens, so 100 sized uploads covers
the real case at bounded cost (≤100 extra requests, explicit-action-only path); an unbounded
ListParts fan-out would recreate the US1 problem on a different API. Verbs `List*` keep
`check-readonly.sh:43-45` green; SDK symbols (`CreateMultipartUpload` in the integration
seeder) stay inside `internal/storage` (excluded at `check-readonly.sh:21-27`).

**Alternatives considered**: no sizing at all (honest but guts the "what is wasted" promise —
size IS the operator question); sizing everything (unbounded request fan-out); RGW admin API
`bucket stats` (out of scope, non-S3).

## D7. Health card = new full-screen `modeHealth`

**Decision**: New `mode` constant (after `modeAddBucket`, `internal/ui/app.go:25-33`), entered
via `keys.Health = "H"` / `:health` from a bucket/prefix focus, restored via the existing
`prevMode` pattern (`app.go:130`, `app.go:863-868`). The card renders: header (target +
exact/lower-bound state), totals, 6-row age histogram, 6-row size histogram, class
distribution, MPU block (lazy probe under `healthGen`), warning lines. Histogram bars reuse
`usageBar` (`internal/ui/analyze.go:279-287`); tables reuse the shared table/`colHeadStyle`
components.

**Rationale**: Clarification decision. The inline details zone has ~11 body rows at 130×24
(016 budget analysis, `boxViewWith` hard-cap at `internal/ui/styles.go:348-350`); the card
needs ~20+. A dedicated mode sidesteps the budget fight, gives operators the SC-005 "one
screen", and the mode/`prevMode` machinery is already proven by `modeHelp`/`modeObject`.

**Alternatives considered**: sections cycled in the pane (rejected in clarify — card sliced to
ribbons); overlay popup like reveal (reveal popup is value-sized, not page-sized; footer
interaction untested at that size).

## D8. Small-object warning defaults: 128 KiB threshold, 50% share

**Decision**: Warning fires when `share(objects < 128 KiB) > 0.5` over the enumerated set.
Config keys `healthSmallObjectKiB` (default 128) and `healthSmallObjectShare` (default 0.5),
same validation/plumbing as D2. Card line names both numbers (spec FR-023).

**Rationale**: RGW bucket-index pressure is driven by object *count*, not bytes; 128 KiB is
well under any sane multipart threshold and matches the lowest histogram edge (D5) so the
evidence for the warning is visible in the same view. 50% share avoids crying wolf on mixed
buckets.

**Alternatives considered**: absolute-count trigger (scale-dependent, false on huge balanced
buckets); no configurability (operators with intentional small-object workloads — e.g.
thumbnails — need to retune or silence it).

## D9. Presigned GET: `s3.NewPresignClient` inside `internal/storage`, URL treated as a secret

**Decision**: New read-view method `PresignGet(ctx, bucket, key, ttl time.Duration) (URL
string, warn string, err error)` implemented with `s3.NewPresignClient(client)` +
`PresignGetObject` — pure client-side SigV4 signing, zero network calls. TTL is one of the
four spec presets only (15m/1h/24h/7d — FR-015). `warn` is non-empty when the resolved
`aws.Credentials` have `CanExpire && Expires` before `now+ttl` (FR-017). Logging records
`{op:"presign", bucket, key, ttl}`; the URL string itself NEVER reaches slog (constitution V;
same posture as `logging.Secret`).

**Rationale**: The presigner ships in the already-imported `service/s3` module — no new
dependency; signing needs the credential provider chain that only `internal/storage` may
touch (constitution I + 014 keychain/cmd sources). Symbol verb `Presign` is outside the
guard's verb set (`check-readonly.sh:43`) and the method is genuinely read-only (it can only
mint GET capabilities).

**Alternatives considered**: hand-rolled SigV4 (error-prone, duplicate of SDK); presign in a
new package (would need SDK import outside `internal/storage` — guard violation); allowing
PUT presign (write capability — violates the read-only posture and the spec).

## D10. Copy menu: `Y` overlay; clipboard = existing OSC52 path; display fallback

**Decision**: `keys.CopyMenu = "Y"` opens a small list overlay (shares the list-overlay
pattern of the connections manager, constitution VII); items are focus-aware (spec
clarification): object → URI / HTTPS URL / download command / presigned link (TTL sub-pick);
bucket/prefix → URI; health card or usage report visible → export CSV / export JSON. Copy
emits the SAME best-effort OSC52 clipboard command already used by reveal
(`internal/ui/reveal.go:10,43,81`, test `spec012_test.go:86-93`); the fallback when OSC52
cannot land is the reveal popup showing the full value for manual copy (FR-019). Footer
confirms "copied <what> for <key>".

**Rationale**: `y` is taken by write-copy (`keys.go:59`); `Y` is free and stays in the yank
family. OSC52 works over SSH (where an external `pbcopy` does not exist) and is already
tested in this codebase — zero new clipboard machinery.

**Alternatives considered**: exec `pbcopy`/`xclip` (platform matrix + fails over SSH —
rejected as primary; OSC52 covers local terminals too); separate hotkeys per artifact
(5 bindings + hintbar noise — rejected in clarify); `:copy <kind>` commands only (kept as
synonyms via the command bar, not the primary path).

## D11. Export: `internal/share` serializers + `DownloadDir` destination

**Decision**: New pure package `internal/share`: link/snippet builders (D12) plus
`ExportCSV(report)`/`ExportJSON(report)` returning bytes; the UI writes them via a `tea.Cmd`
into the existing configured `DownloadDir` (default-dir machinery from 005,
`internal/config/config.go:33`) as
`s3s-report-<bucket>[-<prefix-slug>]-<YYYYMMDD-HHMMSS>.{csv,json}`. Write failures surface in
the footer error line; on failure the partial file is removed (spec edge case). Footer +
log line name the absolute path.

**Rationale**: Serialization is domain logic → UI-agnostic package (constitution I), trivially
unit-tested (partial-vs-exact labelling included in the schema — `bounded` column/field).
`DownloadDir` is where users already expect s3s file artifacts; no new config knob.

**Alternatives considered**: cwd destination (surprising under launchers; cwd ≠ user intent);
clipboard-only export (reports exceed OSC52 practical limits); new `internal/export` package
(one-package split suffices — share + export are both "turn browse state into an artifact").

## D12. Link/snippet builders: pure functions, style-aware URL

**Decision**: `internal/share` pure builders: `S3URI(bucket, key)`;
`HTTPURL(endpoint, pathStyle bool, bucket, key)` honoring the context's `PathStyle` config
(`internal/config/config.go:57`) — path-style `https://host/bucket/key` vs virtual-host
`https://bucket.host/key`; `CLISnippet` (an `aws s3api get-object` form carrying
`--endpoint-url`) and `CurlSnippet` (curl against the presigned URL). All builders
percent-escape keys correctly (unit-tested with spaces/unicode/`+`).

**Rationale**: The UI already holds endpoint + path-style via the resolved context
(`m.info.Endpoint`, `keys.go:192`-area help view); building strings from them is pure and
must not live in render code (constitution I). Virtual-host vs path-style matters on real
Ceph installs (memory: Avito RGW exposes BOTH styles on different hosts) — honoring the
configured style reproduces what the user's own tooling expects.

**Alternatives considered**: always path-style (breaks vhost-only endpoints); asking the
backend (no S3 API for it); embedding in `internal/storage` (no SDK needed — keep the SDK
package minimal).

## D13. Preview upgrades: classify-then-transform in `internal/preview`, caps preserved

**Decision**:
- **gzip**: detect by magic bytes `1f 8b` on the fetched range (primary), with
  `Content-Encoding: gzip` and `.gz` suffix as corroborating hints; decompress with
  `compress/gzip` into an output capped at `preview.Limit` (5 MiB,
  `internal/preview/text.go:15`) via `io.LimitReader` — compression-bomb-safe by
  construction. Show "N compressed → M shown (truncated)" line. The decompressed bytes
  re-enter `Classify` (`text.go:74`) so a gzipped JSON pretty-prints.
- **JSON/NDJSON**: `Classify` already maps JSON content types (`text.go:51-60`); add
  `KindJSON`/`KindNDJSON` discrimination (NDJSON = >1 newline-separated parseable values).
  Pretty = `json.Indent` (object/array) or per-line indent (NDJSON), computed once per
  payload; parse failure → silent raw fallback (FR-025). Toggle key `p` (unbound today,
  `keys.go:43-74`) flips pretty↔raw in `modeObject`.
- **hexdump**: `encoding/hex.Dumper`-format builder (offset + 16 hex bytes + printable
  column) replacing the one-line binary summary as the scrollable body; the summary line
  stays as the header.

**Rationale**: All three are pure transforms over bytes the preview already fetched — zero new
requests, zero new dependencies, all unit-testable in `internal/preview` without Bubble Tea
(constitution I). Magic-bytes-first detection works when ranged GET strips
`Content-Encoding` semantics or the key lacks `.gz`.

**Alternatives considered**: syntax highlighting (chroma — heavy dependency, palette-role
conflict with constitution VII; revisit later); zstd/bzip2 (no stdlib support — gzip covers
the dominant log case; extensible later); streaming decompression UI (preview is capped at
5 MiB — batch transform is instant at that size).

## D14. Relative dates: pure helper with injected clock

**Decision**: `relTime(now, t)` pure helper in `internal/ui` rendering "3d ago"-style
durations next to the exact `formatDate` output (`internal/ui/styles.go:596`); `App` carries
`now func() time.Time` (defaulting to `time.Now`, fixed in tests) — used by metadata rows
(US2) and health-card age labels (US4).

**Rationale**: Deterministic white-box view tests (the established `App.View().Content`
assertion style) require an injectable clock; one helper keeps the dual format consistent
across pane + card (constitution VII).

**Alternatives considered**: a relative-time dependency (stdlib arithmetic suffices for 6
coarse units); only-relative display (loses the exact timestamp — VI requires the value
itself to stay readable).

## D15. Multipart ETag annotation: derive from the ETag shape, no extra call

**Decision**: In `metaFieldRows` (`internal/ui/metadata.go:42`), an ETag matching
`^[0-9a-f]{32}-(\d+)$` renders the existing value plus annotation `(multipart, N parts — not
a content hash)`; plain 32-hex ETags render unchanged. Presentation-only.

**Rationale**: The part count is encoded in the ETag suffix S3-wide; `HeadObject` already
delivered the value (016 US1) — no `GetObjectAttributes` round-trip needed for the spec's
explanatory promise (FR-011).

**Alternatives considered**: `GetObjectAttributes` for true part sizes (extra request + not
universally supported on RGW versions; the spec asks for explanation, not part listing).
