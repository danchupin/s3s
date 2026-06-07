# Feature Specification: Blocked command bar (info · read · write), capability-visible in read-only

**Feature Branch**: `007-command-bar-blocks`

**Created**: 2026-06-06

**Status**: Draft

**Input**: User description: "Продолжаем улучшать интерфейс. Мне все еще не нравится меню. Давай разделим его по блокам: слева столбец/строка с информацией о подключении и s3s, далее столбец/строка с read действиями, далее столбец/строка с write действиями, причём они должны быть видны даже в read only режиме, только быть явно подсвечены неактивными — чтобы оператор имел понимание о всех возможностях даже в read only. Столбец или строка — предложи варианты и проанализируй. Должно присутствовать логичное цветовое выделение в нашей палитре. Плюс я не вижу в интерфейсе кнопку для создания нового подключения. Необходима также фича удаления подключения — на экране контекстов. Для длительных операций хочу такие же лоадеры (прогресс-бар с процентом)."

## Background & Inspiration

This iterates on the 006 redesign. 006 replaced the modal action menu with a single
always-visible hint line, but two problems remain:

1. **The hint line is an undifferentiated strip.** Connection info, read actions, and
   write actions are mixed on one (wrapping) row with no visual grouping, so the
   operator cannot scan "what can I do" at a glance.
2. **Write actions disappear in read-only.** 006 *hides* write actions when the
   context is not writable (006 FR-004). An operator on a production (read-only)
   context therefore has no idea the tool can delete/copy/move/upload at all — the
   capability map is invisible exactly when caution matters most.
3. **Adding a connection is undiscoverable.** The in-app connection manager (006 US4)
   is only reachable via the `:conn` command or a `+` in the context switcher — there
   is no visible affordance, so the operator "doesn't see a button" to add a cluster.
4. **Destructive actions fire on a single bare keystroke.** A potentially dangerous
   action (delete object, bulk delete, recursive/directory delete, delete bucket,
   move/rename, overwrite) is triggered by one un-modified key and confirmed on a footer
   status line — easy to fire by accident and easy to miss. The operator wants dangerous
   actions gated behind a **modifier chord (Ctrl+key)** and confirmed via a **centered
   popup dialog (k9s-style)**, not a footer line; and the confirmation strength should
   scale with blast radius — a **binary y/N** for single-object/group/move/overwrite, but
   **typing the exact identifier** (directory path, bucket name, connection name) for
   actions that remove a whole container.
5. **Connections can be added but never removed.** 006 added an in-app add-connection
   flow, but there is no way to delete a saved connection from inside the app — the
   operator must hand-edit the config file. A **delete-connection action on the contexts
   screen** is missing.
6. **Long operations give no progress feedback.** Operations that take a while
   (download, recursive delete, bulk copy/move/delete, `du` analyze over a deep prefix)
   show no progress — the operator cannot tell whether the tool is working or stuck. The
   operator wants a **progress bar with a percentage** in the **Claude Code style** they
   referenced — a single horizontal determinate bar (filled / unfilled track) with a
   trailing percent and an elapsed-time / label hint, drawn in the existing palette.

This feature restructures the bar into **three labelled blocks — `info · read ·
write` — laid out as side-by-side columns** (k9s-style header), keeps **all three
blocks always visible**, shows **write actions even in read-only but clearly dimmed /
inactive** (so the full capability set is always legible), surfaces a **visible
"add connection" affordance in the info block**, applies **logical, palette-
consistent color** so the operator can distinguish info / read / write / inactive at
a glance, gates **dangerous actions behind a Ctrl+key chord plus a centered popup
confirmation** so destruction is never one stray keystroke away, adds a **delete-
connection action on the contexts screen** so saved clusters can be removed in-app, and
shows a **determinate progress bar with a percentage for long-running operations** so
the operator always knows work is advancing.

**Layout decision (confirmed with the user): columns.** Analysis of the two options:

- **Columns** (chosen): three vertical blocks left→right (`info | read | write`). Best
  readability and grouping; the whole write block dims as one unit in read-only;
  matches the user's "слева … далее … далее" mental model and the k9s header. Cost:
  ~5–6 rows of height.
- **Rows** (rejected): three stacked horizontal bands. More compact in height (3 lines)
  and closer to the current footer, but the blocks read as a list, not a grouped map,
  and a wide write band is visually noisier when dimmed.

This is a UI/UX refinement: **every existing capability is preserved**; only the
arrangement, the read-only visibility of write actions, the add-connection
discoverability, and the color grouping change.

## Clarifications

### Session 2026-06-06

- Q: Deleting the last/only saved connection — allow it? → A: Allowed; on zero
  contexts the app falls back to its no-connection / add-connection state (never
  crashes with zero contexts).
- Q: Delete-connection confirmation tier? → A: Typed confirmation — the operator
  types the exact connection name in the centered popup (highest 005/006 tier),
  matching the irreversibility of removing the config triple + keychain secret.
- Q: Where is the long-operation progress bar rendered? → A: Inline in the footer
  status zone (a horizontal determinate bar + percent, Claude-Code style), leaving the
  list visible; not a centered overlay and not replacing the list body.
- Q: Confirmation strength per delete target? → A: Tier scales with blast radius —
  **binary y/N** for single-object/group/move/overwrite; **typed exact identifier** for
  container removal: directory → path, bucket → bucket name, connection → connection name.
- Q: Does whole-bucket deletion need confirmation, and how? → A: Yes — bucket delete is a
  chord-gated dangerous action requiring the operator to type the exact bucket name (added
  to the dangerous-action set; §FR-021/FR-024b).
- Q: Confirmation surface per tier? → A: Split — binary y/N uses a centered popup
  (k9s-style); the typed-identifier tier uses a prominent inline form (NOT a separate
  window) with a real editable field sized for long identifiers (§FR-023/FR-023a).
- Q: Relationship between the two surfaces? → A: They MUST share one consistent visual
  style — same palette, badge, label conventions — differing only in the binary-vs-typed
  input affordance (§FR-027a).
- Q: How to formalize the action-label rule ("simple, unambiguous, no extra words")? →
  A: A single imperative verb, ≤2 words, lowercase, no articles, no trailing punctuation;
  the object is implied by block + selection (§FR-005a, SC-014).
- Q: Bucket-delete precondition on contents? → A: Empty bucket only — a non-empty bucket
  is refused with a "purge first" nudge; bucket delete never recursively purges (§FR-024b,
  SC-015).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - See every capability grouped, even in read-only (Priority: P1)

An operator on any context sees a command bar split into three labelled, side-by-side
blocks: **info** (active context, cluster, user, region, s3s version), **read**
(download, analyze, filter/search, refresh, open), and **write** (delete, copy, move,
recursive delete, upload, new folder). All three blocks are always visible. On a
read-only context the entire write block remains visible but is rendered dimmed /
inactive, so the operator understands the tool's full capability set without having to
arm write or guess.

**Why this priority**: This is the core request — make the full capability map legible
and grouped at all times, especially in read-only. It is the reason the feature exists
and is independently shippable.

**Independent Test**: Open a read-only context; confirm the bar shows three labelled
blocks and that the write block lists delete/copy/move/etc. in a visibly dimmed style.
Arm write (`w`); confirm the write block changes to its active style. Confirm read
actions and info are always shown.

**Acceptance Scenarios**:

1. **Given** any list view, **When** the operator looks at the command bar, **Then**
   it shows three labelled blocks — info, read, write — laid out as columns.
2. **Given** a read-only context, **When** the bar renders, **Then** the write block
   is visible but dimmed/inactive (delete, copy, move, recursive delete, upload, new
   folder are all shown, not hidden).
3. **Given** a writable (armed) context, **When** the bar renders, **Then** the write
   block is shown in its active style, visually distinct from the dimmed state.
4. **Given** a read-only context, **When** the operator presses a dimmed write key
   (e.g. delete), **Then** nothing is mutated and a status line explains the context
   is read-only and that `w` arms write.
5. **Given** any context, **When** the operator scans the bar, **Then** read actions
   and the info block are always present regardless of write state.

### User Story 2 - Add a cluster connection from a visible affordance (Priority: P1)

The operator can see, in the info block, a discoverable way to add/manage cluster
connections (a labelled key, e.g. "add connection"), and activating it opens the
existing in-app connection manager. The operator no longer has to know the `:conn`
command or discover the `+` in the context switcher.

**Why this priority**: Direct user complaint ("I don't see a button to create a new
connection"). Small, high-value discoverability fix that belongs with the info block.

**Independent Test**: From the bucket list, confirm the info block shows an
"add connection" affordance with its key; press that key; confirm the connection
manager / add-connection form opens.

**Acceptance Scenarios**:

1. **Given** the command bar, **When** the operator looks at the info block, **Then**
   a labelled affordance to add/manage connections is visible with its key.
2. **Given** that affordance, **When** the operator activates it, **Then** the in-app
   connection manager (006 US4) opens.
3. **Given** the connection manager opened this way, **When** the operator adds and
   saves a connection, **Then** it behaves exactly as the existing 006 add flow
   (reachability test, keychain secret, live switch).

### User Story 3 - Distinguish blocks by logical, palette-consistent color (Priority: P2)

Each block and state is color-coded using the existing palette so the operator can
tell info from read from write, and active write from inactive write, at a glance —
without relying on reading labels.

**Why this priority**: Requested ("логичное цветовое выделение в нашей палитре") and
reinforces the grouping from US1, but the grouping is legible by layout even before
color, so it ranks below the structural change.

**Independent Test**: Render the bar in read-only and in armed states; confirm info,
read, and write blocks use distinct palette roles, the inactive write block uses the
dim/faint role, and the active write block uses its caution role.

**Acceptance Scenarios**:

1. **Given** the bar, **When** it renders, **Then** the three blocks use visually
   distinct palette roles (info, read, write) drawn only from the existing palette.
2. **Given** a read-only context, **When** the write block renders, **Then** it uses
   the dim/faint role uniformly (signalling "inactive").
3. **Given** an armed context, **When** the write block renders, **Then** it uses a
   caution role distinct from both the read block and the dimmed state.
4. **Given** every color cue, **When** color is unavailable (`NO_COLOR`), **Then** the
   active/inactive distinction is still conveyed by a non-color cue (e.g. a label such
   as "(w to arm)" / dim glyph), so meaning survives.

### User Story 4 - Dangerous actions need a Ctrl chord + a centered confirmation (Priority: P1)

A potentially dangerous action (delete object, recursive delete, move/rename, bulk
delete, or an overwrite) cannot be triggered by a single bare key. It requires a
**modifier chord (Ctrl+key)**, and on a valid chord a **centered popup dialog** (like
k9s) appears asking the operator to confirm — the highest-risk actions keep the typed
confirmation (type the exact target / count) inside that popup. Safe, reversible writes
(new folder, copy to a new key, upload to a new key) keep their single bare key.

**Why this priority**: Safety. A stray keystroke must never destroy data; this is the
core protection the operator asked for and complements the always-visible (but dimmed)
write block — the bar advertises the chord so the gate is discoverable.

**Independent Test**: In a writable context, press the bare dangerous key (e.g. `x`)
on an object → nothing happens (no delete, no prompt). Press the Ctrl chord → a
centered popup confirmation appears; complete it → the delete runs; cancel (Esc) → no
change. Confirm a safe write (new folder) still works on its bare key.

**Acceptance Scenarios**:

1. **Given** an object selected in a writable context, **When** the operator presses
   the un-modified dangerous key, **Then** nothing is mutated and no confirmation opens
   (the bare key does not trigger a dangerous action).
2. **Given** the same selection, **When** the operator presses the Ctrl chord for that
   action, **Then** its tier's confirmation surface appears — a centered popup for the
   binary tier, or a prominent inline typed form for the typed-identifier tier.
3. **Given** a container-removing action (directory/recursive delete, bucket delete,
   connection delete), **When** its popup is shown, **Then** it requires typing the exact
   identifier (path / bucket name / connection name) before proceeding; single-object,
   group, move, and overwrite use a binary (y/N) confirmation instead.
4. **Given** either confirmation surface, **When** the operator cancels (Esc), **Then**
   nothing is changed and focus returns to the prior view.
5. **Given** a safe, reversible write (new folder, copy/upload to a new key), **When**
   the operator triggers it, **Then** it uses its single bare key (no Ctrl chord) and
   its existing simple confirmation.
6. **Given** the write block, **When** it renders, **Then** dangerous entries display
   their Ctrl chord (e.g. "^X delete") so the gate is discoverable.

### User Story 5 - Delete a saved connection from the contexts screen (Priority: P2)

The operator can remove a saved cluster connection from inside the app, on the contexts
screen, without hand-editing the config file. Deleting a connection is a destructive
config action, so it is gated (chord + centered confirmation, US4) and cleans up the
config triple and the keychain secret. The active connection cannot be deleted out from
under the running session.

**Why this priority**: Completes the connection lifecycle (006 added create; this adds
remove). Valuable but not blocking the bar redesign, so it ranks below the P1 structure
and safety work.

**Independent Test**: Open the contexts screen with ≥2 saved connections; select a
non-active one; trigger delete → a centered confirmation appears; confirm → the
connection disappears from the list, its config triple is removed, and its keychain
secret is deleted. Try to delete the active connection → it is refused with an
explanation.

**Acceptance Scenarios**:

1. **Given** the contexts screen with a non-active connection selected, **When** the
   operator triggers delete-connection (Ctrl chord), **Then** a centered confirmation
   dialog appears requiring the operator to type the exact connection name.
2. **Given** that confirmation, **When** the operator confirms, **Then** the connection
   is removed from the config (cluster + user + context triple) and its secret is
   deleted from the keychain, and the contexts list updates live.
3. **Given** that confirmation, **When** the operator cancels (Esc), **Then** nothing is
   changed.
4. **Given** the currently active connection is selected, **When** the operator triggers
   delete, **Then** it is refused with a status line explaining the active context
   cannot be deleted (switch away first).
5. **Given** the contexts screen, **When** it renders, **Then** the delete-connection
   key is discoverable on that screen alongside add/switch.
6. **Given** the last/only connection is deleted, **When** zero contexts remain, **Then**
   the app falls back to its no-connection / add-connection state without crashing.

### User Story 6 - Progress bar with percent for long operations (Priority: P2)

When an operation runs long enough to notice (download, recursive delete, bulk
copy/move/delete, `du` analyze over a deep prefix), a Claude-Code-style determinate
progress bar with a percentage and an elapsed/label hint is shown, so the operator can
tell the tool is advancing and roughly how far along it is. The progress display never
blocks the UI (consistent with the non-blocking TUI principle) and clears on completion
or cancel.

**Why this priority**: Strong UX improvement for the write/bulk operations added in
003/005, but the operations already function without it; it is additive feedback, not a
new capability.

**Independent Test**: Start a long operation (e.g. a large download or a bulk delete of
many objects); confirm a horizontal progress bar with a percent appears and advances;
confirm the UI stays responsive (the operation can be cancelled); confirm the bar clears
when the operation finishes.

**Acceptance Scenarios**:

1. **Given** a long-running operation, **When** it is in progress, **Then** a single
   horizontal determinate progress bar with a trailing percent and an elapsed/label hint
   is shown.
2. **Given** the operation advances, **When** progress updates, **Then** the bar's filled
   portion and the percent update monotonically toward 100%.
3. **Given** the operation completes, **When** it finishes (or is cancelled), **Then**
   the progress bar clears and the prior view / result is shown.
4. **Given** an operation whose total size/count is unknown, **When** progress cannot be
   computed as a percentage, **Then** an indeterminate variant (activity indicator) is
   shown instead of a misleading percent.
5. **Given** any operation showing progress, **When** it runs, **Then** the UI remains
   responsive and the operation is cancellable (non-blocking).

### Edge Cases

- **Narrow / short terminal**: the three columns must collapse gracefully (reflow to
  fewer columns or a compact form) without clipping the list, the loud write/read-only
  badge, or dropping the write block entirely; the capability map must remain legible.
- **Multi-select active**: the read/write blocks reflect the bulk variants (e.g.
  "delete N", "download N") consistent with 006 multi-select.
- **`readonly: true` context vs disarmed**: both render the write block dimmed; the
  badge already distinguishes the absolute lock from a re-armable session.
- **Selection-dependent actions**: actions that require a specific selection (e.g.
  recursive delete needs a folder; move needs an object) are shown in their block but
  indicated as not-applicable to the current selection, distinct from write-disabled.
- **No connections / single connection**: the "add connection" affordance is always
  present even when only one context exists.
- **Confirmation surface on a tiny terminal**: both the centered popup and the inline
  typed form must shrink/wrap to fit rather than clip their prompt or input; a long path /
  bucket name in the typed form must stay legible (scroll or wrap).
- **Wrong typed identifier**: a directory path / bucket name / connection name that does
  not match exactly MUST abort with no mutation and allow retry (no partial/prefix match).
- **Mixed selection (objects + a folder)**: escalates to the typed-path tier (the folder
  raises the blast radius above a binary group delete).
- **Bucket delete on a non-empty bucket**: refused with a status line to empty/purge the
  bucket first; bucket delete requires an already-empty bucket and never recursively
  purges contents itself.
- **Dangerous chord while read-only**: the Ctrl chord MUST NOT open a popup in a
  read-only context — it falls through to the same read-only nudge as the bare key.
- **Terminal that cannot emit a given Ctrl chord**: the chord set MUST avoid
  combinations terminals reserve/cannot send (e.g. flow-control), so every dangerous
  action stays reachable.
- **Delete the active connection**: refused — the running session depends on it; the
  operator must switch context first.
- **Delete the last/only connection**: allowed — the app falls back to its
  no-connection / add-connection state and MUST NOT crash with zero contexts (resolved
  in Clarifications).
- **Keychain secret already absent on delete**: deleting a connection whose secret is
  missing from the keychain MUST still remove the config triple cleanly (best-effort
  secret cleanup, no hard failure).
- **Operation finishes faster than the progress threshold**: a short operation MUST NOT
  flash a progress bar; the bar appears only once the operation crosses a brief
  "is-taking-a-while" threshold.
- **Progress during a centered confirmation**: progress for an operation is shown after
  its confirmation completes, not behind/over the still-open popup.
- **Unknown total (indeterminate progress)**: when the operation's total is unknowable
  up front, show an indeterminate activity indicator rather than a fabricated percent.

## Requirements *(mandatory)*

### Functional Requirements

#### Block structure & layout (US1)

- **FR-001**: The command bar MUST be organized into three labelled blocks in this
  order: **info**, **read**, **write**.
- **FR-002**: The blocks MUST be laid out as side-by-side columns (info left, then
  read, then write).
- **FR-003**: The **info** block MUST show the active context, cluster, user, region,
  and s3s version, plus the add-connection affordance (US2).
- **FR-004**: The **read** block MUST list the read actions: download, analyze (`du`),
  filter/search, refresh, and open (Enter).
- **FR-005**: The **write** block MUST list the write actions: delete, copy,
  move/rename, recursive delete, upload, new folder.
- **FR-005a**: Every action label (read and write blocks) MUST be a single imperative
  verb, one word where possible and at most two words for compound actions (e.g.
  "delete", "copy", "upload", "new folder"), lowercase, with no articles and no trailing
  punctuation; the object is implied by the block and current selection, not repeated in
  the label.
- **FR-006**: All three blocks MUST be visible at all times (subject to responsive
  collapse, FR-016) — none is hidden based on write state.

#### Read-only capability visibility (US1 — headline)

- **FR-007**: In a read-only context (disarmed or `readonly: true`) the write block
  MUST remain visible but be rendered dimmed/inactive (it MUST NOT be hidden, reversing
  006 FR-004).
- **FR-008**: In a writable (armed) context the write block MUST be rendered in an
  active style, visually distinct from the dimmed state.
- **FR-009**: Pressing a write action's key in a read-only context MUST NOT mutate
  anything and MUST surface a status line explaining the context is read-only and that
  `w` arms write (no silent no-op).
- **FR-010**: The transition between dimmed and active write styling MUST follow the
  runtime write-arm state and update immediately when write is armed/disarmed.

#### Add-connection discoverability (US2)

- **FR-011**: The info block MUST present a visible, labelled affordance (with its key)
  to add/manage cluster connections, available in any context.
- **FR-012**: Activating that affordance MUST open the existing in-app connection
  manager (006 US4) with no change to its add/test/save behavior.

#### Color coding (US3)

- **FR-013**: The three blocks MUST be color-coded using only the existing palette
  (006 FR-031 token set); no new hues are introduced. Info, read, and write MUST use
  distinct roles, and the inactive write block MUST use the dim/faint role.
- **FR-014**: The active write block MUST use a caution role distinct from the read
  block and from the dimmed state.
- **FR-015**: Every meaning conveyed by color (active vs inactive write, block
  identity) MUST have a redundant non-color cue so it survives `NO_COLOR` (consistent
  with 006 FR-034/FR-041/FR-042).

#### Responsiveness & preservation (cross-cutting)

- **FR-016**: On a terminal too small for three columns, the bar MUST collapse
  gracefully (reflow to fewer columns or a compact form) without clipping the list, the
  loud write/read-only badge, or omitting the write block; the capability map MUST stay
  legible.
- **FR-017**: The bar MUST reflect 006 multi-select: when objects are marked, the
  read/write blocks show the bulk variants and counts.
- **FR-018**: Actions that are inapplicable to the current selection (e.g. recursive
  delete with no folder selected) MUST be indicated as not-applicable, distinct from
  the write-disabled (read-only) dimming.
- **FR-019**: Every action reachable in 006 MUST remain reachable; the single-key
  direct-action behavior and its confirmations are unchanged — this feature changes
  presentation/visibility, not the action flows.
- **FR-020**: The loud always-on write/read-only badge (005 FR-027) MUST remain present
  and unmistakable alongside the blocks.

#### Dangerous-action gating & centered confirmation (US4)

- **FR-021**: Dangerous actions — delete object, bulk delete, recursive (directory)
  delete, delete bucket, move/rename, and any detected overwrite — MUST require a modifier
  chord (Ctrl+key) to trigger; the corresponding un-modified key MUST NOT trigger the
  dangerous action.
- **FR-022**: Safe, reversible writes (new folder, copy to a new key, upload to a new
  key, download, bulk download) MUST keep their single bare key (no chord).
- **FR-023**: A valid dangerous chord MUST open a confirmation surface chosen by tier
  (not a footer status line):
  - **Binary (y/N) tier** — single-object/group delete, move, overwrite: a centered
    popup confirmation dialog (k9s-style).
  - **Typed-identifier tier** — directory/recursive delete, bucket delete, connection
    delete: a **prominent inline confirmation form** (NOT a separate/modal window) with a
    real editable input sized for long identifiers, visually prominent so it is not missed.
- **FR-023a**: The typed-identifier inline form MUST provide a usable editable field:
  single line, editable, paste-capable, with horizontal scroll or wrap so a long path /
  bucket name remains legible and verifiable as the operator types; it MUST be visually
  prominent (distinct palette role + the loud write badge) without being a separate
  window.
- **FR-024**: The confirmation tier MUST be chosen per target, not per action name, each
  on its tier's surface (FR-023):
  - **Binary (y/N)** — single-object delete, selected-group (bulk) delete, move/rename,
    and detected overwrite: a binary confirmation in the centered popup (no typed input).
  - **Typed exact identifier** — actions that remove a whole container/identifier require
    the operator to type that identifier exactly in the inline confirmation form before
    proceeding: directory/recursive delete → the directory's **exact path**; bucket
    delete → the **exact bucket name**; connection delete → the **exact connection name**
    (FR-030).
- **FR-024a**: The typed-identifier tier MUST be one shared confirmation mechanism with
  three identifier targets (path / bucket name / connection name); the typed input MUST
  require an exact, case-sensitive full match, and a wrong/typo'd entry MUST abort with no
  mutation (retry allowed).
- **FR-024b**: Bucket deletion MUST be in the dangerous-action set (chord-gated, FR-021)
  and MUST use the typed-exact-bucket-name tier. It requires an **empty bucket**: a
  non-empty bucket MUST NOT be deleted — the confirmation/attempt MUST be refused with a
  status line telling the operator to empty (purge) the bucket first. No recursive purge
  is performed as part of bucket delete.
- **FR-025**: Cancelling either confirmation surface (Esc) MUST change nothing and return
  to the prior view; confirming MUST run the existing operation flow unchanged (same
  backend calls, logging, generation handling). Cancel/submit behavior MUST be identical
  across the binary popup and the inline typed form.
- **FR-026**: The write block MUST display the Ctrl chord for each dangerous entry
  (e.g. "^X delete") so the gate is discoverable; safe writes show their bare key.
- **FR-027**: Both confirmation surfaces (centered popup and inline typed form) MUST
  remain readable and not clip on small terminals, and MUST carry the loud write/read-only
  badge like every other screen (005 FR-027).
- **FR-027a**: The two confirmation surfaces MUST share one consistent visual style — same
  palette roles, badge placement, title/label conventions, and key hints — so they read as
  one design language, differing only in the binary-vs-typed input affordance.
- **FR-028**: Dangerous chords are only triggerable in a writable context; in read-only
  the dimmed write block + `w` nudge (FR-007/FR-009) still applies (no popup opens).

#### Delete connection (US5)

- **FR-029**: The contexts screen MUST offer a discoverable delete-connection action
  (key shown on that screen alongside add/switch).
- **FR-030**: Deleting a connection MUST be treated as a highest-tier dangerous action:
  it MUST be gated by the Ctrl chord and use the shared typed-exact-identifier
  confirmation (FR-024/FR-024a) with the connection name as the identifier.
- **FR-031**: On confirmation, the connection's config triple (cluster + user + context)
  MUST be removed from config and persisted, and its secret MUST be deleted from the
  keychain (best-effort: a missing secret MUST NOT block config removal).
- **FR-032**: The active/current connection MUST NOT be deletable; attempting it MUST be
  refused with a status line telling the operator to switch context first.
- **FR-033**: After a successful delete, the contexts list MUST update live (no restart),
  consistent with the 006 live-switch behavior.

#### Progress feedback for long operations (US6)

- **FR-034**: Long-running operations (download, recursive delete, bulk copy/move/delete,
  `du` analyze) MUST display a Claude-Code-style determinate progress bar with a trailing
  percentage and an elapsed/label hint, drawn only from the existing palette, rendered
  inline in the footer status zone (the list stays visible; not a centered overlay, not
  replacing the list body).
- **FR-035**: The progress bar MUST appear only after the operation crosses a brief
  "taking a while" threshold, so fast operations do not flash a bar.
- **FR-036**: Progress MUST advance monotonically toward 100% and the bar MUST clear when
  the operation completes or is cancelled, returning to the prior view/result.
- **FR-037**: When the total work is unknowable up front, an indeterminate activity
  indicator MUST be shown instead of a fabricated percentage.
- **FR-038**: Progress display MUST NOT block the UI: the operation MUST stay cancellable
  and the TUI responsive while progress is shown (constitution II, non-blocking).

### Key Entities *(include if data involved)*

- **Command-bar block**: a labelled group (`info` / `read` / `write`) with a title, a
  palette role, and an ordered set of entries; the write block additionally has an
  active/inactive state derived from the runtime write-arm state.
- **Bar entry**: one item in a block — for info, a labelled field or the add-connection
  affordance; for read/write, an action with its key, label, applicability to the
  current selection, and (write only) enabled/dimmed state.
- **Progress state**: per-operation feedback — a label, a determinate fraction (0–100%)
  or an indeterminate flag, an elapsed-time hint, and a cancel handle; transient (created
  when an operation crosses the "taking a while" threshold, cleared on completion/cancel).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a read-only context, 100% of write actions (delete, copy, move,
  recursive delete, upload, new folder) are visible in the bar (dimmed), versus 0%
  visible today (006 hides them).
- **SC-002**: A first-time operator can name the tool's full read and write capability
  set from the bar alone, in read-only, without arming write or opening help
  (task-based observation: ≥90% name all six write actions).
- **SC-003**: The operator can locate and open the add-connection flow from a visible
  affordance in under 10 seconds, without prior knowledge of the `:conn` command.
- **SC-004**: Info, read, and active-write, and inactive-write are each visually
  distinguishable (by an observer asked to point at each block) in 100% of renders.
- **SC-005**: The bar (three blocks + badge) renders without clipping the list or badge
  across terminal sizes from 80×24 to large.
- **SC-006**: 100% of actions reachable in 006 remain reachable; pressing a write key
  in read-only mutates nothing in 100% of cases.
- **SC-007**: Color-conveyed distinctions remain legible under `NO_COLOR` (active vs
  inactive write distinguishable by a non-color cue in 100% of renders).
- **SC-008**: A bare (un-modified) keystroke triggers a dangerous action in 0% of
  cases — every object/bulk/recursive delete, bucket delete, move, and overwrite requires
  the Ctrl chord plus the centered popup before any mutation.
- **SC-008a**: The confirmation tier matches the target in 100% of cases: object/group/
  move/overwrite show a binary y/N; directory, bucket, and connection deletes require the
  exact typed identifier and abort on any mismatch (0% deletions on a wrong identifier).
- **SC-009**: Both confirmation surfaces (centered popup, inline typed form) render
  without clipping their prompt or input across terminal sizes from 80×24 to large; a long
  path / bucket name stays legible (scroll or wrap) in the typed form.
- **SC-009a**: An observer comparing the two confirmation surfaces judges them one
  consistent style (same palette, badge, label conventions) in 100% of renders.
- **SC-010**: An operator can delete a saved (non-active) connection entirely from within
  the app — config triple removed and keychain secret deleted — in 0 hand-edits of the
  config file.
- **SC-011**: Deleting the active connection is refused in 100% of attempts (the running
  session is never left without its backend).
- **SC-012**: For an operation that runs longer than the "taking a while" threshold, a
  progress indicator (determinate bar + percent, or indeterminate fallback) is shown in
  100% of cases, and the UI stays responsive/cancellable throughout.
- **SC-013**: Operations that finish under the threshold show no progress bar (0% flash
  rate), so the indicator never flickers for fast actions.
- **SC-014**: 100% of read/write action labels conform to the label rule (single
  imperative verb, ≤2 words, lowercase, no articles, no trailing punctuation) — checkable
  by inspecting the label set.
- **SC-015**: Deleting a non-empty bucket is refused in 100% of attempts (no bucket with
  objects is ever removed by the bucket-delete action).

## Assumptions

- **Layout = columns** (info | read | write), confirmed with the user; rows/hybrid are
  out of scope.
- **Dimmed write key press = no-op + a "read-only, press `w`" nudge** (confirmed);
  pressing a dimmed key does not auto-arm write.
- **Blocks are always visible**, collapsing only on terminals too small to fit them
  (confirmed); there is no manual toggle in scope.
- The palette and the runtime write-arm toggle (`w`, with its loud badge) from 005/006
  are reused unchanged; this feature re-skins the bar, it does not change arming.
- The add-connection affordance opens the existing 006 connection manager; no change to
  the connection add/test/save flow itself.
- Read actions include `open` (Enter) and filter/search as navigational reads; the
  exact label set may be tuned in planning without changing scope.
- The full-screen help overlay remains; the always-visible blocks reduce reliance on it
  but do not replace it.
- **Dangerous actions (Ctrl chord + centered popup)**: delete object, bulk delete,
  recursive/directory delete, delete bucket, move/rename, detected overwrite, and delete
  connection. Reversible writes (new folder, copy/upload to a new key, download) stay
  bare-key + simple confirm. The exact chord letters are chosen in planning (avoiding
  terminal-reserved combos); the spec only requires "a Ctrl chord".
- **Confirmation tier scales with blast radius (resolved in Clarifications + follow-ups)**:
  - **Binary y/N** inside the popup: single-object delete, selected-group (bulk) delete,
    move/rename, overwrite.
  - **Typed exact identifier** inside the popup (one shared mechanism, case-sensitive full
    match, abort on mismatch): directory/recursive delete → path; bucket delete → bucket
    name; connection delete → connection name.
- Destructive confirmations leave the footer status line, split by tier: the **binary
  y/N** tier uses a **centered popup** (k9s-style); the **typed-identifier** tier uses a
  **prominent inline form** (not a separate window) with a real editable field sized for
  long identifiers. Both surfaces MUST share one consistent visual style (FR-027a) —
  same palette, badge, label conventions — differing only in the input affordance.
  Non-destructive prompts (name/destination entry) may stay inline.
- **Delete connection is a highest-tier destructive config action**: it reuses the US4
  chord + centered-confirm path (typed connection name) and the 006 config/keychain
  mechanisms in reverse (remove triple + delete secret). The active context is
  non-deletable; the last/only connection IS deletable, falling back to the
  no-connection / add-connection state (resolved in Clarifications).
- **Progress = Claude-Code-style determinate bar** (filled/unfilled track + trailing
  percent + elapsed/label hint), rendered **inline in the footer status zone**, reusing
  the existing palette and the non-blocking `tea.Cmd` + generation model; the exact
  "taking a while" threshold and whether progress is per-item or byte-based per operation
  are tuned in planning. No backend interface change is required beyond surfacing progress
  already available to the operation.
