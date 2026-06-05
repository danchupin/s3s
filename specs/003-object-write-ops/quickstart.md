# Quickstart: Object Write Operations (003)

How to exercise the five object write operations once implemented. Builds on the
002 write foundation — operations only work with `--write` and on a non-`readonly`
context.

## Prerequisites

- A writable context (a context **not** marked `readonly: true` in config).
- Launch with writes enabled:

  ```bash
  s3s --write
  ```

  Without `--write`, every operation below is refused with a read-only hint and
  storage is unchanged (the safety contract from 002).

## Try each operation

Navigate into a writable bucket, then:

### Delete an object (`d`)
1. Select an object, press `d`.
2. A **typed** confirmation appears — type the object's exact key, press Enter.
   A mismatch aborts with no change.
3. After the level refreshes, the object is gone. The action + outcome are in the
   log file.

### Upload a local file (`u`)
1. At the level where you want the object, press `u`.
2. A **local file browser** opens at your working directory. Navigate with the
   arrow keys (`enter` to descend into a folder, `esc`/`backspace` to go up), and
   press `enter` on a file to choose it.
3. If the target key is new, a **simple** confirmation; if a key with that name
   already exists, a **typed overwrite** confirmation.
4. Progress is shown while the file uploads; the interface stays responsive and the
   upload is cancellable. After refresh, the object appears.

### Copy an object (`c`)
1. Select an object, press `c`.
2. A destination-key field opens, prefilled with the source key — edit it to the
   new key (same bucket).
3. Free destination → simple confirm; existing destination → typed overwrite.
4. After refresh, the object exists at both the source and the destination.

### Move / rename an object (`m`)
1. Select an object, press `m`.
2. Edit the destination key. Because the source is removed, a **typed** confirmation
   is always required.
3. After refresh, the object appears only at the new key. If the copy succeeds but
   the source could not be deleted, you get a **partial** message (data safe at the
   destination; source still present) — never a false success.

### Delete a folder/prefix recursively (`D`)
1. Select a folder (common prefix), press `D` (shift-d).
2. A **typed** confirmation demands the exact prefix.
3. Progress shows a running deleted/failed count; the operation is cancellable.
4. Best-effort: it removes everything it can; if some objects fail, you get a
   partial report (e.g. "deleted 42, 3 failed"), not a clean success. After refresh
   the prefix is gone (or its survivors are shown truthfully).

## Verify safety

- On a `readonly` context (or without `--write`), every key above is refused —
  storage unchanged.
- Inspect the log file: each attempt logs action / source / destination / context
  before execution and the outcome after, with no secrets.

## Run the tests

```bash
make test              # unit: storage Fake + guard refusals + UI flows + localfs
make test-integration  # MinIO: real delete/upload/copy/move/recursive + partial paths
make check-readonly    # confirms new SDK mutations stay inside internal/storage
make fmt vet lint
```

Integration tests `t.Skip` when no Docker provider is found; for Lima, set
`DOCKER_HOST` to the Lima socket and `TESTCONTAINERS_RYUK_DISABLED=true`.
