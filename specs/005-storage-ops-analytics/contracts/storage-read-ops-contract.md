# Contract: Storage read-op additions (`GetObject`, `UsageOf`)

Two **read-only** methods added to `storage.Storage`. Both pass through `readOnlyGuard`
unchanged, are usable in read-only contexts, and introduce no write SDK symbols
(`check-readonly.sh` unaffected).

## C1 — `GetObject(ctx, bucket, key) (io.ReadCloser, error)` (US1)

- **MUST** stream the full object body (no range cap, unlike `GetObjectRange`).
- **MUST** honor `ctx` cancellation: a cancelled read stops and the caller closes the reader.
- **MUST** classify failures into the existing sentinels (`ErrNotFound`, `ErrAccessDenied`,
  `ErrUnreachable`) without leaking detail.
- Caller owns closing the returned reader. The method itself writes nothing locally and mutates
  nothing remotely.
- **Guard**: `readOnlyGuard` exposes it via the embedded `Storage` (no override) — available
  read-only (FR-002).

**Tests**: unit (Fake) returns seeded bytes byte-for-byte; integration (MinIO) downloads a
large/multipart-sized object and verifies the full length + content; cancellation mid-stream
surfaces `ctx.Err()`; missing key → `ErrNotFound`.

## C2 — `UsageOf(ctx, bucket, prefix, onProgress) (UsageReport, error)` (US2)

- **MUST** recursively aggregate every object under `prefix` (paginated, delimiter-less list),
  returning `TotalSize`, `TotalCount`, and `Children` (immediate sub-prefixes + direct objects).
- `Children` **MUST** be ranked by `Size` descending, ties by `Name` ascending (FR-009).
- An immediate child is the first path segment after `prefix`: a **sub-prefix** (`IsDir=true`,
  trailing `/`) if more `/` follow, else a **direct object** (`IsDir=false`).
- **MUST** call `onProgress` (when non-nil) periodically with running `ScannedCount/ScannedSize`
  (FR-011); **MUST NOT** block the caller beyond pagination I/O.
- On `ctx` cancellation: return the partial `UsageReport` with `Complete=false` and `ctx.Err()`
  (truthful partial, FR-011).
- Empty prefix: `TotalSize=0, TotalCount=0, Children=nil, Complete=true`, **no error** (FR-012).
- Accounting is **current versions only** (a plain list); delete markers / historical versions are
  excluded (Assumption).

**Tests**: unit (Fake) with a known size distribution asserts totals + child ranking + shares;
empty prefix → zero report; integration (MinIO) crosses a pagination boundary (>1000 keys) and a
deep nested prefix; cancellation returns `Complete=false` with partial counts.

## C3 — Guard & read-only invariant

- `readOnlyGuard` gains **no** overrides for C1/C2 (they are reads). A guard test asserts both
  succeed through the guard (parity with the underlying client).
- `scripts/check-readonly.sh` stays green: `GetObject`/`ListObjectsV2` are read SDK symbols and
  live only in `internal/storage`; no UI package imports the SDK.
