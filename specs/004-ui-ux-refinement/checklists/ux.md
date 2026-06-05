# UX Requirements Quality Checklist: UI/UX Refinement

**Purpose**: Unit-test the *requirements* (spec.md + contracts) for the footer redesign,
help discoverability, status feedback, and cross-cutting/a11y dimensions — validating
completeness, clarity, consistency, measurability, and coverage before implementation.
**Created**: 2026-06-05
**Feature**: [spec.md](../spec.md) · contracts: [footer-hints](../contracts/footer-hints-contract.md), [help-surface](../contracts/help-surface-contract.md)
**Depth**: Standard review gate · **Audience**: PR reviewer

## Footer Layout & Responsive

- [ ] CHK001 Is the footer's maximum height defined with an exact row count rather than a vague "compact" descriptor? [Clarity, Spec §FR-006]
- [ ] CHK002 Is it unambiguous which 3 rows compose the footer and which are conditional vs always-present? [Completeness, Spec §FR-006]
- [ ] CHK003 Is the hint-count cap stated as an exact number and consistent between FR-001, SC-003, research D2, and contract C3/C4? [Consistency, Spec §FR-001/§SC-003]
- [ ] CHK004 Are the degrade triggers (priority cap vs width pressure) and their ordering distinctly specified so both paths are unambiguous? [Clarity, Spec §FR-004, Contract C4]
- [ ] CHK005 Is the `? more` cue's trigger condition specified for BOTH drop reasons (cap and width), and its non-appearance when all hints fit? [Completeness, Contract C4]
- [ ] CHK006 Are the supported terminal width bounds (40–200) explicitly stated, with defined behavior below the lower bound? [Coverage, Spec §Edge Cases/§SC-002]
- [ ] CHK007 Is "no horizontal overflow" defined as an objectively measurable property (line width ≤ terminal width)? [Measurability, Spec §FR-019]
- [ ] CHK008 Is the compact identity row's exact content and its optional-cluster fit rule specified? [Clarity, Spec §FR-007]
- [ ] CHK009 Is the RW/RO glanceability requirement specified independently of the help surface (i.e. it must remain in the primary view)? [Completeness, Spec §FR-008]
- [ ] CHK010 Are the hint visibility predicates (mode, writable, selection kind, multi-context) documented for every hint in the catalog? [Coverage, Contract C3]

## Help Discoverability

- [ ] CHK011 Are the help section categories enumerated explicitly and consistently between FR-011 and contract H2? [Consistency, Spec §FR-011]
- [ ] CHK012 Is the requirement that every keybinding alias appear in help stated with a single source of truth (`defaultKeys()`) to prevent drift? [Clarity, Spec §FR-014, Contract H3]
- [ ] CHK013 Is the help dismissal affordance requirement specified (explicit close instruction)? [Completeness, Spec §FR-012]
- [ ] CHK014 Are the write-capability reflection rules unambiguous (hidden vs marked-unavailable) rather than left to interpretation? [Ambiguity, Spec §FR-013, Contract H4]
- [ ] CHK015 Is the Connection-section field list (endpoint/region/user/version/context/cluster) defined, with empty-value behavior specified? [Completeness, Spec §FR-014a, Contract H5]
- [ ] CHK016 Is "discoverable in at most one step" defined measurably (reachable via a single key from any mode)? [Measurability, Spec §SC-004]

## Status & Feedback

- [ ] CHK017 Are the named-loading variants enumerated per mode (buckets/contents/object) rather than a generic "name what loads"? [Clarity, Spec §FR-015, Contract S1]
- [ ] CHK018 Is the search-pending state's trigger condition (scheduled-but-unfired debounce) precisely defined? [Clarity, Spec §FR-016, Contract S2]
- [ ] CHK019 Is the notice-vs-error visual distinction specified with concrete, verifiable attributes (green vs red) rather than "distinguishable"? [Measurability, Spec §FR-018, Contract S4]
- [ ] CHK020 Are typed-confirmation requirements (target shown alongside input + safe mismatch) preserved verbatim against the existing two-tier model, with no behavioral drift? [Consistency, Spec §FR-017/§FR-020]
- [ ] CHK021 Are requirements defined for status-row precedence when multiple states could apply (e.g. loading during an active op)? [Coverage, Gap]

## Cross-Cutting, Keymap Scope & Accessibility

- [ ] CHK022 **Scope conflict**: The spec asserts the keymap is retained and shortcuts only *relocate* (FR-022, Assumptions), but the stakeholder wants *fewer keys*. Is reducing/remapping keybindings explicitly in or out of scope for this feature? [Conflict, Spec §FR-022/§Assumptions]
- [ ] CHK023 If key reduction is in scope, are the criteria for which keys to merge/remove and any backward-compatibility/alias requirements specified? [Gap]
- [ ] CHK024 Is the "presentation-only, no behavior change" boundary stated unambiguously enough to test (no storage/SDK/confirm-tier change)? [Clarity, Spec §FR-020]
- [ ] CHK025 Is the redaction guarantee scoped correctly — Connection sources only non-secret Backend fields, no credential path — and stated as a structural requirement? [Consistency, Spec §FR-021, Contract H5/2a]
- [ ] CHK026 Are color-only distinctions (RW/RO tag, notice/error, dir vs object) backed by a non-color cue requirement for low-color/colorblind terminals? [Accessibility, Gap]
- [ ] CHK027 Are requirements defined for terminals without 256-color/truecolor support (degraded palette behavior)? [Edge Case, Gap]
- [ ] CHK028 Is localization/character-width handling specified (wide/CJK glyphs in context names, cluster, keys) so width math stays correct? [Coverage, Gap]
- [ ] CHK029 Are mode-transition reactivity requirements (object→tree restores write hints immediately) defined as observable, no-extra-keypress behavior? [Measurability, Spec §Edge Cases]
- [ ] CHK030 Is a requirement & acceptance-criteria ID scheme established and traceable from FR → SC → contract obligation → task? [Traceability]

## Notes

- Check items off as completed: `[x]`
- **CHK022/CHK023 are the highest-value items** — they surface a live scope decision
  (keymap reduction) the stakeholder raised that the current spec contradicts. Resolve via
  `/speckit-specify` refinement or an explicit Assumptions update before implementation.
- CHK026–CHK028 probe a11y/terminal-capability gaps the spec currently leaves implicit
  (the spec assumes the existing cell renderer + palette; no colorblind/low-color fallback
  is stated).
- Items reference `[Spec §FR-/§SC-]`, contract obligations, or `[Gap]/[Conflict]/[Accessibility]` markers; ≥80% carry a traceability reference.
