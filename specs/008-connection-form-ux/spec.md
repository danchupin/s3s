# Feature Specification: Connection Management UX Fixes

**Feature Branch**: `008-connection-form-ux`

**Created**: 2026-06-07

**Status**: Draft

**Input**: User description: "Во первых опять не понятно как удалять подключение - кнопка невидимая в ui. Во вторых - почему кнопки удаления и т п подписаны как \"^ кнопка\", если должно быть \"Ctrl + кнопка\". В третьих, в интерфейсе добавления подключения формы неюзбальны - в них нельзя вставлять значения из буфера - это недопустимо, также в них нельзя переключать каретку между символами - это очень неудобно. Также не понятно как вводить секреты - нужна какая то подсказка из доступных вариантов. Давай это исправим"

## Overview

Four usability defects in the connection-management surfaces (connection list +
add-connection form, introduced in 006/007) block real use. None change *what*
the feature does — they fix discoverability and basic text-input ergonomics so an
operator can actually delete and add connections without prior knowledge of the
keybindings or fighting an editor that cannot paste or move its caret.

## Clarifications

### Session 2026-06-07

- Q: Secret-source scope of the add-connection form (keychain is the only save path today)? → A: Hint-only — the form keeps saving to the keychain; add explanatory text listing valid inputs (including `${ENV}` references). No source selector, no config-writer changes.
- Q: How is the connection-delete keystroke surfaced (delete is not in the command-bar action catalog)? → A: Inline hint in the connections view itself (alongside the existing ↑/↓ · Enter · Esc help line), not by adding delete to the command-bar action catalog.
- Q: Modifier-label format ("Ctrl + X" with spaces vs "Ctrl+C" no-space)? → A: "Ctrl+X" with no spaces, consistent with the existing "Ctrl+C".
- Scope expansion (2026-06-07, follow-up): four further UX defects folded into 008 as US6–US9 — post-mutation visibility for ALL actions incl. cross-level copy/move (US6), visible connection-switch affordance (US7), filter-reset affordance (US8), no duplicate delete labels (US9). All stay UI-only (no storage/config change); cache invalidation must cover source AND destination levels.

### Session 2026-06-07 (US6–US9 design)

- Q: US9 — remove the duplicate "delete" by relabeling the recursive one, or by showing only the selection-applicable delete? → A: Show only the selection-applicable delete entry in the write group (a targeted exception to 007's "all write actions always shown" — for the delete pair only).
- Q: US7 — surface a separate `c switch` entry, relabel the existing connection-manager affordance, or both? → A: Relabel the existing `n` affordance from "new conn" to "connections" (one entry; the manager it opens already does switch + add + delete). No separate switch entry.
- Q: US6 — invalidate the whole cache after a mutation, or only the affected keys? → A: Precise — invalidate only the source and destination level keys (not a whole-cache Clear()).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discoverable connection delete (Priority: P1)

An operator viewing the connection list wants to remove a stale connection. The
keystroke that deletes a connection is currently never shown anywhere on that
screen, so the only way to find it is to read the source. The operator must be
able to *see* how to delete a connection while looking at the list.

**Why this priority**: A destructive action that exists but is invisible is
effectively missing — the user explicitly reported it twice ("опять не понятно").
Highest discoverability cost, lowest implementation cost.

**Independent Test**: Open the connection list, render it, and confirm the delete
keystroke and its label appear on screen for a deletable (non-active) selection,
and that an explanation appears (rather than silence) when the active connection
is selected.

**Acceptance Scenarios**:

1. **Given** the connection list with a non-active connection highlighted, **When** the screen renders, **Then** a hint showing the delete keystroke (spelled in full, e.g. "Ctrl+X") and a "delete" label is visible.
2. **Given** the connection list with the active connection highlighted, **When** the delete keystroke is pressed, **Then** a message explains the active connection cannot be deleted (switch context first) instead of doing nothing.
3. **Given** the "+ add connection" row highlighted, **When** the screen renders, **Then** the delete hint is absent (not rendered) — nothing to delete on that row (FR-003 single behaviour).

---

### User Story 2 - Keystroke labels spell out modifiers (Priority: P1)

Across the UI, modifier-key chords are labelled with caret shorthand ("^x", "^o")
that non-power-users do not recognise as "Ctrl+X" / "Ctrl+O". Every place a chord
is advertised must spell the modifier the same human-readable way.

**Why this priority**: Trivial fix, removes a recurring point of confusion the
user called out directly. Consistency: one chord (`Ctrl+C`) is already spelled
out while the others use carets.

**Independent Test**: Render every surface that advertises a dangerous-action
chord (command bar, connection list, confirmation prompts, help) and assert no
caret-style chord glyph ("^x") remains — all read "Ctrl+X" style.

**Acceptance Scenarios**:

1. **Given** any surface that shows the delete or move chord, **When** it renders, **Then** the chord reads "Ctrl+X" / "Ctrl+O" (full modifier name), never "^x" / "^o".
2. **Given** the existing quit chord, **When** rendered, **Then** it still reads "Ctrl+C" (already correct) and the new spelling is consistent with it.

---

### User Story 3 - Usable text entry in the add-connection form (Priority: P1)

When adding a connection the operator types into name / endpoint / region / access
key / secret fields. Today the field is a write-only tail: characters can only be
appended or backspaced from the end, the caret cannot move, and pasting a value
from the clipboard does not work. Endpoints, keys and secrets are long, copied
values — typing them by hand is error-prone and pasting is essential.

**Why this priority**: The user calls clipboard paste "недопустимо" (unacceptable)
to lack. Without paste and caret movement the form is not usable for its actual
inputs (long URLs, 40-char secret keys). Core of the feature.

**Independent Test**: Open the add-connection form, paste a multi-character value
into a field and confirm the whole value lands; move the caret into the middle of
a field, insert and delete a character, and confirm the edit happens at the caret
(not the end).

**Acceptance Scenarios**:

1. **Given** the cursor on a text field, **When** the operator pastes clipboard content, **Then** the entire pasted string is inserted at the caret in one action (not character-dropped or ignored).
2. **Given** a field with existing text, **When** the operator moves the caret left/right (and to start/end), **Then** the caret moves between characters and subsequent typing inserts at that position.
3. **Given** the caret in the middle of a field, **When** the operator deletes, **Then** the character at the caret boundary is removed (not always the last character).
4. **Given** the secret field, **When** the operator pastes or types, **Then** the value is editable like the others but still rendered masked, and is never written to the config in plaintext (unchanged from today).
5. **Given** focus on a non-text row (the toggles), **When** the operator presses a caret-movement or paste key, **Then** it is a harmless no-op (no crash, toggles still work on space).

---

### User Story 4 - Guidance for entering secrets (Priority: P2)

The secret field offers no indication of what is expected or what alternatives
exist. The configuration model already supports several credential sources (an
inline / environment-variable secret, the OS keychain, an external command, an AWS
profile). The form must tell the operator what the field expects and which secret
sources are available, so entering credentials is not a guessing game.

**Why this priority**: Improves first-time success but the form is functional
without it once US3 lands. Depends on US3's usable field for the inline case.

**Independent Test**: Open the add-connection form, focus the secret field, and
confirm on-screen help names the expected input and lists the available credential
source options.

**Acceptance Scenarios**:

1. **Given** the add-connection form, **When** the secret field is focused, **Then** a hint describes what to enter (the secret access key, stored in the OS keychain) and notes that other credential sources can be set via the config file.
2. **Given** the secret guidance, **When** the operator reads it, **Then** it is clear the form stores to the keychain and does NOT resolve `${ENV}` / cmd / AWS-profile references (those are config-file-only), so no one is misled into typing an env reference.
3. **Given** any focused field, **When** the screen renders, **Then** a short per-field expectation is discoverable (the secret field is not the only one left unexplained).

---

### User Story 5 - Quieter command bar without block headings (Priority: P3)

The command bar groups its entries into INFO / READ / WRITE blocks, each labelled
with a heading. The headings are visual noise — the grouping (separate columns,
separated by the existing inter-column gap) already communicates the distinction.
All three heading labels (INFO, READ, WRITE) must go while the logical grouping
(entries still arranged in their columns) stays.

**Why this priority**: Pure visual cleanup; independent of the other stories and
lowest risk. Nice-to-have polish.

**Independent Test**: Render the command bar and confirm the literal "INFO" /
"READ" / "WRITE" heading labels are gone while the info, read, and write entries
remain in distinct columns separated by the inter-column gap.

**Acceptance Scenarios**:

1. **Given** the command bar at full width, **When** it renders, **Then** no "INFO", "READ", or "WRITE" heading text appears, yet info / read / write entries remain in separate columns (the ≥2-space inter-column gap is the separator).
2. **Given** a read-only context, **When** the command bar renders, **Then** the literal text "w to arm" (amber, with a text-only marker so it survives NO_COLOR) appears in/adjacent to the write group — the read-only cue formerly carried by the WRITE heading is preserved at a defined surface, not "some form".
3. **Given** the collapsed (narrow) command bar, **When** it renders, **Then** it remains readable, the read row and write row stay on separate lines (the grouping), and no orphaned heading text appears.

---

### User Story 6 - Changes visible immediately after every action (Priority: P1)

After a successful mutation the operator expects to see the result without pressing
refresh. Today same-location mutations (new folder, upload, delete, recursive
delete) re-show automatically, but a copy or move (single or bulk) whose destination
is a different **prefix** leaves that destination showing stale data until a manual
refresh — because only the currently-viewed level's cache is invalidated. Every
action (add / delete / copy / move / upload / bulk copy) must reflect immediately,
including when the change lands in a different prefix of the same bucket.

Scope note: copy/move are same-bucket operations (the storage contract's `CopyKey`/
`MoveObject` take one bucket); cross-prefix within that bucket is the only "different
location" case, so this story is bounded to same-bucket cross-prefix visibility — no
cross-bucket handling and no storage change (keeps US6 UI-only, FR-018).

**Why this priority**: A storage browser that shows stale contents after a write
reads as broken / data-losing. The user reported it directly ("неудобно"). Data
trust is core.

**Independent Test**: Copy an object to a different prefix, navigate to that prefix,
and confirm the copied object is present without a manual refresh; repeat for move
(source no longer present, destination present).

**Acceptance Scenarios**:

1. **Given** an object is copied to a prefix other than the current view (same bucket), **When** the operator navigates to the destination prefix, **Then** the copied object is present without a manual refresh.
2. **Given** an object is moved to another prefix in the same bucket, **When** the operator views the source and the destination, **Then** the source no longer shows it and the destination shows it, both without manual refresh.
3. **Given** a bulk copy to a destination prefix other than the current view, **When** the operator navigates to that prefix, **Then** the copied objects are present without a manual refresh.
4. **Given** any same-location mutation (new folder, upload, delete, recursive delete, bucket delete), **When** it completes, **Then** the current view already reflects it (no regression).
5. **Given** any mutation, **When** it completes, **Then** the visibility behaviour is uniform — no action requires a manual refresh that another action performs automatically.

---

### User Story 7 - Visible connection-switch affordance (Priority: P1)

The command bar advertises adding a new connection ("new conn") but no longer shows
how to **switch** the active connection, even though a switch key exists. The
operator must be able to see, from the command bar, how to switch connection/context.

**Why this priority**: Switching connection is a primary, frequent action; an
existing capability made invisible is effectively lost. The user reported it
disappeared.

**Independent Test**: Render the command bar in the list views and confirm the
connection affordance is labelled "connections" (the entry that opens the manager
where one switches, adds, or deletes a connection) — not the narrower "new conn".

**Acceptance Scenarios**:

1. **Given** the command bar renders in the bucket/object list, **When** a connection manager is available, **Then** the affordance is labelled "connections" (conveying it covers switching, not only adding).
2. **Given** the "connections" affordance, **When** triggered, **Then** it opens the connection manager from which the operator switches the active connection (existing behaviour).
3. **Given** a narrow/collapsed command bar, **When** it renders, **Then** the "connections" affordance remains discoverable — it MUST be ordered ahead of droppable read entries (or in the retained keep-min set) so width-trimming does not drop it first.

---

### User Story 8 - Reset an active filter (Priority: P2)

When a bucket-name filter or a level search is applied, the list views (which render
the command bar, not the legacy hint line) show no affordance to clear it. The
operator must be able to see how to reset an active filter while looking at the
filtered list.

**Why this priority**: A filter the operator cannot tell how to clear feels stuck;
medium impact, low cost (a conditional hint).

**Independent Test**: Apply a filter, render the list, and confirm a reset/clear
affordance (keystroke + label) is visible; trigger it and confirm the full
unfiltered list returns.

**Acceptance Scenarios**:

1. **Given** an applied filter or search (not actively typing), **When** the list renders, **Then** a reset/clear affordance (keystroke + label) is visible.
2. **Given** no active filter, **When** the list renders, **Then** the reset affordance is absent (not shown when there is nothing to clear).
3. **Given** the reset affordance is triggered, **When** it completes, **Then** the full unfiltered list is restored.

---

### User Story 9 - No duplicate delete entries in the command bar (Priority: P2)

The command bar's write group can show two entries both labelled "delete" (single/
group object delete and recursive folder delete), which reads as a duplicate. Only
the delete action applicable to the current selection must be shown, so no identical
label appears twice in the same group.

**Why this priority**: Cosmetic/clarity defect, but the user reported it and it
undermines trust in the bar. Low cost.

**Independent Test**: Render the command bar for an object selection and a folder
selection and confirm exactly one delete entry appears in each (object delete for
the object cursor, recursive delete for the folder cursor) — never two.

**Acceptance Scenarios**:

1. **Given** an object cursor (with or without marks), **When** the command bar renders, **Then** the write group shows the object/group delete entry and NOT the recursive-delete entry.
2. **Given** a folder cursor, **When** the command bar renders, **Then** the write group shows the recursive-delete entry and NOT the object-delete entry.
3. **Given** any selection, **When** the command bar renders, **Then** no two entries in the write group share an identical label.

---

### Edge Cases

- Pasting a value containing a trailing newline (clipboard often appends one) into a single-line field — the newline must not break the field or submit the form prematurely.
- Pasting into the secret field must keep masking; the masked render must reflect the new length.
- Caret-movement keys on the boolean toggle rows must not move a non-existent caret or panic.
- A field value longer than the visible width must scroll horizontally so the caret stays visible (already partially handled by truncation; caret must remain on screen).
- Deleting a connection that leaves zero connections returns to the appropriate empty state (unchanged from 007 behaviour — must not regress).
- The delete hint must remain visible/correct on a narrow terminal where the command bar collapses.
- A copy/move whose destination level was previously visited and is cached must not show stale contents after the operation (its cache entry must be invalidated, not just the current view's).
- A copy/move to a brand-new prefix not yet in the cache must show correctly on first navigation (no stale empty/placeholder).
- The "connections" affordance opens the manager regardless of how many connections exist (one can still add); switching among them happens inside the manager.
- The filter-reset affordance must not appear while the operator is actively typing the filter (that state already shows its own Enter/Esc hints) — only once a filter is applied.
- When both delete actions are inapplicable to the current selection, neither should produce a misleading duplicate in the write group.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The connection list MUST display, inline in the connections view (alongside the existing help line, NOT via the command-bar action catalog), the keystroke and label for deleting the highlighted connection whenever a deletable (non-active) connection is selected.
- **FR-002** (non-regression): Pressing the delete keystroke on the active connection MUST keep showing the explanatory message (active connection cannot be deleted) — already implemented; this is a non-regression assertion that the FR-001 hint makes reachable, not new work.
- **FR-003**: The delete hint MUST be absent (not rendered) on the "+ add connection" row and on an empty list. (Single chosen behaviour — not "absent or marked inapplicable".)
- **FR-004**: Every surface that advertises a modifier-key chord MUST spell the modifier in full with no spaces around the "+" ("Ctrl+X", "Ctrl+O"), consistent with the existing "Ctrl+C", and MUST NOT use caret shorthand ("^x", "^o").
- **FR-005**: Text fields in the add-connection form MUST accept clipboard paste, inserting the entire pasted content at the caret in a single action.
- **FR-006**: Text fields MUST support caret movement between characters (left/right and to start/end of the field), with insertion and deletion occurring at the caret position. When the value is longer than the visible width, the field MUST scroll horizontally so the caret stays on screen.
- **FR-007**: The secret field MUST be editable with the same paste/caret capabilities as the other text fields while remaining masked on screen and never persisted to config in plaintext.
- **FR-008**: Caret-movement and paste inputs on non-text rows (toggles) MUST be harmless no-ops and MUST NOT affect the existing space-to-toggle behaviour.
- **FR-009**: The add-connection form MUST provide on-screen guidance (text only) for the secret field: name the expected input (the secret access key, stored in the OS keychain) and note that other credential sources (environment variable, external command, AWS profile) are available by editing the config file directly. No source selector and no config-writer change.
- **FR-010**: The form's secret save path MUST remain keychain-only; the form does NOT resolve or persist `${ENV}` / cmd / AWS-profile sources (those stay configurable via the config file). The guidance MUST make this division clear so an operator is not misled into typing an env reference the form would store verbatim.
- **FR-011**: All existing add/test/save/delete behaviour (validation, reachability test, save-anyway, keychain storage, refusing the active context) MUST be preserved — these are ergonomics fixes, not behavioural changes.
- **FR-012**: The delete hint and chord labels MUST remain correct and visible when the command bar collapses on a narrow terminal.
- **FR-013**: The command bar MUST NOT render ANY block heading text ("INFO", "READ", "WRITE"); the info / read / write grouping MUST remain visually distinct through column layout alone, with the ≥2-space inter-column gap as the separator (and, on the collapsed bar, read vs write on separate lines).
- **FR-014**: After the WRITE heading is removed, the read-only write-state cue MUST be preserved at a defined surface: the literal text "w to arm" rendered in/adjacent to the write group (amber), including a text-only form so it survives NO_COLOR. ("Some form" is not acceptable — the surface is the write group.)
- **FR-015**: After any successful mutation (new folder, upload, copy, move, bulk copy, delete object/group, recursive delete, bucket delete), the resulting state MUST be visible without a manual refresh, including when the change lands in a different prefix of the same bucket. This subsumes the uniformity guarantee (formerly FR-017): no action may require a manual refresh that another action performs automatically.
- **FR-016**: A copy / move / bulk copy MUST precisely invalidate the cached level(s) for BOTH the source prefix and the destination prefix (same bucket — copy/move are single-bucket), so a later navigation to either shows fresh contents. Invalidation MUST be precise (the affected source + destination keys only), NOT a whole-cache clear (clarified).
- **FR-017**: (Merged into FR-015 — the uniform-visibility guarantee.) Retained as a non-normative pointer; FR-015 is the single normative source.
- **FR-018**: The post-mutation visibility behaviour MUST stay within the UI layer (cache invalidation + reload); it MUST NOT require changes to the storage or config packages, preserving the read-only structural guard.
- **FR-019**: The command bar's connection affordance MUST be labelled "connections" (it opens the manager where one switches, adds, or deletes a connection), replacing the narrower "new conn" label, so switching is discoverable. (Clarified: relabel the existing affordance; no separate switch entry.)
- **FR-020**: The "connections" affordance MUST remain discoverable when the command bar collapses: it MUST be ordered ahead of droppable read entries (or included in the retained keep-min set) so width-trimming never drops it first. It is shown whenever a connection manager is available (as the existing affordance is today). (Note: today it is appended LAST in the collapsed read row, which trimming drops first — the implementation MUST reorder it.)
- **FR-021**: When a filter term exists AND the search input is closed (precise predicate: a non-empty bucket filter or level search, with the input not in typing mode), the list views MUST display a reset/clear affordance (keystroke + label); triggering it MUST restore the full unfiltered list. The affordance MUST be absent when no filter term exists or while the input is being typed (that mode shows its own Enter/Esc hints).
- **FR-022**: The command bar write group MUST show only the delete action applicable to the current selection (object/group delete for an object cursor; recursive delete for a folder cursor) so no two entries share an identical "delete" label. This is a targeted exception to the 007 rule that all write actions are always shown — applied to the delete pair only.

### Key Entities

- **Connection list selection**: The currently highlighted row in the connection manager; determines whether the delete hint is shown (non-active connection), absent (the "+ add" row / empty list), or triggers the "active connection" guard message on press.
- **Add-connection form field**: An editable text value with a caret position (offset within the value) and a horizontal scroll/visible window; the secret field additionally carries a masked render.
- **Credential source option**: A way to supply the secret (direct secret value, environment-variable reference, and the other config-supported sources) surfaced as guidance in the form.
- **Cached level**: The per-session cached contents of a (context, bucket, prefix, search) location; a copy/move must precisely invalidate the source and destination prefix keys (same bucket) it affects, not only the displayed one.
- **Command-bar affordance**: A keystroke+label entry in the command bar ("connections", reset filter "clear", the selection-applicable delete); its visibility is conditional on state (manager available, active filter, selection applicability).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new operator can locate and trigger connection delete using only what is shown on the connection-list screen (no docs, no source reading).
- **SC-002**: No surface in the application displays a caret-style chord label; 100% of advertised chords read in "Ctrl+KEY" form.
- **SC-003**: An operator can populate every add-connection field by pasting from the clipboard and complete the form without typing any long value by hand.
- **SC-004**: An operator can correct a typo in the middle of any field by moving the caret to it, without clearing and retyping the whole value.
- **SC-005**: When focused on the secret field, the operator can name at least two valid ways to supply a secret from the on-screen hint alone.
- **SC-006**: The command bar shows no "INFO"/"READ"/"WRITE" heading text while a viewer can still tell the info, read, and write entries apart by column layout alone.
- **SC-007**: After any add/delete/copy/move/upload/bulk-copy (incl. a same-bucket cross-prefix destination), the operator sees the change reflected at its location without pressing refresh — 0 manual refreshes needed to observe a completed mutation.
- **SC-008**: The operator can identify how to switch connection from the command bar alone (the "connections" affordance), no docs.
- **SC-009**: With a filter applied, the operator can identify how to clear it from the on-screen affordance alone, and clearing restores the full list.
- **SC-010**: The command-bar write group never shows two identical labels for any selection.

## Assumptions

- The existing connection delete keybinding (the dangerous-action delete chord) is retained; this feature makes it *discoverable*, it does not introduce a new binding.
- The terminal and Bubble Tea runtime deliver clipboard paste as input the application can receive (bracketed paste); no OS-level clipboard integration beyond what the terminal provides is required.
- Caret movement is single-line (the fields are single-line values); multi-line editing is out of scope.
- The form's save path stays keychain-only (clarified 2026-06-07); guidance only *names* the other config-supported sources (env var, cmd, AWS profile) as config-file options. This feature adds no new credential-source type and no source selector to the form.
- Read-only posture and the constitution's safe-operations principle are unchanged; this feature touches UI ergonomics only.
- Wording/label conventions established in 007 (imperative verb labels, shared confirmation surfaces) are reused.
- Post-mutation visibility is achieved by invalidating the affected cached levels (source + destination, precisely) and reloading; the per-session level cache otherwise stays (manual refresh remains valid). A whole-cache clear is NOT used (clarified).
- The connection manager and filter-clear path already exist; US7 relabels the existing connection affordance to "connections", US8 surfaces the existing Esc-clear in the bar — neither introduces a new binding.
- The two delete actions are mutually exclusive by selection (object cursor vs folder cursor); US9 shows only the applicable entry — no behaviour change to the delete operations themselves.
