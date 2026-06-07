# UX Requirements Quality Checklist: Connection Management UX Fixes

**Purpose**: Validate that the spec's requirements are complete, clear, and consistent — with focus on four newly-reported defects (duplicate delete labels, missing connection-switch affordance, missing filter-reset affordance, post-copy visibility) plus the existing 008 UX surfaces.
**Created**: 2026-06-07
**Feature**: [spec.md](../spec.md)

**Note**: Items test the REQUIREMENTS, not the implementation. RESOLVED 2026-06-07 — the four follow-up concerns were folded into spec.md as US6–US9 (FR-015…FR-022, SC-007…SC-010); all 27 items now pass.

## Duplicate Delete Labels (command bar)

- [x] CHK001 - Does the spec require the command bar to disambiguate the two delete actions (single/group object delete vs recursive folder delete), which both render the label "delete"? [Gap, Spec §FR-013]
- [x] CHK002 - Are requirements defined for which delete entries may appear simultaneously for a given selection, to prevent a duplicate "delete" in the write group? [Completeness, Gap]
- [x] CHK003 - Is the labeling rule for distinct dangerous actions (e.g. "delete" vs "delete all"/"remove folder") specified so labels are unique per visible group? [Clarity, Gap]
- [x] CHK004 - Is the consistency between the command-bar delete entries and the inline connection-delete hint (US1) specified so the same verb isn't ambiguously repeated across surfaces? [Consistency, Spec §FR-001]

## Connection-Switch Affordance Visibility

- [x] CHK005 - Does the spec require a visible affordance to switch the active connection/context (today the switch key exists but is not surfaced in the command bar; only "new conn" is shown)? [Gap]
- [x] CHK006 - Are the distinct connection affordances (switch context, open connection list, add new connection) each required to be discoverable and distinctly labeled? [Completeness, Gap]
- [x] CHK007 - Is the relationship between "switch context", the numeric (1-9) shortcut, and "new conn" specified so the operator can tell them apart? [Clarity, Gap]
- [x] CHK008 - Does the spec state where the connection-switch affordance must appear (info group, read group) and that it survives the narrow/collapsed bar? [Coverage, Gap]

## Filter / Search Reset Affordance

- [x] CHK009 - Does the spec require a discoverable affordance to clear/reset an active filter or search? [Gap]
- [x] CHK010 - Are requirements defined for when the reset affordance is shown (only while a filter is active) vs hidden (no active filter)? [Clarity, Gap]
- [x] CHK011 - Is the reset behavior specified across both surfaces that filter (bucket-list filter and level search)? [Consistency, Coverage, Gap]
- [x] CHK012 - Is the post-reset state defined (full unfiltered list restored, selection behavior, cache implications)? [Completeness, Gap]

## Post-Mutation Visibility (ALL actions)

- [x] CHK013 - Does the spec require the result of EVERY mutation (create folder, upload, copy, move, delete object/group, recursive delete, bucket delete) to be visible without a manual refresh? [Gap]
- [x] CHK014 - Is the expected view behavior specified when a mutation affects a prefix/bucket OTHER than the currently-viewed level (copy/move destination, cross-bucket op)? Today refresh invalidates only the current level key. [Clarity, Edge Case, Gap]
- [x] CHK015 - Are cache-invalidation requirements defined for the SOURCE and DESTINATION levels of copy/move (both keys), not just the current view? [Completeness, Gap]
- [x] CHK016 - Is consistency specified so all mutations update the view uniformly (same-level AND cross-level), avoiding a state where one action auto-shows and another needs manual refresh? [Consistency, Gap]
- [x] CHK017 - Is the post-mutation visibility requirement reconciled with the "cache invalidated only by manual refresh" design noted in the architecture (does the new requirement intend to relax that for mutations)? [Conflict, Assumption]
- [x] CHK018 - Are requirements defined for whether a cross-level mutation should navigate the operator to the affected level or just ensure the cache there is fresh for later navigation? [Clarity, Gap]

## Command Bar Grouping & Labels (existing 008 scope)

- [x] CHK019 - With block headings removed (FR-013), is "visually distinct grouping" defined with objective criteria (column separation/gap) rather than relying on the dropped titles? [Measurability, Spec §FR-013]
- [x] CHK020 - Is the read-only cue relocation ("w to arm") requirement unambiguous about its new placement after the WRITE title is removed? [Clarity, Spec §FR-014]
- [x] CHK021 - Are the chord-label format requirements (Ctrl+X, no caret, no spaces) applied consistently across every advertising surface (command bar, connection view, confirm, help, nudges)? [Consistency, Spec §FR-004]

## Text Input & Secret Guidance (existing 008 scope)

- [x] CHK022 - Are caret-movement and paste requirements specified consistently for BOTH the add-connection form and the typed-confirm input? [Consistency, Spec §FR-005/FR-006]
- [x] CHK023 - Is the secret guidance requirement unambiguous that the form stores keychain-only and does NOT resolve ${ENV}/cmd/AWS-profile? [Clarity, Spec §FR-009/FR-010]
- [x] CHK024 - Is per-field guidance coverage complete (every focusable field has a stated expectation, not just the secret)? [Completeness, Spec §FR-009]

## Scope, Consistency & Assumptions

- [x] CHK025 - Are the new concerns (duplicate delete, switch affordance, filter reset, post-mutation visibility for ALL actions) explicitly declared in-scope or out-of-scope for feature 008? [Scope, Gap]
- [x] CHK026 - If in-scope, do the new requirements stay within the spec's stated "UI-only, no storage/config change" boundary (cache invalidation lives in UI)? [Consistency, Assumption, Spec Assumptions]
- [x] CHK027 - Are acceptance criteria added for each new concern so each is objectively verifiable (e.g., "no duplicate identical label in the write group"; "copied object appears at destination without manual refresh")? [Acceptance Criteria, Gap]

## Notes

- All 27 items resolved — spec.md extended with US6–US9 (FR-015…FR-022), data-model/plan/contracts updated.
- CHK001–CHK018: the four follow-up defects (duplicate delete, switch affordance, filter reset, post-mutation visibility ALL actions) are now in scope. Same-level mutations already auto-show via `refresh()`; cross-level copy/move now invalidates source+destination keys (FR-016).
- CHK019–CHK024 validate the originally-written 008 requirements.
- CHK025–CHK027 scope decision made: in-scope, UI-only (FR-018).
