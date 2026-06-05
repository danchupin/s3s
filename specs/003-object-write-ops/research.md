# Phase 0 Research: Object Write Operations

Feature: 003-object-write-ops. Resolves the open technical choices behind the plan.
Each item: Decision / Rationale / Alternatives.

## R1. Upload mechanism — single PutObject vs multipart manager

- **Decision**: Use `feature/s3/manager`'s `Uploader` for all uploads. It picks
  single-part for small files and automatic multipart for large ones, exposing a
  single call that streams from an `io.Reader`.
- **Rationale**: One code path covers small and large files (US2 large-file edge
  case). Multipart gives resumable-sized chunks and avoids loading the whole file
  into memory. The `Uploader` honors `context.Context` cancellation (FR-010).
- **Alternatives**: Raw `PutObject` only — simplest, but buffers/streams a single
  body and is awkward for very large files and progress; rejected. Hand-rolled
  multipart (`CreateMultipartUpload`/`UploadPart`/`Complete`) — more control but
  reinvents the manager; rejected.

## R2. Upload progress reporting

- **Decision**: The UI wraps the local file's `io.Reader` in a **counting reader**
  that reports bytes read to a callback; the upload `tea.Cmd` runs the `Uploader`
  in a goroutine and pushes throttled `{uploaded, total}` updates onto a buffered
  channel. A `waitForProgress(ch)` command reads one update and returns an
  `operationProgressMsg`, re-issuing itself until the terminal `operationDoneMsg`.
- **Rationale**: Keeps progress measurement out of the `storage` interface (the
  method signature stays `UploadObject(ctx, bucket, key, r, size)`), so the core
  stays clean and the counting wrapper is a UI concern. The channel +
  `waitForProgress` loop is the idiomatic Bubble Tea streaming pattern and keeps the
  frame non-blocking (Constitution II). Throttle to ≤1 update / ~50 ms so a fast
  upload does not flood the event loop.
- **Alternatives**: Pass a `progress func()` into `UploadObject` — leaks UI cadence
  into the storage contract and complicates `Fake`; rejected. `manager`'s body with
  no progress — fails the live-progress success criterion for large files; rejected.

## R3. Recursive delete — enumeration + batched deletes + best-effort

- **Decision**: `DeleteRecursive` paginates `ListObjectsV2` over the prefix and
  deletes in batches with `DeleteObjects` (up to 1000 keys/call, the S3 limit). It
  is **best-effort**: per-object errors in the `DeleteObjects` response are counted,
  not fatal; enumeration and deletion continue to completion. Returns a
  `DeleteSummary{Deleted, Failed int}`. A progress callback reports
  `{deleted, failed}` after each batch.
- **Rationale**: Matches the clarified best-effort policy (no abort-on-first, no
  per-failure prompt) and FR-009/FR-011. `DeleteObjects` is far fewer round-trips
  than per-key `DeleteObject`. Counting `Errors` in the batch response yields the
  truthful deleted/failed split (SC-006).
- **Alternatives**: Per-object `DeleteObject` in a loop — simpler error attribution
  but 1000× the round-trips; rejected for scale. Abort-on-first — contradicts the
  clarification; rejected.

## R4. Recursive delete progress + cancellation

- **Decision**: Same channel + `waitForProgress` pattern as upload. The goroutine
  pushes `{deleted, failed, total?}` after each batch; cancellation via the
  operation's `context.CancelFunc` stops further enumeration/deletion. A cancelled
  run reports the partial counts achieved so far and is **never** a clean success
  (FR-011, US5 AS4).
- **Rationale**: Reuses one progress mechanism for both streaming ops. Total is not
  known until enumeration completes, so progress shows a running deleted/failed
  count (and total once known) rather than a strict percentage.
- **Alternatives**: Pre-count all keys before deleting to show a percentage —
  doubles the listing cost on huge prefixes; rejected. Show only a spinner — loses
  the "how many removed" signal users want; rejected.

## R5. Move / rename — composition + no-data-loss

- **Decision**: `MoveObject(ctx, bucket, srcKey, dstKey)` = server-side
  `CopyObject(src→dst)` then `DeleteObject(src)`. If the copy fails, return the
  classified error and do **not** delete the source. If the copy succeeds but the
  delete fails, return a distinct `ErrMovePartial` (the data exists at the
  destination; the source remains). Lives in `storage` so the ordering guarantee is
  core logic and integration-testable.
- **Rationale**: S3 has no native move; copy-then-delete is the standard. Putting
  the ordering and the partial sentinel in `storage` (not the UI) satisfies
  Constitution I and lets MinIO integration tests prove no-data-loss (FR-007, SC-005).
- **Alternatives**: UI orchestrates two `Mutator` calls — pushes core logic into the
  TUI (violates I) and is harder to test; rejected.

## R6. Overwrite / collision detection (upload + copy)

- **Decision**: Detection is **advisory**, computed in the UI from the already-loaded
  level listing (the same approach 002 uses for create-folder "already exists"). If
  the destination key is present in the current level, the UI escalates the op to
  the typed-confirmation tier ("overwrite"); otherwise it uses the simple tier. The
  storage methods do **not** hard-precondition existence (PutObject/CopyObject
  overwrite at the S3 level).
- **Rationale**: A definitive server-side existence check (`HeadObject`) before every
  upload/copy adds a round-trip and a TOCTOU gap for marginal benefit; the advisory
  check from the listing is consistent with 002 and satisfies SC-004 for the normal
  case. Confirmation (not detection) is the real safety gate.
- **Alternatives**: `HeadObject` pre-check — extra latency, still racy; rejected as
  the default (may be reconsidered later). Always typed tier for upload/copy —
  punishes the common non-colliding case; rejected.

## R7. Local file browser for upload source

- **Decision**: A new UI-agnostic `internal/localfs` package exposes
  `ReadDir(path) ([]Entry, error)` (entries sorted dirs-first then name, each
  flagged dir/file with size) and `IsReadableFile(path) error`. A new
  `internal/ui/filebrowser.go` renders the listing and handles keys (navigate
  in/out, select a file, cancel). Starts at the process working directory; `..`
  ascends.
- **Rationale**: Keeps filesystem logic out of Bubble Tea so it is unit-testable
  (Constitution I), mirroring how `storage` keeps S3 out of the UI. A keyboard
  browser fits the k9s-style UX and the clarified choice (in-TUI browser, not a
  typed path).
- **Alternatives**: Typed path prompt — rejected by clarification. A third-party
  file-picker bubble — extra dependency for a small need; rejected.

## R8. Destination-key entry for copy/move

- **Decision**: A text-entry phase (reusing the existing name-input rendering from
  create-folder) prefilled with the source key, letting the operator edit it to the
  destination key within the current bucket. Reject empty/whitespace, invalid key
  characters, and a destination identical to the source (no-op) before any call
  (FR-013).
- **Rationale**: Copy/move targets are S3 keys, not local paths, so a text field is
  the right control; prefilling with the source key makes rename ergonomic. Same
  validation path as `normalizeFolderKey` keeps rejection consistent.
- **Alternatives**: A key browser within the bucket — heavier; the destination often
  does not exist yet, so browsing has limited value; rejected for this feature.

## R9. Keybindings

- **Decision**: On an object: `d` delete (typed confirm), `c` copy, `m` move/rename,
  `u` upload (into current level). On a folder/common-prefix selection: `D`
  (shift-d) recursive delete (typed confirm of the prefix). Confirmation overlay and
  file browser consume keys while active. Final spelling is a task-level detail; all
  are surfaced in the footer hints.
- **Rationale**: Single-letter, mnemonic, consistent with existing bindings; capital
  `D` distinguishes the higher-risk recursive delete from single `d`.
- **Alternatives**: A command palette — out of scope; rejected for now.

## R10. Cache invalidation after writes

- **Decision**: On a successful or partial outcome, invalidate the affected
  level(s): the current level for delete/upload/copy; for move, both the source and
  destination levels; for recursive delete, the parent level of the removed prefix.
  Then reload so the view reflects ground truth (FR-016).
- **Rationale**: The per-session cache (keyed by context/bucket/prefix/search) is
  otherwise TTL-free; an explicit invalidation keeps the listing truthful after a
  mutation without a global flush. Matches 002's post-create-folder refresh.
- **Alternatives**: Flush the whole cache — correct but discards unrelated cached
  levels; rejected. Optimistically mutate the cached listing — risks asserting an
  outcome the backend did not produce (violates FR-016); rejected.

## Open questions

None blocking. Copy/move destination-key UX and upload progress granularity were
deferred from clarification and are resolved above (R8, R2) with reasonable
defaults; nothing here requires returning to `/speckit-clarify`.
