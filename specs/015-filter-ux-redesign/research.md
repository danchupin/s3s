# Research: Filter UX Redesign

Phase 0 decisions, grounded in the current code (file:line verified) + the top-5 TUI survey
recorded in the spec's Research provenance.

## R1 — Always-visible strip as conditional reserved chrome

**Decision**: Render the filter input as a dedicated always-visible 1-row strip between the
body and the footer, reserved ONLY in the filterable browse modes (`modeBuckets`, `modeTree`).
In `View()` (app.go:1138-1142), after `footerH`, add `filterStripH` (1 in filterable modes, 0
otherwise) and compute `rows := m.height - footerH - filterStripH - 2`; render
`body + "\n" + filterStripView(w) + "\n" + footer` (extending app.go:1209).

**Rationale**: The footer height is measured and subtracted to protect the
"footer never scrolls off" invariant (app.go:1140-1141 comment). Putting the strip BETWEEN
body and footer, and subtracting its row from the same budget, makes the LIST absorb the cost
(`windowBounds`/`treeView` are stateless and adapt to fewer rows) while the footer is untouched
— exactly the fzf rule ("the list gives up rows, never the chrome"). Reserving it only in
filterable modes avoids stealing a row from object/usage/connection/form views that have no
filter.

**Alternatives rejected**: a strip inside the footer (would grow footer height → risks
scrolling it off); a persistent strip in all modes (wastes a row where filtering is impossible).

## R2 — Decouple the input from `statusLine`

**Decision**: Remove the `m.searching` filter-input case from `statusLine` (app.go:1450-1459);
`statusLine` keeps only loading / notice / error / op-prompt. The strip owns the input.

**Rationale**: Today the input is transient and competes with other status for the single footer
status line — part of why it "doesn't always fit." Decoupling (k9s's lesson: separate input
from indicator) lets the input be always-present and lets status messages coexist with an active
filter.

## R3 — Idle vs active strip appearance

**Decision**: `m.searching == true` → active strip: `▌ filter <pane>: <input>` with caret +
live hints (`(live) · Enter apply · Esc cancel`). `m.searching == false` → a dim strip: the
focused scope's committed term if any (`▌ filter <pane>: term`), else a placeholder
(`/ to filter <pane>`). One shared strip bound to the focused pane's scope (`filterIsBucketList`
already tells which scope is focused).

**Rationale**: broot/fzf keep the input line permanently; the idle state must still read as "this
is the filter, press / to use it" without a caret. Reuses `warnStyle`/`accentStyle` (no new hue,
Constitution VII).

## R4 — Match count on the chip; format and source

**Decision**: Extend `filterChipText` (app.go:1309) to `(term, matched, total, hasTotal)` →
`filter: <term> · M/T` when `hasTotal` (bucket), `filter: <term> · N` otherwise (object). Sources:
bucket `matched = len(filteredBuckets())`, `total = len(m.buckets)` (both local, instant); object
`matched = m.level.count()` (loaded dirs+objects after the prefix search), no total (paginated
server-side — clarified). Separator `· ` matches the footer segment style.

**Rationale**: The chip is the always-visible indicator (013) at zero body-row cost; baking the
count there (clarified placement) keeps the count always visible without another line. Object
total is intentionally not fetched (FR-013 — no extra requests).

## R5 — Term-gated, zone-agnostic chips (both visible at once)

**Decision**: Render each scope's chip whenever that scope has a committed term, independent of
which pane is focused — so the bucket chip (on the bucket box) and the object chip (on the
objects box) show simultaneously in the two-pane layout (`listWithPane`, app.go:1251, already
uses `boxViewFocusChip` on both boxes). A scope's chip is hidden only while THAT scope is being
actively edited (its live term is in the strip), to avoid duplicating the term.

**Rationale**: Today `filterChipText` hides while `m.searching` and the chip is selected
focus-relatively; FR-001/002 require both committed filters visible regardless of focus. The
two-pane structure already carries a chip per box — only the gating logic changes.

## R6 — Degradation and elision under width

**Decision**: Keep the existing chip degrade order in `boxViewWith`/`buildRight`
(styles.go:355-410): center label → filter chip → mode chip, with the mode chip (safety-critical)
always surviving. The filter chip drops WHOLE under width pressure (it is never clipped
mid-text); the strip still shows the active filter, so the user never loses the filter state.
`filterChipTermMax` (app.go:1304, 14) budgets for the ` · M/T` suffix by eliding the TERM first.

**Rationale**: Preserves the established layout invariant (FR-005); the redundancy (chip + strip)
means dropping the chip on a narrow box is safe (FR-004 — recoverable, the strip remains).

## R7 — NO_COLOR distinguishability

**Decision**: The strip and chip include the literal text `filter` and the term/count; they do
not rely on color to signal "a filter is active" (Constitution VI / spec SC-007). Existing
`TestChipsTextNoColor` (spec013_test.go:218) extends to assert the count text is readable
mono.

## R8 — TDD order (Constitution III)

Failing-first tests:
- **US2 (fit)**: extend `assertWidthSweep` (footer_test.go:92) and add a height-sweep so the
  always-visible strip + footer fit at every size and the LIST is what shrinks (FR-005/006/007).
- **US1 (visibility)**: `TestFilterStripAlwaysVisible` (strip present when `m.searching==false`);
  `TestBothChipsVisibleTogether` (both committed chips at once, focus-agnostic); migrate
  spec013_test `TestBucketFilterChipCommitted`/`TestObjectsFilterChipCommitted` to the always-
  visible model + assert counts.
- **US4 (count)**: `filterChipText` count rendering (bucket `M/T`, object `N`); live count while
  typing.
- **US3 (scopes)**: existing `search_test` (TestSearchNarrowsLevel, TestSearchClearRestores,
  TestSearchEnterConfirmsAndCloses) stay green; both scopes independent.
- Migrate `app_test` `TestStatusSearchPending` to the new `filterStripView` (input no longer in
  `statusLine`); add `TestStatusLineNeverHasFilterInput`.
