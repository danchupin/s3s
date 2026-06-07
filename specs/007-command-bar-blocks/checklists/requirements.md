# Specification Quality Checklist: Blocked command bar (info · read · write)

**Purpose**: Validate specification completeness and quality before planning
**Created**: 2026-06-06
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] No implementation details (languages, frameworks, APIs)
- [x] Focused on user value and business needs
- [x] Written for non-technical stakeholders
- [x] All mandatory sections completed

## Requirement Completeness

- [x] No [NEEDS CLARIFICATION] markers remain
- [x] Requirements are testable and unambiguous
- [x] Success criteria are measurable
- [x] Success criteria are technology-agnostic (no implementation details)
- [x] All acceptance scenarios are defined
- [x] Edge cases are identified
- [x] Scope is clearly bounded
- [x] Dependencies and assumptions identified

## Feature Readiness

- [x] All functional requirements have clear acceptance criteria
- [x] User scenarios cover primary flows
- [x] Feature meets measurable outcomes defined in Success Criteria
- [x] No implementation details leak into specification

## Notes

- Three design decisions resolved up front with the user (no clarification markers):
  layout = **columns** (info | read | write); dimmed write-key press = **no-op + `w`
  nudge**; blocks **always visible**, collapsing on small terminals.
- Added scope from a follow-up message: a **visible add-connection affordance** in the
  info block (US2 / FR-011/FR-012), fixing "I don't see a button to create a connection".
- Light references to the palette / write-arm toggle / connection manager point at
  reused 005/006 mechanisms (where behavior lives), not new implementation.
- This feature intentionally reverses 006 FR-004 (which hid write actions in read-only)
  — that supersession is stated in FR-007.
- Added scope from a follow-up message: **dangerous actions require a Ctrl chord + a
  centered popup confirmation** (US4 / FR-021..FR-028, SC-008/SC-009). "Dangerous" = the
  005/006 typed-confirm tier; safe writes keep their bare key.
- Added scope from follow-up messages: **delete-connection on the contexts screen**
  (US5 / FR-029..FR-033, SC-010/SC-011 — gated like US4, removes config triple + keychain
  secret, active context non-deletable) and a **Claude-Code-style progress bar with
  percent for long operations** (US6 / FR-034..FR-038, SC-012/SC-013 — determinate bar +
  percent + elapsed hint, indeterminate fallback, non-blocking, threshold-gated).
