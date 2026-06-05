# Phase 1 Data Model: Object Write Operations

Entities from the spec mapped to concrete types. Storage types live in
`internal/storage`; UI state lives in `internal/ui`; the local browser entry lives
in `internal/localfs`. No persisted/datastore schema — these are in-memory runtime
types.

## Storage layer (`internal/storage`)

### Mutator (extended)

The 002 `Mutator` interface gains the object write methods. The real client,
`Fake`, and `readOnlyGuard` all implement every method.

| Method | Classification | Notes |
|--------|----------------|-------|
| `CreateFolder(ctx, bucket, prefix)` | reversible | existing (002) |
| `RemoveObject(ctx, bucket, key)` | destructive | US1 |
| `UploadFile(ctx, bucket, key, r io.Reader, size int64)` | reversible / overwrite→destructive | US2; UI does counting for progress |
| `CopyKey(ctx, bucket, srcKey, dstKey)` | reversible / overwrite→destructive | US3; same bucket |
| `MoveObject(ctx, bucket, srcKey, dstKey)` | destructive | US4; copy+delete, no data loss |
| `DeleteRecursive(ctx, bucket, prefix, onProgress)` | destructive | US5; best-effort; returns `DeleteSummary` |

Method names are guard-safe (avoid the `check-readonly.sh` verb+entity pattern) so
UI code may call them; the SDK symbols (`DeleteObject`, `PutObject`, `CopyObject`,
`DeleteObjects`) stay inside `internal/storage`. See
`contracts/object-mutator-interface.md` for exact signatures, the naming table, and
behaviour.

### DeleteSummary

The truthful result of a recursive delete (FR-011, SC-006).

| Field | Type | Meaning |
|-------|------|---------|
| `Deleted` | `int` | objects successfully removed |
| `Failed` | `int` | objects that could not be removed |

Invariants: `Deleted + Failed` == objects enumerated and attempted. `Failed > 0`
=> the operation is reported as **partial**, never a clean success. A cancelled run
returns the counts achieved before cancellation plus a non-nil error.

### Error sentinels (added)

| Sentinel | When |
|----------|------|
| `ErrMovePartial` | move's copy succeeded but source delete failed (data exists at destination; source remains) — FR-007 |

Reused from 001/002: `ErrNotFound`, `ErrAccessDenied`, `ErrUnreachable`,
`ErrInvalidConfig`, `ErrReadOnly`, `ErrInvalidName` (reused for invalid destination
key / identical src==dst). No secrets in any error (FR-014).

## Local filesystem (`internal/localfs`)

### Entry

One row in the upload file browser.

| Field | Type | Meaning |
|-------|------|---------|
| `Name` | `string` | base name |
| `Path` | `string` | absolute path |
| `IsDir` | `bool` | directory vs file |
| `Size` | `int64` | bytes (files only; 0 for dirs) |

Ordering: directories first, then files, each alphabetical. `ReadDir` surfaces a
classifiable error for unreadable/permission-denied directories.

## UI layer (`internal/ui`)

### operation (extended)

The in-flight mutating intent (extends the 002 `operation`). New kinds and phases;
existing `tier`/`expect`/`input`/`phase` reused.

| Field | Type | Meaning |
|-------|------|---------|
| `kind` | `string` | `delete_object` \| `upload` \| `copy` \| `move` \| `delete_recursive` (+ existing `create_folder`) |
| `bucket` | `string` | current bucket |
| `parent` | `string` | parent prefix of the level the op started in |
| `srcKey` | `string` | source object key (delete/copy/move) |
| `dstKey` | `string` | destination key entry (copy/move) |
| `prefix` | `string` | target prefix (recursive delete) |
| `localPath` | `string` | chosen upload source (upload) |
| `localSize` | `int64` | source file size, for progress total (upload) |
| `target` | `string` | resolved identifier acted on; for confirm + logging |
| `tier` | `confirmTier` | `confirmSimple` \| `confirmTyped` (overwrite escalates copy/upload to typed) |
| `expect` / `input` | `string` | typed-tier exact-match strings (byte-for-byte) |
| `phase` | `opPhase` | see below |
| `progress` | `opProgress` | live counters during `phaseRunning` |

### opPhase (extended)

| Phase | Used by | Meaning |
|-------|---------|---------|
| `phaseBrowse` | upload | local file browser open (NEW) |
| `phaseDest` | copy, move | destination-key text entry (NEW) |
| `phaseName` | create_folder | folder-name entry (existing) |
| `phaseConfirm` | all | confirmation overlay (existing) |
| `phaseRunning` | all | dispatched; awaiting progress/outcome (existing) |

Flows:
- delete_object: (select) → phaseConfirm(typed) → phaseRunning
- upload: phaseBrowse → [overwrite? escalate] → phaseConfirm → phaseRunning(progress)
- copy: phaseDest → [overwrite? escalate] → phaseConfirm → phaseRunning
- move: phaseDest → phaseConfirm(typed) → phaseRunning
- delete_recursive: phaseConfirm(typed, expect=prefix) → phaseRunning(progress)

### opProgress

Live progress for streaming ops, rendered during `phaseRunning`.

| Field | Type | Meaning |
|-------|------|---------|
| `uploaded` | `int64` | bytes uploaded so far (upload) |
| `total` | `int64` | total bytes (upload) / objects known (recursive) |
| `deleted` | `int` | objects removed (recursive) |
| `failed` | `int` | objects failed (recursive) |

### Outcome messages (extended)

- `operationProgressMsg{gen, progress opProgress}` — NEW; one tick of live progress.
- `operationDoneMsg{gen, err, summary *DeleteSummary, partial bool}` — extends the
  002 message: `summary` non-nil for recursive delete; `partial==true` for a
  recursive partial or a move that hit `ErrMovePartial`. A non-nil `err` (incl.
  `context.Canceled`) or `partial==true` is **never** rendered/logged as a clean
  success (FR-011).

## Relationships & lifecycle

```text
operator key → operation{kind,phase} ──(browse/dest/name)──▶ phaseConfirm
                                                                  │ tier
                                                  simple ◀────────┴────────▶ typed (destructive/overwrite)
                                                       │                        │ exact match
                                                       └──────────┬─────────────┘
                                                                  ▼
                                                            dispatchOp() → tea.Cmd (goroutine)
                                                                  │
                              streaming ops ──▶ progress channel ─┼─▶ waitForProgress → operationProgressMsg*
                                                                  ▼
                                                          operationDoneMsg → invalidate affected level(s) → reload
```

A superseded generation or a cancelled context drops late progress/outcome messages
(Constitution II, FR-010); the subsequent reload shows ground truth (FR-016).
