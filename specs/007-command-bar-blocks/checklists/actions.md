# Requirements Quality Checklist: Action labels & delete confirmation tiers

**Purpose**: Unit-test the *requirements* for action labelling and delete-confirmation
tiers before planning — are they clear, complete, consistent, and measurable?
**Created**: 2026-06-06
**Feature**: [spec.md](../spec.md)
**Focus**: action-label clarity; per-target delete confirmation (object/group = binary,
directory = typed path)

## Action Label Clarity

- [x] CHK001 - Is "simple, unambiguous label that names the action, no extra words"
  quantified with an objective rule (e.g. single imperative verb, max word count)?
  [Clarity, Spec §FR-005a]
- [x] CHK002 - Are the exact label strings for every read action (download, analyze,
  filter/search, refresh, open) specified, not just the action set? [Completeness, Spec §FR-004/FR-005a]
- [x] CHK003 - Are the exact label strings for every write action (delete, copy,
  move/rename, recursive delete, upload, new folder) specified? [Completeness, Spec §FR-005/FR-005a]
- [x] CHK004 - Is the chord-prefixed label form (e.g. "^X delete", §FR-026) defined so it
  still satisfies the "no extra words" rule? [Consistency, Spec §FR-026/FR-005a]
- [x] CHK005 - Is label consistency required across the bar, the centered popup, and the
  contexts screen (same action → same word)? [Consistency, Spec §FR-027a/FR-005a]
- [x] CHK006 - Can "names the action / no extra words" be objectively verified (a
  reviewer can pass/fail each label)? [Measurability, Spec §SC-014]

## Delete Confirmation Tiers — Completeness

- [ ] CHK007 - Does the spec define a distinct confirmation tier per delete target type:
  single object, selected group (bulk), and directory/prefix? [Completeness, Gap, Spec §FR-024]
- [ ] CHK008 - Is the single-object delete tier specified as a binary (y/N) confirmation?
  [Completeness, Spec §FR-024]
- [ ] CHK009 - Is the selected-group (bulk) delete tier specified as a binary (y/N)
  confirmation? [Completeness, Spec §FR-024]
- [ ] CHK010 - Is the directory/recursive delete tier specified as requiring the operator
  to type the directory's exact path? [Completeness, Spec §FR-024]
- [ ] CHK011 - Is "the directory's path" defined precisely (full key prefix vs. folder
  name only; trailing slash; case sensitivity)? [Clarity, Gap, Spec §FR-024]
- [ ] CHK012 - Is "binary confirmation" defined (the accepted keys, default on Enter/Esc)?
  [Clarity, Gap, Spec §FR-024]

## Consistency & Conflicts

- [ ] CHK013 - **Conflict**: §FR-024 places single-object and bulk delete in the *typed*
  confirmation tier, but the intent is *binary* for object/group and *typed path* only for
  directory — is this contradiction resolved? [Conflict, Spec §FR-024]
- [ ] CHK014 - Is the relationship between the Ctrl-chord gate (§FR-021) and the
  per-target confirmation tier (binary vs typed-path) stated, so a chord still gates but
  the popup content differs by target? [Consistency, Spec §FR-021/FR-023]
- [ ] CHK015 - Are move/rename and detected overwrite (§FR-024 typed tier) reconciled with
  the new object/group=binary rule, or explicitly kept at the typed tier? [Consistency, Gap, Spec §FR-024]
- [ ] CHK016 - Is delete-connection's typed-name confirmation (§FR-030) consistent with
  the directory typed-path rule (both "type the exact identifier")? [Consistency, Spec §FR-030]
- [ ] CHK017 - Does the typed-path/typed-name tier reuse one stated confirmation mechanism,
  rather than two divergent definitions? [Consistency, Spec §FR-024/FR-030]

## Scenario & Edge-Case Coverage

- [ ] CHK018 - Is the mixed-selection case (objects + at least one folder marked together)
  assigned a tier — does it escalate to typed-path? [Coverage, Gap, Spec §FR-017/FR-024]
- [ ] CHK019 - Is the behavior on a wrong/typo'd directory path specified (abort, no
  mutation, allow retry)? [Edge Case, Gap, Spec §FR-024]
- [ ] CHK020 - Is empty-selection delete (chord pressed with nothing selected) defined as
  not-applicable rather than a confirmation prompt? [Edge Case, Spec §FR-018]
- [ ] CHK021 - Is the read-only path consistent — a delete chord in read-only opens no
  confirmation of either tier (§FR-028)? [Consistency, Spec §FR-028]
- [ ] CHK022 - Is cancel (Esc) behavior identical across binary and typed-path tiers
  (nothing mutated, return to prior view)? [Consistency, Spec §FR-025]

## Acceptance Criteria & Measurability

- [ ] CHK023 - Is there a measurable success criterion that single-object/group delete uses
  binary confirm and directory delete requires the typed path (analogous to SC-008)?
  [Measurability, Gap, Spec §SC-008]
- [ ] CHK024 - Is "selected group" defined (multi-select marks from §FR-017) so the
  binary-tier scope is unambiguous? [Clarity, Spec §FR-017]
- [ ] CHK025 - Are the confirmation-tier requirements testable headless (a test can assert
  which prompt type each target produces)? [Measurability, Spec §FR-024]

## Non-Functional / Safety Alignment

- [ ] CHK026 - Does the per-target tiering still honor the loud write/read-only badge and
  pre-execution logging of destructive ops (Constitution V / §FR-027)? [Consistency, Spec §FR-027]
- [ ] CHK027 - Is the centered-popup no-clip requirement (§SC-009) stated to cover the
  typed-path input length for deep directory prefixes? [Coverage, Spec §SC-009]

## Bucket Deletion — Typed-Name Tier

- [ ] CHK028 - Is whole-bucket deletion included in the dangerous-action set at all
  (§FR-021 lists object/recursive/move/bulk/overwrite but not bucket)? [Gap, Spec §FR-021]
- [ ] CHK029 - Does the spec require typing the exact bucket name to confirm a bucket
  delete (the highest typed tier)? [Completeness, Gap, Spec §FR-024]
- [ ] CHK030 - Is "bucket name" defined as an exact, case-sensitive full match (no prefix,
  no partial)? [Clarity, Gap, Spec §FR-024]
- [ ] CHK031 - Is the typed-bucket-name tier consistent with the directory typed-path
  (CHK010) and delete-connection typed-name (§FR-030) — one shared typed-confirm
  mechanism, three identifier targets? [Consistency, Spec §FR-024/FR-030]
- [ ] CHK032 - Is bucket delete gated by the Ctrl chord like other dangerous actions
  (§FR-021), with the typed-name confirmation inside the centered popup (§FR-023)?
  [Consistency, Gap, Spec §FR-021/FR-023]
- [x] CHK033 - Are preconditions for bucket delete specified (e.g. empty vs non-empty
  bucket, recursive purge of contents first)? [Coverage, Spec §FR-024b]
- [ ] CHK034 - Is wrong/typo'd bucket-name entry behavior defined (abort, no mutation,
  allow retry), mirroring the directory typed-path rule (CHK019)? [Edge Case, Gap, Spec §FR-024]

## Typed-Confirm Input Ergonomics

- [ ] CHK035 - **Conflict**: §FR-023 mandates a *centered popup dialog* for dangerous
  confirmations, but the typed-identifier confirm is requested as a convenient, prominent
  *inline form* ("not a separate window"). Is the surface for the typed tier reconciled
  (inline prominent form vs modal popup)? [Conflict, Spec §FR-023/FR-024]
- [ ] CHK036 - Is "convenient / noticeable form" turned into objective criteria (inline
  placement, palette-prominent styling, a label naming the required identifier)?
  [Clarity, Ambiguity, Gap, Spec §FR-024]
- [ ] CHK037 - Does the spec distinguish the confirm *surface* per tier — binary y/N may
  stay a centered popup, while the typed-identifier tier uses a prominent inline form?
  [Consistency, Gap, Spec §FR-023/FR-024]
- [ ] CHK038 - Are the typed-input field requirements specified (single-line, editable,
  width for long paths, horizontal scroll or wrap, paste support)? [Completeness, Gap, Spec §FR-024a]
- [ ] CHK039 - Is the typed form's prominence requirement defined so it is not missed
  (distinct palette role / the loud write badge), without becoming a separate window?
  [Clarity, Spec §FR-027]
- [ ] CHK040 - Is long-identifier behavior specified (a deep directory path or long bucket
  name that exceeds the input width) so the operator can still see/verify what they typed?
  [Edge Case, Gap, Spec §FR-024a]
- [ ] CHK041 - Is the inline typed form's no-clip / responsiveness requirement stated for
  small terminals, consistent with §SC-009 (which currently assumes a centered popup)?
  [Coverage, Spec §SC-009]
- [ ] CHK042 - Is Esc-cancel and submit behavior identical for the inline typed form and
  the binary popup (nothing mutated on cancel, §FR-025)? [Consistency, Spec §FR-025]

## Notes

- This checklist tests requirement *quality*, not behavior. It surfaces a live
  **conflict**: spec §FR-024 currently lumps single-object + bulk delete into the typed
  tier, while the requested model is **binary (y/N) for single object and selected group**
  and **typed exact path for directory/recursive delete**. CHK013 gates resolving §FR-024
  before planning.
- Added scope (CHK035–042): the **typed-identifier confirm should be a convenient,
  prominent inline form, NOT a separate window** — this is a **conflict** with §FR-023
  (centered popup for all dangerous confirms). Reconcile by splitting the confirm surface
  per tier: binary y/N MAY stay a centered popup; the typed-identifier tier (path / bucket
  name / connection name) uses a prominent inline form with a real editable field sized
  for long identifiers. §SC-009 (assumes centered popup) must extend to the inline form.
- Label-clarity items (CHK001–006) test whether "simple, unambiguous, names the action,
  no extra words" is turned into an objective, reviewable rule rather than left as an
  adjective.
- Added scope (CHK028–034): **whole-bucket deletion must require typing the exact bucket
  name**. This is a **gap** — §FR-021/FR-024 do not currently mention bucket delete. The
  emerging typed-confirm model now has three identifier targets: **directory → exact
  path**, **bucket → exact name**, **connection → exact name**; object/group stay
  **binary**. Planning should fold bucket delete into the dangerous-action set and unify
  the typed-confirm mechanism.
