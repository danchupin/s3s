# Feature Specification: UI Legibility, Hotkey Parity, Breadcrumbs & Write-Mode Clarity

**Feature Branch**: `012-ui-visibility-write-clarity`

**Created**: 2026-06-07

**Status**: Draft

**Input**: User description: "имена бакетов в левой части панели не влезают — это нужно исправить, при этом в средней части еще много свободного пространства. Раскрывать враппинг активной строки (если не ломает интерфейс). Не хватает хлебных крошек — полного пути, на который мы погрузились (например в центре, где сейчас имя бакета). `[RW]` красит в красный лишний пробел слева. `w to arm` — непонятная подпись, нужно конкретизировать что это включает write mode; плюс пропадает хоткей перевода write обратно в read. Промпт `arm WRITE mode? mutations will be enabled (y / N)` очень незаметен внизу — легко не увидеть. Поправить конституцию: правило, что каждый атрибут интерфейса (ключ объекта, имя бакета и т.п.) должен быть полностью виден, а если из-за этого страдает интерфейс — должна быть возможность раскрыть его, чтобы посмотреть/скопировать. Плюс правило, что все элементы должны соответствовать дизайн-коду и быть консистентны (похожие промпты, похожие подписи, цвет как акцент)." Дополнено в ходе работы: хоткеи не работают при фокусе в центральной (objects) зоне; фильтр должен фильтровать текущий уровень дерева объектов, не только бакеты; в правом details-блоке остался старый формат хоткеев («^x»); непонятно работает ли мультиселект; не хватает сортировки по дате модификации и её хоткей не виден в основном меню; чистка лишних «комментариев» касается не только кода, но и самого интерфейса.

## Clarifications

### Session 2026-06-07

- Q: How should the `/` filter behave in the objects zone (US7/FR-029)? → A: Server-side filter of the current prefix (non-recursive) — matches across the whole level, not only the loaded page; the bucket filter stays an instant local filter.
- Q: How is the full value of a long name/key revealed (US1/FR-003/FR-004)? → A: Both — the active row wraps in place when it fits, and a dedicated reveal/inspect popup handles values too long to wrap and the copy action.
- Q: How is the declutter (US9) vs. hotkey advertising (US2/US8) tension resolved? → A: Every action stays advertised in at least one always-visible place (command bar) or the help overlay; declutter removes only DUPLICATE on-screen hints, never the last advertisement of an action.
- Q: What does "copy" mean in the reveal action (US1/FR-004)? → A: Best-effort terminal clipboard (OSC52) plus always displaying the full value for manual selection, degrading gracefully where OSC52 is unsupported.
- Q: While typing in the objects-zone filter input, is the preview live or applied only on Enter (US7/FR-040)? → A: Live — each keystroke (debounced server-side) narrows the level as you type, like an editor's command palette; the debounce + generation guard + cache bound the cost.
- Q: How does the active row reveal its full value when truncated (US1/FR-003)? → A: Automatically — the highlighted row wraps in place when its value is truncated (no extra keypress); it falls back to the reveal popup only when wrapping would exceed the layout budget.
- Q: Where does the read/write mode chip live in multi-zone tiers (US2/FR-038)? → A: On the primary list box (the leftmost bucket-list box in multi-zone tiers; the single box otherwise) — one fixed, always-visible location.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Resource names are never silently hidden (Priority: P1)

While browsing, a user sees bucket names, object keys, and folder/prefix names in full. When a name is too long for the column it sits in, the layout first uses available slack (e.g. the buckets column borrows width from a near-empty objects zone) before truncating anything; and when a value genuinely cannot fit, the user can reveal it in full — readable and copyable — with a single, consistent action. The currently highlighted row in particular always exposes its full value.

**Why this priority**: This is the pain that triggered the feature — bucket names overflow the left column while the middle zone has free space. A resource you cannot read or copy the name of is a broken browser. Legibility of identifiers is the core value.

**Independent Test**: Open a context whose longest bucket name exceeds the default buckets-column width on a wide terminal. Verify the column grows to fit (using the slack in the objects zone) rather than truncating; for a name longer than any reasonable column, verify the highlighted row reveals the full value and the value can be copied.

**Acceptance Scenarios**:

1. **Given** a wide terminal where the objects zone has unused horizontal space, **When** the bucket list contains a name longer than the current buckets column, **Then** the buckets column grows (up to fitting the longest visible name or its allowed maximum) before any name is truncated.
2. **Given** a name longer than the maximum column width, **When** the row is highlighted, **Then** the full value is made visible (the active row expands/wraps or a reveal surface shows it) without pushing the footer or other zones off-screen.
3. **Given** any highlighted resource (bucket / object / folder), **When** the user invokes the reveal action, **Then** the complete identifier is shown in a form that can be read in full and copied.
4. **Given** the active row is expanded, **When** the cursor moves to another row, **Then** the previous row returns to its normal single-line form (expansion is non-destructive and bounded to the active row).

---

### User Story 2 - Write mode is always legible and reversible (Priority: P1)

A user can tell at a glance whether the session is read-only or write-armed, the on-screen cue is clean (no mis-colored whitespace), the affordance to enter write mode plainly says it enables writes, and — crucially — while armed there is always a visible affordance to go back to read-only.

**Why this priority**: Write mode is the safety-critical state of the tool. An ambiguous label (`w to arm`), a cue that disappears once armed (no way back shown), and a mis-colored badge all undermine the operator's confidence about whether a keypress can mutate data.

**Independent Test**: In a writable context, observe the read-only state cue (clean badge, clear "enable write" label). Arm write; verify the badge flips and a clearly labelled "return to read-only" affordance is now visible. Verify no stray space around the `[RO]`/`[RW]` badge carries the state color.

**Acceptance Scenarios**:

1. **Given** any screen, **When** the badge renders, **Then** only the badge text (`[RO]` / `[RW]`) carries the state color — adjacent whitespace is not colored.
2. **Given** a writable, read-only (disarmed) session, **When** the write affordance renders, **Then** its label clearly states it enables write mode (not the ambiguous "to arm").
3. **Given** a write-armed session, **When** the bar renders, **Then** a clearly labelled affordance to disarm (return to read-only) is visible, bound to the same toggle key.
4. **Given** a read-only context (write forbidden), **When** the bar renders, **Then** the cue communicates that writes are unavailable for this context, consistent with the armed/disarmed cues.
5. **Given** any browse screen, **When** the main box renders, **Then** the current mode is shown as a chip inset into the box's top border (e.g. `RO` / `WRITE`), styled with the shared palette — a distinct accent when write-armed, neutral/dim when read-only — so the mode is glanceable at the frame edge, mirroring an editor's border-mounted mode label.

---

### User Story 3 - Current location breadcrumb (Priority: P2)

A user always sees the full path of where they currently are — context, bucket, and the chain of prefixes they have drilled into — in a prominent, consistent place, and it updates on every navigation.

**Why this priority**: Without a path indicator, a user deep inside nested prefixes loses track of where they are. It is a strong orientation aid but the browser is still usable without it, so it ranks below legibility and write-safety.

**Independent Test**: Drill several prefixes deep into a bucket and verify the breadcrumb shows the full path; ascend and search and verify it updates each time. Confirm it sits in the agreed prominent location.

**Acceptance Scenarios**:

1. **Given** a user browsing inside `bucket/a/b/c/`, **When** the view renders, **Then** the breadcrumb shows the full path (context → bucket → `a/b/c`).
2. **Given** any navigation (drill in, ascend, switch bucket, apply/clear search), **When** the action completes, **Then** the breadcrumb updates to the new location.
3. **Given** a path too long for its space, **When** the breadcrumb renders, **Then** it elides the middle (keeping bucket and the deepest segment) rather than dropping the deepest segment, and the full path stays revealable via the reveal action (US1).

---

### User Story 4 - Unmissable write-arm confirmation (Priority: P2)

When a user presses the key to arm write mode, the confirmation prompt is prominent and unmistakable — centered/overlaid rather than a faint line at the very bottom — so it is impossible to miss that the tool is waiting for a deliberate yes/no.

**Why this priority**: The current prompt is easy to overlook at the bottom of the screen, which risks both accidental confirmation and confusion about why input seems stuck. Important for safety, but it builds on the write-mode signalling of US2.

**Independent Test**: Press the arm key and verify the confirmation appears in a prominent surface that draws the eye, clearly states the consequence and the confirm/cancel keys, defaults to cancel, and leaves the write badge visible. Verify disarming stays instant (no prompt).

**Acceptance Scenarios**:

1. **Given** a disarmed writable session, **When** the user presses the arm key, **Then** a prominent confirmation surface appears (visually distinct, centered/overlaid — not only a bottom status line).
2. **Given** the arm confirmation is shown, **When** the user reads it, **Then** it states that mutations will be enabled and shows the exact confirm/cancel keys with cancel as the default.
3. **Given** a write-armed session, **When** the user presses the toggle key to disarm, **Then** write is disarmed instantly with no confirmation (asymmetric friction preserved).
4. **Given** the arm confirmation is shown, **When** it renders, **Then** the read/write badge remains visible (the prompt does not hide the current state).

---

### User Story 5 - Consistent design system across all UI elements (Priority: P2)

Every prompt, confirmation, label, and hint follows one shared design language: confirmations look and read alike, action labels share one vocabulary and formatting, and color is used as a consistent distinguishing accent on top of that shared base rather than as ad-hoc per-surface styling. New surfaces introduced by this feature reuse the existing component and color vocabulary.

**Why this priority**: A consistent design system makes the interface predictable and learnable and prevents this feature's new surfaces (reveal, breadcrumb, prominent confirmation) from drifting into one-off looks. It is a quality/consistency guarantee layered over the functional stories.

**Independent Test**: Compare every confirmation prompt (write-arm, delete, recursive delete, overwrite) and verify they share one pattern; compare action/hint labels and verify one vocabulary and formatting, differentiated only by the established palette roles. Verify the new reveal/breadcrumb/confirmation surfaces reuse existing styles, not new parallel ones.

**Acceptance Scenarios**:

1. **Given** the set of confirmation prompts in the app, **When** they are compared, **Then** they share a single visual pattern and wording structure ("ACTION? consequence (keys)", cancel default).
2. **Given** the action/hint labels, **When** they are compared, **Then** they follow one vocabulary and formatting (key glyph + verb), differing only by the established per-role color accents.
3. **Given** the new surfaces added by this feature, **When** they render, **Then** they reuse the existing palette roles and shared components rather than introducing new one-off styling.

---

### User Story 6 - Every hotkey works while focused in the objects zone (Priority: P1)

When focus is in the central objects zone of the two-pane browse, the full level toolset works exactly as it does in the full-screen level view: multi-select/mark, sort and sort-direction, context switch, and every per-item action (download, analyze, delete and its destructive safety chord, copy, move, upload, new folder, refresh). Today those keys are silently dead in that zone — only cursor movement, search, open, and back are honoured.

**Why this priority**: A confirmed regression from the two-pane feature. On a wide terminal the user browses through the objects zone, yet marking and every per-item action there do nothing (the keypress falls through to a no-op). It makes the two-pane layout look broken and blocks multi-select entirely on wide terminals — directly matching the reports "hotkeys don't work when you switch to the central pane" and "I can't get multi-select to work."

**Independent Test**: On a wide terminal, move focus into the objects zone; press the mark key on an object and verify it is marked; press the sort key and verify the level re-sorts; press a per-item action key and verify the same flow runs as in the full-screen level view.

**Acceptance Scenarios**:

1. **Given** focus in the objects zone with an object highlighted, **When** the user presses the mark key, **Then** the object is added to / removed from the multi-select set and the marked count is reflected.
2. **Given** focus in the objects zone, **When** the user presses the sort or sort-direction key, **Then** the objects level re-sorts with the same semantics as the full-screen view.
3. **Given** focus in the objects zone with an object highlighted, **When** the user presses any per-item action key (download, analyze, copy, move, upload, new folder, refresh) or the destructive chord, **Then** the same action/confirmation flow runs as in the full-screen level view, gated by the same write-capability rules.
4. **Given** focus in the objects zone, **When** the user presses the context-switch key, **Then** the context switcher opens, consistent with the bucket list and the full-screen view.
5. **Given** a key does not apply to the current selection or capability, **When** it is pressed in the objects zone, **Then** it behaves identically to the full-screen view (inert, or a read-only nudge) — never a silent dead key that works elsewhere.

---

### User Story 7 - Filter applies to the current level, not only buckets (Priority: P1)

The filter key filters whatever the focused zone is showing: the bucket list when focus is on buckets, and the current objects level when focus is in the objects zone (or in the full-screen level view). Today the filter key always targets the bucket list even when the user is focused on the objects zone. Activating the filter opens a dedicated, prominent single-line input (in the style of an editor command/search prompt); results preview live as the user types; pressing Enter commits the filter, closes the input, and hands focus to the now-filtered pane so the user can immediately navigate the results.

**Why this priority**: Filtering is a primary way to find an object in a large level; on a wide terminal the filter key currently narrows the wrong thing (the bucket list), which reads as broken. A prominent input with a commit-and-focus flow makes filtering discoverable and fluid.

**Independent Test**: Focus the objects zone of a populated level, press the filter key, verify a prominent input opens and results preview as you type; press Enter and verify the input closes, focus is in the objects zone, and the level shows only matches while the bucket list is unaffected; re-open to refine, and clear to restore.

**Acceptance Scenarios**:

1. **Given** any browse view, **When** the user presses the filter key, **Then** a dedicated, prominent single-line input opens indicating which pane it filters, and the footer/command bar remain visible.
2. **Given** the filter input is open, **When** the user types, **Then** the target pane previews the narrowed results live (instant for the bucket list; debounced server-side for the objects level), without affecting the other pane.
3. **Given** the filter input is open with a term, **When** the user presses Enter, **Then** the filter is committed, the input closes, and focus moves into the filtered pane (the objects zone when filtering objects), with an indicator showing the active term and how to clear it.
4. **Given** a committed filter, **When** the user presses the filter key again, **Then** the input re-opens pre-filled with the current term for refinement, and re-committing replaces it.
5. **Given** the filter input is open, **When** the user presses Esc, **Then** the in-progress input is cancelled and the view reverts to the last committed state — a previously committed filter is preserved; if none was committed, the pane returns to unfiltered.
6. **Given** a committed filter and the input closed, **When** the user invokes clear/back (per the FR-009 precedence), **Then** the committed filter is cleared and the full content of that pane is restored.

---

### User Story 8 - Sort by modification date is reachable and discoverable (Priority: P2)

The user can sort the current level by modification date (in addition to name and size) from any browse context, and the sort affordance plus the current sort state (field + direction) are visible in the main command bar — not buried only in the help overlay.

**Why this priority**: Sorting by recency is a common need; the capability already exists but is undiscoverable (never advertised) and unreachable where the user actually browses (the objects zone, per US6). Surfacing it turns a hidden feature into a usable one.

**Independent Test**: From the main browse, confirm the command bar shows the sort key and the current sort field + direction; cycle sort to modification date and verify the level reorders and the indicator updates; confirm it works in the objects zone (per US6).

**Acceptance Scenarios**:

1. **Given** any browse view, **When** it renders, **Then** the main command bar shows the sort affordance and the current sort field + direction.
2. **Given** the user cycles sort, **When** it reaches modification date, **Then** the level reorders by modification date and the indicator names it.
3. **Given** focus in the objects zone, **When** the user sorts, **Then** the same sort applies there (consistent with US6).

---

### User Story 9 - Declutter the interface (Priority: P2)

On-screen helper text is minimal, consistent, and non-redundant. Redundant or noisy annotations — the interface's own "comments" — are removed; the hints that remain follow one shared pattern, come from a single source, and never duplicate what an adjacent zone already shows.

**Why this priority**: A cluttered surface competes with the content for attention and undermines the legibility and consistency goals; trimming it makes the essential affordances stand out. (Per the clarification that "cleaning comments" applies to the interface, not only the code.)

**Independent Test**: Survey each view and confirm there is no redundant/duplicated hint text, no annotation that merely restates the obvious, and that the remaining hints share one vocabulary/format and match the command bar.

**Acceptance Scenarios**:

1. **Given** any view, **When** it renders, **Then** helper text is limited to actionable affordances not already shown elsewhere on screen.
2. **Given** the hint text across views, **When** compared, **Then** it follows one vocabulary and format (consistent with US5) and is sourced from the keymap, not hand-written.
3. **Given** a decluttered view, **When** compared to before, **Then** no actionable affordance was lost — only redundant/noisy annotation was removed.

---

### Edge Cases

- A bucket name or object key longer than the entire terminal width: the reveal surface must still present the full value (wrapped/scrollable), since no column can ever fit it.
- Narrow (single-column) terminal: active-row expansion must not consume the rows that hold the footer/command bar — visibility of the footer wins; fall back to the reveal surface if expanding in place would clip the footer.
- Active-row expansion while the details/preview pane is showing for the same selection: the two must not conflict or double-render the value.
- Breadcrumb at bucket root (no prefix) vs. deep prefix vs. with an active search filter applied.
- Arm confirmation requested while another interactive prompt (operation name/destination entry, delete confirm) is already pending: defined precedence so the two surfaces never overlap ambiguously.
- The reveal action's key must not collide with an existing binding for the current mode/selection.
- `NO_COLOR` / monochrome terminals: every state cue that currently relies on color (badge, write/read distinction, arm confirmation) must remain distinguishable by text, not color alone.
- An objects-zone action that opens a full-screen flow (upload file browser, a confirmation popup, the full-screen object view): focus must return to the objects zone afterward, not snap back to the bucket list.
- Marking objects in the objects zone, then changing the highlighted bucket or navigating to a different level: the marked-set lifecycle must be defined and consistent with the full-screen view (marks belong to a loaded level).
- Filtering the objects level while a debounced level load is still in flight (the filter must apply to the settled level, not a stale one).
- A destructive chord pressed in the objects zone while the context is read-only: must follow the same read-only refusal as the full-screen view (no surface opened).
- The sort indicator in a narrow command bar: it must fit without forcing the bar to wrap or hide a keybinding (respect the footer-visibility invariant).
- A rebound action key: every on-screen hint (details pane, confirm dialog, status line, command bar, help) must show the rebound key, with no hardcoded literal left behind.

## Requirements *(mandatory)*

### Functional Requirements

**Attribute visibility (US1)**

- **FR-001**: The bucket list column MUST size so that bucket names render in full when terminal width allows; when an adjacent zone (e.g. the objects zone) has unused horizontal space, the buckets column MUST grow to fit the longest visible name (up to a defined maximum) before any name is truncated.
- **FR-002**: When a name still cannot fit the available width, the system MUST truncate it with a clear marker AND make the full value revealable; a value MUST NEVER be permanently hidden by truncation alone.
- **FR-003**: When the currently highlighted row's value is truncated, the row MUST wrap it in place AUTOMATICALLY (no extra keypress) while that fits within the layout budget, WITHOUT pushing the footer or command/hint bar off-screen and WITHOUT corrupting adjacent zones; when the value is too long to wrap in place, the reveal popup (FR-004) MUST be used instead.
- **FR-004**: The system MUST provide a single, consistent reveal action that opens a dedicated reveal/inspect popup showing the full identifier of the current selection (bucket name, object key, folder/prefix, or breadcrumb path) for reading and copying. Copy MUST be best-effort to the terminal clipboard (OSC52) AND the full value MUST always be displayed for manual selection, so the action degrades gracefully where the terminal does not support clipboard escapes.
- **FR-005**: Active-row expansion MUST be non-destructive and bounded to the active row — moving the cursor away restores the normal single-line row.

**Write-mode legibility & reversibility (US2)**

- **FR-006**: The read/write badge (`[RO]` / `[RW]`) MUST apply the state color only to the badge text; surrounding whitespace MUST NOT carry the state color.
- **FR-007**: The affordance that enters write mode MUST be labelled to clearly state that it enables write mode (replacing the ambiguous "w to arm").
- **FR-008**: While write is armed, the system MUST keep visible a clearly labelled affordance to return to read-only (disarm), bound to the same toggle key.
- **FR-009**: The write-state cue MUST be present and symmetric across all states (read-only-disarmed, armed, and context-forbidden), so the toggle key and current state are always discoverable.
- **FR-038**: The read/write mode MUST be rendered as a chip inset into the top border of the primary list box (a border-mounted mode label, modeled on an editor's mode indicator) — in multi-zone tiers this is the leftmost bucket-list box; in the single-column tier it is the sole box — one fixed, always-visible location. It MUST be styled only via the shared palette roles — a distinct accent for write-armed, neutral/dim for read-only — and remain distinguishable without color (text `RO`/`WRITE`). The write-state cue is the one explicit exception to the declutter dedup rule (FR-033): because it is safety-critical, the border chip and the footer badge MAY both display it.

**Breadcrumb (US3)**

- **FR-010**: The system MUST display the full path of the current location (context → bucket → prefix chain) while browsing.
- **FR-011**: The breadcrumb MUST be placed in a prominent, consistent location — the center label of the objects zone (where the bucket name currently shows) in multi-zone tiers, and the box title in the single-column tier.
- **FR-012**: The breadcrumb MUST update on every navigation event (drill in, ascend, switch bucket, apply/clear search) to reflect the current depth.
- **FR-013**: When the breadcrumb exceeds its available width, it MUST elide the middle (preserving the bucket and the deepest segment) rather than drop the deepest segment, and the full path MUST remain revealable via FR-004.

**Prominent write-arm confirmation (US4)**

- **FR-014**: The write-arm confirmation MUST be presented in a prominent, visually distinct surface (centered/overlaid) — not only a faint bottom status line — so the user cannot miss that input is awaited.
- **FR-015**: The confirmation MUST state the consequence (mutations will be enabled) and the exact confirm/cancel keys, with cancel as the default.
- **FR-016**: Disarming MUST remain instant with no confirmation; only arming requires the prominent confirmation (asymmetric friction preserved).
- **FR-017**: While the arm confirmation is shown, the read/write badge MUST remain visible.

**Design-system consistency (US5)**

- **FR-018**: All confirmation prompts (write-arm, delete, recursive delete, overwrite, and any future destructive confirmation) MUST share a single visual pattern and wording structure ("ACTION? consequence (keys)", cancel default).
- **FR-019**: All action and hint labels MUST follow one consistent vocabulary and formatting (key glyph + verb), differing only by the established per-role color accents — no one-off label styling.
- **FR-020**: New UI surfaces introduced by this feature (reveal surface, breadcrumb, prominent arm confirmation) MUST reuse the existing palette roles and shared components rather than introduce parallel styling.

**Cross-cutting guarantees & governance**

- **FR-021**: Every interface attribute that identifies a resource (bucket name, object key, folder/prefix, breadcrumb path) MUST be either fully visible OR revealable in full via the consistent reveal action (FR-004).
- **FR-022**: All changes MUST preserve existing layout invariants — the footer and command/hint bar MUST remain fully visible (never scrolled off) at every supported terminal width and layout tier.
- **FR-023**: The project constitution MUST gain a **UI Legibility** principle encoding FR-021 (no identifying attribute permanently hidden; always fully visible or revealable to view/copy). (Executed via the constitution workflow; see Dependencies.)
- **FR-024**: The project constitution MUST gain a **UI Consistency / Design System** principle encoding FR-018–FR-020 (all interface elements conform to a shared design language: consistent prompts, labels, and component vocabulary; color as a consistent distinguishing accent, not ad-hoc styling). (Executed via the constitution workflow; see Dependencies.)
- **FR-025**: Every state cue affected by this feature (badge, write/read distinction, arm confirmation, reveal surface, breadcrumb elision) MUST remain distinguishable without color (e.g. under `NO_COLOR`), relying on text in addition to color.

**Focus-zone hotkey parity (US6) — regression fix**

- **FR-026**: When focus is in the objects zone, the system MUST honour the same key set as the full-screen level view — multi-select/mark, sort, sort-direction, context switch, and every per-item action including the destructive safety chord — with identical availability and write-capability gating.
- **FR-027**: A key that performs an action in the full-screen level view MUST NOT be a silent no-op in the objects zone; where an action does not apply to the current selection or capability, the objects zone MUST give the same feedback (inert, or read-only nudge) as the full-screen view.
- **FR-028**: Multi-select marking MUST be operable in the objects zone, and the marked-set indication (count) MUST be visible while focus is there.

**Level filter scope (US7)**

- **FR-029**: The filter key MUST target the content of the focused zone. On the bucket list it is an instant local filter (unchanged). In the objects zone (and the full-screen level view) it MUST be a server-side filter of the current prefix (non-recursive) that matches across the whole level — not only the entries already loaded on screen — so a match on an unloaded page is still found.
- **FR-030**: Clearing a filter MUST restore the full content of the zone it was applied to and follow the established clear/back precedence.
- **FR-039**: Activating the filter MUST open a dedicated, prominent single-line input (reusing the shared input/field component) that indicates which pane it filters; the footer and command bar MUST remain visible while it is open.
- **FR-040**: While the filter input is open, the target pane MUST preview the narrowed results live (instant local for the bucket list; debounced server-side for the objects level). Pressing the commit key (Enter) MUST commit the filter, close the input, and move focus into the filtered pane, with an indicator showing the active term and the clear affordance.
- **FR-041**: Re-activating the filter while a filter is committed MUST pre-fill the input with the current term for refinement. Esc MUST cancel the in-progress input and revert to the last committed state — a previously committed filter is preserved; if none was committed, the pane returns to unfiltered — which is distinct from clearing a committed filter (FR-030).

**Sort reachability & discoverability (US8)**

- **FR-031**: Sorting by modification date MUST be reachable from every browse context, including the objects zone, alongside name and size.
- **FR-032**: The main command bar MUST advertise the sort affordance and display the current sort field and direction (not only the help overlay).

**Interface declutter (US9)**

- **FR-033**: On-screen helper text MUST be limited to actionable affordances not already shown by an adjacent zone; redundant or purely descriptive annotations MUST be removed. Declutter MUST remove only DUPLICATE on-screen hints — every action MUST remain advertised in at least one always-visible place (the command bar) or the help overlay, so the last advertisement of an action is never removed.
- **FR-034**: All remaining on-screen hints MUST be sourced from the single keymap and rendered with the shared formatting (consistent with FR-019), so no actionable affordance is lost while redundant annotation is trimmed.

**Keymap single-source & stale glyphs (extends US5)**

- **FR-035**: Every on-screen key hint (details pane, confirmation dialogs, status lines, command bar, help overlay) MUST render its key via the shared keymap + glyph formatting; no hardcoded key literal — a hand-typed chord glyph (e.g. `^x`) or a hand-typed "key/key/key" list — may remain, and a rebind MUST propagate to every surface.
- **FR-036**: Key dispatch in confirmation and prompt surfaces MUST consult the keymap (e.g. the Back binding) rather than a hardcoded key string, so a rebind takes effect there too.
- **FR-037**: The focus-toggle key MUST be part of the single keymap source rather than a hardcoded literal, so it is rebindable and advertised consistently with every other key.

### Key Entities *(include if feature involves data)*

- **Resource identifier**: the displayable name of a browseable item — bucket name, object key, or folder/prefix. Has a full value and a possibly-truncated rendered form; the full value must always be recoverable.
- **Breadcrumb path**: the ordered chain (context → bucket → prefix segments, plus optional active-search marker) describing the current browse location; rendered prominently and elidable.
- **Write-state cue**: the always-visible indicator of read-only vs. write-armed vs. context-forbidden, including the toggle affordance and its arm/disarm labels.
- **Confirmation surface**: the shared prominent prompt pattern used for write-arm and destructive confirmations (consequence text + confirm/cancel keys + default).
- **Reveal surface**: the consistent surface that shows a selected resource identifier or breadcrumb path in full for reading/copying.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of bucket, object, and folder names are either fully visible or fully revealable — no identifying name is permanently unreadable in any layout tier.
- **SC-002**: A user can reveal the full value of any truncated name or path in at most one keystroke.
- **SC-003**: On a wide terminal, a buckets column with slack available beside it grows to fit the longest visible bucket name with zero truncation before the objects zone runs out of slack.
- **SC-004**: The current full path is visible while browsing and updates within one frame of every navigation action (verified across drill-in, ascend, bucket switch, and search).
- **SC-005**: The read/write state is identifiable at a glance on every screen, with no whitespace carrying the state color.
- **SC-006**: Both the arm and the disarm affordances are visible in their respective states 100% of the time, bound to the same toggle key.
- **SC-007**: In a usability check, users notice the arm confirmation before responding to it — it occupies a central/overlay region rather than a single bottom line.
- **SC-008**: The footer and command/hint bar remain fully visible at every supported terminal width after all changes (no regression versus the current build).
- **SC-009**: All confirmation prompts and action labels conform to one documented design pattern — zero one-off prompt/label styles are introduced, and every new surface reuses existing palette roles/components.
- **SC-010**: The constitution contains two new enforceable principles — UI Legibility and UI Consistency / Design System.
- **SC-011**: 100% of the level-toolset keys that work in the full-screen level view also work while focus is in the objects zone (verified key-by-key: mark, sort, sort-direction, context, and every per-item action + chord) — zero silent dead keys.
- **SC-012**: Filtering while focus is in the objects zone narrows the objects level (not the bucket list) in 100% of cases, and clearing restores it.
- **SC-013**: Sort by modification date is reachable in every browse context, and the current sort field + direction are visible in the command bar at all times.
- **SC-014**: Zero hardcoded or stale key hints remain; rebinding any action key updates every on-screen hint (verified by a rebind test across details pane, confirm dialog, status line, command bar, and help).
- **SC-015**: The declutter removes only redundant annotation — the set of actionable affordances reachable after the change is a superset-or-equal of the set before (no affordance lost).

## Assumptions

- **Reveal & copy** (resolved, see Clarifications): the reveal action opens a dedicated popup that always displays the full value and additionally copies it to the terminal clipboard via OSC52 on a best-effort basis, degrading to display-only where unsupported.
- **Active-row expansion** (resolved, see Clarifications): the mechanism is "both" — the active row wraps in place while that preserves footer/command-bar visibility and zone integrity (FR-022); where it would not (a value longer than the wrap budget, or a very short terminal), the reveal popup is used instead.
- **Breadcrumb placement** follows the user's suggestion: the center label of the objects zone (currently the bucket name) in multi-zone tiers; the box title in the single-column tier.
- **Constitution amendments** (FR-023, FR-024) are performed via the dedicated constitution workflow, not this feature's implementation; this spec records the requirements they must satisfy and their acceptance is tracked by SC-010.
- **No storage-contract change**: this is a presentation/UX feature; it adds no storage method and no write-capable S3 symbol outside the storage package, so the read-only structural guard and the integration-test surface are unaffected.
- The existing per-role color palette (info/read/write-active/write-dimmed/inapplicable, per-field hues) is the design system's color layer to be reused — this feature consolidates usage rather than introducing new hues.
- **US6 and US7 are regression fixes** of the two-pane objects zone (its key handler was implemented with navigation/search/open/back only, omitting marking, sorting, context, and per-item action dispatch). The objects zone reuses the same level primitives as the full-screen view, so "parity" means routing its keys through the same dispatch + action availability the full-screen level view uses — not inventing new behaviour.
- **Sort by modification date already exists** as a sort field; US8 makes it reachable (everywhere the user browses) and discoverable (advertised in the command bar), rather than adding a new sort algorithm.
- **Marked-set scope**: marks belong to a loaded level; navigating to a different bucket or level clears them — existing behaviour preserved across the objects zone.
- **Code-comment cleanup** (stripping spec/US/FR citation noise from the UI source) is a non-behavioural refactor folded into this feature's implementation, distinct from the user-facing interface declutter (US9). It must keep `gofmt`/`vet`/lint and the read-only structural guard green and preserve machine directives (`//go:build`, `//nolint`, `//go:generate`) and minimal doc comments on exported symbols.

## Dependencies

- **Constitution update** (`/speckit-constitution`): add the UI Legibility principle (FR-023) and the UI Consistency / Design System principle (FR-024). This is a governance prerequisite for marking the feature complete (SC-010) and should run alongside or before implementation so reviews can enforce the new principles.
