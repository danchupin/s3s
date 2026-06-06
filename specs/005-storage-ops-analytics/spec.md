# Feature Specification: Storage Operations & Analytics

**Feature Branch**: `005-storage-ops-analytics`

**Created**: 2026-06-05

**Status**: Draft

**Input**: User description: "Теперь ты в роли devops инженера который поддерживает s3 кластера. Каких фичей недостаточно в s3s? Без чего нет смысла пользоваться продуктом? Нужны киллер-фичи чтобы инженеры захотели пользоваться продуктом, а не голым s3cmd. Прорывные идеи, выгодно выделяющие продукт по удобству и возможностям."

## Why this feature (problem framing)

Today s3s can *browse* storage and *put data in* (create folder, upload, copy, move,
delete), but a DevOps engineer cannot **get data out**, cannot **act on many objects at
once**, and cannot answer the single most common operational question — *"what is eating
my space and where?"* — without dropping back to `s3cmd`/`aws s3` and doing manual
arithmetic. Those three gaps are why an engineer keeps the old CLI open in the next pane.

This feature closes them with capabilities that turn s3s from a viewer into a daily
driver: **download to local disk**, **storage analytics (`du`)**, **multi-select bulk
operations**, and **sortable lists**. Download and analytics are read-only and therefore
usable against production (no write mode needed); only the mutating bulk actions require write.

Because bulk mutations make it far easier to destroy a lot of data with one keystroke, the
feature also reworks the safety model into a **runtime read-only ↔ write toggle** with
**loud, unmistakable write-mode signalling**. Instead of a session's write capability being
fixed at launch by the `--write` flag, the engineer arms and disarms write on a hotkey — and
whenever write is armed, the UI screams it (a persistent, high-contrast indicator) so the
engineer can never mutate production thinking they were safe. The `--write` flag becomes a
convenience to *start* armed, not a hard gate; a context marked `readonly: true` can still
never be armed.

Finally, the feature removes a daily friction-and-security wart in how credentials are
handled. Today a context's secret is an `${ENV}` reference, so the engineer must re-`export`
it into every new terminal — and environment variables leak (visible in the process
environment, inherited by child processes, captured in shell history). For a tool aimed at
production that is both annoying and a security smell. The feature lets a context resolve its
secret from any of several **secure credential sources** (OS keychain, an external command,
an AWS shared profile, the existing `${ENV}`, or a secure prompt) so the secret never needs to
live in the shell environment or on disk in plaintext.

## Clarifications

### Session 2026-06-05

- Q: Bulk download of N objects — how are they laid out locally? → A: Preserve the object key
  hierarchy as local subdirectories (mirror S3 structure; collisions practically eliminated).
- Q: What can be marked in multi-select and how does bulk delete behave? → A: Objects only;
  folders/prefixes are not markable. Recursive delete stays a separate single-folder action in
  the action menu (minimal blast radius — no accidental mass recursive delete via selection).
- Q: Default download destination and can the path be chosen? → A: A default directory (current
  working directory, configurable) with the option to override per download via the existing
  in-TUI file browser (the same one used for upload).
- Q: Does the chosen sort order persist when navigating between levels? → A: Yes — the sort
  setting persists for the session and applies to each newly entered level until changed.
- Q: Primary approach to secret storage (to stop per-session env export, securely)? → A:
  Pluggable credential sources per context — OS keychain (macOS Keychain / Linux Secret
  Service), an external command (`cmd:`, covering pass/Vault/1Password/sops/aws-vault), an AWS
  shared profile (`~/.aws/credentials`), the existing `${ENV}` reference, and a secure startup
  prompt fallback. Headless-friendly via the command/profile sources; secrets never on disk in
  plaintext.
- Q: How are multiple credential sources on one context handled? → A: A context configures
  exactly one source; specifying more than one is a config validation error at load. The secure
  prompt is the implicit fallback used only when the single configured source resolves nothing.
  No precedence chain / source guessing.
- Q: Is the `s3s config init` wizard extended for the new sources? → A: Yes — the wizard asks
  which credential source to use and, for the keychain source, stores the secret directly into
  the OS keystore (instead of printing an `export` line). It is the primary out-of-box path and
  drives discoverability.
- Q: How is the `cmd:` source protected against a tampered config (the command runs at launch)?
  → A: The `cmd:` source executes only when the config file is owner-only (not group/world
  writable) and owned by the running user; otherwise s3s refuses to run it with a clear
  explanation. This blocks "attacker edits the YAML → command runs on next launch."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Download an object to local disk (Priority: P1)

An engineer browsing a bucket finds a config dump, a log archive, or a backup object and
needs the actual file on their machine — to inspect it locally, feed it to another tool, or
keep a copy. They select the object, trigger **download**, and the full object streams to a
local file with visible progress. Because reading an object does not mutate the bucket, this
works even in a read-only context and against production.

**Why this priority**: This is the most fundamental missing capability. The product can put
data in but cannot take data out — there is no way to retrieve a full object today (preview is
bounded to the first few MiB). Without download there is no reason to close `s3cmd`.

**Independent Test**: Point s3s at a bucket (read-only context), select an object, run
download, and confirm a byte-for-byte identical file appears on local disk with a progress
indication during transfer. Fully testable on its own; delivers immediate standalone value.

**Acceptance Scenarios**:

1. **Given** a selected object and a read-only context, **When** the engineer triggers
   download, **Then** the complete object is written to a local file and the engineer is told
   where it landed — no `--write` required.
2. **Given** a large object, **When** download is running, **Then** live progress (bytes /
   percentage) is shown and the transfer can be cancelled, leaving no partial file behind.
3. **Given** a local file with the same name already exists at the target path, **When** the
   engineer triggers download, **Then** they are asked to confirm before any local file is
   overwritten.
4. **Given** a download in progress, **When** the network fails mid-transfer, **Then** the
   engineer sees a clear error, the partial local file is removed, and the rest of the UI is
   unaffected.

---

### User Story 2 - See what is eating space (`du` analytics) (Priority: P2)

An engineer suspects a bucket or a prefix has bloated and needs to know *how big it is* and
*which sub-prefixes are responsible* — without exporting a listing and summing it by hand.
They trigger **analyze** on a bucket or folder and get the total size, the total object count,
and a ranked breakdown of the immediate child prefixes/objects by size (an `ncdu`-style view).
The scan is read-only and works against production.

**Why this priority**: This is the killer differentiator. "How much space and where" is the
most common storage admin question, and existing CLIs answer it slowly, blindly, and with
manual math. A live, ranked, in-TUI breakdown is something no everyday S3 client does well.

**Independent Test**: Seed a prefix with a known size distribution, run analyze on it, and
confirm the reported total size and object count match the seeded data and the child breakdown
is correctly ranked largest-first. Testable without any other story.

**Acceptance Scenarios**:

1. **Given** a bucket or prefix, **When** the engineer triggers analyze, **Then** they see the
   aggregate total size and total object count for everything beneath that prefix.
2. **Given** the analysis result, **When** it is displayed, **Then** the immediate children
   (sub-prefixes and direct objects) are listed ranked by size, largest first, each with its
   size and share of the parent total.
3. **Given** a very large prefix (millions of keys), **When** analysis is running, **Then**
   progress is shown, running totals update as the scan proceeds, and the scan can be
   cancelled at any time without freezing the UI.
4. **Given** an empty prefix, **When** the engineer triggers analyze, **Then** they see a
   total of zero objects / zero bytes rather than an error.
5. **Given** an analysis result, **When** the engineer drills into a listed child prefix,
   **Then** they can continue navigating/analyzing downward to locate the exact consumer.

---

### User Story 3 - Multi-select and act on many objects at once (Priority: P3)

An engineer needs to operate on dozens of objects — pull 40 log files, delete a batch of
stale artifacts, copy a set elsewhere. Doing this one object at a time is unusable. They mark
multiple **objects** (folders/prefixes are not markable), see the selection count and combined
size, and apply a single action (download, delete, or copy) to the whole set. Bulk download is
read-only and preserves the key hierarchy as local subdirectories; bulk delete and copy require
the armed write state and confirmation. Recursive deletion of a whole folder remains its own
dedicated single-folder action (not driven by multi-select), keeping the blast radius small.

**Why this priority**: Real storage work is done in batches. Single-object-only operations make
the tool a toy for anything beyond a one-off. This multiplies the value of every other
operation, but it builds on download (US1) and the existing single-object write ops, so it
follows them.

**Independent Test**: Mark several objects in a level, confirm the UI shows the count and
combined size, run a bulk download, and confirm every marked object is written to disk with a
truthful per-item success/failure summary. Testable independently of US2.

**Acceptance Scenarios**:

1. **Given** a list of objects, **When** the engineer marks several and triggers a bulk action,
   **Then** the action applies to exactly the marked set and an unambiguous count is shown
   before execution.
2. **Given** a marked set, **When** the engineer runs bulk download, **Then** every object is
   transferred into local subdirectories mirroring its key hierarchy, and a per-item result
   (succeeded / failed, with reasons) is reported at the end, even if some items failed.
3. **Given** a marked set in a read-only context, **When** the engineer opens the action menu,
   **Then** only the read-only bulk action (download) is offered and bulk delete/copy are
   absent.
4. **Given** a marked set including destructive intent (bulk delete), **When** the engineer
   confirms, **Then** the existing typed-confirmation safety applies to the batch and every
   deletion is logged before execution.
5. **Given** an active selection, **When** the engineer navigates away from the current level,
   **Then** the selection is cleared (selection is scoped to one level) and this is evident in
   the UI.

---

### User Story 4 - Sort lists by size, name, or last-modified (Priority: P4)

An engineer wants the biggest object, the newest upload, or an alphabetical view on demand —
not just default order. They cycle a **sort** control to reorder the current list by name,
size, or last-modified, and toggle ascending/descending. This pairs naturally with analytics:
sort a level by size to see the heaviest objects instantly. The chosen sort persists across
navigation for the session, applied to each newly entered level until changed.

**Why this priority**: Pure presentation, read-only, low risk, and it makes the analytics story
more useful, but the tool is still usable without it — so it is the lowest of the four.

**Independent Test**: Load a level with mixed sizes/dates, cycle the sort to "size descending,"
and confirm the largest object is first; toggle direction and confirm the order reverses.
Testable on its own.

**Acceptance Scenarios**:

1. **Given** a list of objects, **When** the engineer cycles the sort to size, **Then** the
   list reorders by object size and the direction (ascending/descending) is indicated.
2. **Given** a sorted list, **When** the engineer toggles direction, **Then** the same column
   reorders the opposite way.
3. **Given** a level containing both folders and objects, **When** sorted by size, **Then**
   entries without a meaningful size (folders) are ordered predictably and consistently rather
   than appearing at random.

---

### User Story 5 - Arm/disarm write at runtime with loud signalling (Priority: P1)

An engineer launches s3s read-only (safe by default) and browses production. When they
genuinely need to mutate — delete a stale batch, upload a fix — they press a hotkey to **arm
write**, deliberately confirming the switch. From that moment the interface makes the danger
impossible to miss: a persistent, high-contrast WRITE indicator stays on screen at all times.
When done, one keystroke disarms back to read-only instantly. A context marked `readonly: true`
refuses to arm under any circumstance. The whole point is to make a destructive keystroke
something the engineer can only do *on purpose, while visibly armed* — never by accident.

**Why this priority**: This is foundational safety, not a convenience. Making bulk mutations
(US3) easier without an equally strong, always-visible guardrail would *increase* the risk of
catastrophic accidental deletion against production. The toggle and its signalling must land
together with — and logically gate — the mutating operations. It is P1 alongside download (US1)
and should be implemented before bulk delete/copy are exposed.

**Independent Test**: Launch read-only, confirm mutating actions are absent; press the toggle,
confirm the arming prompt appears and that after confirming, a loud persistent WRITE indicator
is shown and mutating actions appear; press the toggle again and confirm instant reversion to
read-only. Point at a `readonly: true` context and confirm arming is refused. Testable without
any other story.

**Acceptance Scenarios**:

1. **Given** a session in read-only, **When** the engineer presses the write-toggle, **Then**
   they are asked to deliberately confirm arming write, and only on confirmation does the
   session enter write mode.
2. **Given** a session armed in write mode, **When** the engineer views any screen, **Then** a
   persistent, high-contrast WRITE indicator is visible at all times and cannot scroll off or
   be confused with the read-only state.
3. **Given** a session armed in write mode, **When** the engineer presses the write-toggle,
   **Then** it reverts to read-only immediately with no confirmation, and the WRITE indicator
   clears.
4. **Given** a context marked `readonly: true`, **When** the engineer presses the write-toggle,
   **Then** arming is refused with a clear reason and the session stays read-only.
5. **Given** a write-armed session, **When** the engineer switches to a `readonly: true`
   context, **Then** the session automatically drops to read-only for that context.
6. **Given** any change of write/read-only state, **When** it happens, **Then** the state
   change is recorded in the log as a security-relevant event.
7. **Given** s3s is launched with the write opt-in flag, **When** it starts, **Then** the
   session begins already armed in write mode with the loud indicator shown from the first
   frame.

---

### User Story 6 - Stop exporting secrets every session; secure credential sources (Priority: P1)

An engineer configures a cluster once and wants to open s3s in any new terminal — including a
fresh SSH session on a jump host — and just connect, without re-`export`-ing a secret each time
and without the secret sitting in their shell environment or on disk in plaintext. They point a
context's secret at a **secure source**: the OS keychain (stored once via the tool), an external
command (`pass`, Vault, 1Password CLI, sops, aws-vault — anything that prints the secret), an
existing AWS shared profile, or the current `${ENV}` reference (kept for CI/automation). A
context configures exactly one of these; the secure startup prompt is the implicit fallback
when that single source resolves nothing. The `config init` wizard walks the engineer through
picking a source and, for the keychain, stores the secret straight into the OS keystore. The
secret is resolved at launch, never written to an s3s-managed file, and never required in the
environment.

**Why this priority**: This is the credential-handling backbone for a production-facing tool.
The current "export the secret into every shell" model is both a daily friction and a real
security exposure (env leakage, history capture). With write operations now arm-able at runtime
(US5), credential hygiene matters even more. It is P1 security work and independently valuable.

**Independent Test**: Store a context's secret in the OS keychain via the tool, open a brand-new
terminal with no `S3S_*` variables exported, launch s3s, and confirm it connects with no prompt
and no env export — while no plaintext secret exists in the config or any s3s file and the
secret never appears in logs. Testable without any other story.

**Acceptance Scenarios**:

1. **Given** a context whose secret lives in the OS keychain, **When** the engineer launches
   s3s in a fresh terminal with nothing exported, **Then** it connects with no prompt and no
   environment variable required.
2. **Given** a context with an external-command source, **When** s3s launches, **Then** it runs
   the command, uses its output as the secret, and never writes that secret to disk or logs.
3. **Given** a context referencing an AWS shared profile, **When** s3s launches, **Then**
   credentials are read from the shared credentials file for that profile.
4. **Given** a context with no non-interactive secret available, **When** s3s launches, **Then**
   it prompts securely (no echo) and offers to save the secret into the OS keychain for next
   time.
5. **Given** the existing `${ENV}` reference with the variable set, **When** s3s launches,
   **Then** it still works unchanged (back-compat for CI/automation).
6. **Given** any credential source, **When** secrets are rendered in the UI, logs, or error
   output, **Then** they are redacted everywhere.
7. **Given** a config file readable by other users, **When** s3s launches, **Then** it warns
   about the insecure permissions.
8. **Given** the engineer uses the tool's credential-store command, **When** they store, rotate,
   or remove a context's secret, **Then** the change is written to / removed from the OS keystore
   only — never the config file.
9. **Given** the engineer runs `s3s config init` and chooses the keychain source, **When** they
   enter the secret (no echo), **Then** it is stored in the OS keystore and the config references
   it — no `export` line is printed.
10. **Given** a context configured with a `cmd:` source on a config file that is group/world
    writable, **When** s3s launches, **Then** it refuses to execute the command and explains the
    insecure-permissions reason instead of connecting.
11. **Given** a context configured with two credential sources, **When** s3s loads the config,
    **Then** it fails validation with a message to choose exactly one source.

---

### Edge Cases

- **Download target unwritable / out of space**: the engineer gets a clear error, the UI stays
  responsive, and no partial/corrupt file is left implying success.
- **Download filename collision or unsafe characters in the key**: the local filename is
  derived safely from the object key's base name; collisions prompt before overwrite.
- **Cancel mid-download / mid-analysis**: cancelling cleans up (removes a partial download)
  and returns to a consistent view; a superseded operation never corrupts the display.
- **Bulk action partial failure**: some items succeed, some fail; the result reports truthful
  per-item outcomes and never claims a whole-batch success when items failed.
- **Selection vs pagination**: when a level has more keys than are currently loaded, a
  "select all" acts on the loaded/visible set, and the UI makes clear that unseen keys are not
  included.
- **Analyze on a huge prefix**: the scan is incremental and cancellable; the engineer is never
  blocked waiting on a full recursive listing, and very large totals are displayed in
  human-readable units without overflow.
- **Analyze on a versioned bucket**: totals reflect current object versions (see Assumptions),
  and this basis is stated so the numbers are not mistaken for all-versions storage.
- **Sort + active search/filter**: sorting reorders the current (possibly filtered) result set;
  the two compose without one silently discarding the other.
- **Read-only context**: download and analyze are fully available; any mutating bulk action is
  hidden/refused exactly as single-object mutations are today.
- **Disarm with a mutation in flight**: pressing the toggle to read-only stops *new* mutations
  from being startable; an already-committed in-flight operation is governed by the existing
  cancellation behaviour, not silently corrupted by the mode change.
- **Arm refused on a locked context**: the toggle on a `readonly: true` context never enters a
  half-armed state — it stays unambiguously read-only and says why.
- **Context switch while armed**: switching to a writable context preserves the armed state;
  switching to a `readonly: true` context forces read-only — the indicator must always reflect
  the *current* context's true state, never a stale one.
- **Indicator can never be missed**: in narrow terminals or busy screens the WRITE signal must
  still be present and distinguishable from read-only; it must not be the first thing dropped
  when space is tight.
- **Keystore unavailable** (headless Linux without Secret Service, locked or absent keychain):
  s3s falls back to another configured source or the secure prompt with a clear message, and
  never crashes or connects with an empty secret.
- **External command fails / times out / returns empty**: s3s surfaces a clear error and does
  not attempt to connect with an empty or partial secret.
- **External command on an insecure config**: if the config file is group/world writable or not
  owned by the running user, s3s refuses to execute the `cmd:` source (rather than risk running
  an attacker-injected command) and says why.
- **Multiple sources on one context**: this is a config validation error caught at load — the
  engineer is told to pick exactly one source; there is no silent precedence resolution.
- **Anonymous contexts** (already supported) need no secret and are unaffected by all of this.
- **Secret rotation mid-session**: secrets are resolved at launch; picking up a rotated value
  requires relaunch (documented, not a silent staleness bug).
- **Insecure config permissions**: a world/group-readable config triggers a warning rather than
  a silent accept.

## Requirements *(mandatory)*

### Functional Requirements

**Download (US1)**

- **FR-001**: Users MUST be able to download the full contents of a selected object to a local
  file.
- **FR-002**: Download MUST be available in a read-only context and against a read-only
  cluster, because it does not mutate the remote store (it only reads the object and writes
  locally).
- **FR-003**: The system MUST show live progress for a download (bytes transferred and/or
  percentage) and MUST keep the rest of the UI responsive during the transfer.
- **FR-004**: Users MUST be able to cancel an in-progress download; cancellation MUST remove
  any partially written local file.
- **FR-005**: When the chosen local destination already contains a file of the same name, the
  system MUST require explicit confirmation before overwriting it.
- **FR-006**: On download failure (network, permissions, disk), the system MUST report a clear
  error, MUST NOT leave a partial file that looks complete, and MUST log the operation outcome.
- **FR-007**: The system MUST derive the local filename safely from the object key's final
  segment and place downloads in a default directory resolved as: the `S3S_DOWNLOAD_DIR`
  environment variable if set, else a top-level `downloadDir` config key if set, else the current
  working directory (env wins over config). Users MUST be able to override the destination per
  download via the existing in-TUI file browser (the same component used for upload).

**Analytics / `du` (US2)**

- **FR-008**: Users MUST be able to trigger a recursive storage analysis on a bucket or any
  prefix and receive the aggregate total size and total object count beneath it.
- **FR-009**: The analysis MUST present a breakdown of the immediate children (sub-prefixes and
  direct objects) ranked by size, largest first, each with its size and its share of the parent
  total.
- **FR-010**: Analysis MUST be read-only and available in read-only contexts.
- **FR-011**: For large prefixes the analysis MUST be incremental and cancellable, MUST surface
  progress / running totals, and MUST NOT block the UI.
- **FR-012**: Sizes MUST be displayed in human-readable units; an empty prefix MUST report zero
  rather than an error.
- **FR-013**: Users MUST be able to drill from an analysis result into a child prefix to
  continue locating a consumer.

**Multi-select & bulk (US3)**

- **FR-014**: Users MUST be able to mark and unmark multiple **objects** in the current level
  and see the number of marked items and their combined size. Folders/prefixes MUST NOT be
  markable for selection.
- **FR-015**: Users MUST be able to apply a single action — download, delete, or copy — to the
  entire marked set.
- **FR-015a**: Bulk download MUST recreate the objects' key hierarchy as local subdirectories
  under the chosen destination (mirroring the S3 path), rather than flattening all objects into
  one directory.
- **FR-016**: Bulk download MUST be available read-only; bulk delete and bulk copy MUST require
  the armed write mode and MUST be hidden/refused in read-only contexts, consistent with
  single-object mutations. Recursive deletion of a folder MUST remain a separate dedicated
  single-folder action and MUST NOT be reachable through multi-select.
- **FR-017**: Bulk destructive actions MUST reuse the existing two-tier confirmation (typed
  confirmation for destructive operations) and MUST log each operation before execution.
- **FR-018**: A bulk action MUST report a truthful per-item outcome (succeeded/failed with
  reason) and MUST continue past individual failures rather than aborting the whole batch on
  the first error.
- **FR-019**: The selection MUST be scoped to the current level and MUST be cleared when the
  user navigates to a different level; the UI MUST make the active selection visible.

**Sorting (US4)**

- **FR-020**: Users MUST be able to sort the current list by name, size, or last-modified and
  toggle ascending/descending; the active sort column and direction MUST be visible. The chosen
  sort MUST persist across navigation for the session and apply to each newly entered level
  until the user changes it.
- **FR-021**: Entries without a meaningful size or date (e.g. folders) MUST be ordered
  predictably and consistently when sorting by those columns.

**Write-mode toggle & signalling (US5)**

- **FR-025**: Users MUST be able to switch the session between read-only and write at runtime
  via a dedicated hotkey, without restarting the application.
- **FR-026**: Arming write MUST require a deliberate confirmation; disarming back to read-only
  MUST be immediate and require no confirmation (asymmetric friction — easy to get safe, harder
  to get dangerous).
- **FR-027**: While write is armed, the interface MUST display a persistent, high-contrast,
  unmistakable WRITE indicator that is visible on every screen and cannot scroll off, be hidden,
  or be confused with the read-only state.
- **FR-028**: A context marked `readonly: true` MUST never be armable to write; the toggle MUST
  refuse with a clear reason and the session MUST remain read-only.
- **FR-029**: Switching to a `readonly: true` context while armed MUST automatically force the
  session to read-only; the indicator MUST always reflect the current context's true write
  capability.
- **FR-030**: Mutating operations (single-object and bulk) MUST be available only while write is
  armed and MUST be hidden/refused while read-only, exactly as today's write-gated actions.
- **FR-031**: The write opt-in launch flag MUST set the *initial* armed state (start in write)
  but MUST NOT be required to arm write later; default launch (no flag) MUST start read-only.
- **FR-032**: Every transition between read-only and write MUST be logged as a
  security-relevant event (which state, which context).

**Credential sources & secret handling (US6)**

- **FR-033**: A context MUST be able to source its secret from any of: the OS keychain, an
  external command's output, an AWS shared-credentials profile, an `${ENV}` reference, or a
  secure startup prompt.
- **FR-034**: Launching in a fresh terminal MUST NOT require exporting the secret into the
  environment when a non-env source is configured.
- **FR-035**: s3s MUST NOT write secrets into its config file or any s3s-managed file in
  plaintext.
- **FR-036**: For the external-command source, only the command's output MUST be used as the
  secret, and that output MUST be treated as secret (redacted, never logged). The command MUST
  be executed only when the config file is owner-only (not group/world writable) and owned by
  the running user; otherwise s3s MUST refuse to run it with a clear explanation.
- **FR-037**: s3s MUST provide a way to store, rotate, and remove a context's secret in the OS
  keychain, writing only to the keystore (never the config file).
- **FR-038**: When no secret can be resolved non-interactively, s3s MUST prompt securely (no
  echo) and MAY offer to persist the entered secret into the OS keychain.
- **FR-039**: Secrets MUST remain redacted in all logs, UI, and error output regardless of
  source.
- **FR-040**: s3s MUST detect and warn when the config file is readable by other users
  (insecure permissions).
- **FR-041**: A context MUST configure exactly one credential source; configuring more than one
  MUST be a config validation error at load. The secure prompt is the implicit fallback used
  only when the single configured source resolves nothing — there is no precedence chain.
- **FR-041a**: The `s3s config init` wizard MUST let the engineer choose the credential source
  and, for the keychain source, store the secret directly into the OS keystore rather than
  printing an `export` line.
- **FR-042**: The existing `${ENV}` reference MUST continue to work unchanged for back-compat
  and CI/automation.
- **FR-043**: If a configured source is unavailable (e.g., keychain absent on a headless host),
  s3s MUST surface a clear, actionable error and MUST NOT silently connect with an empty or
  incorrect secret.

**Cross-cutting**

- **FR-022**: All new operations MUST follow the existing non-blocking model — running off the
  UI loop, cancellable, with superseded operations discarded so stale results never corrupt the
  view.
- **FR-023**: The new *operations* (download, analyze, bulk delete/copy) MUST be reachable
  through the existing contextual action menu (no dedicated top-level keys), keeping the footer
  uncluttered; selection-gated and write-gated items MUST appear only when applicable. Only
  interaction *primitives* — mark (multi-select), sort, and the write toggle — get dedicated
  keys, advertised in the help surface.
- **FR-024**: No operation in this feature MUST weaken the read-only safety posture: reads
  (download, analyze) work everywhere; mutations (bulk delete/copy) require the armed write
  state plus their existing confirmation. Default-safe (start read-only) and the absolute
  `readonly: true` lock MUST hold.

### Key Entities *(include if feature involves data)*

- **Download transfer**: a single object being streamed to a local destination; attributes —
  source object key, local target path, total size, bytes transferred, state
  (running/done/failed/cancelled), outcome.
- **Selection set**: the set of marked items within the current level; attributes — marked
  keys, count, combined size; lifetime is bounded to the current level.
- **Bulk operation result**: the outcome of applying one action to a selection set; attributes —
  per-item success/failure with reason, totals of succeeded/failed.
- **Storage usage report**: the result of analyzing a prefix; attributes — analyzed prefix,
  total size, total object count, ordered list of children (name, size, share of total),
  completeness state (in-progress/complete/cancelled).
- **Sort order**: the active ordering of a list; attributes — column (name/size/modified),
  direction (asc/desc).
- **Session write state**: the current arm state of the session; attributes — read-only vs
  write-armed, whether the active context is hard-locked (`readonly: true`), and the source of
  the current state (launch flag / hotkey / forced-by-context). Drives which actions are
  offered and the loud indicator.
- **Credential source**: how a context resolves its secret; attributes — kind (keychain /
  command / aws-profile / env / prompt) and its reference (keychain entry id, command line,
  profile name, or env var name). The resolved value is held only as a redacting secret in
  memory and is never persisted to an s3s-managed file.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An engineer can download a multi-hundred-megabyte object to local disk, see
  progress throughout, and end with a byte-for-byte identical file — from a read-only context,
  with no `--write` flag.
- **SC-002**: An engineer can answer "how big is this prefix and what are its top consumers"
  for any bucket/prefix entirely within s3s, with no manual summation and no switch to another
  tool, and can cancel the scan on an oversized prefix without the UI ever freezing.
- **SC-003**: An engineer can act on at least 50 objects in a single bulk action (download,
  delete, or copy) and receive a truthful per-item result summary.
- **SC-004**: An engineer can surface the largest objects in a level with at most a couple of
  keystrokes (cycle sort to size-descending) and reverse the order with one more.
- **SC-005**: The end-to-end operational workflow "find what is eating space → identify the top
  consumer → remove or pull it" is completable entirely inside s3s, replacing the equivalent
  `s3cmd du` + `aws s3 cp` + manual-math sequence.
- **SC-006**: Every new operation remains cancellable and non-blocking — there is no input
  during which the UI is frozen waiting on a transfer or a recursive listing.
- **SC-007**: No new operation can mutate a remote store in a read-only context; download and
  analyze succeed there while bulk delete/copy are unavailable.
- **SC-008**: An engineer can arm and disarm write at runtime on a single hotkey with no
  restart; arming takes one deliberate confirmation, disarming takes one keystroke.
- **SC-009**: At any moment while write is armed, an observer can tell the session is dangerous
  from the screen alone — the WRITE state is unmistakable and never absent — and a session
  pointed at a `readonly: true` context can never be armed.
- **SC-010**: In usability checks, engineers do not mistakenly believe they are read-only while
  armed (or vice-versa); the indicator's state matches the session's actual write capability in
  100% of observed states.
- **SC-011**: An engineer can open a brand-new terminal and launch s3s against a configured
  context with zero secret-related steps — no `export`, no prompt — when a persistent source
  (keychain / command / AWS profile) is set.
- **SC-012**: No secret managed by s3s is ever written to disk in plaintext or required in the
  shell environment; verifiable by inspecting on-disk artifacts and the process environment.
- **SC-013**: An engineer can store a context's secret once and never re-enter it across
  sessions.
- **SC-014**: Secrets never appear in logs, UI, or error output for any credential source.
- **SC-015**: On a headless host without a keystore, an engineer can still authenticate via the
  command, AWS-profile, env, or prompt source without the tool crashing or leaking the secret.

## Assumptions

- **Download is a read operation.** Because it does not mutate the remote store, download (and
  bulk download) are allowed in read-only contexts; the only confirmation required is for
  overwriting a *local* file. This preserves the structural read-only posture — no remote
  mutation symbols are introduced outside the storage layer.
- **Default download location**: downloads go to a default directory (resolved `S3S_DOWNLOAD_DIR`
  env > `downloadDir` config key > current working directory) using the object key's base name;
  the engineer can override the destination per download via the existing in-TUI file browser.
  Bulk download recreates the key hierarchy as local subdirectories under the chosen destination.
- **Bulk mutations reuse existing safety**: bulk delete/copy are built on the existing
  single-object write operations, per-context `readonly` flag, and two-tier confirmation — no
  new *destructive-op* confirmation framework is introduced.
- **`--write` semantics change (supersedes 002 behaviour)**: the flag changes from a hard,
  fixed-at-launch master gate into the *initial* armed state only. Runtime arming via the
  hotkey replaces "restart with `--write`" as the way to enable mutations. Default launch is
  read-only; the absolute `readonly: true` context lock is unchanged. This is strictly more
  protective at the moment of mutation (loud always-on indicator + deliberate arming) and does
  not require a constitution amendment — Principle V already mandates confirmation + logging for
  destructive actions, and this adds an arming gate on top.
- **Arming confirmation is a simple deliberate confirm** (not the per-op typed confirmation):
  arming itself mutates nothing, so a single intentional confirm is enough; the destructive
  operations keep their own typed confirmation. Auto-revert-to-read-only on inactivity is a
  possible future safety enhancement and is out of scope for 005.
- **Selection scope**: a selection is per-level, holds objects only (not folders/prefixes), and
  is cleared on navigation; cross-level "carts" are out of scope for this iteration. Recursive
  folder deletion stays the existing dedicated single-folder action, deliberately kept out of
  multi-select to bound the blast radius.
- **`du` counts current object versions**, not historical versions or delete markers; full
  version-aware accounting is out of scope here (it belongs with a future versioning feature).
- **`du` and large bulk actions are best-effort and streamed**: they aggregate as they page
  through results and present running/partial truth rather than waiting for a full materialized
  listing.
- **Select-all on an unbounded level** acts on the currently loaded/visible set with a clear
  indication when more keys exist beyond what is loaded; fetching an entire huge level purely
  to select it is out of scope.
- **Surface reuse**: these operations are exposed through the existing action menu and
  non-blocking/cancellation machinery rather than a new top-level UI paradigm.
- **Credential source targets & threat model**: keystore targets are macOS Keychain and Linux
  Secret Service (libsecret/gnome-keyring); a Windows credential store is best-effort/documented.
  The AWS-profile source means static keys from the shared credentials file — SSO / role
  assumption / session tokens are out of scope for this iteration (documented). The external
  command runs with the user's own privileges and is not sandboxed, but is executed only when
  the config file is owner-only and owned by the running user (guarding against a tampered
  config injecting a command); only its output is treated as a secret. This work protects against secrets-on-disk
  in plaintext, secrets-in-environment leakage, and shoulder/history exposure; it does not
  defend against a fully compromised local account (same trust boundary as the keystore / AWS
  config the engineer already relies on). It extends the existing config loading and secret
  redaction and changes neither the read-only posture nor the storage layer.
- **Out of scope for 005** (candidates for later features): presigned URLs, clipboard yank,
  bucket creation/deletion and bucket-config inspection (policy/lifecycle/encryption/CORS),
  object tag/metadata editing, object versioning management, and incomplete-multipart-upload
  cleanup.
