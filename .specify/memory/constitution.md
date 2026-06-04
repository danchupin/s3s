<!--
Sync Impact Report
==================
Version change: TEMPLATE (unversioned) → 1.0.0
Bump rationale: Initial ratification. All placeholder tokens replaced with concrete
  principles for the s3s interactive S3 TUI. First adopted version → MAJOR baseline 1.0.0.

Modified principles:
  [PRINCIPLE_1_NAME] → I. Core/UI Separation
  [PRINCIPLE_2_NAME] → II. Non-Blocking TUI
  [PRINCIPLE_3_NAME] → III. Test-First (NON-NEGOTIABLE)
  [PRINCIPLE_4_NAME] → IV. Integration Testing
  [PRINCIPLE_5_NAME] → V. Observability & Safe Operations

Added sections:
  - Technology & Security Constraints (was [SECTION_2_NAME])
  - Development Workflow (was [SECTION_3_NAME])

Removed sections: none

Templates requiring updates:
  ✅ .specify/templates/plan-template.md  — Constitution Check gate is generic ("[Gates
     determined based on constitution file]"); no hardcoded principle conflicts. No edit needed.
  ✅ .specify/templates/spec-template.md  — no constitution-specific mandatory sections; aligned.
  ✅ .specify/templates/tasks-template.md — task categories cover tests/integration/observability;
     aligned with TDD and Integration Testing principles.

Follow-up TODOs: none. RATIFICATION_DATE set to first-adoption date 2026-06-04.
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

## Technology & Security Constraints

- Language: Go. Code MUST pass `gofmt`/`go vet` and the project linter before merge.
- S3 access: use a maintained S3 SDK; the storage client MUST be an interface to allow
  fakes in unit tests and a real backend in integration tests.
- Credentials MUST come from environment, AWS-style config/profile, or explicit prompt — never
  hardcoded and never committed. `.env` and credential files MUST be git-ignored.
- TLS verification MUST default to on; disabling it (for local MinIO) MUST be an explicit,
  documented opt-in flag.
- No telemetry or network calls beyond the configured S3 endpoint.

## Development Workflow

- Branch per feature; no direct commits to `main` for feature work.
- Every PR MUST: include tests written test-first, pass the full `go test` suite plus
  integration tests, and pass fmt/vet/lint gates.
- Code review MUST verify compliance with the five Core Principles before approval; a reviewer
  rejects any change that puts logic in the TUI layer or lacks a preceding test.
- Complexity that violates a principle MUST be justified in the PR description or removed.

## Governance

This constitution supersedes other practices for the s3s project. Amendments MUST be proposed
via PR that states the change, its rationale, and a version bump. Versioning follows semantic
rules: MAJOR for principle removals or backward-incompatible redefinitions, MINOR for a new
principle or materially expanded section, PATCH for clarifications and wording. All PRs and
reviews MUST verify compliance; unjustified complexity is grounds for rejection. Use the project
plan and `CLAUDE.md` for runtime development guidance.

**Version**: 1.0.0 | **Ratified**: 2026-06-04 | **Last Amended**: 2026-06-04
