# Feature Specification: UI Redesign (k9s-style, menu-less actions, in-app connections)

**Feature Branch**: `006-ui-redesign`

**Created**: 2026-06-06

**Status**: Draft

**Input**: User description: "Мне все еще не нравится ui - он очень неудобный. Я хочу чтобы ты проанализировал популярные tui и подобрал удачный ui для нашего проекта (мне нравится k9s, но нужно посмотреть другие подходящие также). Контекстное меню действий оказалось очень неудобным, я не хочу лишних кнопок перед действием. Меню можно сделать больше, нам не нужно такая большая область под объекты — вряд ли их будут смотреть большими пачками. Давай переработаем ui полностью, чтобы было просто понятно и удобно. Также я хочу возможность добавления подключения к кластеру прямо из меню."

## Background & Inspiration

This rework replaces the current layout — a full-width list box plus a modal
**action menu** opened with `a` — that the user finds slow and unintuitive. A
survey of well-liked keyboard-driven TUIs informed the target experience:

- **k9s** (the user's stated favourite): a full-width resource table, a `:`
  command bar to jump between views, **single-key actions performed directly on
  the highlighted row** (no intermediate menu), `/` to filter, `?` for help, and
  a persistent breadcrumb header.
- **ranger / lf / yazi** (terminal file managers): instant, consistent
  single-key bindings; a **preview/details pane shown alongside the list** so the
  list never needs to occupy the entire screen; immediate visual feedback.
- **ncdu** (already echoed by the `du` view): a ranked, drill-down breakdown.

The chosen direction (confirmed with the user): a **k9s-style single full-width
table with a persistent details/preview pane**, **menu-less single-key actions**
backed by an always-visible contextual hint bar, a **`:` command bar** for
jumping and discovery, and the ability to **add a cluster connection from inside
the app** (persisted to config; secrets stored in the OS keychain, reusing the
005 credential backbone).

This is a UI/UX rework: **every existing capability is preserved** (browse,
context switch, object view/preview, download, `du` analytics, multi-select bulk
download/delete/copy, sort, the runtime read-only↔write toggle and its loud
badge, the two-tier destructive confirmations). What changes is *how the user
reaches and perceives them*, not what they do.

## Clarifications

### Session 2026-06-06

- Q: Details pane load strategy as the selection moves? → A: Debounced — fetch after the selection settles (~150–250ms idle); supersede in-flight loads on the way (no fetch per row during fast scroll).
- Q: Must a new connection pass a live reachability test before it can be saved? → A: Test + override — run a connection test on save and show the result; on failure offer "save anyway" (never block a deliberate offline save).
- Q: How does a new connection map onto the existing cluster/user/context config model? → A: Auto-create a cluster + user + context triple from one form; entry names derived from the connection name; on-disk schema unchanged.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Act on an item with one keypress, no menu (Priority: P1)

A user browsing a bucket highlights an object and performs the action they want —
download, delete, copy, move, analyze — with a **single, discoverable keypress**,
without first opening any menu. The available actions for the current selection
are always visible in a contextual hint bar at the bottom, so the user never has
to remember or hunt for them. The modal action menu (`a`) is removed entirely.

**Why this priority**: This is the user's central complaint — "контекстное меню
оказалось очень неудобным, я не хочу лишних кнопок перед действием." Removing the
extra step in front of every action is the single biggest usability win and the
reason this feature exists. It is independently shippable: even with no layout or
connection changes, direct keys + a visible legend already make the tool feel
faster.

**Independent Test**: With the redesigned build, highlight an object and press the
download key; the download starts immediately (no menu appears). Repeat for
delete/copy/move/analyze. Confirm the bottom hint bar lists exactly the actions
valid for the current selection and capability (write actions hidden/disabled
when read-only).

**Acceptance Scenarios**:

1. **Given** an object is highlighted in a writable context, **When** the user
   presses the delete key, **Then** the existing two-tier delete confirmation
   opens directly — no action menu is shown at any point.
2. **Given** an object is highlighted, **When** the user presses the download key,
   **Then** the download begins immediately with progress shown.
3. **Given** the active context is read-only (armed off or `readonly: true`),
   **When** the user looks at the hint bar, **Then** write actions (delete, copy,
   move, upload, new folder) are not offered as available keys, and pressing a
   write key is a safe no-op with an explanatory status line.
4. **Given** any list view, **When** the user looks at the bottom of the screen,
   **Then** a contextual hint bar shows the valid single-key actions for the
   current selection without the user opening help.
5. **Given** the user presses the former menu key (`a`), **Then** no modal action
   menu opens (the menu no longer exists).

---

### User Story 2 - See the list and the selected item's details at once (Priority: P1)

The browse screen shows a **compact list on one side and a persistent details /
preview pane on the other**. As the user moves the selection, the pane updates to
show the highlighted item's metadata (size, type, last-modified, ETag, storage
class) and, for an object, a bounded content preview. The list no longer consumes
the entire screen, because objects are rarely scanned in huge batches; the freed
space surfaces useful per-item information continuously.

**Why this priority**: Directly addresses "нам не нужно такая большая область под
объекты" and "меню можно сделать больше." It reshapes the dominant screen the user
spends most time on. Together with US1 it delivers the "просто, понятно, удобно"
goal. Independently testable and valuable on its own (information is visible
without pressing Enter into a separate object screen).

**Independent Test**: Navigate a bucket and move the selection up/down; verify the
details pane updates live to reflect the highlighted item, that the list occupies
a sensibly bounded portion of the width/height (not the whole frame), and that an
object's preview appears in the pane without entering a separate full-screen view.

**Acceptance Scenarios**:

1. **Given** a bucket level with objects and folders, **When** the user moves the
   selection, **Then** the details pane updates within one frame to show the
   highlighted item's attributes.
2. **Given** a text/image object is highlighted, **When** the details pane
   renders, **Then** a bounded content preview is shown inline in the pane.
3. **Given** a folder or the level itself is highlighted, **When** the pane
   renders, **Then** it shows level/folder summary information instead of an
   object preview.
4. **Given** a terminal too narrow to fit list + pane side by side, **When** the
   screen renders, **Then** the layout degrades gracefully (pane stacks below, or
   collapses to an on-demand toggle) without clipping the hint bar or footer.
5. **Given** the details pane is visible, **When** the user navigates into a
   folder or back up, **Then** the pane follows the new selection and never shows
   stale data from a superseded load.

---

### User Story 3 - Jump and run via a command bar (Priority: P2)

The user presses `:` to open a **command bar** and types a short command to jump
to a view or run an action — e.g. switch to the bucket list, open contexts, open
the connection manager, trigger analyze/refresh, or quit. The command bar offers
the set of available commands (and accepts a typed prefix), giving a discoverable,
keyboard-only way to reach everything without memorizing every single-key binding.

**Why this priority**: Mirrors the k9s pattern the user likes and complements US1:
single keys for the frequent actions, a command bar for the long tail and for
discovery. Valuable but secondary to the direct-action and layout changes.

**Independent Test**: Press `:`, type a known command name (e.g. the one for the
context list), and confirm the app navigates there. Press `:`, observe the list of
available commands. Press Esc to dismiss the bar with no effect.

**Acceptance Scenarios**:

1. **Given** any view, **When** the user presses `:`, **Then** a command bar opens
   and accepts typed input.
2. **Given** the command bar is open, **When** the user types a valid command and
   confirms, **Then** the app performs that command (jump or action) and closes
   the bar.
3. **Given** the command bar is open, **When** the user presses Esc, **Then** the
   bar closes and nothing else changes.
4. **Given** the command bar is open, **When** the user types an unknown command
   and confirms, **Then** a non-destructive error/notice is shown and no action is
   taken.

---

### User Story 4 - Add a cluster connection from inside the app (Priority: P2)

A user who wants to point s3s at a new cluster opens a **connection manager** from
the UI, fills in a short form (a display name, endpoint, region, an access key id,
a secret access key, and an optional read-only flag), and connects — **without
hand-editing the YAML config**. On save, the new connection is **persisted to the
config file** so it is available on the next launch and appears in the context
list; the **secret access key is stored in the OS keychain** (reusing the 005
credential backbone), never written to the config in plaintext. After saving, the
user can switch to the new connection immediately.

**Why this priority**: A concrete user request ("возможность добавления
подключения к кластеру прямо из меню") and a real onboarding improvement, but the
tool is still fully usable for existing contexts without it, so it ranks below the
core interaction rework.

**Independent Test**: From the UI, open the connection manager, add a connection
pointing at a reachable test backend (e.g. local MinIO), save it, and confirm: the
new context appears in the context list, switching to it lists that backend's
buckets, the config file gained the new context (without the secret in plaintext),
and the secret is retrievable from the keychain on the next launch.

**Acceptance Scenarios**:

1. **Given** the connection manager is open, **When** the user enters a valid
   connection and saves, **Then** the connection is written to the config file,
   the secret is stored in the keychain, and the new context appears in the
   context list.
2. **Given** a newly saved connection, **When** the user switches to it, **Then**
   the app connects and lists that backend's buckets (or surfaces a clear,
   secret-free error if unreachable).
3. **Given** the user enters a name that duplicates an existing context, **When**
   they try to save, **Then** the app rejects the save with a clear message and
   does not overwrite the existing context.
4. **Given** the connection form, **When** the user leaves a required field empty
   or enters an obviously invalid endpoint, **Then** validation blocks the save
   with a field-level message.
5. **Given** a saved connection, **When** the user inspects the config file,
   **Then** the secret access key is absent from it (only a keychain reference is
   stored), consistent with the project's "secrets are never committed" rule.
6. **Given** the connection is saved, **When** s3s is restarted, **Then** the
   connection is still present and usable, with its secret resolved from the
   keychain.

---

### Edge Cases

- **Narrow / short terminal**: the side-by-side list+pane must collapse to a
  stacked or toggled layout; the hint bar must wrap rather than drop; the write
  badge must remain visible (per existing FR-027 from 005).
- **Empty selection / empty level**: the details pane and hint bar must show a
  sensible state (no actions that require a selection) without errors.
- **Read-only context**: every write single-key and every write command must be a
  safe, explained no-op; the connection manager may still add connections (a local
  config edit, not a backend mutation), but the `readonly` flag of the *active*
  context is independent of being able to define new ones.
- **Selection moves faster than previews load**: preview/detail loads for
  superseded selections must be dropped (existing generation mechanism), so the
  pane never shows data for a row the user already moved off.
- **Connection save fails** (config not writable, keychain unavailable): the user
  gets a clear error and is not left believing the connection was saved; partial
  writes (config updated but secret not stored, or vice versa) must be avoided or
  clearly reported.
- **Multi-select active**: single-key actions that have a bulk counterpart act on
  the marked set; the hint bar reflects the bulk variant and count.
- **Command bar vs filter**: `:` (command) and `/` (filter) must be unambiguous and
  not interfere with each other or with in-progress text entry.
- **Migration**: users coming from the old build press `a` (old menu) or `r` (old
  refresh) out of habit; these must fail safely with a hint pointing at the new
  binding rather than doing nothing silently.

## Requirements *(mandatory)*

### Functional Requirements

#### Menu-less direct actions (US1)

- **FR-001**: The system MUST remove the modal action menu; no keypress opens a
  menu that lists actions to choose before performing one.
- **FR-002**: The system MUST let the user invoke each item action (download,
  delete, copy, move/rename, analyze, upload, new folder, recursive delete) with a
  single keypress on the relevant selection, going straight into that action's
  existing flow (including its confirmation, where applicable).
- **FR-003**: The system MUST display an always-visible, contextual hint bar
  listing the single-key actions valid for the current view, selection kind, and
  write capability.
- **FR-004**: The system MUST hide or clearly disable write actions (and not act on
  their keys beyond a safe, explained no-op) when the active context is not
  writable (disarmed or `readonly: true`).
- **FR-005**: The system MUST keep the existing two-tier destructive-confirmation
  behavior for every mutating action reached directly (simple `y/N` vs typed
  target/count), satisfying Constitution V — direct invocation MUST NOT bypass
  confirmation.
- **FR-006**: When the multi-select set is non-empty, single-key actions that have
  a bulk counterpart MUST act on the marked set, and the hint bar MUST reflect the
  bulk variant and the selected count.
- **FR-007**: Former bindings that no longer apply (old menu `a`, old top-level
  refresh `r` if relocated) MUST fail safely with a brief hint toward the new way,
  not silently do nothing.

#### Rebalanced k9s-style layout (US2)

- **FR-008**: The browse screen MUST present a list and a persistent details /
  preview pane simultaneously, with the list bounded so it does not occupy the
  entire frame.
- **FR-009**: The details pane MUST reflect the currently highlighted item as the
  selection moves. Instantly-known list fields (name, size, last-modified) render
  immediately; the full metadata + preview fetch is **debounced** — it fires only
  after the selection settles (~150–250 ms idle), and in-flight fetches for rows
  the user has scrolled past are superseded/cancelled. Fast scrolling MUST NOT fire
  a metadata/preview fetch per row.
- **FR-010**: For an object selection, the details pane MUST show key metadata
  (at least size, content type, last-modified, ETag, storage class when available)
  and a bounded inline content preview.
- **FR-011**: For a folder or level selection, the details pane MUST show
  level/folder summary information rather than an object preview.
- **FR-012**: The system MUST drop detail/preview loads for superseded selections
  so the pane never shows data for a row the user has moved off (reusing the
  existing generation/cancellation mechanism — Constitution II).
- **FR-013**: The system MUST degrade the layout gracefully on small terminals
  (stack or toggle the pane) without clipping the hint bar, footer, or the loud
  write badge.
- **FR-014**: The system MUST keep the breadcrumb/location header, the bucket /
  prefix / count / sort indicators, and the multi-line status footer from the
  current design (or an equivalent), including the always-on write/read-only badge
  (005 FR-027).
- **FR-015**: The full-screen object view reached by Enter MAY be retained for a
  larger preview, but it MUST NOT be the *only* way to see an object's basic
  metadata (the pane covers the at-a-glance case).

#### Command bar (US3)

- **FR-016**: The system MUST provide a command bar opened with `:` that accepts
  typed commands to jump to views (buckets, contexts, connection manager, help)
  and to run actions (e.g. analyze, refresh, quit).
- **FR-017**: The command bar MUST present the set of available commands (for
  discovery) and accept a typed command name or prefix.
- **FR-018**: Esc MUST dismiss the command bar with no side effect; an unknown
  command MUST produce a non-destructive notice and take no action.
- **FR-019**: The command bar (`:`) and the filter (`/`) MUST be unambiguous and
  MUST NOT interfere with each other or with in-progress text entry in operation
  prompts.

#### In-app connection management (US4)

- **FR-020**: The system MUST provide a connection manager, reachable from the UI
  (a single key and/or a `:` command), that lists existing connections and offers
  to add a new one.
- **FR-021**: The add-connection form MUST collect at least: a display/context
  name, endpoint, region, access key id, secret access key, and an optional
  read-only flag; with field-level validation for required/format errors.
- **FR-022**: On save, the system MUST persist the new connection to the config
  file so it survives restart and appears in the context list, **without** writing
  the secret access key into the config in plaintext.
- **FR-022a**: The writer MUST map one filled form onto the existing config model
  by auto-creating a **cluster** (endpoint/region), a **user** (access key id +
  keychain credential reference), and a **context** (cluster + user + readonly)
  triple; entry names are derived from the connection display name. The on-disk
  config schema MUST remain unchanged (a config so written is indistinguishable
  from a hand-written one and interoperates with existing contexts/tooling).
- **FR-023**: On save, the system MUST store the secret access key in the OS
  keychain via the existing 005 credential backbone (`internal/secret`), recording
  only a keychain reference in the config.
- **FR-024**: The system MUST reject a new connection whose derived names collide
  with an existing context (or its derived cluster/user entries), with a clear
  message, without overwriting any existing entry.
- **FR-025**: After a successful save, the user MUST be able to switch to the new
  connection within the same session without restarting.
- **FR-025a**: On save, the system MUST run a live reachability test against the
  new connection (off the event loop) and show its result. If the test fails, the
  system MUST offer an explicit "save anyway" path — it MUST NOT silently persist a
  failing connection, and MUST NOT block a user who deliberately saves an offline /
  not-yet-reachable cluster. A reachability failure is reported secret-free.
- **FR-026**: If saving fails (config not writable, keychain unavailable), the
  system MUST report a clear, secret-free error and MUST NOT leave an inconsistent
  state where the user believes a connection was saved when it was not (avoid or
  clearly surface partial config/keychain writes).
- **FR-027**: Editing the config from the UI MUST preserve the rest of the config
  file's existing contexts, clusters, users, and settings (no data loss on
  rewrite).

#### Preservation (cross-cutting)

- **FR-028**: The redesign MUST preserve all existing capabilities — browse,
  context switch (including digit `1`–`9` quick switch), object view/preview,
  download, `du` analytics with drill-down, multi-select bulk
  download/delete/copy, sort by name/size/modified, the runtime write toggle (`w`)
  with deliberate arm / instant disarm and the loud always-on badge, and the
  structured logging of destructive operations.
- **FR-029**: The redesign MUST NOT move any S3/SDK logic into the UI layer
  (Constitution I); new behavior (connection persistence, keychain writes) MUST
  live in UI-agnostic packages (config / secret), with the UI as a thin adapter.
- **FR-030**: Every backend or keychain call introduced (connection test, secret
  store, config write) MUST run off the event loop and report back via messages so
  the TUI never blocks (Constitution II).

#### Visual design: palette, emphasis & restraint (US1/US2 cross-cutting)

These requirements bound *how* color and emphasis are used so the interface stays
calm and legible while still guiding the eye. They reconcile the two goals: **color
is used sparingly and purposefully, not generously** — emphasis is the exception
against a quiet, mostly-neutral baseline.

**Authoritative palette (single source of truth).**

- **FR-031**: The interface MUST use one authoritative palette — the existing 256-
  color token set in `internal/ui/styles.go` (warm Claude Code-style: a single
  coral/orange accent on muted grays) — and all new surfaces MUST reuse those
  tokens, never redefine ad-hoc colors. The palette roles are:

  | Role | Token | 256-color | Use |
  |------|-------|-----------|-----|
  | Accent (the one emphasis hue) | `colAccent` | 173 coral | titles, selected-cue, action keys, metadata keys |
  | Directory | `colDir` | 180 tan | folder rows |
  | Text (default) | `colText` | 252 | normal object text |
  | Dim (baseline) | `colDim` | 244 | labels, separators, secondary data |
  | Border | `colBorder` | 240 | box borders, rules |
  | Selected row | `colSelBg`/`colSelFg` | 238 / 223 | current selection |
  | Success | `colOK` | 108 muted green | `[RO]` badge, success notices |
  | Warn | `colWarn` | 179 | "needs --write" cues |
  | Error | `colErr` | 174 | error line |
  | Write badge (sole loud element) | `writeBadgeStyle` | 231 on 196 | `[RW]` only |
  | Footer param hues (bounded set) | cyan/blue/purple | 109 / 74 / 139 | user / endpoint / region in identity/help |

- **FR-032**: New surfaces (details pane, hint bar, command bar, connection form,
  connection list) MUST map their elements onto the roles above; the per-surface
  color mapping MUST be documented and MUST NOT introduce a new hue outside the
  table.

**Emphasis — defined, measurable, bounded.**

- **FR-033**: "Important element" is defined as exactly this set, and only these get
  emphasis beyond the neutral baseline: the current selection, the `[RW]` write
  badge, the active error/notice line, the active context name, and required/invalid
  connection-form fields. Anything not in this set MUST render in the neutral
  baseline (`colText`/`colDim`).
- **FR-034**: Each important element MUST specify its emphasis mechanism, and
  emphasis MUST combine a **non-color** cue with color (never color alone): the
  selection uses a `▶` gutter + inverse background; the `[RW]` badge uses bold +
  bracket text `[RW]`; errors use a bold `error:` prefix; marked (multi-select)
  rows use a `✓` glyph. (Satisfies color-blind / monochrome legibility.)
- **FR-035**: The `[RW]` badge is the **single** deliberately loud element (bold
  231-on-196). No other element may use a saturated background or exceed the badge's
  visual weight, so the one loud signal stays unique and the rest of the screen
  reads calm (reconciles FR-014/FR-027 with the restraint goal).
- **FR-036**: When multiple important elements are present at once, the visual
  priority order MUST be: write badge > error/notice > current selection > active
  context. At most this set may be emphasized simultaneously; transient emphasis
  (notice, spinner, progress) MUST return to the neutral baseline once the operation
  settles (no lingering color).

**Restraint — calm, not gaudy (measurable).**

- **FR-037**: A screen MUST present **no more than 4 distinct accent/param hues**
  simultaneously beyond the neutral grays and the (conditional) single badge — i.e.
  the coral accent plus at most the three footer param-hues. No surface may add hues
  beyond this budget.
- **FR-038**: The neutral baseline (`colText`/`colDim`/`colBorder` grays) MUST cover
  the **majority** of on-screen cells; color is reserved for the emphasis set
  (FR-033) and the bounded footer params. Accents are the exception, not the rule.
- **FR-039**: Saturation/brightness MUST stay muted (the existing muted-gray +
  soft-coral tokens); no pure/maximally-saturated foregrounds are permitted except
  inside the single `[RW]` badge.
- **FR-040**: The advertised-action surface (hint bar) MUST keep the existing
  single-row, priority-capped budget (`maxHints`, currently 6) with a "more" cue
  when actions overflow, rather than filling the screen with keys — density is
  bounded by reflow, not expansion.

**Accessibility & environment.**

- **FR-041**: Every meaning conveyed by color MUST have the redundant non-color cue
  required by FR-034, so the UI remains legible without color.
- **FR-042**: Behavior on reduced-color terminals MUST be defined: honor `NO_COLOR`
  (render with no color, relying on the FR-034 glyph/weight cues) and degrade
  gracefully on non-truecolor terminals (the palette is already 256-color, not
  truecolor-dependent).

**On-screen data inventory (completeness).**

- **FR-043**: Each screen's required data MUST be enumerated and all of it surfaced.
  The inventory is:
  - **Bucket list**: name, creation date; count in the box title; selection cue.
  - **Tree level**: name, type, size, modified per row; bucket/prefix + count + sort
    indicator in the box title; multi-select count + combined size when marking.
  - **Details pane (object)**: key/name, size, content-type, last-modified, ETag,
    storage class, and a bounded content preview; truncation marker when cut.
  - **Details pane (folder/level)**: child count + a hint to analyze (`a`).
  - **Hint bar**: the valid action keys for the current selection/capability
    (priority-capped with a "more" cue).
  - **Footer identity**: context + `[RW]`/`[RO]` badge always; cluster when it fits;
    user / endpoint / region / version reachable in help.
  - **Command bar**: the matching available commands as the user types.
  - **Connection form**: every field (name, endpoint, region, access key id, secret
    [masked], read-only) + per-field validation messages + the reachability-test
    result.
- **FR-044**: Long values that exceed their pane/line width MUST be truncated with
  an ellipsis (reusing the existing `truncate`), never silently dropped or allowed
  to break the layout; the box title already caps long keys/prefixes.
- **FR-045**: When the pane stacks/collapses on a narrow terminal, the data-priority
  order for what is kept MUST be defined (list rows > footer identity + badge > hint
  bar > details pane); no required element from FR-043 is dropped outright — only the
  pane may collapse to a toggle (FR-013).
- **FR-046**: The details pane MUST define its loading, empty, folder, and error
  states (spinner while the debounced fetch is in flight; summary on a folder; a
  secret-free message on fetch error), consistent with the generation-drop rule
  (FR-012).

### Key Entities *(include if feature involves data)*

- **Connection (Context)**: a named target the user can switch to — display name,
  endpoint, region, read-only flag, access key id. Persisted in config as an
  auto-created cluster + user + context triple (names derived from the display
  name; schema unchanged); the secret is stored out-of-band (keychain). Relates to
  the existing context/cluster/user config model.
- **Credential reference**: a pointer (keychain entry id) recorded in config in
  place of a plaintext secret; resolved at connect time via the 005 credential
  source mechanism.
- **Action / Command**: a user-invokable operation with an identity, a label, a
  single-key binding and/or a command-bar name, a scope (view + selection kind),
  and a write-gated flag. Drives both the hint bar (US1) and the command bar (US3).
- **Details-pane content**: the rendered per-selection view — object metadata +
  bounded preview, or folder/level summary — derived from existing HeadObject /
  ranged GetObject / level state, under the current generation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can perform any primary item action (download, delete, copy,
  move, analyze) in **one keypress** from the highlighted item, down from the two+
  steps the old action menu required.
- **SC-002**: A first-time user can identify and successfully perform the correct
  action for a selected object **without opening the help screen**, because the
  valid actions are visible in the hint bar (verified by task-based usability
  observation: ≥90% success on first attempt).
- **SC-003**: While browsing, the user can read a selected object's key metadata
  and preview **without leaving the list view** (the details pane), eliminating the
  Enter→back round-trip for the at-a-glance case in ≥80% of inspections.
- **SC-004**: A user can add a working new cluster connection and switch to it
  **entirely from within the app in under 2 minutes**, with **no manual editing of
  the config file**.
- **SC-005**: After adding a connection, inspection of the config file shows the
  secret access key is **never present in plaintext** (100% of saves).
- **SC-006**: 100% of existing capabilities from features 001–005 remain reachable
  and functional in the redesigned UI (regression check against the prior feature
  set).
- **SC-007**: Destructive actions invoked by single key still require explicit
  confirmation in 100% of cases (no direct-action path bypasses the two-tier
  confirmation).
- **SC-008**: The interface renders without clipping the hint bar, footer, or write
  badge across terminal sizes from a small (e.g. 80×24) to a large window.
- **SC-009**: On any single screen, at most **4 distinct accent/param hues** appear
  beyond the neutral grays and the single optional `[RW]` badge (FR-037) — objectively
  countable from the palette roles, so "not gaudy" is verifiable.
- **SC-010**: The neutral baseline (grays) covers the **majority** of on-screen cells;
  only the defined emphasis set and bounded footer params carry color (FR-033/FR-038).
- **SC-011**: Exactly **one** element on screen may use a saturated/loud background —
  the `[RW]` badge — and only when write is armed (FR-035); never two.
- **SC-012**: Every color-carried meaning has a redundant non-color cue (glyph/weight/
  prefix), so the UI is fully legible under `NO_COLOR` (FR-034/FR-041/FR-042).
- **SC-013**: Each screen surfaces its full data inventory (FR-043) with zero
  required fields missing; over-long values are ellipsis-truncated, never dropped
  (FR-044).

## Assumptions

- **Layout direction** is the k9s-style single full-width table with a persistent
  details/preview pane (confirmed with the user). The pane sits to the side on wide
  terminals and stacks/toggles on narrow ones.
- **New connections persist to the config file** permanently and appear as normal
  contexts (confirmed). Session-only connections are out of scope.
- **Secrets entered in the add-connection form go to the OS keychain** via the
  existing 005 `internal/secret` backbone; plaintext-in-config is explicitly
  rejected (confirmed; aligns with the constitution's secret-handling rule).
- The existing config schema (clusters / users / contexts) and the 005 credential
  sources are reused; this feature adds an in-app writer and a keychain-backed
  add-flow, not a new config format.
- Direct single-key bindings reuse, where possible, the existing key map and
  conventions (vim-style nav, `/` filter, `?` help, `w` write toggle, `space`
  multi-select, digits for context switch); new bindings are chosen to avoid
  collisions and surfaced in the hint bar.
- The full-screen object view (Enter) is retained as an optional richer view; the
  persistent pane covers the at-a-glance case. Removing it entirely is out of
  scope for this feature.
- Connection editing/removal from the UI beyond *adding* (e.g. inline edit, delete
  of an existing context) MAY be deferred; the confirmed scope is adding a
  connection. The existing `s3s cred` subcommand remains for credential rotation.
- No new telemetry or network calls beyond the configured/added S3 endpoints and
  the local keychain (Constitution security constraints unchanged).
