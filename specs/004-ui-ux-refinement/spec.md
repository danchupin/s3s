# Feature Specification: UI/UX Refinement — Footer Redesign & Key Discoverability

**Feature Branch**: `004-ui-ux-refinement`

**Created**: 2026-06-05

**Status**: Draft

**Input**: User description: "Теперь я хочу улучшить ui и ux. Действуй как ux/ui дизайнер и посмотри на наш интерфейс, чтобы ты захотел исправить, что неудобно? Как минимум мне не нравится как выглядит футер и огромное количество хоткеев - нужно что то тут придумать чтобы повысить удобство. Но посмотри и глобально на весь ui и ux, предложи улучшения"

## Overview & Design Intent

`s3s` is a keyboard-driven TUI for browsing S3-compatible storage. A UX audit of the
current interface surfaced one dominant problem and several smaller ones:

- **The footer is overloaded.** It renders up to 5 stacked lines — a separator, an
  identity line (context · cluster · user), an endpoint line (endpoint · region ·
  version), a keybinding-hints line listing up to ~13 shortcuts at once, and a status
  line. On terminals narrower than 80 columns the hint line wraps to 2–3 additional
  rows, so the footer can consume 6+ rows and crowd the content above it. Connection
  metadata and action hints compete for the same scarce vertical space.
- **Too many shortcuts are shown at once.** With write mode active in the tree view,
  the hints line advertises `enter`, `/`, `r`, `+`, `d`, `u`, `y`, `m`, `D`, `c`,
  `1-9`, `?`, `q` — a wall of keys that is hard to scan and includes actions that may
  not even apply to the current selection.
- **Discoverability and feedback gaps.** The help overlay is an undifferentiated list;
  the loading spinner never says *what* is loading; debounced tree search gives no
  "pending" feedback; typed-confirmation prompts give no progress signal as the user
  types the confirmation string.

The guiding principle of this feature is **progressive disclosure**: show the few
actions that matter *right now*, keep connection metadata available but out of the way,
and make the full keymap reachable on demand through a redesigned help surface. No
backend, storage, or write-semantics behavior changes — this feature is presentation,
layout, and interaction only.

## Clarifications

### Session 2026-06-05

- Q: Where should connection metadata (cluster, user, endpoint, region, version) live after the redesign? → A: One compact identity line in the footer (context + RW/RO, optionally cluster); the full details (endpoint, region, user, version) move into the help surface.
- Q: What is the footer's maximum total height budget (identity + hints + status rows combined)? → A: 3 rows max — 1 identity, 1 hints, 1 status (status row present only when there is something to show).
- Q: When terminal width forces dropping low-priority hints, how should the user know hints were hidden? → A: Show a "? more" affordance at the end of the hint line so the user knows the full keymap lives in help.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A calm, context-aware footer (Priority: P1)

As a user browsing buckets and objects, I want the footer to show only the handful of
actions relevant to where I am and what I have selected, so the bottom of the screen
stays quiet and readable instead of presenting a wall of shortcuts.

**Why this priority**: This is the user's primary complaint ("мне не нравится как
выглядит футер и огромное количество хоткеев") and the highest-leverage change. A
decluttered, mode-aware footer immediately reduces visual noise and reclaims vertical
space for content on every screen and at every terminal width.

**Independent Test**: Launch the app at several terminal widths and in each mode
(bucket list, tree, object view, with/without an active search, read-only vs write
context). Confirm the footer presents a curated, prioritized set of hints that fits its
height budget, adapts to the current mode and selection, and never wraps into an
ever-growing stack of rows.

**Acceptance Scenarios**:

1. **Given** the bucket list in a read-only context, **When** the footer renders,
   **Then** only navigation/browse-relevant hints are shown (open, search, refresh,
   context switch, help, quit) and no write-action hints appear.
2. **Given** the tree view in a writable context with an object selected, **When** the
   footer renders, **Then** write actions applicable to that selection are surfaced and
   actions that do not apply to the selection are not shown.
3. **Given** any mode at a terminal width below 80 columns, **When** the footer renders,
   **Then** the hint area stays within its height budget by dropping the
   lowest-priority hints first (graceful degradation) rather than wrapping every hint
   onto new rows, and `help` remains visible as the escape hatch to the full keymap.
4. **Given** an active search/filter in the tree, **When** the footer renders, **Then**
   a "clear search" affordance is shown and the back-key ambiguity (clear search vs.
   ascend level) is signalled to the user.
5. **Given** a single-context configuration, **When** the footer renders, **Then** the
   numeric quick-switch hint (`1-9`) is omitted because it has no effect.

---

### User Story 2 - Discover every shortcut on demand (Priority: P2)

As a user who can no longer see every shortcut in the footer, I want a single,
well-organized place that lists the complete keymap — grouped by purpose and showing key
aliases — so I never lose access to a command just because the footer hides it.

**Why this priority**: Removing hints from the footer (P1) only improves UX if the
hidden commands remain easy to find. A redesigned help surface is the safety net that
makes aggressive footer decluttering safe. It depends conceptually on P1 but is
independently testable.

**Independent Test**: Open the help surface from each mode and verify it lists all
available actions, grouped into clear categories, including key aliases (arrows + vim
keys), and clearly tells the user how to dismiss it.

**Acceptance Scenarios**:

1. **Given** any mode, **When** the user opens help, **Then** a categorized reference of
   all keybindings is shown (e.g., Navigation, Search & View, Context, Write, Global),
   each action listing all keys that trigger it.
2. **Given** the help surface is open, **When** it renders, **Then** it states how to
   close it.
3. **Given** a write-enabled context, **When** the user opens help, **Then** write
   actions are listed and labelled as available; **Given** a read-only context, **Then**
   write actions are either hidden or clearly marked unavailable.
4. **Given** the help surface, **When** it lists an action that has both an arrow key and
   a vim-style key (e.g., move down = `↓` / `j`), **Then** both aliases are shown.

---

### User Story 3 - Clearer status, loading, and confirmation feedback (Priority: P3)

As a user performing loads, searches, and confirmations, I want status feedback that
names what is happening and reflects my progress, so I always understand the current
state and never cancel or confirm the wrong thing.

**Why this priority**: These are refinements that polish trust and clarity but are not
the user's primary pain. They are valuable and independently shippable after the footer
and help work land.

**Independent Test**: Trigger each feedback state (loading a level, loading an object,
pending debounced search, typed confirmation in progress) and verify the status line
communicates *what* is happening and reflects progress where applicable.

**Acceptance Scenarios**:

1. **Given** a backend fetch is in flight, **When** the loading indicator renders,
   **Then** it names what is loading (e.g., bucket contents vs. object) rather than a
   generic "loading…".
2. **Given** a debounced tree search where the user has typed but the search has not yet
   fired, **When** the status renders, **Then** a "search pending/searching" indicator is
   shown so the delay is understood as intentional.
3. **Given** a typed-confirmation prompt for a destructive action, **When** the user
   types part of the required confirmation string, **Then** the prompt continues to show
   the exact required target alongside the input so the user can verify their progress,
   and a mismatch on submit cancels safely without performing the action.
4. **Given** a transient success notice (e.g., a completed recursive delete), **When** it
   is shown, **Then** it is visually distinguishable from an error message and clears on
   the next interaction.

---

### Edge Cases

- **Very narrow terminals (< 50 columns)**: footer must still show at least the most
  critical affordance (a path to help and to quit) and must not push content out of view
  or overflow the width.
- **Very wide terminals (> 160 columns)**: the footer must not look sparse or
  awkwardly stretched; hints and identity remain readable and reasonably grouped.
- **Mode transitions**: when the user moves from object view back to the tree, the
  footer must immediately reflect the actions now available (e.g., write actions
  reappear) without requiring another keypress to refresh.
- **Selection-dependent actions**: when nothing is selected or the selection is a folder
  vs. an object, the footer must only advertise actions valid for that selection.
- **Read-only context**: no write-action hint may appear anywhere in the footer.
- **Help open during an in-flight load**: opening and closing help must not disturb or
  cancel the load, and the underlying status must be intact when help closes.

## Requirements *(mandatory)*

### Functional Requirements

#### Footer & hints (P1)

- **FR-001**: The footer MUST present action hints as a curated, prioritized set scoped
  to the current mode and the current selection, rather than listing all possible
  shortcuts at once. The number of hints shown MUST be capped at a fixed maximum of 6 —
  when more than 6 hints apply to the current state, only the 6 highest-priority ones are
  shown (the priority cap applies before any width-driven degrade in FR-004).
- **FR-002**: The footer MUST NOT display write-action hints when the active context is
  read-only.
- **FR-003**: The footer MUST omit hints for actions that do not apply to the current
  selection or configuration (e.g., numeric context quick-switch when only one context
  exists; write actions when no eligible item is selected).
- **FR-004**: When available width is insufficient to show all hints, the footer MUST
  degrade by dropping the lowest-priority hints first while keeping the highest-priority
  affordances visible, instead of wrapping every hint onto additional rows. When one or
  more hints are dropped for this reason, the hint line MUST end with a "? more"
  affordance signalling that the full keymap is available in the help surface.
- **FR-005**: The footer MUST always keep a visible path to the full keymap (a help
  affordance) regardless of width or mode.
- **FR-006**: The footer's total rendered height MUST NOT exceed 3 rows — at most one
  identity row, one hint row, and one status row — at any supported terminal width. The
  status row is present only when there is a status to show (loading, search, confirm,
  notice, or error); the identity and hint rows MUST each stay a single row (they degrade
  per FR-004/FR-007 rather than wrapping onto additional rows).
- **FR-007**: The footer MUST render connection/identity metadata as a SINGLE compact
  identity row showing, at minimum, the context name and the read-write/read-only status
  (cluster name MAY also appear if width permits). The remaining metadata — endpoint,
  region, user, and version — MUST NOT appear in the footer and MUST instead be presented
  in the help surface, so it does not crowd the action hints.
- **FR-008**: The read-write vs. read-only status of the active context MUST remain
  visible at a glance in the primary view (not only inside help).
- **FR-009**: When a search/filter is active, the footer MUST disambiguate the back
  action by replacing the `esc back` hint with an `esc clear` hint (so the rendered cue
  reflects that the next back-key press clears the search rather than ascending a level);
  the `esc back` hint MUST NOT be shown while a search/filter is active.

#### Help & discoverability (P2)

- **FR-010**: The application MUST provide a single help surface, reachable from every
  mode, that lists every available action and all key aliases that trigger it.
- **FR-011**: The help surface MUST group actions into labelled categories (at minimum:
  Navigation, Search & View, Context, Write, Global).
- **FR-012**: The help surface MUST indicate how to dismiss it.
- **FR-013**: The help surface MUST reflect context capability: write actions are shown
  as available in a writable context and hidden or marked unavailable in a read-only
  context.
- **FR-014**: For any action with multiple bound keys (arrow + vim alias, primary +
  secondary), the help surface MUST display all of those keys.
- **FR-014a**: The help surface MUST include a connection section presenting the full
  metadata that was removed from the footer per FR-007 — endpoint, region, user, and
  version (plus context and cluster for completeness) — with secret-bearing values
  redacted per FR-021.

#### Status, loading & confirmation feedback (P3)

- **FR-015**: The loading indicator MUST name what is being loaded (e.g., distinguishing
  a level/listing load from an object metadata/content load).
- **FR-016**: When a debounced search has been entered but not yet executed, the status
  MUST indicate that a search is pending/in progress.
- **FR-017**: A typed-confirmation prompt MUST continuously display the exact required
  confirmation target alongside the user's input, and a mismatch on submit MUST cancel
  the operation safely without performing it (preserving the existing two-tier
  confirmation safety model).
- **FR-018**: Transient success notices MUST be visually distinguishable from error
  messages and MUST clear on the next interaction.

#### Cross-cutting constraints

- **FR-019**: All footer, help, and status output MUST never exceed the terminal width
  (no horizontal overflow) at any supported width.
- **FR-020**: This feature MUST NOT change backend behavior, storage operations, write
  semantics, the two-tier confirmation safety model, or which actions exist — it changes
  only how they are presented, laid out, and discovered.
- **FR-021**: Secret-bearing values MUST continue to be redacted everywhere they could
  appear in the footer, help, or status output (no regression of existing redaction).
- **FR-022**: The full set of existing keybindings MUST continue to function; no shortcut
  is removed by this feature, only relocated in terms of where it is *advertised*.

### Key Entities

- **Footer**: The composite bottom region of the screen. Comprises an identity area
  (connection/context metadata), a contextual hint area (curated action shortcuts), and a
  status area (loading, search, confirmation, notice, error). Adapts to mode, selection,
  context capability, and terminal width.
- **Hint**: A single advertised action consisting of a key (or key group) and a short
  label, carrying a priority used for graceful degradation under width pressure.
- **Help surface**: The on-demand, full keymap reference, organized by category, showing
  all key aliases and reflecting context capability.
- **Status message**: A transient feedback item in the status area — loading (named),
  search-pending, confirmation prompt, success notice, or error — with a visual category
  that distinguishes success from error from neutral.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: At an 80-column terminal in write mode within the tree view, the footer's
  hint area occupies exactly one row (down from the current up-to-3-row wrap of ~13
  hints), and content above it gains the reclaimed rows.
- **SC-002**: In every mode and at every supported terminal width (from 40 to 200
  columns), the footer renders in at most 3 rows (identity + hints + optional status) and
  produces zero horizontal overflow.
- **SC-003**: The number of action hints shown simultaneously in the footer never exceeds
  6, at any terminal width and in any mode/selection state; when more than 6 apply, the 6
  highest-priority hints are shown (enforced by a hard cap, per FR-001).
- **SC-004**: 100% of existing keybindings remain invokable and are discoverable through
  the help surface in at most one step (open help) from any mode.
- **SC-005**: In a read-only context, zero write-action hints appear anywhere in the
  footer, in any mode or selection state.
- **SC-006**: Every loading state names what is loading; a user reading the status line
  can identify the in-flight operation without prior context.
- **SC-007**: A first-time user can identify how to open help and how to quit from the
  default view without opening help first (both affordances remain visible).

## Assumptions

- **Scope is presentation-only.** No storage methods, write semantics, confirmation
  tiers, or backend calls change. The constitution's read-only/safe-operation invariants
  and the two-tier confirmation model are preserved exactly as-is.
- **Existing keymap is retained.** Every current shortcut keeps working; this feature
  changes where shortcuts are *advertised* (curated footer + full help), not which keys
  do what. Adding or remapping keys is out of scope unless required to resolve an
  ambiguity surfaced during design.
- **Progressive disclosure is the chosen direction.** Connection metadata is split: a
  single compact identity row in the footer (context + RW/RO, optionally cluster) and the
  remaining detail (endpoint, region, user, version) moved into the help surface rather
  than removed; action hints become contextual and prioritized; the full keymap lives in a
  redesigned help surface. The footer is capped at 3 rows (identity + hints + optional
  status). See Clarifications (Session 2026-06-05).
- **Read-write status stays glanceable.** The compact identity line retains an at-a-glance
  RW/RO indicator even after metadata is consolidated.
- **Supported width range.** The interface targets terminals from roughly 40 to 200
  columns; below 40 columns only the most critical affordances are guaranteed.
- **No new dependency or terminal capability is introduced.** The redesign works within
  the existing cell renderer and color palette; no new image protocol, mouse support, or
  external library is assumed.
- **Command-palette / fuzzy-action-search is out of scope** for this iteration; the
  redesigned help surface is the discovery mechanism. It may be revisited later.
- **Existing test conventions apply.** UI behavior is verified white-box by driving the
  model and asserting on rendered view content, consistent with the project's TDD
  approach.
