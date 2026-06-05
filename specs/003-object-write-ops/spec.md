# Feature Specification: Object Write Operations

**Feature Branch**: `003-object-write-ops`

**Created**: 2026-06-05

**Status**: Draft

**Input**: User description: "Object write operations for s3s: build on the 002
write foundation (global `--write` switch, per-context `readonly` override,
two-tier confirmation, non-blocking logged execution) to add the object-level
mutations operators actually need day to day — upload a local file, delete a
single object, copy an object to a new key, move/rename an object, and delete a
whole folder/prefix recursively — each safe, confirmed, observable, and logged."

## Clarifications

### Session 2026-06-05

- Q: Which object write operations belong in 003 scope? → A: Delete object
  (single), Upload object, Copy object, Move/rename object.
- Q: Is recursive delete of a folder/prefix (delete-all-under) in 003 scope? → A:
  Yes — in scope (highest-risk operation; typed confirmation, per-object progress,
  partial-failure handling).
- Q: How does the operator select the local file to upload? → A: A built-in,
  keyboard-driven local file browser inside the TUI (navigate the local filesystem
  and pick a file); no typed-path prompt.
- Q: What destination can a copy/move target? → A: Same bucket only — the operator
  enters a destination key within the current bucket. Cross-bucket copy/move is out
  of scope for this feature (a later increment).
- Q: On recursive delete, when an object fails to delete, what happens? → A:
  Best-effort — continue past the failure, delete every object possible, then report
  deleted and failed counts (no abort-on-first, no per-failure prompt).

## User Scenarios & Testing *(mandatory)*

The 002 foundation proved the safety scaffolding on a single low-risk mutation
(create-folder). This feature delivers the object-level write operations that make
s3s a usable read/write client: putting data in, taking data out, and reorganizing
it. Every operation reuses the foundation — it is refused unless writes are enabled
and the context is writable, it passes through the correct confirmation tier, it
runs without freezing the interface, and it is logged before and after execution.
Operations that destroy or overwrite data use the stronger typed-confirmation tier.

Each user story is an independent vertical slice: implementing any one of them on
top of 002 yields a usable increment.

### User Story 1 - Delete a single object (Priority: P1)

As an operator on a writable context, I want to delete the selected object after a
deliberate confirmation, so I can remove data I no longer need without fear of an
accidental keystroke destroying it.

**Why this priority**: Deleting a single object is the most-requested write after
create-folder and the cleanest exercise of the destructive (typed-confirmation)
tier against real data. It is the smallest slice that proves destructive object
mutation end-to-end.

**Independent Test**: On a writable context, select an object and trigger delete →
a typed-confirmation prompt demands the object's exact identifier → on a correct
match the object is removed and is gone after the level refreshes → a non-matching
entry aborts with no change → the action and outcome appear in the log.

**Acceptance Scenarios**:

1. **Given** a writable context and a selected object, **When** the operator
   triggers delete and types the exact target identifier to confirm, **Then** the
   object is deleted and no longer appears after the level refreshes.
2. **Given** the typed-confirmation prompt for a delete, **When** the operator
   enters a non-matching identifier or cancels, **Then** nothing is deleted and the
   operator returns to browsing.
3. **Given** a delete that the backend rejects (e.g. access denied), **When** the
   action fails, **Then** storage is unchanged and a clear, non-leaking error is
   shown.
4. **Given** any delete attempt (success, abort, or failure), **When** the operator
   inspects the log, **Then** the action, target, context, and outcome are recorded
   with no secrets present.

---

### User Story 2 - Upload a local file as an object (Priority: P1)

As an operator on a writable context, I want to upload a file from my local
machine into the current bucket/prefix, watch its progress without the interface
freezing, and be warned before overwriting an existing object, so I can add data
safely and observably.

**Why this priority**: Upload is the primary "write data in" capability and the
counterpart to delete; together they make s3s a two-way client. It also exercises
progress feedback for a potentially long transfer and the overwrite-detection path.

**Independent Test**: On a writable context, choose a local file and an upload to
the current level → progress is visible while it runs and the interface stays
responsive → the object appears after refresh → repeating the upload to the same
key warns that it will overwrite and requires the stronger confirmation → uploading
a missing/unreadable file fails cleanly with no partial object claimed.

**Acceptance Scenarios**:

1. **Given** a writable context and a readable local file, **When** the operator
   uploads it to a key that does not yet exist and confirms, **Then** the object is
   created with the file's contents and appears after the level refreshes.
2. **Given** an upload whose target key already exists, **When** the operator
   initiates it, **Then** the system detects the collision and requires the
   destructive (typed) confirmation for the overwrite before proceeding.
3. **Given** an upload in progress, **When** the operator observes the interface,
   **Then** progress is surfaced, the interface stays responsive, and the upload can
   be cancelled.
4. **Given** the chosen local file is missing, unreadable, or the upload is
   cancelled mid-flight, **When** the operation ends, **Then** it is not reported as
   success, storage is left in a state the next refresh reveals truthfully, and the
   outcome is logged.

---

### User Story 3 - Copy an object to a new key (Priority: P2)

As an operator on a writable context, I want to copy the selected object to a new
key within the current bucket without downloading and re-uploading it, so I can
duplicate data efficiently.

**Why this priority**: Copy is a non-destructive convenience that builds on the
same target-selection and collision-detection logic upload needs, and is the
prerequisite half of move/rename. It is P2 because delete and upload deliver the
core round-trip first.

**Independent Test**: On a writable context, copy an object to a new key → a simple
confirmation suffices when the destination is empty → the copy appears at the
destination and the source is unchanged after refresh → copying onto an existing
key triggers the overwrite (typed) confirmation.

**Acceptance Scenarios**:

1. **Given** a writable context and a selected object, **When** the operator copies
   it to a destination key that does not exist and confirms, **Then** the object
   exists at both the source and the destination after refresh.
2. **Given** a copy whose destination key already exists, **When** the operator
   initiates it, **Then** the overwrite requires the destructive (typed)
   confirmation before proceeding.
3. **Given** a copy initiated in a read-only context (or with writes disabled),
   **When** the operator triggers it, **Then** the operation is refused and storage
   is unchanged.

---

### User Story 4 - Move or rename an object (Priority: P2)

As an operator on a writable context, I want to move or rename the selected object
to a new key, so I can reorganize storage. Because object storage has no native
move, this is a copy followed by deletion of the source, and it must not lose data
if either half fails.

**Why this priority**: Move/rename is the most common reorganization action but
depends on copy (US3) existing and is destructive on the source, so it follows the
non-destructive copy. It is the highest-value P2.

**Independent Test**: On a writable context, rename an object to a new key → the
operation requires the destructive (typed) confirmation because the source is
removed → after refresh the object exists only at the new key and not the old one →
if the source deletion fails after a successful copy, the object remains at both
keys (no data loss) and the partial outcome is reported and logged.

**Acceptance Scenarios**:

1. **Given** a writable context and a selected object, **When** the operator
   moves/renames it to a new key and confirms with the typed confirmation, **Then**
   after refresh the object appears only at the destination and the source key is
   gone.
2. **Given** a move whose copy half succeeds but whose source-deletion half fails,
   **When** the operation ends, **Then** the data still exists (at least at the
   destination), the result is reported as a partial/failed move rather than a clean
   success, and both steps' outcomes are logged.
3. **Given** a move whose destination key already exists, **When** the operator
   initiates it, **Then** the overwrite is surfaced and requires confirmation before
   the source is removed.

---

### User Story 5 - Delete a folder/prefix recursively (Priority: P3)

As an operator on a writable context, I want to delete an entire folder/prefix and
everything under it in one action, after the strongest confirmation, with visible
progress and a truthful report of how many objects were removed, so I can clean up
whole trees without deleting objects one by one.

**Why this priority**: Recursive delete is the highest-risk operation in the
feature — it can destroy many objects at once — so it ships last, after the
single-object destructive path (US1) is proven. It is P3 because it is powerful but
less frequent and the most dangerous.

**Independent Test**: On a writable context, target a non-empty folder/prefix for
recursive delete → a typed confirmation of the exact prefix is required → progress
shows objects being removed and the action can be cancelled → after refresh the
prefix and its contents are gone → if some objects cannot be deleted, the operation
reports a partial result (counts of deleted vs failed) rather than a clean success
→ the action and outcome are logged.

**Acceptance Scenarios**:

1. **Given** a writable context and a non-empty folder/prefix, **When** the operator
   triggers recursive delete and types the exact prefix to confirm, **Then** every
   object under that prefix is removed and the prefix no longer appears after
   refresh.
2. **Given** a recursive delete in progress over many objects, **When** the operator
   observes the interface, **Then** progress (e.g. count or proportion removed) is
   surfaced, the interface stays responsive, and the operation can be cancelled.
3. **Given** a recursive delete where some objects fail to delete (e.g. access
   denied on a subset), **When** the operation ends, **Then** it reports a partial
   result with the number deleted and the number failed, is not reported as a clean
   success, and the next refresh reflects ground truth.
4. **Given** a recursive delete cancelled mid-flight, **When** the operation stops,
   **Then** the objects already deleted stay deleted, the outcome is reported as
   cancelled/partial (never clean success), and a refresh reveals the true remaining
   contents.

---

### Edge Cases

- **Delete of an object that has already disappeared** (deleted by someone else
  between listing and confirming): the operation completes or reports not-found
  without error noise; the refresh shows the object gone either way.
- **Upload target key already exists**: treated as an overwrite — destructive tier,
  never a silent replace.
- **Upload source unreadable, missing, empty, or changed during read**: fail
  cleanly before or during transfer; never claim success for a partial object.
- **Upload/move/copy of a very large object**: progress is surfaced and the
  operation is cancellable; a cancel leaves an indeterminate result resolved by the
  next refresh.
- **Copy/move destination equals the source key**: reject as a no-op with guidance
  rather than deleting the source.
- **Copy/move destination key collides with an existing object**: overwrite is
  surfaced and requires the typed confirmation.
- **Move where copy succeeds but source delete fails**: no data loss — object
  remains at the destination (and possibly the source); reported as partial, logged.
- **Recursive delete of an empty or non-existent prefix**: nothing to do; reported
  clearly, no error.
- **Recursive delete spanning many pages of objects**: all matching objects are
  enumerated and removed; progress reflects the whole set, not just the first page.
- **Recursive delete partial failure**: deleted-vs-failed counts are reported; never
  a clean success when any object failed.
- **Any operation in a read-only context or with writes disabled**: refused,
  storage unchanged, explanatory message (read-only always wins).
- **Invalid destination key** (empty, whitespace-only, invalid key characters):
  rejected with guidance before any backend call.
- **Cancellation or context switch mid-operation**: a superseded or cancelled result
  must not corrupt the view of another context/level; outcome is indeterminate and
  never shown as a clean success.

## Requirements *(mandatory)*

The 002 foundation requirements (write enablement via `--write`, per-context
`readonly` override, two confirmation tiers, non-blocking execution, pre/post
logging, secret redaction, indeterminate-on-cancel semantics) are inherited and
apply to every operation below. The requirements here specify the new operations
and their classification.

### Functional Requirements

- **FR-001**: The system MUST provide a delete operation for a single selected
  object; it MUST be classified as destructive and gated by the typed-confirmation
  tier requiring a byte-for-byte exact match of the target object identifier.
- **FR-002**: The system MUST provide an upload operation that creates an object at
  the current bucket/prefix from a local file the operator selects via a built-in,
  keyboard-driven local file browser (navigating the local filesystem and choosing a
  file); the object's contents MUST match the source file on success.
- **FR-003**: The upload operation MUST detect when the target key already exists
  and, in that case, MUST treat the operation as an overwrite gated by the
  destructive typed-confirmation tier; a non-colliding upload MAY use the simple
  confirmation tier. Collision detection is **advisory** — based on the currently
  loaded level listing, not a server-side precondition — so a key absent from the
  loaded listing may not be detected; confirmation, not detection, is the safety
  gate.
- **FR-004**: The system MUST provide a copy operation that duplicates a selected
  object to an operator-specified destination key within the current bucket without
  round-tripping the data through the local machine; the source MUST remain
  unchanged. (Cross-bucket copy is out of scope for this feature.)
- **FR-005**: The copy operation MUST detect a destination collision and gate an
  overwrite with the destructive typed-confirmation tier; a copy to a free
  destination MAY use the simple tier. As with upload (FR-003), collision detection
  is **advisory** (from the currently loaded listing), not a server-side
  precondition.
- **FR-006**: The system MUST provide a move/rename operation implemented as a copy
  to the destination followed by deletion of the source; because it removes the
  source it MUST be classified as destructive and use the typed-confirmation tier.
- **FR-007**: The move/rename operation MUST NOT lose data on partial failure: if
  the copy half fails, the source MUST be left intact and the destination MUST NOT
  be claimed; if the copy succeeds but the source deletion fails, the data MUST
  remain available (at least at the destination) and the operation MUST be reported
  as a partial/failed move, never a clean success.
- **FR-008**: The system MUST provide a recursive delete operation that removes a
  selected folder/prefix and every object beneath it; it MUST be classified as
  destructive and gated by a typed-confirmation tier requiring an exact match of the
  target prefix identifier.
- **FR-009**: The recursive delete operation MUST enumerate all objects under the
  target prefix across pagination boundaries and attempt to delete the complete set,
  not merely the first page. It MUST be best-effort: a failure on one object MUST
  NOT abort the run — the operation continues deleting every remaining object it can,
  and never prompts the operator mid-run on a per-object failure.
- **FR-010**: The system MUST surface **live progress** for the streaming
  long-running operations — upload (bytes transferred) and recursive delete (objects
  deleted/failed) — within the responsiveness budget (SC-007), and MUST allow them
  to be cancelled where the backend supports it. Server-side copy and move are single
  backend calls with no client-streamable byte progress; they MUST show a busy
  indicator and stay non-blocking rather than a byte/percentage bar.
- **FR-011**: The system MUST report a truthful outcome for every operation —
  success, failure, cancellation, or partial completion — and MUST NOT report a
  partial or cancelled result as a clean success. For recursive delete, the report
  MUST include the count of objects deleted and the count that failed.
- **FR-012**: The system MUST refuse every operation in this feature when writes are
  not enabled or the active context is read-only, leaving storage unchanged and
  showing an explanatory message.
- **FR-013**: The system MUST validate destination keys/prefixes (non-empty, valid
  key characters, and reject a destination identical to the source) and reject
  invalid input with guidance before attempting any backend call.
- **FR-014**: The system MUST log every operation — its action, source target,
  destination target (where applicable), and context — before execution, and MUST
  log the outcome (including partial counts for recursive delete) after, writing
  only to the file log and never leaking secrets.
- **FR-015**: A failed or denied operation MUST leave storage unchanged (or, for
  multi-step operations, in a truthfully-reported partial state that loses no data)
  and surface a clear error that does not leak credentials or internal detail.
- **FR-016**: After any successful or partial mutation, the affected level's view
  MUST be refreshable to reflect ground truth (the operation MUST NOT leave the
  listing asserting an outcome the backend did not actually produce).

### Key Entities *(include if feature involves data)*

- **Object Mutation**: A requested change to one object. Attributes: action type
  (delete, upload, copy, move), source target (bucket + key), destination target
  (bucket + key, for upload/copy/move), local source path (for upload), context,
  classification (reversible / destructive), status
  (pending/confirmed/running/succeeded/failed/cancelled/partial), and timestamps.
- **Recursive Deletion**: A requested removal of a prefix subtree. Attributes:
  target prefix, context, enumerated object set, running progress (deleted count,
  failed count, total seen), status, and timestamps. Always destructive.
- **Overwrite Decision**: The detection that a destination key is already occupied,
  which escalates an otherwise-simple operation (upload/copy) to the destructive
  typed-confirmation tier.
- **Operation Outcome**: The truthful result of a mutation — one of succeeded,
  failed, cancelled, or partial — carrying any per-object counts (for recursive
  delete) and the message surfaced to the operator and the log.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the always-destructive operations (single delete, move/rename,
  recursive delete) require a typed confirmation of the exact target identifier; an
  upload or copy **detected** as an overwrite is likewise escalated to the typed
  tier. A mismatched typed entry aborts with zero changes.
- **SC-002**: An operator can delete a single object on a writable context and
  confirm it is gone within one refresh, with a log entry for the action and
  outcome.
- **SC-003**: An operator can upload a local file and, after one refresh, retrieve
  an object whose contents are byte-identical to the source file.
- **SC-004**: Every upload or copy whose target key is present in the current level
  listing is routed through the overwrite (typed) confirmation — no upload or copy
  silently replaces a **detected** existing object. (Detection is advisory, from the
  loaded listing; see FR-003/FR-005.)
- **SC-005**: 100% of move/rename operations that fail after the copy step leave the
  data retrievable (no data loss) and are reported as partial/failed, never as a
  clean success.
- **SC-006**: A recursive delete of a non-empty prefix removes 100% of the objects
  the operator could delete, reports accurate deleted-vs-failed counts, and after
  one refresh shows the prefix gone (or its surviving objects truthfully).
- **SC-007**: Every operation remains responsive — progress or a busy indicator
  appears within 100 ms (the next render tick) of the operation starting, no frame
  freezes for the duration of the backend call, and the streaming operations (upload,
  recursive delete) are cancellable; a cancelled operation is never reported as a
  clean success.
- **SC-008**: A read-only context refuses 100% of these operations, with storage
  verifiably unchanged.
- **SC-009**: 100% of operations produce a pre-execution log entry and an outcome
  log entry, with zero secrets present in any log line.

## Assumptions

- This feature builds directly on 002: the `--write` switch, per-context `readonly`
  override, two-tier confirmation framework, non-blocking generation/cancellation
  execution model, file-only structured logging, and secret redaction are reused,
  not reinvented.
- The target backends (MinIO, Ceph RGW) support a server-side copy primitive so
  copy and move do not require downloading and re-uploading object data.
- Object storage has no native move/rename; move is modeled as copy-then-delete,
  with the no-data-loss guarantee of FR-007.
- Upload sources are local files identified by a path the operator provides/selects;
  uploading from arbitrary streams, directories, or recursive local trees is out of
  scope for this feature.
- Recursive delete operates on the prefix/delimiter model already used for browsing;
  "folder" means "all keys sharing the selected prefix."
- Out of scope: cross-bucket copy/move (destinations are within the current bucket
  only), bucket creation/deletion, multi-object/batch operations other than
  recursive delete (e.g. multi-select arbitrary deletes), object metadata/ACL/tag
  edits, versioned-object operations and version-aware deletes, server-side
  sync/mirroring, and presigned-URL generation. These are separate later features.
- Download-to-local-disk (saving an object as a file) is a read-side capability and
  is not part of this write-operations feature.
- The constitution (Principle V — Safe Operations) governs every destructive path
  here: confirmation and pre-execution logging are mandatory and inherited from the
  002 implementation of that contract.
