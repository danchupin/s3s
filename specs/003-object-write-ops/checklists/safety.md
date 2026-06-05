# Safety Requirements Quality Checklist: Object Write Operations

**Purpose**: Unit-test the *requirements* (not the code) for the safety surface of
003 — destructive object mutations (delete, overwrite, move, recursive delete),
no-data-loss guarantees, truthful partial/cancel outcomes, and logging — before
`/speckit-implement`. PR-gate depth, reviewer audience.
**Created**: 2026-06-05
**Reviewed**: 2026-06-05 (38/38 pass — CHK014 resolved via FR-003/FR-005 advisory-detection clause; CHK035 resolved via SC-007 restating the 100 ms / next-render-tick budget; both from analyze findings F1–F4)
**Feature**: [spec.md](../spec.md)

## Requirement Completeness

- [x] CHK001 Is a confirmation tier specified for every one of the five operations (delete, upload, copy, move, recursive delete), with no operation left unmapped? [Completeness, Spec §FR-001/FR-003/FR-005/FR-006/FR-008, SC-001]
- [x] CHK002 Is overwrite escalation to the typed tier documented for BOTH upload and copy when the destination key already exists? [Completeness, Spec §FR-003/FR-005, SC-004]
- [x] CHK003 Are no-data-loss requirements defined for BOTH move failure branches — copy fails (source intact, no delete) AND copy succeeds but source delete fails (data at destination)? [Completeness, Spec §FR-007, US4 AS2]
- [x] CHK004 Is the best-effort recursive-delete requirement documented with truthful deleted-vs-failed counts on completion? [Completeness, Spec §FR-009/FR-011, US5 AS3]
- [x] CHK005 Are logging requirements defined for the new fields — action, source, destination (where applicable), context, and recursive counts — across success, partial, failure, and cancellation? [Completeness, Spec §FR-014, US5 AS3/AS4]
- [x] CHK006 Is the cancellation outcome specified for both streaming operations (upload and recursive delete), including that storage may be indeterminate and is never reported as a clean success? [Completeness, Spec §FR-011, US2 AS4/US5 AS4]
- [x] CHK007 Are upload-source validation requirements documented (missing/unreadable/empty/changed local file) before or during transfer? [Completeness, Spec §FR-015, US2 AS4, Edge Cases]
- [x] CHK008 Is read-only / writes-disabled refusal specified for every one of the five operations (not just some)? [Completeness, Spec §FR-012, SC-008]
- [x] CHK009 Is a refresh-to-ground-truth requirement defined after any successful or partial mutation (no asserting an outcome the backend did not produce)? [Completeness, Spec §FR-016, US5 AS4]
- [x] CHK010 Is the out-of-scope boundary explicitly stated (cross-bucket copy/move, buckets, metadata/ACL, versions, sync, download) to bound the safety surface? [Completeness, Spec §Assumptions]

## Requirement Clarity

- [x] CHK011 Is "best-effort" for recursive delete defined unambiguously (continue past a per-object failure, no abort-on-first, no per-failure prompt)? [Clarity, Spec §FR-009, Clarifications]
- [x] CHK012 Are destination-key validation criteria explicitly enumerated (non-empty, valid characters, and destination ≠ source as a no-op)? [Clarity, Spec §FR-013, Edge Cases]
- [x] CHK013 Is the typed-tier target identifier precise per operation (object key for delete, destination key for move/overwrite, exact prefix for recursive delete)? [Clarity, Spec §FR-001/FR-006/FR-008]
- [x] CHK014 Is "collision / already exists" defined precisely enough to know what triggers the overwrite escalation? [Ambiguity, Spec §FR-003/FR-005]
- [x] CHK015 Is the copy/move scope ("within the current bucket only") stated unambiguously, with cross-bucket explicitly excluded? [Clarity, Spec §FR-004, Clarifications]
- [x] CHK016 Is the upload source-selection requirement clearly stated as an in-TUI local file browser (not a typed path)? [Clarity, Spec §FR-002, Clarifications]
- [x] CHK017 Is recursive-delete progress clearly defined as a running deleted/failed count, distinct from upload's byte progress? [Clarity, Spec §FR-010, US5 AS2]

## Requirement Consistency

- [x] CHK018 Is the tier classification consistent across operations (delete/move/recursive always typed; upload/copy simple unless overwrite → typed)? [Consistency, Spec §FR-003/FR-005/FR-006/FR-008, SC-001]
- [x] CHK019 Is "never reported as a clean success" applied consistently to all three risky outcomes (cancelled upload, partial move, partial recursive delete)? [Consistency, Spec §FR-011, US2 AS4/US4 AS2/US5 AS3]
- [x] CHK020 Are destination-key validation requirements consistent between copy and move? [Consistency, Spec §FR-013]
- [x] CHK021 Does the read-only refusal requirement align with the 002 foundation behavior (explanatory message, never a silent no-op)? [Consistency, Spec §FR-012, 002 §FR-003]

## Acceptance Criteria Quality

- [x] CHK022 Is the upload-fidelity criterion objectively measurable (retrieved object byte-identical to the source file)? [Measurability, Spec §SC-003]
- [x] CHK023 Can the no-data-loss criterion for failed moves be objectively verified (data still retrievable; reported partial)? [Measurability, Spec §SC-005]
- [x] CHK024 Are recursive-delete success criteria measurable (accurate deleted/failed counts; prefix absent or survivors shown after refresh)? [Measurability, Spec §SC-006]
- [x] CHK025 Is the read-only refusal criterion stated as a verifiable absolute (100% of these ops refused, storage unchanged)? [Measurability, Spec §SC-008]
- [x] CHK026 Is the overwrite-gating criterion measurable (100% of existing-key uploads/copies routed through typed confirmation; none silently replaced)? [Measurability, Spec §SC-004]

## Scenario Coverage

- [x] CHK027 Are exception-flow requirements present for each destructive op (backend denial, not-found, partial)? [Coverage, Spec §FR-015, US1 AS3/US5 AS3]
- [x] CHK028 Are recovery requirements defined for a partial move (operator told data is safe at destination and source remains)? [Coverage, Recovery, Spec §FR-007, US4 AS2]
- [x] CHK029 Is the concurrent-modification scenario addressed (object deleted by another actor between listing and confirmation)? [Coverage, Edge Case, Spec §Edge Cases]
- [x] CHK030 Are non-functional responsiveness requirements covered for the new long-running ops (progress appears promptly; no frame freeze; cancellable)? [Coverage, Non-Functional, Spec §FR-010, SC-007]

## Edge Case Coverage

- [x] CHK031 Is the destination-equals-source case specified as a rejected no-op rather than a source-destroying operation? [Edge Case, Spec §Edge Cases, FR-013]
- [x] CHK032 Is recursive delete of an empty or non-existent prefix specified (clear no-op, no error)? [Edge Case, Spec §Edge Cases]
- [x] CHK033 Is multi-page enumeration required so recursive delete covers the whole subtree, not just the first page? [Edge Case, Coverage, Spec §FR-009, Edge Cases]
- [x] CHK034 Are large-object upload requirements covered (visible progress, cancellable, indeterminate-on-cancel)? [Edge Case, Spec §Edge Cases, US2 AS3/AS4]

## Non-Functional Requirements

- [x] CHK035 Is the responsiveness budget for new operations quantified rather than only referenced as "the foundation's render-tick budget"? [Clarity, Non-Functional, Spec §SC-007]
- [x] CHK036 Do the logging requirements assert no secrets in any new log line, including the destination and local-source-path fields? [Non-Functional, Security, Spec §FR-014/SC-009]

## Dependencies & Assumptions

- [x] CHK037 Is the assumption of a server-side copy primitive (for copy/move without local round-trip) documented and validated? [Assumption, Spec §Assumptions]
- [x] CHK038 Is the reuse of the 002 foundation (guard, two-tier confirm, non-blocking model, logging, redaction) stated as a dependency rather than re-specified? [Dependency, Spec §Assumptions]

## Notes

- Items marked `[Gap]`/`[Ambiguity]`/`[Conflict]` that fail review require a spec
  edit before `/speckit-implement`. Watch CHK014 (collision definition) and CHK035
  (responsiveness budget quantification) — both reference-by-inheritance points that
  may need an explicit number/criterion in the 003 spec.
- This checklist tests requirement *quality*, not implementation. Behavior
  verification belongs in tasks/tests, not here.
