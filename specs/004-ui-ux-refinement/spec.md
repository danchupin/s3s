# Feature Specification: UI/UX Refinement — Action Menu, Footer Redesign & Key Discoverability

**Feature Branch**: `004-ui-ux-refinement`

**Created**: 2026-06-05

**Status**: Draft

**Input**: User description: "Теперь я хочу улучшить ui и ux. Действуй как ux/ui дизайнер и посмотри на наш интерфейс, чтобы ты захотел исправить, что неудобно? Как минимум мне не нравится как выглядит футер и огромное количество хоткеев - нужно что то тут придумать чтобы повысить удобство. Но посмотри и глобально на весь ui и ux, предложи улучшения" — extended to actually reduce the keymap (not only relocate it).

## Overview & Design Intent

`s3s` is a keyboard-driven TUI for browsing S3-compatible storage. A UX audit surfaced
one dominant problem and several smaller ones:

- **Too many top-level shortcuts.** The current keymap exposes ~18 logical actions across
  ~28 bindings; the worst offender is six separate top-level write keys
  (`+`, `d`, `u`, `y`, `m`, `D`) plus `r` refresh and `x` cancel. In write mode the footer
  advertised `enter`, `/`, `r`, `+`, `d`, `u`, `y`, `m`, `D`, `c`, `1-9`, `?`, `q` at once
  — a wall of keys that is hard to scan and partly inapplicable to the current selection.
- **The footer is overloaded.** It renders up to 5 stacked rows — separator, identity
  line (context · cluster · user), endpoint line (endpoint · region · version), the
  keybinding-hints line, and a status line. Under 80 columns the hints wrap to extra rows,
  so the footer can consume 6+ rows and crowd the content above it.
- **Discoverability and feedback gaps.** The help overlay is an undifferentiated list; the
  loading spinner never says *what* is loading; debounced tree search gives no "pending"
  feedback; typed-confirmation prompts give no progress signal.

This feature attacks the root cause of the key clutter by **reducing** the keymap, not
just hiding it: the six write operations and refresh move into a single **contextual
action menu** opened by one key (`a`); cancel folds into `Esc`. The footer becomes a
calm, capped, contextual hint row; connection metadata and the full keymap (including the
menu) move into a redesigned help surface. **No operation semantics change** — the action
menu is only a new *entry point* into the existing flows (name/destination entry plus the
two-tier confirmation model are preserved exactly).

## Clarifications

### Session 2026-06-05

- Q: Where should connection metadata (cluster, user, endpoint, region, version) live after the redesign? → A: One compact identity line in the footer (context + RW/RO, optionally cluster); the full details (endpoint, region, user, version) move into the help surface.
- Q: What is the footer's maximum total height budget (identity + hints + status rows combined)? → A: 3 rows max — 1 identity, 1 hints, 1 status (status row present only when there is something to show).
- Q: When terminal width forces dropping low-priority hints, how should the user know hints were hidden? → A: Show a "? more" affordance at the end of the hint line so the user knows the full keymap lives in help.

### Session 2026-06-05 (scope extension — keymap reduction)

- Q: How should the keymap be reduced (not only relocated)? → A: Introduce a contextual action menu opened by a single leader key (`a`); selected with ↑/↓ + Enter.
- Q: Which top-level keys are removed? → A: The six write keys (`+`, `d`, `u`, `y`, `m`, `D`) and `r` refresh move into the action menu; `x` cancel folds into `Esc` (contextual: cancels an in-flight load when one is running). Navigation aliases (vim + arrows) are kept.
- Q: Arrow keys vs vim keys — which is primary? → A: Arrow keys (and Enter/Esc) are the PRIMARY, advertised navigation shown in the footer/menu. Vim-style aliases (`h/j/k/l`, `g/G`) remain fully functional but are documented ONLY in the help surface, not advertised in the footer.
- Q: After removing top-level `r`, where does the action menu open and how is the bucket list refreshed? → A: The menu opens in BOTH list modes (bucket list and tree); in the bucket list it offers Refresh only (no bucket-level write ops exist yet), which is the mechanism to refresh buckets now that `r` is removed.
- Q: Which key opens the action menu? → A: `a` (mnemonic for "actions"); footer hint is `a actions`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Contextual action menu replaces the wall of write keys (Priority: P1) 🎯 MVP

As a user, I want a single key to open a short menu of the actions that apply to what I've
selected — instead of memorizing six separate write shortcuts — so the interface has far
fewer top-level keys and the available actions are self-documenting.

**Why this priority**: This is the direct answer to the user's primary complaint ("огромное
количество хоткеев"). Collapsing 7 top-level keys (`+ d u y m D r`) into one `a` menu is the
single biggest reduction in key clutter and makes write actions discoverable without the
footer or help.

**Independent Test**: In a writable context, select an object vs a folder vs an empty level
and press the menu key; confirm the menu lists exactly the applicable actions, that choosing
one enters the existing operation flow unchanged (entry + confirmation), that the removed
top-level keys no longer trigger anything, and that the menu also works (showing Refresh) in
a read-only context.

**Acceptance Scenarios**:

1. **Given** a writable tree with an **object** selected, **When** the user opens the action
   menu, **Then** it lists Delete, Copy, Move/Rename, Upload here, New folder, and Refresh —
   and NOT recursive delete (folder-only).
2. **Given** a writable tree with a **folder** selected, **When** the menu opens, **Then** it
   lists Recursive delete, Upload here, New folder, and Refresh — and NOT object-only Delete/
   Copy/Move.
3. **Given** a **read-only** context, **When** the menu opens, **Then** it lists Refresh only
   (no write actions) — and the menu is still usable.
4. **Given** the menu open, **When** the user chooses an action, **Then** the existing
   operation flow runs unchanged (name/destination entry where applicable; simple vs typed
   confirmation tier exactly as today), and **When** the user presses the dismiss key, the
   menu closes with no side effect.
5. **Given** the redesign is in effect, **When** the user presses any removed top-level write
   key (`+`, `d`, `u`, `y`, `m`, `D`) or `r`, **Then** nothing happens (those keys are
   unbound at top level); the actions are reachable only via the menu.
6. **Given** an in-flight load, **When** the user presses `Esc`, **Then** the load is
   cancelled (cancel folded into `Esc`); **When** no load is running, `Esc` performs back.

---

### User Story 2 - A calm, context-aware footer (Priority: P1)

As a user browsing buckets and objects, I want the footer to show only the handful of
actions relevant to where I am, so the bottom of the screen stays quiet and readable.

**Why this priority**: The footer is the other half of the user's complaint. With write keys
now behind the menu, the footer advertises a single `a actions` hint instead of six — making
the calm, capped, contextual footer both simpler and more achievable.

**Independent Test**: At several widths and modes (read-only vs writable, object/folder
selected, search active, single vs multi context), assert the footer is ≤ 3 rows, the hint
row is one line capped at ≤ 6 hints, it shows `a actions` (not individual write keys), drops
lowest-priority hints first with a `? more` cue, and never overflows.

**Acceptance Scenarios**:

1. **Given** the bucket list in a read-only context, **When** the footer renders, **Then**
   only navigation/browse hints appear (open, search, actions, context, help, quit) and no
   individual write-action hints.
2. **Given** a writable tree, **When** the footer renders, **Then** it shows a single
   `a actions` hint rather than `d/u/y/m/D/+`, and the hint count stays ≤ 6.
3. **Given** any mode below 80 columns, **When** the footer renders, **Then** the hint row
   stays one line by dropping lowest-priority hints first and appending `? more`; `? help`
   and `q quit` always survive.
4. **Given** an active search/filter in the tree, **When** the footer renders, **Then** it
   shows `esc clear` (not `esc back`) to disambiguate the back action.
5. **Given** a single-context configuration, **When** the footer renders, **Then** the
   numeric quick-switch hint (`1-9`) is omitted.

---

### User Story 3 - Discover every shortcut on demand (Priority: P2)

As a user, I want one organized place that lists the complete keymap — including the action
menu and its contents — plus the connection details, so nothing is lost when the footer
hides it.

**Why this priority**: Removing keys and footer hints (US1/US2) is only safe if the full
keymap and the menu's actions remain easy to find. The redesigned help is that safety net.

**Independent Test**: Open help from each mode; verify categorized sections, all key aliases,
an Actions section describing the menu and its items, a Connection section with the
footer-evicted metadata, write-capability reflection, and a stated dismissal.

**Acceptance Scenarios**:

1. **Given** any mode, **When** the user opens help, **Then** a categorized reference is shown
   (Navigation, Search & View, Actions, Context, Global, Connection), each action listing all
   its keys.
2. **Given** the help surface, **When** it renders the Actions section, **Then** it documents
   the menu key and lists the menu's actions (delete, copy, move, upload, new folder,
   recursive delete, refresh) and which are write-only.
3. **Given** the help surface is open, **When** it renders, **Then** it states how to close it.
4. **Given** an action with multiple keys (e.g. `↓` / `j`), **When** help lists it, **Then**
   all aliases are shown.

---

### User Story 4 - Clearer status, loading & confirmation feedback (Priority: P3)

As a user performing loads, searches, and confirmations, I want status feedback that names
what is happening and reflects progress, so I always understand the current state.

**Why this priority**: Polish that builds trust; valuable but not the primary pain.

**Independent Test**: Trigger each state (level vs object load, pending debounced search,
typed confirmation in progress, success notice) and verify the status communicates what is
happening and distinguishes success from error.

**Acceptance Scenarios**:

1. **Given** a fetch in flight, **When** the loading indicator renders, **Then** it names what
   is loading (bucket contents vs object) and shows that `Esc` cancels.
2. **Given** a debounced tree search typed but not yet fired, **When** status renders, **Then**
   a "searching/pending" indicator is shown.
3. **Given** a typed-confirmation prompt, **When** the user types part of the string, **Then**
   the exact required target stays visible alongside the input and a mismatch on submit
   cancels safely.
4. **Given** a transient success notice, **When** it is shown, **Then** it is visually distinct
   from an error and clears on the next interaction.

---

### Edge Cases

- **Empty level / nothing selected**: the action menu still opens, listing only level-scoped
  actions (New folder, Upload here, Refresh) plus Refresh; object/folder-specific actions are
  absent.
- **Read-only context**: the menu shows Refresh only; no write action is listed or invokable
  anywhere (footer, menu, or help).
- **Menu open during an in-flight load**: opening/closing the menu must not disturb or cancel
  the load.
- **Very narrow terminals (< 50 cols)**: footer keeps at least `a actions`/`? help`/`q quit`
  reachable and never overflows; the menu renders within the width.
- **Very wide terminals (> 160 cols)**: footer is not sparse; menu is not awkwardly stretched.
- **Mode transitions**: returning from object view to the tree immediately restores the
  correct menu contents and the `a actions` footer hint.

## Requirements *(mandatory)*

### Functional Requirements

#### Action menu & keymap reduction (US1)

- **FR-023**: The application MUST provide a contextual action menu opened by the `a` key
  (the "actions" key) from BOTH list modes — the bucket list and the tree. In the bucket
  list the menu offers Refresh only (no bucket-level write operations exist in this
  iteration); in the tree it offers the full contextual set per FR-024/FR-025.
- **FR-024**: The menu MUST list only the actions valid for the current selection and context
  — object-only actions (delete, copy, move/rename) only when an object is selected;
  recursive delete only when a folder is selected; level actions (new folder, upload here) in
  the tree; and they MUST be absent when the context is read-only.
- **FR-025**: The menu MUST always include Refresh, in every list mode (bucket list and
  tree) and in read-only contexts too. Refresh in the bucket list reloads the bucket list;
  in the tree it reloads the current level. This is the sole refresh entry point now that
  the top-level `r` key is removed (FR-028).
- **FR-026**: Choosing a menu item MUST enter the EXISTING operation flow unchanged — the same
  name/destination entry and the same two-tier (simple vs typed) confirmation as today. The
  menu changes only the entry point, never the operation or its safety.
- **FR-027**: The menu MUST be keyboard-navigable using the existing navigation keys (↑/↓ and
  vim aliases to move, Enter to invoke, a back/escape key to dismiss) and MUST state how to
  dismiss it.
- **FR-028**: The per-operation top-level write keys (`+`, `d`, `u`, `y`, `m`, `D`) and the
  top-level refresh key (`r`) MUST be removed from the top-level keymap; these operations are
  reachable ONLY through the action menu. Pressing a removed key at top level MUST do nothing.
- **FR-029**: Cancellation of an in-flight load MUST be performed via the back/escape key
  (contextual: when a load is running, the key cancels it; otherwise it performs back); the
  standalone cancel key (`x`) MUST be removed. **Modal precedence**: when the action menu (or
  any modal overlay) is open, `Esc` MUST first close that overlay and MUST NOT cancel a
  background load; the load/run-cancel meaning of `Esc` applies only when no modal overlay is
  open.
- **FR-030**: The reduced top-level keymap MUST contain no more than 12 always-live interactive
  actions (down from ~18), and every removed action MUST remain reachable (write ops + refresh
  via the menu; cancel via the back/escape key). **Counting rule**: an "interactive action" is a
  distinct logical action routed at the top level; each action counts once regardless of how
  many key aliases it has (e.g. `↑`/`k` = one), and the `1-9` numeric quick-switch counts as one.
  Actions reachable only inside the menu or inside another mode are NOT top-level actions.
- **FR-031**: Arrow keys (`↑`/`↓`/`←`/`→`) plus `Enter` and `Esc` MUST be the PRIMARY navigation
  advertised in the footer and the action menu. Vim-style aliases (`h`/`j`/`k`/`l`, `g`/`G`)
  MUST remain fully functional but MUST NOT be advertised in the footer or menu — they are
  documented only in the help surface (per FR-014/FR-014c). Footer/menu navigation cues MUST use
  the arrow glyphs, not the vim letters. **Top/Bottom** (jump to first/last) MUST NOT be
  advertised in the footer or menu; it remains reachable via `Home`/`End` (and `g`/`G`) and is
  documented only in the help surface.

#### Footer & hints (US2)

- **FR-001**: The footer MUST present action hints as a curated, prioritized set scoped to the
  current mode and selection, rather than listing all possible shortcuts. The footer MUST
  advertise the action menu with a single `a actions` hint INSTEAD of individual write-op
  hints. The number of hints shown MUST be capped at a fixed maximum of 6 — when more than 6
  apply, only the 6 highest-priority are shown (the priority cap applies before the width
  degrade in FR-004).
- **FR-002**: The footer MUST NOT display the `a actions` hint's write capability as available
  when the active context is read-only; in read-only the menu still appears (Refresh only), so
  `a actions` MAY still be shown but MUST NOT imply write capability.
- **FR-003**: The footer MUST omit hints for actions that do not apply to the current
  configuration (e.g. numeric context quick-switch when only one context exists).
- **FR-004**: When width is insufficient, the footer MUST drop the lowest-priority hints first
  while keeping the highest-priority affordances, instead of wrapping; when ≥1 hint is dropped,
  the hint row MUST end with a `? more` affordance.
- **FR-005**: The footer MUST always keep a visible path to the full keymap (a help affordance)
  regardless of width or mode.
- **FR-006**: The footer's total rendered height MUST NOT exceed 3 rows — at most one identity
  row, one hint row, and one status row (status present only when there is a status to show);
  the identity and hint rows MUST each stay a single row.
- **FR-007**: The footer MUST render connection/identity metadata as a SINGLE compact identity
  row (context name + read-write/read-only status, plus cluster if width permits). Endpoint,
  region, user, and version MUST NOT appear in the footer and MUST instead appear in the help
  surface.
- **FR-008**: The read-write vs. read-only status of the active context MUST remain visible at
  a glance in the primary view (not only inside help).
- **FR-009**: When a search/filter is active, the footer MUST disambiguate the back action by
  replacing the `esc back` hint with an `esc clear` hint; the `esc back` hint MUST NOT be shown
  while a search/filter is active.

#### Help & discoverability (US3)

- **FR-010**: The application MUST provide a single help surface, reachable from every mode,
  listing every available action and all key aliases that trigger it.
- **FR-011**: The help surface MUST group actions into labelled categories (at minimum:
  Navigation, Search & View, Actions, Context, Global) plus a Connection section.
- **FR-012**: The help surface MUST indicate how to dismiss it.
- **FR-013**: The help surface MUST reflect context capability: write actions (in the Actions
  section) are shown as available in a writable context and hidden or marked unavailable in a
  read-only context.
- **FR-014**: For any action with multiple bound keys, the help surface MUST display all keys.
- **FR-014a**: The help surface MUST include a Connection section presenting the metadata
  removed from the footer per FR-007 — endpoint, region, user, and version (plus context and
  cluster) — with secret-bearing values redacted per FR-021.
- **FR-014b**: The help surface MUST document the action menu: the key that opens it and the
  list of actions it contains (delete, copy, move/rename, upload, new folder, recursive
  delete, refresh), marking which require write mode.
- **FR-014c**: The help surface MUST list the vim-style navigation aliases (`h`/`j`/`k`/`l`,
  `g`/`G`) alongside the primary arrow keys for each navigation action, so the only place the
  vim bindings are advertised is help (per FR-031).

#### Status, loading & confirmation feedback (US4)

- **FR-015**: The loading indicator MUST name what is being loaded (level/listing vs object)
  and MUST show that the back/escape key cancels (per FR-029).
- **FR-016**: When a debounced search has been entered but not executed, the status MUST
  indicate that a search is pending/in progress.
- **FR-017**: A typed-confirmation prompt MUST continuously display the exact required target
  alongside the user's input, and a mismatch on submit MUST cancel the operation safely
  (preserving the existing two-tier confirmation model).
- **FR-018**: Transient success notices MUST be visually distinguishable from error messages
  and MUST clear on the next interaction.
- **FR-018a**: When multiple status conditions hold simultaneously, the single status row MUST
  show exactly one, in this priority order (highest first): operation prompt (name/dest entry or
  confirmation) > running-op progress > loading > search-pending > success notice > error.

#### Cross-cutting constraints

- **FR-019**: All footer, menu, help, and status output MUST never exceed the terminal width
  at any supported width (40–200 columns).
- **FR-020**: This feature MUST NOT change backend behavior, storage operations, write
  semantics, or the two-tier confirmation safety model — it changes how actions are entered
  (menu vs key), laid out, and discovered, not what they do.
- **FR-021**: Secret-bearing values MUST continue to be redacted everywhere they could appear
  (footer, menu, help, status); the help Connection section MUST source only non-secret
  display fields and reference no credential path.
- **FR-022**: Apart from the intentional reductions in FR-028/FR-029, every existing operation
  MUST remain reachable; this feature removes top-level *keys* for write ops/refresh/cancel by
  relocating them (to the menu / to `Esc`), and MUST NOT remove any *capability*.

### Key Entities

- **Action menu**: A contextual, keyboard-navigable overlay opened by the actions key, listing
  the operations valid for the current selection and context. Its items are entry points into
  the existing operation flows; it adds no new operation or confirmation behavior.
- **Menu item**: A single action in the menu — a label, the operation it triggers, and a
  visibility predicate (selection kind, write capability, level scope).
- **Footer**: The ≤ 3-row bottom region — compact identity row, capped contextual hint row
  (advertising `a actions`), and an optional status row.
- **Hint**: An advertised footer action (key + short label + priority for degrade).
- **Help surface**: The on-demand categorized keymap reference, including an Actions section
  (the menu) and a Connection section (footer-evicted metadata).
- **Status message**: A transient status item (named loading, search-pending, confirmation
  prompt, success notice, or error) with a visual category distinguishing success from error.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a writable tree at 80 columns, the footer's hint row occupies exactly one row
  and advertises `a actions` (not six individual write keys); the footer is ≤ 3 rows total.
- **SC-002**: In every mode and at every supported width (40–200 columns), the footer is ≤ 3
  rows and produces zero horizontal overflow; the action menu also fits within the width.
- **SC-003**: The number of action hints shown simultaneously in the footer never exceeds 6, at
  any width and in any mode/selection state.
- **SC-004**: 100% of existing operations remain reachable; all write operations and refresh
  are reachable within 2 keypresses (open menu, then select), and the full keymap is
  discoverable in the help surface in one step from any mode.
- **SC-005**: In a read-only context, zero write actions appear or are invokable in the footer,
  the action menu, or help.
- **SC-006**: Every loading state names what is loading.
- **SC-007**: A first-time user can identify how to open the actions menu, open help, and quit
  from the default view without opening help first (all three affordances are visible).
- **SC-008**: The number of always-live top-level interactive key actions is reduced from ~18
  to ≤ 12; the six write keys, refresh, and the standalone cancel key are no longer top-level
  bindings.
- **SC-009**: The footer and action menu advertise navigation using arrow glyphs only; vim
  aliases appear nowhere except the help surface, yet remain fully functional when pressed.

## Assumptions

- **Operation semantics are unchanged.** The action menu and `Esc`-cancel are new *entry
  points*; storage methods, write semantics, the two-tier confirmation model, and pre-execution
  logging are preserved exactly (Constitution V honored).
- **Keymap is reduced, not merely relocated.** Top-level write keys (`+ d u y m D`), refresh
  (`r`), and cancel (`x`) are removed from the top level; their capabilities live in the menu
  (writes + refresh) and on `Esc` (cancel).
- **Arrows are primary; vim is secondary.** Arrow keys + Enter/Esc are the advertised
  navigation (footer/menu show arrow glyphs). Vim aliases (`h/j/k/l`, `g/G`) stay bound and
  functional but are advertised ONLY in the help surface (FR-031). No navigation capability is
  removed — only its advertising.
- **The actions key is `a`** (mnemonic "actions"); it is currently unbound at top level so it
  does not collide with an existing action. Footer/help advertise it as `a actions`.
- **Progressive disclosure for the footer.** Connection metadata is consolidated into one
  compact identity row plus a help Connection section; hints are contextual, capped at 6, and
  degrade with a `? more` cue. Footer ≤ 3 rows. See Clarifications.
- **Supported width band.** The ≤ 3-row footer and zero-overflow guarantees apply within
  40–200 columns. Below 40 columns is unsupported/best-effort (no layout guarantee); above 200
  the layout simply does not look sparse.
- **Accessibility / terminal-capability scope.** This iteration assumes the existing cell
  renderer and 256-color palette; colour-blind/low-colour fallbacks, non-colour cue duplication,
  and CJK/wide-glyph width edge cases are explicitly OUT OF SCOPE for 004 (candidate follow-up).
- **No new dependency or terminal capability** is introduced (no mouse, no new image protocol).
- **Command-palette / fuzzy action search is out of scope**; the action menu is contextual
  (per-selection), not a global fuzzy finder. Help is the discovery surface for the full keymap.
- **Existing test conventions apply.** UI behavior is verified white-box by driving the model
  and asserting on rendered view content (TDD per Constitution III).
