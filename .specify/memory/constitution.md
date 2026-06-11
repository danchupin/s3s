<!--
Sync Impact Report
==================
Version change: 1.2.0 → 1.3.0
Bump rationale: MINOR. Materially expanded the Development Workflow section with a normative
  comment-hygiene rule (user-mandated): code and product-visible text MUST NOT reference spec-kit
  artifacts (user-story numbers, FR-/SC-/T-identifiers, feature numbers, research/contract section
  pointers) — comments explain intent in plain language instead. No Core Principle added, removed,
  or redefined → not MAJOR; a new enforceable workflow rule → not PATCH.

Modified principles:
  (none renamed) I–VII retained verbatim.

Modified sections:
  - Development Workflow — added the comment-hygiene bullet (no spec-kit references in comments;
    reviewers reject violations).

Added sections: none
Removed sections: none

Templates requiring updates:
  ✅ .specify/templates/plan-template.md  — Constitution Check gate is generic. No edit needed.
  ✅ .specify/templates/spec-template.md  — no constitution-specific mandatory section added. Aligned.
  ✅ .specify/templates/tasks-template.md — task categories unaffected. Aligned.
  ✅ CLAUDE.md — comment-hygiene rule + constitution version reference (v1.2.0 → v1.3.0) updated.

Follow-up TODOs: existing spec-reference comments across the codebase predate this rule; they are
  scrubbed opportunistically when a file is touched (tracked in ROADMAP.md).
  RATIFICATION_DATE unchanged (2026-06-04). LAST_AMENDED_DATE = 2026-06-11.
-->

# s3s Constitution
<!-- s3s: interactive Terminal UI client for S3-compatible object storage -->

## Core Principles

### I. Core/UI Separation
All S3 domain logic (bucket/object operations, auth, pagination, transfers) MUST live in a
standalone, UI-agnostic package that compiles and tests without any TUI dependency. The TUI
layer is a thin adapter: it renders state and dispatches intents, it MUST NOT embed business
logic. Rationale: a testable, reusable core enables headless testing, alternate front-ends
(plain CLI, scripts), and prevents render code from hiding correctness bugs.

### II. Non-Blocking TUI
The render/event loop MUST never block on network or disk I/O. Every S3 call MUST run off the
UI goroutine and report back via messages/commands; long operations MUST surface progress and
be cancellable. Rationale: an interactive TUI is judged on responsiveness — a frozen frame
during a slow `ListObjects` or large upload is a defect, not a delay.

### III. Test-First (NON-NEGOTIABLE)
TDD is mandatory: write the failing test, confirm it fails (Red), implement to pass (Green),
then refactor. No production code is merged without a test that exercised it before it existed.
Core-package logic MUST reach high unit coverage; bug fixes MUST start with a regression test.
Rationale: tests written after the fact encode the implementation's bugs as expected behavior.

### IV. Integration Testing
S3 interactions MUST be covered by integration tests run against a real S3-compatible backend
(e.g. MinIO or LocalStack) — not only mocks. Required focus areas: credential/auth flows,
pagination boundaries, multipart/large transfers, error and retry paths, and any change to the
storage-client contract. Rationale: S3 semantics (eventual edges, pagination, error codes)
diverge from hand-written mocks; only a real backend catches the divergence.

### V. Observability & Safe Operations
The core MUST emit structured logs to a file or stderr (never into the TUI frame) with operation,
target, and outcome. Destructive actions (delete object/bucket, overwrite, recursive remove)
MUST require explicit user confirmation and MUST be logged before execution. Secrets MUST NOT be
logged. Rationale: a TUI hides its own stdout, so debuggability depends on side-channel logs, and
a single keystroke must never silently destroy data.

### VI. UI Legibility
Every interface attribute that identifies a resource — bucket name, object key, folder/prefix,
breadcrumb path — MUST be either fully visible OR revealable in full via a single, consistent
action, so it can be read and copied. No identifying value may be permanently hidden by
truncation: where rendering it in full would harm the layout, an explicit expand/reveal
affordance MUST exist. Layout invariants MUST be preserved — the footer and command/hint bar
MUST remain fully visible at every supported terminal width and layout tier; legibility changes
MUST NOT scroll them off. State cues MUST stay distinguishable without color (e.g. under
`NO_COLOR`), relying on text in addition to color. Rationale: a browser whose identifiers you
cannot read or copy is broken; truncation that drops the only on-screen copy of a name defeats
the tool's purpose.

### VII. UI Consistency & Design System
All interface elements MUST conform to a shared design language. Confirmation prompts MUST share
one pattern and wording structure (action → consequence → confirm/cancel keys, with cancel as the
default); action and hint labels MUST share one vocabulary and formatting (key glyph + verb).
Color MUST be used as a consistent distinguishing accent layered on that shared base — mapped
through the established palette roles — never as ad-hoc, per-surface styling. New surfaces MUST
reuse existing shared components and palette roles rather than introduce parallel styling.
Rationale: a predictable, consistent surface is learnable and reviewable; one-off prompts and
labels erode trust and let visual drift accumulate unnoticed.

## Technology & Security Constraints

- Language: Go. Code MUST pass `gofmt`/`go vet` and the project linter before merge.
- S3 access: use a maintained S3 SDK; the storage client MUST be an interface to allow
  fakes in unit tests and a real backend in integration tests.
- Credentials MUST come from the OS keychain or an external credential command, with an explicit
  no-echo prompt as the interactive fallback — never hardcoded, never committed, and never written
  to the s3s config in plaintext. Anonymous (no-credential) access is permitted. The keychain is the
  blessed default (macOS Keychain / Windows Credential Manager / Linux Secret Service); the external
  command is the headless and power-user path; where the keychain is unavailable, s3s MUST fail loudly
  toward the command source rather than silently fall back to plaintext. `.env` and credential files
  MUST be git-ignored.
- TLS verification MUST default to on; disabling it (for local MinIO) MUST be an explicit,
  documented opt-in flag.
- No telemetry or network calls beyond the configured S3 endpoint.

## Development Workflow

- Branch per feature; no direct commits to `main` for feature work.
- Every PR MUST: include tests written test-first, pass the full `go test` suite plus
  integration tests, and pass fmt/vet/lint gates.
- Code review MUST verify compliance with the seven Core Principles before approval; a reviewer
  rejects any change that puts logic in the TUI layer, lacks a preceding test, hides a resource
  identifier with no way to reveal it (VI), or introduces one-off prompt/label styling outside
  the shared design system (VII).
- Comments — in code, tests, and any product-visible text — MUST explain intent in plain
  language and MUST NOT reference spec-kit artifacts: user-story numbers, FR-/SC-/T-identifiers,
  feature numbers, or research/contract/data-model section pointers. Such references rot as specs
  evolve and say nothing to a reader without the spec tree; state the constraint itself instead.
  A reviewer rejects comments that cite tickets in place of an explanation. Rationale: the code
  is read for years by people who never open `specs/`; a comment must stand on its own.
- Complexity that violates a principle MUST be justified in the PR description or removed.

## Governance

This constitution supersedes other practices for the s3s project. Amendments MUST be proposed
via PR that states the change, its rationale, and a version bump. Versioning follows semantic
rules: MAJOR for principle removals or backward-incompatible redefinitions, MINOR for a new
principle or materially expanded section, PATCH for clarifications and wording. All PRs and
reviews MUST verify compliance; unjustified complexity is grounds for rejection. Use the project
plan and `CLAUDE.md` for runtime development guidance.

**Version**: 1.3.0 | **Ratified**: 2026-06-04 | **Last Amended**: 2026-06-11
