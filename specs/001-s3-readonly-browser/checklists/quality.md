# Requirements Quality Checklist: Read-Only S3 Browser (TUI)

**Purpose**: Validate that requirements for UX/navigation, S3 efficiency, security/credentials, and
config/contexts are complete, clear, consistent, and measurable — before generating tasks.
**Created**: 2026-06-04
**Feature**: [spec.md](../spec.md)
**Depth**: Standard | **Audience/Timing**: Author, pre-`/speckit-tasks`

**Note**: These items test the REQUIREMENTS (are they well-written?), not the implementation.

## Requirement Completeness

- [ ] CHK001 Are keyboard-navigation requirements defined for every interactive view — bucket list, tree, metadata, preview, search, context switcher? [Coverage, Spec §FR-008, contracts/tui-contract.md]
- [ ] CHK002 Are loading, empty, error, no-match, and truncated-preview states each specified as distinct requirements? [Completeness, Spec §FR-012/FR-018/FR-020, Edge Cases]
- [ ] CHK003 Are config validation rules fully specified — cross-references resolve, names unique per list, `current-context` required? [Completeness, contracts/config-schema.md]
- [ ] CHK004 Are credential types completely enumerated (static, static+session token, anonymous)? [Completeness, Spec §FR-005a]
- [ ] CHK005 Is the config file location and permission expectation documented as a requirement? [Completeness, Assumptions]
- [ ] CHK006 Is first-run / empty-configuration behavior specified rather than left implicit? [Gap, Edge Cases]
- [ ] CHK007 Are out-of-scope boundaries (download, mutations, recursive/deep search) explicitly stated? [Coverage, Assumptions]

## Requirement Clarity & Quantification

- [ ] CHK008 Is "server-side prefix search at the current level" defined precisely (effective prefix = level prefix + term, delimiter retained)? [Clarity, Spec §FR-017]
- [ ] CHK009 Are the lazy-load trigger conditions (exactly when the next page is fetched) explicitly specified? [Clarity, Spec §FR-010]
- [ ] CHK010 Are cache scope and lifetime (whole session, no auto-expiry) and manual-refresh invalidation unambiguously defined? [Clarity, Spec §FR-011/FR-011a]
- [ ] CHK011 Is env-over-inline precedence for credential resolution explicitly stated and testable? [Clarity, Spec §FR-005]
- [ ] CHK012 Is the TLS-skip-verify default (off) and per-context opt-in scope unambiguous? [Clarity, Spec §FR-004]
- [ ] CHK013 Is the breadcrumb/path requirement defined for all depths including bucket root? [Clarity, Spec §FR-009]
- [ ] CHK014 Is image-preview fallback for non-capable terminals defined in terms of what is shown? [Clarity, Spec §FR-015]
- [ ] CHK015 Is `${ENV}` reference resolution and missing-variable behavior specified? [Clarity, contracts/config-schema.md]
- [ ] CHK016 Is the term "modern and interactive UI" either quantified or scoped out of requirements (avoid unmeasurable adjective)? [Ambiguity, User input]

## Requirement Consistency

- [ ] CHK017 Is the 5 MiB preview bound stated consistently across FR-014, FR-016, and Assumptions? [Consistency, Spec §FR-014/FR-016]
- [ ] CHK018 Is "one listing request per page" consistent between FR-010 (lazy paging) and SC-003 (bounded server load)? [Consistency, Spec §FR-010/SC-003]
- [ ] CHK019 Is active-context selection precedence (flag > env > current-context) defined without internal conflict? [Consistency, Spec §FR-002]
- [ ] CHK020 Is "current level" used consistently (single delimiter depth) across navigation, search, and refresh requirements? [Consistency]
- [ ] CHK021 Do the caching requirement (FR-011) and freshness expectation avoid conflict only via the manual-refresh requirement (FR-011a)? [Conflict, Spec §FR-011/FR-011a]

## Acceptance Criteria & Measurability

- [ ] CHK022 Are latency targets defined and measurable for each heavy operation — bucket list, level page, search, context switch, preview? [Measurability, Spec §SC-001..SC-006]
- [ ] CHK023 Is the read-only guarantee expressed as an objectively verifiable requirement (zero create/update/delete requests)? [Measurability, Spec §FR-019/SC-009]
- [ ] CHK024 Is the "no frozen frames during in-flight loads" requirement stated in a verifiable way? [Measurability, Spec §SC-007/FR-012]
- [ ] CHK025 Is rendering of non-UTF-8 / unusual key names specified as a checkable requirement rather than left open? [Measurability, Edge Cases]

## Security & Credentials (Non-Functional)

- [ ] CHK026 Are secret-redaction requirements defined for ALL surfaces — log file, error messages, and any in-app display? [Coverage, Spec §FR-005/FR-021]
- [ ] CHK027 Are access-denied, unreachable, and timeout error states specified as distinct requirements (not one generic error)? [Clarity, Spec §FR-020, Edge Cases]
- [ ] CHK028 Is the anonymous-access path (credential-less context for public buckets) specified including the not-public failure case? [Coverage, Spec §FR-005a, Edge Cases]

## Dependencies & Assumptions

- [ ] CHK029 Are assumptions (search scope, preview bound, caching model, validated backends) recorded as explicit assumptions rather than silent defaults? [Assumption, Assumptions]
- [ ] CHK030 Is the external dependency on a reachable backend and valid credentials documented as outside the tool's control? [Dependency, Assumptions]

## Notes

- Check items off as the spec is confirmed/strengthened: `[x]`.
- `[Gap]`/`[Ambiguity]`/`[Conflict]`/`[Assumption]` markers flag items that may require a spec edit.
- Unchecked items at `/speckit-tasks` time = requirement-quality risks to resolve first.
