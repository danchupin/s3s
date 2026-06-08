# Implementation Plan: Filter UX Redesign (always-visible, ergonomic)

**Branch**: `015-filter-ux-redesign` | **Date**: 2026-06-08 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/015-filter-ux-redesign/spec.md`

## Summary

Turn the filter input from a transient footer line into an **always-visible strip** (one
reserved row between the body and the footer, present in the filterable browse modes), and
turn the per-pane applied-filter chip into a **term-gated, count-bearing** indicator shown
for each scope independently. Both the bucket-name filter and the object filter are kept and
both chips are visible at once. The footer never scrolls off; the **list body** absorbs the
strip's row (Constitution VI). Match count rides the chip: bucket = `matched/total` (local),
object = `N matched` (no level total fetched). Live narrowing and the `/`-open / Enter-commit
/ Esc-revert lifecycle are unchanged. Patterns adopted from fzf, broot, k9s, ranger/lf, yazi
(see spec Research provenance).

## Technical Context

**Language/Version**: Go 1.25.

**Primary Dependencies**: `charm.land/bubbletea/v2` + `lipgloss/v2` (TUI). No new dependency.

**Storage**: none touched. This is a pure presentation/UX change in `internal/ui`.

**Testing**: `go test` white-box UI tests (`package ui`); `deliver`/`press`/`viewOf`/
`stripANSI` helpers; `assertWidthSweep` for the footer/strip fit invariant.

**Target Platform**: all terminals the app already supports; no new minimum.

**Project Type**: single-binary CLI/TUI.

**Performance Goals**: render-only; bucket count is `len(filteredBuckets)` (already computed),
object count is `m.level.count()` (already loaded) — no new network calls (FR-013 preserved).

**Constraints**: footer/command-hint bar never scrolls off at any width/height (Constitution
VI); the always-visible strip is reserved chrome — the list shrinks, never the footer; long
terms elide; the active filter is identifiable under `NO_COLOR` (text, not color alone).

**Scale/Scope**: ~4 files edited in `internal/ui` (app.go, styles.go, search.go, plus tests).
No new package; one new render function (`filterStripView`) and count-aware chip helpers.

## Constitution Check

*GATE: re-checked after design. Constitution v1.2.0.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Core/UI Separation | ✅ PASS | All changes in `internal/ui`; no storage/SDK touch. |
| II. Non-Blocking TUI | ✅ PASS | Strip is render-only; the object search stays debounced off-loop (`scheduleSearch`/`onSearchFire`), bursts coalesce (FR-013). |
| III. Test-First | ✅ PASS (required) | TDD: failing tests first — strip always present at all sizes, both chips with counts, list shrinks not footer, width-sweep extended. |
| IV. Integration Testing | ✅ N/A justified | No storage-contract change; no credential/auth/pagination behavior change. |
| V. Observability & Safe Ops | ✅ N/A | No logging/secret/destructive-op change. |
| VI. UI Legibility | ✅ PASS (the driver) | Footer never scrolls (strip reserved, list absorbs the row); both scopes' filters always identifiable; long terms elide + recoverable by re-opening pre-filled; chips text-distinguishable under `NO_COLOR`. |
| VII. UI Consistency & Design System | ✅ PASS | Reuse `warnStyle`/`accentStyle` + the existing chip pattern; no new hue; one strip render path; `· ` count separator matches the footer's segment style. |

**No amendment.** `check-readonly` stays green (no S3 symbol added). No new keymap field, no new hue, no new package.

No violations → **Complexity Tracking empty.**

## Project Structure

### Documentation (this feature)

```text
specs/015-filter-ux-redesign/
├── plan.md, spec.md, research.md, data-model.md, quickstart.md
├── contracts/ (filter-strip, applied-filter-chip-count, layout-budget, dual-scope-visibility)
└── checklists/requirements.md   (16/16)
```

### Source Code (repository root)

```text
internal/ui/
├── app.go        # View()(1124) height budget: reserve filterStripH=1 in filterable modes
│                 #   (modeBuckets/modeTree) → rows := height - footerH - filterStripH - 2; render the
│                 #   strip between body and footer (1209). NEW filterStripView(w) (always-visible:
│                 #   active = "▌ filter <pane>: <input>"; idle = dim "/ filter <pane>" + committed term).
│                 #   statusLine()(1431): DELETE the searching case (1450-1459) — input now owned by the strip.
│                 #   filterChipText(1309): add (matched,total,hasTotal) → "filter: term · M/T" | "term · N";
│                 #   bucketFilterChip(1317)/objectsFilterChip(1320) feed counts (filteredBuckets/buckets;
│                 #   level matched); make chips TERM-GATED + zone-agnostic (both visible at once), the
│                 #   actively-edited scope's chip hidden while its term is live in the strip.
├── styles.go     # boxViewWith(335)/buildRight(355): chip degrade order unchanged (center→filter→mode,
│                 #   mode survives); filter chip drops WHOLE under width pressure (strip still shows it);
│                 #   filterChipTermMax(1304) budget accounts for the " · M/T" suffix (elide term first).
├── search.go     # per-scope match-count accessors (bucketMatchCount/Total, objectMatchCount); the
│                 #   /-open / Enter-commit / Esc-revert lifecycle (startSearch/onSearchKey/runSearch) is KEPT.
└── tree.go/commandbar.go  # NO CHANGES (windowBounds adapts to fewer rows; command bar unchanged).
```

**Structure Decision**: existing package layout. `filterStripView` lives in `app.go` next to
`statusLine`/`footerBlock`. No new file or package.

## Key design decisions (detail in research.md)

1. **Strip as conditional reserved chrome.** Reserve exactly one row for the strip ONLY in the
   filterable browse modes (`modeBuckets`, `modeTree`); other modes (object/usage/connections/
   forms) keep their full body. Compute the flag before the `rows` budget (app.go ~1142) so the
   list shrinks by one row; `windowBounds`/`treeView` adapt automatically (FR-007).
2. **Strip is between body and footer**, not inside either — so the footer invariant (VI) is
   untouched and the strip rides above it (app.go:1209 render).
3. **Idle vs active appearance.** `m.searching` true → active input with caret + live hints;
   false → a dim strip showing the focused scope's committed term (or a `/ to filter <pane>`
   placeholder). One shared strip bound to the focused pane's scope.
4. **Count on the chip, term-gated, dual.** `filterChipText` gains `(matched, total, hasTotal)`;
   bucket passes `len(filteredBuckets), len(buckets), true`, object passes `level matched, 0,
   false`. Chips render per scope independent of focus; a scope's chip hides only while THAT
   scope is being actively edited (its live term is in the strip), so both committed chips show
   at once (FR-001/002).
5. **Degradation unchanged, count elides with the term.** Under width pressure the filter chip
   drops whole (mode chip always survives); the strip still shows the active filter, and a long
   term elides — recoverable by re-opening pre-filled (FR-004).

## Phase 0: Research → `research.md`

Resolved: strip placement + 1-row conditional reserve; idle-strip appearance; chip count format
+ separator; term-gating that keeps both chips visible; degradation/elision under width; object
count source (`m.level.count()`); NO_COLOR distinguishability.

## Phase 1: Design & Contracts

- `data-model.md`: per-scope filter state, the match tally, the layout budget rule.
- `contracts/`: the filter strip, the count-bearing chip, the layout budget, dual-scope visibility.
- `quickstart.md`: how the redesigned filter behaves.
- Agent context: update the `<!-- SPECKIT -->` block in `CLAUDE.md` to point at this plan.

## Complexity Tracking

*No constitution violations — section intentionally empty.*
