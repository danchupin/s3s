# Feature Specification: UI mode chip dedup, footer breathing room, applied-filter state

**Feature Branch**: `013-ui-mode-footer-filter`

**Created**: 2026-06-08

**Status**: Draft

**Input**: User description: "Очередная итерация улучшения UI/UX. Во первых, зачем дублируется лейбл write и read режимов? оставь только новый. Плюс мне не нравится что в футере/меню слишком маленькие отступы между элементами, все слишком слеплено. Дополнительно со строкой фильтра нужны улучшения - я бы хотел чтобы было видно состояние текущего фильтра, если он примененен"

## Clarifications

### Session 2026-06-08

- Q: Where does the persistent applied-filter indicator live? → A: A border-mounted chip on the **filtered pane's box top border** (the same idiom as the mode chip / breadcrumb). It is NOT placed in the footer/command bar (which drops trailing entries under width pressure) nor appended to the breadcrumb title.
- Q: After the duplicate footer mode tag is removed, what carries the read/write state in non-list (object) modes? → A: A **universal border-mounted mode chip** on every browse screen's box top border (bucket list, object level, AND opened object). The old footer/identity mode tag is removed everywhere — one render path, one idiom, no per-mode branching. Modal confirm/arm surfaces keep their own loud badge (exempt).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One read/write mode indicator, never duplicated (Priority: P1)

While browsing, the operator sees the current read/write mode shown in **exactly one
place**. Today the mode appears twice at the same time: a border-mounted chip on the
list box ("WRITE" / "RO") AND a redundant tag in the footer identity line ("[RW]" /
"[RO]"). The operator wants only the newer chip kept, with the duplicate removed — while
still being able to tell the mode at a glance in every browse view.

**Why this priority**: The duplication is the user's first and most explicit complaint
("зачем дублируется … оставь только новый"). It is visual noise on the most-used surface,
and the read/write state is a safety-relevant signal — showing it twice invites the two
copies to disagree as the code evolves. Highest value, lowest risk.

**Independent Test**: Open the browser in a read-only context and in an armed-write
context; confirm the mode is shown once (the chip), the old footer tag is gone, and the
mode is still unmistakable in every browse mode (bucket list, object level, opened
object). Fully testable on its own with no dependency on the other stories.

**Acceptance Scenarios**:

1. **Given** the browser is open on a read-only context, **When** the operator views any
   browse screen, **Then** the read/write mode is shown in exactly one location and never
   appears a second time on the same screen.
2. **Given** write mode is armed, **When** the operator views any browse screen, **Then**
   the single mode indicator clearly reads as the armed/write state (loud, distinct from
   read-only) and there is no second mode label anywhere on screen.
3. **Given** the operator switches between the bucket list, an object level, and an opened
   object, **When** each screen renders, **Then** the read/write mode remains visible on
   every one of them (the mode signal is never lost as a side effect of removing the
   duplicate).
4. **Given** a destructive action's confirmation prompt is open (arm-write, delete,
   overwrite), **When** that prompt is shown, **Then** its own loud write indicator still
   appears (this safety-critical, modal surface is intentionally exempt from the
   single-indicator rule and is not considered a duplicate of the steady-state chip).

---

### User Story 2 - Applied filter state stays visible (Priority: P1)

After the operator applies a filter and the filter input closes, the active filter term
remains visible as a persistent indicator, so the operator always knows the current list
is filtered and by what — without re-opening the input.

**Why this priority**: Today, once a committed filter's input closes, nothing on screen
states the active term; the list silently shows a filtered subset. The operator can be
misled into thinking the level is empty or the listing is complete. The user explicitly
asked to "see the state of the current filter if applied". This is a correctness/clarity
gap, not mere polish — P1.

**Independent Test**: Apply a filter on the bucket list and on an object level; close the
input; confirm a persistent indicator shows the active term and the filtered scope, and
that clearing the filter removes the indicator. Testable independently of the other
stories.

**Acceptance Scenarios**:

1. **Given** no filter is applied, **When** the operator views a list, **Then** no
   applied-filter indicator is shown.
2. **Given** the operator commits a filter term and the input closes, **When** the list
   renders, **Then** a persistent indicator shows the active filter term and is visually
   distinct from the transient filter-input line used while typing.
3. **Given** an applied filter is shown, **When** the operator clears the filter, **Then**
   the indicator disappears and the full list returns.
4. **Given** a filter is applied on the object level vs. the bucket list, **When** the
   indicator renders, **Then** it makes clear which scope the filter applies to (current
   level vs. bucket list).
5. **Given** a long filter term, **When** the indicator renders on a narrow terminal,
   **Then** the term is shown truncated/revealable and the footer still does not wrap off
   or scroll any line out of view.

---

### User Story 3 - Breathing room in the footer / command bar (Priority: P2)

The operator reads the footer and command menu comfortably because elements are separated
by clear visual gaps, instead of being crammed together.

**Why this priority**: The user finds the current spacing too tight ("слишком
маленькие отступы … все слишком слеплено"). It is a legibility complaint but purely
presentational — no functional gap — so it ranks below the two correctness/clarity
stories. Still valuable for the daily-driver surface.

**Independent Test**: Render the footer/command bar at narrow, mid, and wide widths;
confirm adjacent elements (key+label entries, separators, info/read/write blocks) have
visibly larger gaps than before, no two elements visually merge, and the footer still
never wraps a line off-screen or pushes the box content out of its budget.

**Acceptance Scenarios**:

1. **Given** the command bar renders as side-by-side blocks (wide terminal), **When** the
   operator views it, **Then** the gaps between the info, read, and write blocks and
   between adjacent entries are visibly larger than the current spacing and no two
   adjacent elements appear joined.
2. **Given** the command bar collapses to the compact single-column rows (narrow
   terminal), **When** the operator views it, **Then** entries are still separated by
   clear gaps and remain individually readable.
3. **Given** any terminal width, **When** spacing is increased, **Then** the footer
   (including the hints/command line) is never scrolled off-screen and no footer line is
   dropped or clipped mid-element (the existing legibility invariant holds).

---

### Edge Cases

- **Mode signal in modal surfaces**: arm-write / delete / overwrite confirmations carry
  their own loud write badge by design (safety redundancy). Removing the footer duplicate
  must not strip those modal badges.
- **Mode signal in the opened-object (non-list browse) view**: the border chip currently lives on the list
  box only; removing the footer tag must not leave the opened-object screen without a read/write indicator —
  it receives the universal chip.
- **Mode signal in overlay / menu / help surfaces**: the context-switcher, usage/analytics, connections
  manager, add-connection / add-bucket forms, and the upload file-browser show NO mode chip after the footer
  tag is removed (transient surfaces; write-state is shown again on return to a browse box). The help overlay
  retains its own write badge. This is intentional, not a regression.
- **Filter indicator vs. typing input**: while the filter input is open the transient
  "filter …: <input>" line already shows; the new persistent indicator must not double up
  with it (show one or the other, not both for the same filter at the same instant).
- **Empty filter result**: when an applied filter matches nothing, the indicator must
  still show the active term so the operator understands why the list is empty.
- **Increased spacing at the narrowest tier**: widening gaps must not be what finally
  pushes a footer line to wrap/drop; the no-scroll invariant wins over the larger gap.
- **NO_COLOR / monochrome terminals**: the single mode indicator and the filter indicator
  must remain distinguishable without relying on color alone.

## Requirements *(mandatory)*

### Functional Requirements

#### Single read/write mode indicator (US1)

- **FR-001**: The system MUST display the current read/write mode in exactly one location
  per browse screen; the redundant secondary mode label MUST be removed.
- **FR-002**: The retained mode indicator MUST be the newer border-mounted chip
  (the one introduced in the previous iteration), not the older footer/identity tag. The
  older footer/identity mode tag MUST be removed everywhere it appears (this refers to the
  footer/identity row's tag; the modal and help-overlay write badges of FR-005 are a separate
  surface and are retained).
- **FR-003**: The read/write mode MUST remain visible in every browse mode (bucket list,
  object level, opened object) after the duplicate is removed; the mode signal MUST NOT be
  lost in any mode as a side effect. This is achieved by mounting the SAME border chip on
  every browse screen's box top border (a single render path/idiom), not by a per-mode
  fallback indicator.
- **FR-004**: The armed/write state MUST be visually loud and unmistakably distinct from
  the read-only state in the single retained indicator.
- **FR-005**: Modal destructive-action surfaces (arm-write, delete, overwrite
  confirmations) MUST continue to show their own write indicator; these are exempt from
  the single-indicator rule and MUST NOT be treated as duplicates to remove.
- **FR-006**: The mode indicator MUST convey its state without relying on color alone
  (NO_COLOR-safe).

#### Applied-filter state visibility (US2)

- **FR-007**: When a filter term is applied (committed) and the filter input is closed, the
  system MUST show a persistent indicator of the active filter term as a border-mounted chip
  on the filtered pane's box top border (NOT in the footer/command bar, NOT appended to the
  breadcrumb title).
- **FR-008**: The applied-filter indicator MUST be visually distinct from the transient
  filter-input line shown while the operator is typing a filter.
- **FR-009**: The applied-filter indicator MUST convey the scope the filter applies to
  (current object level vs. bucket list) — its placement on the filtered pane's own box
  border is the primary scope cue.
- **FR-010**: When no filter is applied, the system MUST NOT show any applied-filter
  indicator.
- **FR-011**: Clearing the active filter MUST remove the applied-filter indicator and
  restore the unfiltered list.
- **FR-012**: A long filter term in the indicator MUST be truncated or revealable so it
  never causes a footer line to wrap off-screen or scroll out of view.
- **FR-013**: The system MUST NOT simultaneously show both the transient input line and the
  persistent indicator for the same filter at the same moment.

#### Footer / command-bar spacing (US3)

- **FR-014**: The system MUST increase the visual spacing between adjacent footer/command-
  bar elements (entry separators, inter-block gaps, and key↔label grouping) so adjacent
  elements no longer appear merged.
- **FR-015**: Increased spacing MUST be applied consistently across the wide (side-by-side
  blocks) and narrow (collapsed rows) command-bar layouts.
- **FR-016**: Increased spacing MUST NOT cause any footer line (info, hints/command,
  status) to wrap off-screen, drop, or push the box body past its height budget at any
  terminal width tier.
- **FR-017**: Spacing changes MUST reuse the existing visual/design-system conventions
  (shared separators and palette roles); no new color/hue may be introduced solely for
  spacing.

#### Cross-cutting

- **FR-018**: All three changes MUST preserve the existing read-only browsing behavior and
  MUST NOT introduce any new storage write capability.

### Key Entities

- **Read/Write mode indicator**: the single on-screen signal of the current capability
  state (read-only vs. armed write). Attributes: state (read-only / armed), location
  (border-mounted chip), loudness/contrast.
- **Applied-filter indicator**: a persistent on-screen element shown when a filter is
  committed. Attributes: active term, scope (object level vs. bucket list), present/absent.
- **Footer / command-bar element**: a unit in the status footer or command menu (an
  info field, a read/write entry, a separator, or a block). Attribute relevant here: the
  gap separating it from its neighbors.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In every browse screen and in both read-only and armed-write states, the
  read/write mode appears exactly once (0 screens show it twice).
- **SC-002**: The read/write mode is visible in 100% of browse modes (bucket list, object
  level, opened object) after the change.
- **SC-003**: In 100% of states where a filter is committed and the input is closed, the
  active filter term is visible on screen without the operator re-opening the input.
- **SC-004**: When a filter is cleared, the applied-filter indicator is absent in 100% of
  cases and the full list is restored.
- **SC-005**: Across narrow, mid, and wide terminal widths, no footer line is wrapped
  off-screen, dropped, or clipped mid-element after spacing is increased (the legibility
  invariant holds at every tier).
- **SC-006**: Adjacent footer/command-bar elements are separated by a visibly larger gap
  than before in 100% of layouts (wide and collapsed), with no two adjacent elements
  visually merged.
- **SC-007**: No new storage write capability is added; the read-only structural guard
  remains green.
- **SC-008**: The mode indicator and the applied-filter indicator each remain
  distinguishable with color disabled (NO_COLOR), conveying state via text/glyph.

## Assumptions

- The "new" mode label the user wants kept is the border-mounted chip introduced in the
  previous UI iteration (012); the "old" duplicate to remove is the read/write tag carried
  in the footer identity line. This matches the only read/write-mode duplication currently
  rendered in steady-state browse views.
- Modal confirmation surfaces (arm-write / delete / overwrite) keep their own loud write
  badge as a deliberate safety redundancy and are out of scope for the "remove the
  duplicate" change.
- The applied-filter indicator reuses the established chip/breadcrumb + palette-role
  conventions rather than introducing a new visual idiom or color.
- "Increase spacing" means widening existing gaps (separators, inter-block, key↔label),
  not redesigning the footer/command-bar layout; the three-block structure stays.
- The exact gap sizes are an implementation/design-system detail chosen to satisfy
  "visibly larger, not merged" while keeping the no-wrap/no-scroll invariant; they are not
  fixed by this spec.
- This is a presentation/UX-only iteration: no storage-contract change, no new write path,
  and the existing read-only posture is preserved.
