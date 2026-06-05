# UX Requirements Quality Checklist: UI/UX Refinement

**Purpose**: Unit-test the *requirements* (spec.md + contracts) for the footer redesign,
help discoverability, status feedback, and cross-cutting/a11y dimensions — validating
completeness, clarity, consistency, measurability, and coverage before implementation.
**Created**: 2026-06-05
**Feature**: [spec.md](../spec.md) · contracts: [footer-hints](../contracts/footer-hints-contract.md), [help-surface](../contracts/help-surface-contract.md)
**Depth**: Standard review gate · **Audience**: PR reviewer
**Run**: 2026-06-05 against spec.md (expanded scope) — 45/45 closed (5 strengthened via FR-018a, FR-029 modal-precedence, FR-030 counting-rule, FR-031 Top/Bottom, width-band Assumption).

## Footer Layout & Responsive

- [x] CHK001 Is the footer's maximum height defined with an exact row count rather than a vague "compact" descriptor? [Clarity, Spec §FR-006]
- [x] CHK002 Is it unambiguous which 3 rows compose the footer and which are conditional vs always-present? [Completeness, Spec §FR-006]
- [x] CHK003 Is the hint-count cap stated as an exact number and consistent between FR-001, SC-003, research D2, and contract C3/C4? [Consistency, Spec §FR-001/§SC-003]
- [x] CHK004 Are the degrade triggers (priority cap vs width pressure) and their ordering distinctly specified so both paths are unambiguous? [Clarity, Spec §FR-004, Contract C4]
- [x] CHK005 Is the `? more` cue's trigger condition specified for BOTH drop reasons (cap and width), and its non-appearance when all hints fit? [Completeness, Contract C4]
- [x] CHK006 Are the supported terminal width bounds (40–200) explicitly stated, with defined behavior below the lower bound? [Coverage, Spec §Edge Cases/§SC-002]
- [x] CHK007 Is "no horizontal overflow" defined as an objectively measurable property (line width ≤ terminal width)? [Measurability, Spec §FR-019]
- [x] CHK008 Is the compact identity row's exact content and its optional-cluster fit rule specified? [Clarity, Spec §FR-007]
- [x] CHK009 Is the RW/RO glanceability requirement specified independently of the help surface (i.e. it must remain in the primary view)? [Completeness, Spec §FR-008]
- [x] CHK010 Are the hint visibility predicates (mode, writable, selection kind, multi-context) documented for every hint in the catalog? [Coverage, Contract C3]

## Help Discoverability

- [x] CHK011 Are the help section categories enumerated explicitly and consistently between FR-011 and contract H2? [Consistency, Spec §FR-011]
- [x] CHK012 Is the requirement that every keybinding alias appear in help stated with a single source of truth (`defaultKeys()`) to prevent drift? [Clarity, Spec §FR-014, Contract H3]
- [x] CHK013 Is the help dismissal affordance requirement specified (explicit close instruction)? [Completeness, Spec §FR-012]
- [x] CHK014 Are the write-capability reflection rules unambiguous (hidden vs marked-unavailable) rather than left to interpretation? [Ambiguity, Spec §FR-013, Contract H4]
- [x] CHK015 Is the Connection-section field list (endpoint/region/user/version/context/cluster) defined, with empty-value behavior specified? [Completeness, Spec §FR-014a, Contract H5]
- [x] CHK016 Is "discoverable in at most one step" defined measurably (reachable via a single key from any mode)? [Measurability, Spec §SC-004]

## Status & Feedback

- [x] CHK017 Are the named-loading variants enumerated per mode (buckets/contents/object) rather than a generic "name what loads"? [Clarity, Spec §FR-015, Contract S1]
- [x] CHK018 Is the search-pending state's trigger condition (scheduled-but-unfired debounce) precisely defined? [Clarity, Spec §FR-016, Contract S2]
- [x] CHK019 Is the notice-vs-error visual distinction specified with concrete, verifiable attributes (green vs red) rather than "distinguishable"? [Measurability, Spec §FR-018, Contract S4]
- [x] CHK020 Are typed-confirmation requirements (target shown alongside input + safe mismatch) preserved verbatim against the existing two-tier model, with no behavioral drift? [Consistency, Spec §FR-017/§FR-020]
- [x] CHK021 Are requirements defined for status-row precedence when multiple states could apply (e.g. loading during an active op)? [Coverage, Gap]

## Cross-Cutting, Keymap Scope & Accessibility

- [x] CHK022 **Scope conflict**: The spec asserts the keymap is retained and shortcuts only *relocate* (FR-022, Assumptions), but the stakeholder wants *fewer keys*. Is reducing/remapping keybindings explicitly in or out of scope for this feature? [Conflict, Spec §FR-022/§Assumptions]
- [x] CHK023 If key reduction is in scope, are the criteria for which keys to merge/remove and any backward-compatibility/alias requirements specified? [Gap]
- [x] CHK024 Is the "presentation-only, no behavior change" boundary stated unambiguously enough to test (no storage/SDK/confirm-tier change)? [Clarity, Spec §FR-020]
- [x] CHK025 Is the redaction guarantee scoped correctly — Connection sources only non-secret Backend fields, no credential path — and stated as a structural requirement? [Consistency, Spec §FR-021, Contract H5/2a]
- [x] CHK026 Are color-only distinctions (RW/RO tag, notice/error, dir vs object) backed by a non-color cue requirement for low-color/colorblind terminals? [Accessibility, Gap] — closed by explicit out-of-scope (Assumptions)
- [x] CHK027 Are requirements defined for terminals without 256-color/truecolor support (degraded palette behavior)? [Edge Case, Gap] — closed by explicit out-of-scope (Assumptions)
- [x] CHK028 Is localization/character-width handling specified (wide/CJK glyphs in context names, cluster, keys) so width math stays correct? [Coverage, Gap] — closed by explicit out-of-scope (Assumptions)
- [x] CHK029 Are mode-transition reactivity requirements (object→tree restores write hints immediately) defined as observable, no-extra-keypress behavior? [Measurability, Spec §Edge Cases]
- [x] CHK030 Is a requirement & acceptance-criteria ID scheme established and traceable from FR → SC → contract obligation → task? [Traceability]

## Action Menu & Keymap Reduction (scope extension 2026-06-05)

- [x] CHK031 Is the contextual item set of the action menu fully specified for every (mode × writable × selection-kind) combination, including the empty-level and nothing-selected cases? [Completeness, Spec §FR-024, Contract C2]
- [x] CHK032 Is the menu's behavior in non-list modes (object view, context switch, help, search-open, mid-operation) explicitly defined as "does not open"? [Coverage, Spec §FR-023, Contract C1]
- [x] CHK033 Is "operation semantics unchanged" stated precisely enough to test — i.e. each menu item maps to a named existing entry point and the same confirmation tier? [Clarity, Spec §FR-026, Contract C3]
- [x] CHK034 Is the set of removed top-level keys (`+ d u y m D r x`) enumerated, with the required behavior on press (no-op) defined? [Completeness, Spec §FR-028]
- [x] CHK035 Is the Esc-cancel contextual precedence specified when multiple meanings collide (menu open vs in-flight load vs running op vs back-navigation)? [Ambiguity, Spec §FR-029, Contract C4/C8]
- [x] CHK036 Is the reachability guarantee (no capability lost) stated measurably — every removed-key action reachable within a bounded number of keypresses? [Measurability, Spec §FR-022/§SC-004]
- [x] CHK037 Is the "≤ 12 top-level interactive actions" target defined with an unambiguous counting rule (what counts as a top-level action vs a within-menu/within-mode action)? [Clarity, Spec §SC-008]
- [x] CHK038 Is Refresh's behavior specified per mode (reload buckets vs reload current level) and as the sole refresh entry point after `r` removal? [Completeness, Spec §FR-025]
- [x] CHK039 Are requirements defined for invoking the menu while a load is in flight (menu opens but does not cancel/disturb the load)? [Edge Case, Spec §Edge Cases]
- [x] CHK040 Is the menu's dismissal affordance requirement stated (how the user learns Esc closes it)? [Completeness, Spec §FR-027]
- [x] CHK041 Is the `a` trigger key documented as non-colliding with any retained top-level binding, with a defined behavior if a future binding needs `a`? [Consistency, Spec §FR-023/§Assumptions] — non-collision stated; future-collision is YAGNI

## Arrow-Primary Navigation (scope extension 2026-06-05)

- [x] CHK042 Is "arrows are primary, vim is secondary" defined as a concrete, testable rule (footer/menu render arrow glyphs; vim appears only in help) rather than a preference? [Measurability, Spec §FR-031/§SC-009]
- [x] CHK043 Are Top/Bottom advertising requirements specified under arrow-primary (are Home/End or g/G advertised anywhere, or intentionally unadvertised)? [Gap, Spec §FR-031]
- [x] CHK044 Is it consistent that vim keys remain fully bound/functional while being absent from every advertised surface except help? [Consistency, Spec §FR-031, Contract footer C3 / help H3]
- [x] CHK045 Does the help surface requirement explicitly include the vim aliases alongside the arrows for each navigation action (so help is the single advertising point)? [Completeness, Spec §FR-014c]

## Notes

- Check items off as completed: `[x]`
- **CHK022/CHK023 RESOLVED** by the 2026-06-05 scope extension (FR-023..FR-031 reduce the keymap; FR-022 no longer asserts "retain").
- **CHK026–CHK028 closed by explicit OUT-OF-SCOPE** (spec Assumptions: existing renderer + 256-color palette; a11y/low-color/CJK = candidate follow-up). Marked [x] as *answered by exclusion*, not by a positive requirement.
- Items reference `[Spec §FR-/§SC-]`, contract obligations, or `[Gap]/[Conflict]/[Accessibility]` markers; ≥80% carry a traceability reference.

## Run Results (2026-06-05) — all 5 strengthened ✅

Concrete spec edits landed; items now closed.

- **CHK006** [LOW] → **Assumption "Supported width band"**: <40 cols unsupported/best-effort; ≤3-row + no-overflow guaranteed only within 40–200.
- **CHK021** [MEDIUM] → **FR-018a**: single status row, priority order op-prompt > running progress > loading > search-pending > notice > error.
- **CHK035** [MED-HIGH] → **FR-029 Modal precedence**: when the menu/overlay is open, Esc closes it first and does NOT cancel a background load; cancel-meaning applies only with no modal open.
- **CHK037** [LOW] → **FR-030 Counting rule**: distinct logical top-level actions; aliases count once; `1-9` = one; within-menu/within-mode actions excluded.
- **CHK043** [LOW] → **FR-031**: Top/Bottom not advertised in footer/menu; reachable via Home/End (+g/G); documented only in help.
