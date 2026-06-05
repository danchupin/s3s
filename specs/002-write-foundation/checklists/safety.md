# Safety Requirements Quality Checklist: Write Foundation & Safety

**Purpose**: Unit-test the *requirements* (not the code) for the safety surface of
002 — write enablement, confirmation, read-only protection, logging — before
`/speckit-implement`. PR-gate depth, reviewer audience.
**Created**: 2026-06-05
**Reviewed**: 2026-06-05 (37/37 pass — CHK011, CHK030 gaps resolved via spec edits)
**Feature**: [spec.md](../spec.md)

## Requirement Completeness

- [x] CHK001 Are write-enablement requirements documented for every entry path (global flag AND per-context setting)? [Completeness, Spec §FR-001/FR-002]
- [x] CHK002 Is the precedence (read-only context wins over global `--write`) explicitly specified, not implied? [Completeness, Spec §FR-002, Clarifications]
- [x] CHK003 Do requirements mandate confirmation for *every* mutating operation with no unconfirmed path? [Completeness, Spec §FR-004/SC-001]
- [x] CHK004 Are logging requirements defined for all mutation outcomes — success, failure, AND cancellation? [Completeness, Spec §FR-008, US2 AS4]
- [x] CHK005 Is the operator-visible behavior on a refused mutation specified (explanatory message, never a silent no-op)? [Completeness, Spec §FR-003]
- [x] CHK006 Is operation→tier classification (reversible vs destructive) specified so every op maps to exactly one confirmation tier? [Completeness, Spec §FR-005]
- [x] CHK007 Are typed-tier requirements documented even though 002 ships no destructive op? [Completeness, Spec §US3]
- [x] CHK008 Is the out-of-scope boundary (no upload/delete/copy/move in 002) explicitly stated to bound the safety surface? [Completeness, Spec §Assumptions]

## Requirement Clarity

- [x] CHK009 Is the progress-feedback timing quantified rather than described as "promptly"? [Clarity, Spec §SC-004]
- [x] CHK010 Is "non-blocking" defined measurably (no frame freeze for the backend-call duration)? [Clarity, Spec §FR-006/SC-004]
- [x] CHK011 Is the typed-confirmation match rule unambiguous (exact identifier; case/whitespace handling)? [Clarity, Spec §FR-005/SC-003] — RESOLVED: FR-005 + data-model now specify byte-for-byte exact (no trim, no case-fold).
- [x] CHK012 Is create-folder semantics precisely defined (zero-length key, exactly one trailing delimiter)? [Clarity, Spec §FR-009, Assumptions]
- [x] CHK013 Are valid/invalid folder-name criteria explicitly enumerated (empty, whitespace, control chars)? [Clarity, Spec §FR-010, Edge Cases]
- [x] CHK014 Is "leaves storage unchanged" defined verifiably (e.g., a re-list shows no change)? [Clarity, Spec §FR-011/SC-007]

## Requirement Consistency

- [x] CHK015 Do the spec's confirmation-tier requirements align with the confirmation contract? [Consistency, Spec §FR-005 / contracts/confirmation-contract.md]
- [x] CHK016 Is read-only enforcement described consistently as a single guard point (not duplicated UI checks) across spec/plan/contracts? [Consistency, Spec §FR-003/FR-012]
- [x] CHK017 Are the operation status states in the data model consistent with the lifecycle the requirements imply? [Consistency, data-model.md / Spec §FR-006/FR-007]
- [x] CHK018 Is the actor term ("operator") used consistently across all safety requirements? [Consistency]

## Acceptance Criteria Quality

- [x] CHK019 Can "100% of mutations pass a confirmation step" be objectively verified? [Measurability, Spec §SC-001]
- [x] CHK020 Can "read-only refuses 100% of mutations, including under `--write`" be objectively verified? [Measurability, Spec §SC-002]
- [x] CHK021 Is each success criterion tied to an externally observable result (not internal state)? [Measurability, Spec §SC-001..007]
- [x] CHK022 Is "no secrets in any log line" stated as an objectively checkable criterion? [Measurability, Spec §SC-005]

## Scenario Coverage

- [x] CHK023 Are requirements defined for the primary flow (enable → confirm → execute → feedback → log)? [Coverage, Spec §US2]
- [x] CHK024 Are alternate-flow requirements defined (cancel at confirmation; read-only refusal)? [Coverage, Spec §US1/US2 AS2]
- [x] CHK025 Are exception-flow requirements defined (access denied, network hang, invalid input)? [Coverage, Spec §Edge Cases/FR-011]
- [x] CHK026 Are recovery/consistency requirements defined when a mutation is interrupted (context switch / navigate mid-op)? [Coverage, Spec §FR-007/Edge Cases]
- [x] CHK027 Are name-collision / already-exists requirements defined (no silent overwrite)? [Coverage, Spec §FR-010/Edge Cases]

## Edge Case Coverage

- [x] CHK028 Are empty/whitespace/invalid-character folder-name cases addressed in requirements? [Edge Case, Spec §FR-010/Edge Cases]
- [x] CHK029 Is the "writes enabled globally but active context read-only" boundary explicitly covered? [Edge Case, Spec §FR-002/Edge Cases]
- [x] CHK030 Are a hung or cancelled operation's effects on both storage and the view specified? [Edge Case, Spec §FR-007/Edge Cases] — RESOLVED: FR-007 + Edge Cases + data-model now specify the indeterminate storage outcome of a cancelled in-flight mutation (never reported success; refresh shows ground truth).

## Non-Functional Requirements

- [x] CHK031 Are observability requirements complete (pre-execution log + outcome log, file-only)? [Non-Functional, Spec §FR-008/SC-005]
- [x] CHK032 Is the ≤100 ms responsiveness target stated as a non-functional requirement, not a hope? [Non-Functional, Spec §SC-004]
- [x] CHK033 Are no-secret-leakage requirements specified across *all* failure/error paths, not just logs? [Non-Functional, Spec §FR-008/FR-011/SC-005]

## Dependencies & Assumptions

- [x] CHK034 Is the assumption that backends represent a folder as a zero-length delimiter-suffixed key documented and scoped? [Assumption, Spec §Assumptions]
- [x] CHK035 Is reuse of the existing non-blocking/generation/cancellation model stated as an assumption rather than re-specified? [Assumption, Spec §Assumptions]

## Ambiguities & Conflicts

- [x] CHK036 Is the apparent tension "002 ships the typed tier" vs "002 ships no destructive op" resolved explicitly (framework now, action later)? [Conflict/Ambiguity, Spec §US3]
- [x] CHK037 Is the unspecified create-folder keybinding an intentional, documented deferral to planning rather than an omission? [Ambiguity, Spec §US2 / contracts/confirmation-contract.md]

## Notes

- Check items off as the spec is confirmed to satisfy each: `[x]`.
- An unchecked item means the *requirement* needs tightening (clarify/complete/de-conflict) — fix the spec, not the code.
- This is a PR-gate: resolve CRITICAL/HIGH gaps before `/speckit-implement`.
- Total: 37 items across 9 quality dimensions.

### Review findings (2026-06-05) — RESOLVED

Initial review: 35/37. Two gaps found and closed by spec edits the same day:

| Item | Severity | Gap | Resolution |
|------|----------|-----|------------|
| CHK011 | LOW | Typed-confirm "exact identifier" didn't define whitespace/case handling. | FR-005 + data-model: byte-for-byte exact, no trim, no case-fold. Clarifications + T022 updated. |
| CHK030 | MEDIUM | Storage effect of a *cancelled in-flight* mutation unspecified. | FR-007 + Edge Cases + data-model: indeterminate outcome, never reported success, refresh shows ground truth. Confirmation contract + T017 updated. |

Final: **37/37 pass, 0 outstanding.** Ready for `/speckit-implement`.
