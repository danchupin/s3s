# Contract: Storage Mutator Extension + Read-Only Guard (003)

**Package**: `internal/storage` | **Feature**: 003-object-write-ops

Extends the 002 `Mutator` interface with the object write operations. Every new
method's SDK call lives only in `internal/storage`.

## ⚠️ Naming constraint (check-readonly.sh)

`scripts/check-readonly.sh` fails the build if any identifier matching
`\b(Put|Delete|Create|Copy|Upload|Restore|Write)(Object|Bucket|MultipartUpload|
Part|Tagging|Acl|…)[A-Za-z]*\b` appears in a `.go` file **outside**
`internal/storage/` — comments included, case-sensitive. Because the UI calls these
interface methods, their **Go identifiers MUST avoid that verb+entity pattern**.
Chosen safe names (verb not in the set, or entity not an S3 entity):

| Operation | Go method (UI-callable, guard-safe) | Why safe | Internal SDK call (storage only) |
|-----------|-------------------------------------|----------|----------------------------------|
| delete one | `RemoveObject` | `Remove` ∉ verb set | `DeleteObject` |
| upload | `UploadFile` | `File` ∉ entity set | `manager.Uploader` / `PutObject` |
| copy | `CopyKey` | `Key` ∉ entity set | `CopyObject` |
| move/rename | `MoveObject` | `Move` ∉ verb set | `CopyObject` + `DeleteObject` |
| recursive delete | `DeleteRecursive` | `Recursive` ∉ entity set | `ListObjectsV2` + `DeleteObjects` |
| (result type) | `DeleteSummary` | `Summary` ∉ entity set | — |

The SDK symbols (`DeleteObject`, `PutObject`, `CopyObject`, `DeleteObjects`) appear
ONLY inside `internal/storage`, where the guard allows them.

## Go interface

```go
package storage

import (
    "context"
    "errors"
    "io"
)

// Mutator adds write capability on top of Storage. The real client, the in-memory
// Fake, and readOnlyGuard all implement it. Mutating S3 calls live ONLY in this
// package (scripts/check-readonly.sh enforces it). Method names deliberately avoid
// the guard's verb+entity pattern so UI code may reference them.
type Mutator interface {
    // CreateFolder — existing (002).
    CreateFolder(ctx context.Context, bucket, prefix string) error

    // RemoveObject removes a single object. Returns ErrReadOnly (no network call)
    // when read-only; ErrNotFound if the key is already gone (treated as benign by
    // the UI). FR-001, FR-015.
    RemoveObject(ctx context.Context, bucket, key string) error

    // UploadFile creates/overwrites the object at key from r (size bytes). Uses the
    // multipart manager so large files stream without buffering and honor ctx
    // cancellation. Progress is measured by the caller wrapping r (the signature
    // stays progress-free). FR-002, FR-003.
    UploadFile(ctx context.Context, bucket, key string, r io.Reader, size int64) error

    // CopyKey server-side copies srcKey to dstKey within the same bucket; the source
    // is unchanged. Returns ErrInvalidName if dstKey is invalid or equals srcKey.
    // FR-004, FR-005, FR-013.
    CopyKey(ctx context.Context, bucket, srcKey, dstKey string) error

    // MoveObject = CopyKey(src→dst) then RemoveObject(src). If the copy fails, the
    // source is left intact and dst is not claimed. If the copy succeeds but the
    // source delete fails, returns ErrMovePartial (data exists at dst; src remains).
    // No-data-loss guarantee. FR-006, FR-007.
    MoveObject(ctx context.Context, bucket, srcKey, dstKey string) error

    // DeleteRecursive enumerates every object under prefix (paginated) and deletes
    // in batches, best-effort: a per-object failure does not abort the run. Returns
    // the deleted/failed counts. onProgress (may be nil) is called after each batch
    // with cumulative counts. A cancelled ctx stops further work and returns the
    // counts achieved so far with ctx.Err(). FR-008, FR-009, FR-011.
    DeleteRecursive(ctx context.Context, bucket, prefix string, onProgress func(DeleteSummary)) (DeleteSummary, error)
}

// DeleteSummary is the truthful outcome of a recursive delete.
type DeleteSummary struct {
    Deleted int
    Failed  int
}

// ErrMovePartial: move copied the object but could not delete the source. The data
// is safe at the destination; the source still exists. Never a clean success.
var ErrMovePartial = errors.New("storage: move copied object but source delete failed")
```

The concrete client and `Fake` satisfy `Storage` + `Mutator`. The UI depends on
these interfaces, never the SDK.

## Behaviour contract

### RemoveObject
- Read-only refusal returns `ErrReadOnly` before any network call.
- A not-found key returns `ErrNotFound`; the UI treats it as "already gone" (US1
  edge case) and the subsequent refresh shows it absent.
- Errors classified via `classify`; no secrets (FR-014/FR-015).

### UploadFile
- Streams via `manager.Uploader` (auto single/multipart). Honors `ctx`
  cancellation: a cancelled upload returns `ctx.Err()` and MUST NOT be reported as
  success; the object may be absent or partial — the next refresh reveals truth
  (US2 AS4, FR-016).
- Overwrites at the S3 level (no server-side precondition); the UI's typed overwrite
  confirmation (advisory, from the listing) is the gate (FR-003, SC-004).
- A missing/unreadable local source is rejected by the UI (via `localfs`) before
  this call; if a read error surfaces mid-stream, the upload fails and is not a
  success.

### CopyKey / MoveObject
- `CopyKey` is server-side within one bucket; source unchanged (FR-004).
- Reject `dstKey` that is empty/whitespace, has control characters, or equals
  `srcKey` → `ErrInvalidName`, before any network call (FR-013).
- `MoveObject` ordering is fixed: copy first, delete source only on copy success.
  Copy failure → classified error, source intact, no delete attempted. Copy ok +
  delete fail → `ErrMovePartial` (FR-007). Integration test proves both branches
  lose no data (SC-005).

### DeleteRecursive
- Enumerate with `ListObjectsV2` over `prefix` across all pages; delete in
  `DeleteObjects` batches (≤1000 keys). Count `Errors` entries in each batch response
  into `Failed`, successful deletions into `Deleted`; continue past failures
  (best-effort, FR-009).
- `onProgress` (nil-safe) is invoked after each batch with the cumulative
  `DeleteSummary` for live UI progress.
- `Failed > 0` ⇒ partial; the UI never shows a clean success (FR-011, SC-006).
- Cancellation stops further listing/deletion and returns the partial counts with
  `ctx.Err()` (US5 AS4).
- Read-only refusal returns `ErrReadOnly` immediately (guarded), nothing enumerated.

## Read-only guard (extended)

`readOnlyGuard` MUST override **every** new method to return `ErrReadOnly` without
touching the wrapped backend. Adding a `Mutator` method without a matching guard
override is a safety regression — a guard test enforces this.

```go
func (readOnlyGuard) RemoveObject(context.Context, string, string) error { return ErrReadOnly }
func (readOnlyGuard) UploadFile(context.Context, string, string, io.Reader, int64) error { return ErrReadOnly }
func (readOnlyGuard) CopyKey(context.Context, string, string, string) error { return ErrReadOnly }
func (readOnlyGuard) MoveObject(context.Context, string, string, string) error { return ErrReadOnly }
func (readOnlyGuard) DeleteRecursive(context.Context, string, string, func(DeleteSummary)) (DeleteSummary, error) {
    return DeleteSummary{}, ErrReadOnly
}
```

`Guard(b Storage, writable bool) Storage` is unchanged: writable ⇒ backend
unwrapped; otherwise the guard wraps it. Resolution stays at construction time (the
resolver closure), so the UI holds an already-correct backend.

## Test contract

- **Unit (fake)**: `Fake` implements each method against its in-memory map. Assert:
  remove deletes the key; not-found remove returns `ErrNotFound`; upload stores
  bytes; copy duplicates and leaves the source; move's two branches (clean move;
  copy-ok/delete-fail ⇒ `ErrMovePartial` with both keys present); recursive delete
  removes all under a prefix and reports correct `Deleted`/`Failed` (inject a fake
  that fails a chosen key); invalid/identical dst ⇒ `ErrInvalidName`.
- **Guard**: every mutating method on `readOnlyGuard` returns `ErrReadOnly` and the
  wrapped client (a stub that fails the test if any method is called) is never hit.
- **Integration (`//go:build integration`, MinIO)**: real remove; upload of a small
  and a large (multipart-triggering) file with byte-identical readback; server-side
  copy; move (clean + induced partial); recursive delete over a multi-page prefix;
  recursive partial-failure counts; access-denied paths leave storage unchanged;
  guard refuses each without contacting the server.
- **Guard CI**: `make check-readonly` passes — confirms the guard-safe method names
  do not trip the verb+entity scan in UI code.
