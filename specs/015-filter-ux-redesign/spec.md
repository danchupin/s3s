# Feature Specification: Filter UX Redesign (always-visible, ergonomic)

**Feature Branch**: `015-filter-ux-redesign`

**Created**: 2026-06-08

**Status**: Draft

**Input**: User description: "Improve the filter UX. Research the top popular TUIs with good filtering and adopt their patterns. Keep BOTH the bucket filter and the object filter. The filter must always be conveniently visible — today it does not always fit on screen."

## Overview

s3s lets a user narrow the current view with `/`: on the bucket pane an instant local
name filter, on the object pane a debounced server-side prefix search (two scopes that
both stay). Today the filter input appears as a single line in the multi-line status
footer (it temporarily replaces other status), and a committed filter shows as a small
`filter: term` chip on the focused pane's border (feature 013).

Two problems motivate this feature:

1. **Not always visible.** The active filter is only chipped on the *focused* pane, and
   the input shares the footer with other status, so the filter state is easy to lose.
2. **Doesn't fit.** On narrow or short terminals the filter input line plus the footer's
   info + hints lines overflow the height/width budget and get cut off — the user's core
   complaint ("не всегда влезает в экран").

A research pass over five leading TUIs (recorded in **Research provenance** below)
converged on a clear pattern, which this feature adopts:

- **Decouple the input from the indicator, and attach the indicator to the pane it
  scopes.** The persistent "what is filtered" badge rides the pane's own border (costs
  zero body rows) — k9s, ranger, broot, and fzf all do a version of this; yazi's lack of
  a persistent indicator is exactly s3s's reported pain.
- **Reserve the filter chrome and a live match count; sacrifice list rows, never the
  footer, under space pressure** (the fzf model: a query line + an `X/Y` info line are
  always present; the *list* gives up rows).
- **Always-visible, per-pane, term-gated indicators** so both the bucket filter and the
  object filter are readable at once, regardless of which pane has focus.

## Clarifications

### Session 2026-06-08

- Q: Filter input model — always on screen, or only while typing? → A: An **always-visible input strip** (fzf/broot-style) — the filter field is a persistent part of the screen, not a transient line that appears only during editing. It is treated as reserved chrome: the browse LIST absorbs the cost (one line), the footer is never sacrificed.
- Q: Object-scope match count, given the level total is often unknown (paginated server-side search)? → A: Show **"N matched"** for the object scope (matched count only, no total); the bucket scope (local) shows **matched / total**. No extra network requests.
- Q: Where is the match count shown? → A: **On the per-pane indicator chip** (e.g. `filter: term · 12/40`) — baked into the border chip so it costs zero body rows and is always visible.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The active filter is always visible (Priority: P1)

A user who has applied a filter can always see, at a glance, that a filter is active, what
the term is, and which scope it applies to — without it depending on which pane is focused
and without it ever being scrolled or cut off.

**Why this priority**: This is the heart of the request ("always conveniently visible").
Today the indicator is focus-relative and easy to lose; making it persistent and
per-scope is the core value.

**Independent Test**: Apply a bucket filter, then move focus to the object pane — confirm
the bucket filter indicator stays visible; apply an object filter too — confirm both
indicators are visible simultaneously, each naming its scope and term.

**Acceptance Scenarios**:

1. **Given** an applied bucket filter, **When** focus moves to the object pane, **Then** the bucket filter indicator remains visible (it does not disappear with focus).
2. **Given** both a bucket filter and an object filter are applied, **When** the user looks at the screen, **Then** both indicators are visible at once, each clearly labeled with its scope and term.
3. **Given** any applied filter, **When** the terminal is resized to any supported size, **Then** the filter indicator remains visible (its term may be elided with an ellipsis, but the indicator and its scope never vanish).

---

### User Story 2 - The filter and footer always fit on screen (Priority: P1)

On any terminal width or height, the filter input (while typing) and the persistent status
footer / command-hint bar both remain fully visible and uncut; the browse list is what
gives up space when the screen is small.

**Why this priority**: The reported defect — the filter "doesn't always fit." Per
Constitution VI the footer must never scroll off; this story makes the filter obey the
same invariant by sacrificing list rows instead of footer/filter rows.

**Independent Test**: Shrink the terminal to the minimum supported width and height with a
filter active — confirm the always-visible input strip, the active-filter indicator, and the
footer hints are all still readable; only the list shows fewer rows.

**Acceptance Scenarios**:

1. **Given** the always-visible filter input strip, **When** the terminal is at its narrowest supported width, **Then** the input field, its scope label, and the footer hints are all visible (no horizontal clipping of the editable field to nothing).
2. **Given** a filter is active, **When** the terminal is at its shortest supported height, **Then** the footer and the filter indicator are fully visible and only the list body is reduced in height.
3. **Given** a very long filter term, **When** it is displayed, **Then** it is elided to fit and never widens the indicator/input past the box edge or pushes other chrome off-screen.

---

### User Story 3 - Both filter scopes are preserved (Priority: P1)

The user keeps both kinds of filtering: an instant local filter of bucket names, and a
filter/search of objects within the current prefix. Neither is removed or merged.

**Why this priority**: An explicit constraint ("important to keep the bucket filter and
the object filter"). The redesign is presentation; it must not drop a capability.

**Independent Test**: Filter buckets and confirm the bucket list narrows locally; switch
to objects, filter, and confirm the object view narrows — both still work after the
redesign.

**Acceptance Scenarios**:

1. **Given** the bucket pane, **When** the user filters, **Then** the bucket list narrows by name instantly (no network wait).
2. **Given** the object pane, **When** the user filters, **Then** the object view narrows by the term within the current prefix.
3. **Given** a filter applied to one scope, **When** the user applies a filter to the other scope, **Then** the two filters are independent (one does not clear or overwrite the other).

---

### User Story 4 - Live narrowing with a match count (Priority: P2)

As the user types, the view narrows live and a count of matches is shown, so the user sees
how effective the filter is without committing or guessing.

**Why this priority**: Live feedback + a match tally is the ergonomic win shared by fzf,
broot, k9s, ranger, and yazi. Valuable but secondary to visibility/fit (P1).

**Independent Test**: Type into the bucket filter and watch the list narrow and the match
count update per keystroke; type into the object filter and confirm it narrows without
freezing the UI.

**Acceptance Scenarios**:

1. **Given** the bucket filter input is open, **When** the user types, **Then** the bucket list narrows on each keystroke and a match count is shown.
2. **Given** the object filter input is open, **When** the user types, **Then** the object view narrows responsively (debounced) and the UI never blocks while a search is in flight.
3. **Given** an active filter, **When** the match count is displayed, **Then** the bucket scope shows matched **/ total** (computed locally), and the object scope shows **"N matched"** (matched count only — the level total is unknown under server-side pagination and is NOT fetched).

---

### User Story 5 - Refine and clear are obvious (Priority: P2)

A user can re-open the filter to refine it (pre-filled with the current term), cancel an
edit to revert to the last committed term, and clear the filter to return to the full view
— with the indicator disappearing exactly when the filter is cleared.

**Why this priority**: Completes the loop; the lifecycle (set → refine → clear) must read
cleanly given the new always-visible indicator.

**Independent Test**: Apply a filter, re-open it (confirm pre-filled), edit and press
cancel (confirm revert), then clear (confirm the indicator disappears and the full view
returns).

**Acceptance Scenarios**:

1. **Given** a committed filter, **When** the user re-opens the input, **Then** it is pre-filled with the current term for refinement.
2. **Given** an in-progress edit, **When** the user cancels, **Then** the view reverts to the last committed term (the edit is discarded).
3. **Given** a committed filter, **When** the user clears it, **Then** the indicator disappears and the full (unfiltered) view returns.
4. **Given** navigation away from a filtered level (e.g. going up, switching context), **Then** the filter clears automatically and its indicator disappears (existing behavior preserved).

---

### Edge Cases

- A filter that matches nothing → a clear "no matches for <term>" empty state, with the indicator still visible so the user knows a filter is the reason.
- A filter term longer than the indicator/input width → elided with an ellipsis; the full term is recoverable by re-opening the input (pre-filled).
- Both panes filtered AND the terminal at minimum size → both indicators plus the footer remain visible; only list rows are sacrificed.
- The object search is in flight when the user keeps typing → bursts coalesce; a stale result never replaces a newer one; the UI does not freeze.
- `NO_COLOR` / monochrome terminal → the active-filter indicator is distinguishable by text (a label), not by color alone (Constitution VI).

## Requirements *(mandatory)*

### Functional Requirements

#### Visibility & indicator

- **FR-001**: The active filter for each scope MUST be shown as a persistent, always-visible indicator attached to its pane (not dependent on focus), readable at every supported terminal size.
- **FR-002**: When both a bucket filter and an object filter are active, BOTH indicators MUST be visible at the same time, each labeled with its scope and term.
- **FR-003**: The active-filter indicator MUST be distinguishable without color (it MUST include text, e.g. a "filter" label and the term), per Constitution VI.
- **FR-004**: A displayed filter term that exceeds the available width MUST be elided (ellipsis), never widening its container or pushing other UI off-screen; the full term MUST be recoverable by re-opening the input pre-filled.

#### Layout & fit (the core defect)

- **FR-005**: At every supported terminal width and height, the persistent status footer / command-hint bar MUST remain fully visible (never scrolled or clipped) — the existing Constitution VI invariant, now also covering the filter chrome.
- **FR-006**: The filter input MUST be presented as a persistent, always-visible strip (not a transient line that appears only while typing). The editable input field, its scope label, and the footer hints MUST all remain visible at every supported size; under width pressure the input field MUST retain a usable minimum rather than being truncated away. The always-visible strip counts as reserved chrome (FR-007): the LIST absorbs its one line, the footer is never sacrificed.
- **FR-007**: Under height pressure, the browse LIST body MUST be the element that loses rows; the footer, the filter input (when open), and the active-filter indicators MUST NOT be the elements sacrificed.

#### Scopes preserved

- **FR-008**: The system MUST preserve an instant, local (no-network) filter of bucket names.
- **FR-009**: The system MUST preserve a filter/search of objects within the current prefix.
- **FR-010**: The two scopes MUST be independent: applying or clearing one MUST NOT alter the other.

#### Liveness & feedback

- **FR-011**: Typing in either filter MUST narrow the view live (the bucket scope instantly; the object scope debounced and non-blocking so the UI never freezes).
- **FR-012**: While a filter is active or being typed, the system MUST show a match count **on that scope's per-pane indicator chip** (e.g. `filter: term · 12/40`): the bucket scope shows matched / total (local), the object scope shows the matched count only (no level total is fetched).
- **FR-013**: In-flight object searches MUST coalesce keystroke bursts and MUST NOT let a stale result replace a newer one.

#### Lifecycle

- **FR-014**: Re-opening the filter MUST pre-fill the current committed term for refinement.
- **FR-015**: Canceling an in-progress edit MUST revert to the last committed term.
- **FR-016**: Clearing a filter MUST remove its indicator and restore the full view; navigating away from a filtered level MUST clear that level's filter automatically.

### Key Entities

- **Filter state (per scope)**: the committed term and the in-progress input for the bucket scope and the object scope, each independent, each with an always-visible indicator.
- **Match tally**: the count of items matching the active term in a scope (bucket: matched and total; object: matched, with total possibly unknown).
- **Layout budget**: the rule that the footer, filter input, and indicators are reserved first and the list body absorbs space loss.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With a filter applied, the active filter (term + scope) is visible in 100% of supported terminal sizes, regardless of which pane is focused.
- **SC-002**: With both scopes filtered, both indicators are visible simultaneously in 100% of supported sizes.
- **SC-003**: At the minimum supported terminal width and height, the footer hints, the filter indicator(s), and (when open) the filter input field are all fully visible and uncut; only the list body shows fewer rows.
- **SC-004**: Both filter scopes (bucket instant-local, object within-prefix) remain functional and independent after the redesign.
- **SC-005**: Typing in a filter updates the visible match count, and the object filter never freezes the UI while a search is in flight.
- **SC-006**: A user can set, refine (pre-filled), revert (cancel), and clear a filter, with the indicator appearing/disappearing exactly in step with the committed filter.
- **SC-007**: The active-filter indicator remains identifiable under `NO_COLOR` (text, not color alone).

## Assumptions

- "Always visible" means BOTH the persistent per-scope indicator chip (which rides the pane border at zero body-row cost and carries term + count) AND the editable filter input strip are always on screen; the input strip is reserved chrome that costs one line, absorbed by the list (never the footer). This is the fzf/broot always-visible model the user selected in clarification.
- Supported terminal sizes are the existing minimum the app already targets; this feature does not introduce a new minimum, it makes the filter obey the existing footer-visibility invariant.
- The bucket scope can compute matched/total locally; the object scope is server-side and paginated, so its total may be unknown (see US4 clarification).
- This is a presentation/UX iteration: it does not change what the filters match (substring/prefix semantics) beyond what the clarification may decide, and it does not change the storage contract.

## Out of Scope

- Fuzzy matching, regex, or inverse filters (k9s-style sigils) — a possible later enhancement; this feature keeps the current matching semantics.
- Filtering by object metadata (size, date, content-type) — out of scope.
- Persisting filters across app restarts.
- Any change to the storage layer or the read-only guarantee.

## Research provenance

Top-5 TUIs surveyed for filter UX (adversarially verified against primary sources):

- **fzf** — always-reserved query line + live `X/Y` match counter; under pressure the *list* loses rows, never the chrome. The gold-standard always-visible filter.
- **broot** — a permanently-present single input line at the bottom of the focused panel; live tree narrowing; non-blocking.
- **k9s** — transient input prompt decoupled from a persistent committed-filter badge rendered in the resource's own title border; term capped/elided.
- **ranger / lf** — persistent, labeled active-filter token in the bottom status bar; collapsing ruler so the token costs nothing when absent.
- **yazi** — split keys for filter vs find, live narrowing, but NO persistent applied-filter indicator — the exact gap that is s3s's reported problem, confirming the value of the pane-attached badge.
