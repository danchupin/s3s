# Feature Specification: Write Foundation & Safety

**Feature Branch**: `002-write-foundation`

**Created**: 2026-06-05

**Status**: Draft

**Input**: User description: "Write foundation & safety for s3s: extend the tool
so it can perform mutating operations safely. Add a way to enable writes only
where intended (keep production read-only), a confirmation framework (simple
confirm for reversible actions, stronger typed confirmation for destructive
ones), non-blocking operations with progress and logging, and one vertical write
slice (create an empty folder) to prove the framework end-to-end."

## Clarifications

### Session 2026-06-05

- Q: How are writes enabled, and what is the default per-context posture? → A:
  Global `--write` flag (default off = read-only); when on, every context is
  writable except those marked `readonly: true` in config (opt-out per context).
- Q: Within what time must progress feedback appear after a mutating operation
  starts (SC-004 "promptly")? → A: Within 100 ms (next render tick).

## User Scenarios & Testing *(mandatory)*

s3s today is read-only by construction: there is no way to change anything in a
bucket. This feature is the foundation that lets s3s *mutate* storage — but does
so safely. It deliberately ships only **one** mutating operation (create an empty
folder) so the safety scaffolding (write-mode opt-in, confirmation, non-blocking
execution, logging) is proven on a small, low-risk action before higher-risk
operations (upload, delete, copy) are added in later features.

### User Story 1 - Writes are off unless explicitly enabled (Priority: P1)

As an operator who points s3s at production, I want write capability to be
**off by default** and enabled only where I intend, so a single keystroke can
never mutate a protected environment. I can mark specific contexts as read-only
(e.g. production) and they refuse mutations regardless of any global setting.

**Why this priority**: This is the safety contract. Without it, adding any write
capability turns a browsing tool into something that can damage production. The
project constitution (Principle V — Safe Operations) makes confirmation and
protection of destructive paths mandatory. Read-only protection must exist before
the first mutation does.

**Independent Test**: Configure two contexts — one read-only, one writable.
Attempt the create-folder action in the read-only context → it is refused with a
clear message and nothing changes. Switch to the writable context → the action is
permitted (subject to confirmation).

**Acceptance Scenarios**:

1. **Given** the tool launched with writes not enabled, **When** the operator
   triggers a mutating action, **Then** the action is blocked with an explanatory
   message and storage is unchanged.
2. **Given** a context marked read-only, **When** writes are globally enabled and
   the operator triggers a mutating action in that context, **Then** the action is
   still refused (per-context read-only wins).
3. **Given** a writable context with writes enabled, **When** the operator
   triggers a mutating action, **Then** the action proceeds to the confirmation
   step.

---

### User Story 2 - Create a folder, confirmed, non-blocking, logged (Priority: P1)

As an operator on a writable context, I want to create an empty folder at the
current level, see a confirmation before it happens, watch it run without freezing
the interface, and have it recorded in the log — so I can trust that the first
mutating operation behaves safely and observably.

**Why this priority**: This is the vertical slice that proves the whole
foundation. Create-folder is the lowest-risk mutation (reversible, creates no real
data), making it the right action to validate confirmation, non-blocking
execution, progress/outcome feedback, and logging end-to-end. Shipping it is a
demonstrable increment.

**Independent Test**: On a writable context, create a folder named `reports/` at
the current level → a confirmation prompt appears → on confirm, the interface
stays responsive and shows the outcome → after a refresh the new folder appears in
the listing → a log entry for the action exists in the log file.

**Acceptance Scenarios**:

1. **Given** a writable context, **When** the operator initiates create-folder and
   confirms, **Then** the folder is created and visible after the level refreshes.
2. **Given** the confirmation prompt is shown, **When** the operator cancels,
   **Then** nothing is created and the operator returns to browsing.
3. **Given** a mutating action is running, **When** the operator observes the
   interface, **Then** the interface remains responsive and surfaces progress and
   the final success/failure outcome.
4. **Given** any create-folder attempt (success, failure, or cancellation),
   **When** the operator inspects the log file, **Then** the action, its target,
   and its outcome are recorded, with no secrets present.
5. **Given** the backend rejects the operation (e.g. access denied), **When** the
   action fails, **Then** storage is unchanged and a clear, non-leaking error is
   shown.

---

### User Story 3 - Guardrails for destructive actions (Priority: P2)

As an operator, I want the confirmation framework to distinguish reversible
actions from destructive ones, requiring a stronger, deliberate confirmation
(such as typing the target's name) for actions that destroy or overwrite data, so
that future delete/overwrite features cannot be triggered by a casual keypress.

**Why this priority**: This feature ships no destructive operation itself, but the
foundation must provide the stronger-confirmation tier now so later features
(delete object, recursive remove, overwrite) build on a proven guardrail rather
than inventing one each time. It is P2 because it has no end-user-visible action in
this slice beyond the framework capability.

**Independent Test**: Exercise the confirmation framework with a representative
destructive intent → verify it demands a typed confirmation of the target name and
that a non-matching entry aborts the action; verify a reversible action (create
folder) requires only the simple confirmation.

**Acceptance Scenarios**:

1. **Given** an action classified as destructive, **When** the operator confirms,
   **Then** they must type the exact target identifier; a mismatch aborts the
   action with no change.
2. **Given** an action classified as reversible, **When** the operator confirms,
   **Then** a single simple confirmation is sufficient.
3. **Given** any action classified as destructive, **When** it is about to
   execute, **Then** the intended action and target are logged before execution.

---

### Edge Cases

- What happens when create-folder targets a name that already exists as a folder
  or collides with an existing object key? (Surface a clear message; do not
  silently overwrite.)
- What happens when the operator switches context or navigates away while a
  mutating operation is in flight? (The operation's result must not corrupt the
  view of a different context/level — superseded results are dropped.)
- What happens when writes are enabled globally but the active context is
  read-only? (Read-only wins; mutation refused.)
- What happens when the folder name is empty, contains only whitespace, or
  includes characters that are invalid for a key? (Reject with guidance; do not
  attempt the operation.)
- What happens when the network call hangs? (The operation is cancellable and the
  interface stays responsive.)
- What happens if the operator lacks permission on the backend? (Fail safely,
  unchanged storage, clear non-leaking error, logged outcome.)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST treat write capability as disabled by default;
  mutating operations are only possible when writes are explicitly enabled via a
  global launch-time `--write` switch (absent the switch, the tool is read-only).
- **FR-002**: The system MUST allow an operator to designate individual contexts
  as read-only (a per-context `readonly: true` setting); a read-only designation
  MUST override the global `--write` switch (read-only always wins). When `--write`
  is on, every context not marked read-only is writable (opt-out per context).
- **FR-003**: The system MUST refuse every mutating operation in a read-only
  context or when writes are not enabled, leaving storage unchanged and showing an
  explanatory message (never a silent no-op).
- **FR-004**: The system MUST require explicit confirmation before executing any
  mutating operation.
- **FR-005**: The system MUST provide two confirmation tiers — a simple
  confirmation for reversible operations and a stronger, deliberate confirmation
  (typing the exact target identifier) for destructive/irreversible operations —
  and MUST classify operations into the correct tier.
- **FR-006**: The system MUST execute mutating operations without blocking the
  interface; the interface MUST remain responsive, surface progress, and surface
  the final success or failure outcome.
- **FR-007**: The system MUST allow an in-flight mutating operation to be
  cancelled where the operation supports cancellation, and MUST ensure a
  superseded or cancelled operation cannot corrupt the view of another
  context/level.
- **FR-008**: The system MUST log every mutating operation — its action, target,
  and context — before execution, and MUST log the outcome, writing only to the
  file log and never leaking secrets.
- **FR-009**: The system MUST provide exactly one mutating operation in this
  feature — create an empty folder at the current level — working end-to-end
  through enablement, confirmation, execution, feedback, and logging.
- **FR-010**: The system MUST validate the create-folder target (non-empty, valid
  key characters, collision awareness) and reject invalid input with guidance
  before attempting any backend call.
- **FR-011**: A failed or denied mutating operation MUST leave storage unchanged
  and surface a clear error that does not leak credentials or internal detail.
- **FR-012**: The read-only structural guarantees for protected contexts MUST hold
  even when the operator is running a build that is capable of writes.

### Key Entities *(include if feature involves data)*

- **Mutating Operation**: A requested change to storage. Attributes: action type
  (for this feature, only "create folder"), target (bucket + key/prefix), context,
  status (pending/confirmed/running/succeeded/failed/cancelled), and timestamps.
- **Context Write Policy**: Whether a given context permits mutations. Values:
  read-only or writable. Read-only is the protective default for sensitive
  environments and always overrides global write enablement.
- **Confirmation**: The gate before execution. Attributes: tier (simple or typed),
  target identifier to be matched (for the typed tier), and the operator's
  response (confirmed / aborted).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of mutating operations pass through a confirmation step — no
  code path executes a mutation without one.
- **SC-002**: A read-only context refuses 100% of mutation attempts, including
  when writes are globally enabled.
- **SC-003**: 100% of destructive-classified operations require a typed
  confirmation of the exact target; a mismatched entry aborts with zero changes.
- **SC-004**: The interface remains responsive during any mutating operation —
  progress feedback appears within 100 ms (the next render tick) of the operation
  starting, and no frame freezes for the duration of the backend call.
- **SC-005**: 100% of mutating operations produce a log entry before execution and
  an outcome entry after, with zero secrets present in any log line.
- **SC-006**: An operator can create an empty folder on a writable context and see
  it in the listing within one refresh.
- **SC-007**: Every failed or denied mutation results in unchanged storage,
  verifiable by re-listing the target level.

## Assumptions

- Global default is read-only; writes are enabled by an explicit global
  `--write` launch switch, with a per-context `readonly: true` setting as the
  authoritative override (opt-out model: when `--write` is on, contexts are
  writable unless marked read-only). The exact field names/flag spelling are a
  design detail for planning.
- The target backends (MinIO, Ceph RGW) support representing an empty folder as a
  zero-length key ending in the path delimiter; create-folder relies on this.
- The project constitution (Principle V) governs safety: confirmation and
  pre-execution logging for destructive actions are mandatory, and this feature
  implements that contract for the write path.
- This feature is the foundation only. Out of scope: any mutating operation other
  than create-folder (upload, delete, copy, move, rename, metadata/ACL edits,
  bucket creation/deletion, versions, sync) — those are separate later features
  that build on this foundation.
- The existing non-blocking execution model (operations run off the interface loop
  with generation/cancellation) and the existing file-only structured logging and
  secret redaction are reused, not reinvented.
- Read-only enhancements unrelated to writing (sort, hex view, syntax
  highlighting, copy-URI, presigned read URLs) are out of scope and tracked
  independently.
