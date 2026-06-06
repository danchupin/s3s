# UX Requirements Quality Checklist: UI Redesign

**Purpose**: Validate that the requirements for visual emphasis / color-coding (per
the Claude Code palette) and for on-screen data completeness are themselves
complete, clear, consistent, and measurable — *before* implementation. This is a
unit-test suite for the requirements, not for the rendered UI.
**Created**: 2026-06-06
**Feature**: [spec.md](../spec.md) · focus from request: "визуально помечать важные
элементы и выделять разным цветом согласно цветовой схеме claude code" + "отобразить
все доступные данные и элементы на экране"

## Color Scheme — Definition & Source of Truth

- [x] CHK001 Is the "Claude Code color scheme" named and defined as a concrete palette (exact hues/roles), rather than referenced informally? [Clarity, Gap, Spec §Background]
- [x] CHK002 Are the per-role colors enumerated (e.g. context, cluster, user, endpoint, region, size, modified, error, success, badge) with a single authoritative mapping? [Completeness, Spec §FR-014]
- [x] CHK003 Is the existing "one hue per parameter" footer rule extended to the new surfaces (hint bar, details pane, command bar, connection form) with an explicit color mapping? [Completeness, Gap, Spec §FR-003/§FR-014]
- [x] CHK004 Is a fallback color behavior specified for terminals without truecolor / limited palettes? [Edge Case, Gap]
- [x] CHK005 Is the assumption that the host terminal supports the required colors documented and validated? [Assumption, Gap]

## Important-Element Emphasis — Clarity & Measurability

- [x] CHK006 Is "important element" defined with explicit criteria (which elements qualify: selected row, write badge, errors, the active context, required form fields)? [Clarity, Gap]
- [x] CHK007 Are the emphasis mechanisms specified per important element (color, bold, inverse, marker glyph) rather than left as "highlight"? [Clarity, Completeness]
- [x] CHK008 Are "loud", "high-contrast", and "prominent" (write badge, FR-014/FR-027) quantified with measurable visual properties (e.g. contrast ratio, fixed position, min size)? [Measurability, Ambiguity, Spec §FR-014]
- [x] CHK009 Is the visual hierarchy defined when multiple important elements compete simultaneously (e.g. error + write badge + multi-select count)? [Clarity, Gap]
- [x] CHK010 Are the selected-row and marked-row (multi-select) emphasis requirements distinct and unambiguous? [Clarity, Spec §FR-006]
- [x] CHK011 Is the disabled/unavailable write-action styling defined (greyed vs hidden) with a measurable cue, not just "clearly disable"? [Ambiguity, Spec §FR-004]

## Color Semantics — Consistency & Conflicts

- [x] CHK012 Are color meanings consistent across all screens and overlays (success=green, error=red, write=badge color) with no role reused for two meanings? [Consistency, Conflict, Spec §FR-018]
- [x] CHK013 Do the read-only vs write-armed color cues align between the badge, the hint bar, and disabled actions without contradiction? [Consistency, Spec §FR-004/§FR-014]
- [x] CHK014 Is the badge color/contrast requirement identical on the main view and on alt-screen overlays (help, command bar, connection form)? [Consistency, Spec §FR-013/§FR-027]
- [x] CHK015 Are color requirements for the details pane consistent with the list's existing column colors (e.g. size, modified)? [Consistency, Spec §FR-010]

## Accessibility of Color

- [x] CHK016 Is a non-color redundant cue required wherever meaning is conveyed by color (so color-blind users and monochrome terminals are not excluded)? [Coverage, Gap]
- [x] CHK017 Are minimum contrast requirements stated for text-on-background, especially the badge and error/success lines? [Measurability, Gap]
- [x] CHK018 Is behavior specified under `NO_COLOR` / a no-color preference if such a mode is in scope? [Coverage, Gap, Assumption]

## On-Screen Data Completeness — Per Surface

- [x] CHK019 Is the full set of data each screen must show enumerated (bucket list, tree level, details pane, footer, hint bar, badge, status line), with nothing left implicit? [Completeness, Gap, Spec §FR-008/§FR-014]
- [x] CHK020 Are the exact details-pane fields for an object fully listed (size, content-type, last-modified, ETag, storage class — and any others like key/URI/version)? [Completeness, Spec §FR-010]
- [x] CHK021 Are the folder/level-selection pane contents fully specified (which summary fields), not just "summary information"? [Clarity, Spec §FR-011]
- [x] CHK022 Does the hint bar requirement specify that ALL currently-valid actions are shown (not a truncated subset) and the overflow/wrap rule when they don't fit? [Completeness, Spec §FR-003/§FR-013]
- [x] CHK023 Are the footer/identity data items (context, cluster, user, endpoint, region, version) required to remain visible after the layout split? [Completeness, Spec §FR-014]
- [x] CHK024 Are the connection-form fields and their inline validation/error data fully enumerated as on-screen elements? [Completeness, Spec §FR-021]
- [x] CHK025 Is the command-bar's discoverability data specified (the set of available commands shown as the user types)? [Completeness, Spec §FR-017]

## On-Screen Data — Truncation, Overflow & Reflow

- [x] CHK026 Are requirements defined for how long values (long keys, ETags, endpoints) are presented when they exceed pane/line width (truncate vs wrap vs scroll)? [Edge Case, Gap, Spec §FR-010]
- [x] CHK027 Is the data-priority order specified for what stays vs collapses when the pane stacks/hides on narrow terminals? [Clarity, Spec §FR-013]
- [x] CHK028 Is it required that no required element (hint bar, footer, badge) is dropped — only reflowed — across the 80×24 → large size range? [Completeness, Spec §FR-013/§SC-008]
- [x] CHK029 Are loading/empty/error visual states defined for the details pane (e.g. while the debounced fetch is in flight, on a folder, on fetch error)? [Coverage, Gap, Spec §FR-009/§FR-012]

## Acceptance Criteria Quality (for the visual requirements)

- [x] CHK030 Can each color/emphasis requirement be objectively verified (named color + element + state), rather than judged subjectively? [Measurability]
- [x] CHK031 Is there a measurable criterion for "all available data is shown" (an explicit per-screen data inventory to check against)? [Measurability, Gap]
- [x] CHK032 Do the Success Criteria cover the visual/color and data-completeness goals, or are they only behavioral? [Coverage, Gap, Spec §Success Criteria]

## Dependencies, Assumptions & Traceability

- [x] CHK033 Is the dependency on the existing styles layer (color tokens / `styles.go` palette) documented so new surfaces reuse, not redefine, colors? [Dependency, Spec §plan]
- [x] CHK034 Is a stable ID/section reference established for each visual requirement so emphasis and color rules are traceable to acceptance checks? [Traceability, Gap]

## Visual Restraint — Calm, Not Gaudy (request: "не пёстрый, спокойный, не раздражает глаз")

- [x] CHK035 Are "calm", "not gaudy" (пёстрый), and "not eye-irritating" defined with objective criteria rather than left as subjective adjectives? [Ambiguity, Gap]
- [x] CHK036 Is a maximum color budget specified (e.g. an upper bound on distinct accent hues visible on one screen) to bound visual noise? [Measurability, Gap]
- [x] CHK037 Is a calm default baseline required (neutral/dim default text; color reserved for emphasis), so the screen is mostly quiet? [Clarity, Gap]
- [x] CHK038 Is the proportion/ratio of emphasized vs neutral elements constrained (accents are the exception, not the rule)? [Measurability, Gap]
- [x] CHK039 Is a limit defined on how many elements may be simultaneously emphasized before it reads as cluttered? [Clarity, Gap, Spec §FR-003]
- [x] CHK040 Are saturation/brightness restraint requirements stated (muted tones over vivid ones) so accents inform without shouting? [Clarity, Gap]
- [x] CHK041 Do the restraint requirements explicitly reconcile with the color-coding requirements (CHK001–CHK015) so "more color cues" and "stay calm" do not conflict unbounded? [Conflict, Spec §FR-014]
- [x] CHK042 Is the badge's "loud/high-contrast" requirement (FR-014/FR-027) bounded so the one intentionally-loud element does not make the rest feel gaudy by comparison? [Consistency, Conflict, Spec §FR-014]
- [x] CHK043 Is whitespace / separation guidance specified so density (not just color) does not cause visual fatigue? [Coverage, Gap]
- [x] CHK044 Is a steady-state requirement defined so transient color (notices, spinners, progress) settles back to the calm baseline rather than persisting? [Edge Case, Gap, Spec §FR-018]
- [x] CHK045 Are restraint requirements applied consistently across all surfaces (list, pane, hint bar, command bar, connection form, overlays), not just the main view? [Consistency, Gap]
- [x] CHK046 Is there a measurable acceptance criterion for "calm interface" (e.g. accent-hue count ≤ N, neutral-default coverage ≥ X%) so it can be objectively judged? [Measurability, Gap, Spec §Success Criteria]

## Notes

- Check items off as completed: `[x]`
- CHK035–CHK046 deliberately probe the tension between the emphasis/color-coding
  requirements (CHK001–CHK015) and the "stay calm, not gaudy" goal — the spec must
  bound color, not just mandate it. CHK041/CHK042 are the key conflict gates.
- Each item tests whether a *requirement is well-written*, not whether the UI renders correctly.
- **Resolved 2026-06-06**: all CHK001–CHK046 closed by spec additions FR-031..FR-046
  (Visual design: palette, emphasis & restraint) + SC-009..SC-013, grounded in the
  authoritative `internal/ui/styles.go` palette and research R9.
- Heavy `[Gap]` density here is expected: the spec inherits color behavior from the
  existing footer ("one hue per parameter", warm Claude Code palette) but does not
  yet define the palette explicitly nor enumerate per-screen data inventories — those
  gaps are the highest-value fixes before `/speckit-tasks`.
