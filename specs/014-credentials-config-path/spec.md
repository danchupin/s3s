# Feature Specification: Credential Sources Simplification & Config-Path Override

**Feature Branch**: `014-credentials-config-path`

**Created**: 2026-06-08

**Status**: Draft

**Input**: User description: "Simplify credential sources to exactly two (OS keychain + external command), remove the rest (inline/${ENV}, awsProfile), and add the ability to override the s3s config file path so users can keep multiple configs."

## Overview

s3s today offers four ways for a context's user to supply its S3 secret: an inline
`secretAccessKey` (a literal value or a `${ENV}` reference), the OS keychain, an
external command (`cmd`), and an AWS shared profile (`awsProfile`) — plus an
anonymous user (no secret) and a no-echo interactive prompt fallback. A survey of
40+ comparable tools (recorded in `ROADMAP.md` → "Credentials & security") found
the field has converged on two patterns: **the OS keychain as the convenient secure
default**, and **an external command as the escape hatch** for headless machines and
power users. Carrying four sources triples the documentation, validation, and
support surface for the two weakest, least-secure audiences.

This feature narrows the supported secret sources to **exactly two** — `keychain`
(the blessed default) and `cmd` — and **removes** the other secret methods outright.
It also adds a long-requested capability: **overriding the config file path** so a
user can maintain several independent configs (e.g. work vs personal, prod vs
staging) and point s3s at whichever they need.

This is a **breaking change to the config schema**: configs that declare a removed
source no longer load.

## Clarifications

### Session 2026-06-08

- Q: How to handle configs on removed sources (inline/`${ENV}`/awsProfile) on upgrade? → A: No migration at all — s3s has no users yet; remove the sources outright, no migration command, no compatibility messaging, no apiVersion-migration burden.
- Q: How to avoid keychain account collisions across multiple configs (same context name in two configs)? → A: Namespace the keychain account by config identity (a config-path-derived id) + context name, so two configs' same-named contexts never share a secret.
- Q: Does the `cmd` source need to carry a session token (STS / temporary creds), now that awsProfile — the only token source — is removed? → A: No. `cmd` returns the secret only (single value). STS/temporary-credential support is deferred (a separate credential_process-JSON ROADMAP item).
- Q: Config switching — relaunch-only, or an in-TUI runtime switch? → A: Relaunch-only (`--config` / `S3S_CONFIG` at launch); no in-TUI config switch.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Only two credential sources remain (Priority: P1)

A user setting up a new connection is offered, and can only use, two ways to supply
a secret: the OS keychain (the default the wizard pre-selects) or an external
command whose standard output is the secret. No other secret source is accepted.

**Why this priority**: This is the core of the feature — the simplification the
maintainer asked for. Without it nothing else matters. It is the security and
usability posture the whole iteration exists to deliver.

**Independent Test**: Run the config-init wizard and confirm the only secret
choices are keychain (default) and cmd; create a context with each and confirm the
backend authenticates. Confirm no prompt or field offers inline/`${ENV}`/awsProfile.

**Acceptance Scenarios**:

1. **Given** a fresh install, **When** the user runs the config-init wizard, **Then** the prompted credential-source default is `keychain` (not `env`), and the only other offered source is `cmd`.
2. **Given** a context whose user declares `keychain: true`, **When** s3s resolves the backend, **Then** the secret is read from the OS keychain and never written to any s3s file in plaintext.
3. **Given** a context whose user declares a `cmd` source, **When** s3s resolves the backend, **Then** the command is executed (argv, never a shell) and its trimmed stdout is used as the secret.
4. **Given** a non-anonymous user, **When** validation runs, **Then** the user must declare exactly one of `keychain` or `cmd` — zero sources or both is a clear error.

---

### User Story 2 - Removed sources are gone from the schema (Priority: P1)

The inline `secretAccessKey` (literal and `${ENV}`), inline `sessionToken`, and
`awsProfile` sources no longer exist anywhere in s3s — not in the schema, the
wizard, the resolver, or validation. s3s is pre-release with no users, so **no
migration path, migration command, or compatibility messaging is provided**: the
sources are simply removed.

**Why this priority**: Removing the code/schema for the dropped sources is the other
half of the simplification in US1 — it is what actually shrinks the validation,
resolver, and documentation surface. Ships together with US1.

**Independent Test**: Confirm the removed fields are absent from the schema and the
wizard; confirm a non-anonymous user with no `keychain`/`cmd` source fails the
normal "missing credentials" validation (no special migration handling required).

**Acceptance Scenarios**:

1. **Given** the config schema, **When** a user is defined, **Then** `secretAccessKey`, `sessionToken`, and `awsProfile` are not part of the accepted, resolved, or validated user fields.
2. **Given** a non-anonymous user that declares none of the two supported sources, **When** s3s validates the config, **Then** it fails with the standard "missing credentials — declare keychain or cmd" error (the existing missing-source path, not a migration message).
3. **Given** the `${ENV}` resolution machinery, **When** the secret is resolved, **Then** there is no env-reference resolution step for the secret (the secret comes only from the keychain or the cmd's stdout).
4. **Given** the config-init wizard, **When** it asks for the credential source, **Then** the removed sources are not offered as options.

---

### User Story 3 - Override the config file path for multiple configs (Priority: P2)

A user who keeps more than one s3s config (work vs personal, prod vs staging) can
point s3s at a specific config file for a given invocation, without disturbing the
default config, and every part of s3s (the TUI, the `cred` subcommands, config
init/generate) operates on that same chosen file.

**Why this priority**: A frequently requested convenience that is independent of the
credential-source work but naturally batched with it (both touch config loading).
Valuable but not required for the security posture of US1/US2.

**Independent Test**: Launch s3s with a `--config <path>` flag and with the
`S3S_CONFIG` env var pointing at alternate files; confirm the TUI, `s3s cred set`,
and config init all read/write the chosen file, and that flag overrides env
overrides the default path.

**Acceptance Scenarios**:

1. **Given** a config at a non-default path, **When** the user launches s3s with `--config <path>`, **Then** s3s loads that file and the default `~/.config/s3s/config.yaml` is untouched.
2. **Given** `S3S_CONFIG` is set, **When** the user launches s3s with no `--config` flag, **Then** s3s uses the path from `S3S_CONFIG`.
3. **Given** both `--config` and `S3S_CONFIG` are set to different paths, **When** s3s launches, **Then** the `--config` flag wins (precedence: flag > env > default XDG path).
4. **Given** a chosen config path, **When** the user runs `s3s cred set <context>` or the config-init wizard, **Then** those operations read and write the same chosen file, not the default.
5. **Given** the chosen config and the active-context selectors, **When** s3s resolves which context is active, **Then** the existing precedence (`--context` flag > `S3S_CONTEXT` env > `current-context` in the chosen file) is applied against the chosen config.

---

### User Story 4 - Keychain works on every desktop OS; headless fails loudly (Priority: P2)

A macOS, Windows, or Linux-desktop user gets working keychain storage through the
same single `keychain: true` field, backed by their platform's native secret store.
A user on a headless Linux box (no Secret Service) gets a loud, actionable error
telling them to use a `cmd` source — never a silent fall-through to plaintext.

**Why this priority**: Cross-platform parity is what makes keychain a credible
default rather than a macOS-only nicety; the headless error is the safety net that
keeps the removal of plaintext sources from becoming a dead end on servers.

**Independent Test**: On each desktop OS, store and read a secret via keychain
through the same field. On a headless Linux environment with no Secret Service,
confirm keychain resolution emits the actionable "use cmd" error and does not write
or read any plaintext.

**Acceptance Scenarios**:

1. **Given** a macOS user, **When** they store a secret via keychain, **Then** it is held in the macOS login Keychain.
2. **Given** a Windows user, **When** they store a secret via keychain, **Then** it is held in the Windows Credential Manager.
3. **Given** a Linux-desktop user with a running Secret Service, **When** they store a secret via keychain, **Then** it is held in the Secret Service store (GNOME Keyring / KWallet).
4. **Given** a headless Linux box with no Secret Service available, **When** keychain resolution is attempted, **Then** s3s emits a clear error naming the missing keystore and pointing the user at a `cmd` source, and reads/writes no plaintext secret.
5. **Given** multiple configs selected via `--config`/`S3S_CONFIG` that each define a context with the same name, **When** secrets are stored in the keychain, **Then** the two contexts' secrets do not collide, because the keychain account is namespaced by a config-identity component (derived from the resolved config path) in addition to the context name.

---

### User Story 5 - Copy-pasteable documentation for both sources (Priority: P3)

A user deciding how to store their secret finds first-class documentation for the
two supported sources: a per-OS keychain backend table with the `s3s cred`
lifecycle and the headless caveat, and the `cmd` contract with ready-to-use recipes
for common secret managers.

**Why this priority**: Documentation quality determines whether the simplified
scheme is actually adopted correctly, but it does not block the functional change.

**Independent Test**: Review the README/docs and confirm both sources are
documented with the specified content (OS table, cred lifecycle, headless caveat,
cmd contract, recipes for vault/op/pass/sops/secret-tool/security).

**Acceptance Scenarios**:

1. **Given** the docs, **When** a user reads the keychain section, **Then** it shows which native store backs each OS, the `s3s cred set|rotate|rm <context>` lifecycle, and the headless-Linux caveat with the cmd fallback.
2. **Given** the docs, **When** a user reads the cmd section, **Then** it explains the contract (stdout is the secret, argv not a shell, owner-only config-perms gate, bounded timeout) and gives working recipes for HashiCorp Vault, 1Password (`op`), `pass`, `sops`, `secret-tool`, and the macOS `security` command.

---

### Edge Cases

- A non-anonymous user declares **both** `keychain` and `cmd` → rejected as ambiguous (exactly one source required).
- A non-anonymous user declares **neither** keychain nor cmd (and is not anonymous) → rejected as missing credentials.
- The `--config` path does not exist → clear "config not found at <path>" error (the first-run empty-config behavior applies only to the default path's "no connections yet" state; an explicitly-named missing file is an error — see FR-017).
- The `--config` path exists but is group/world-writable → the existing owner-only gate that protects the `cmd` source must still apply to whichever file is selected.
- An anonymous user is preserved and still requires no secret.
- The no-echo interactive prompt fallback still runs (before the TUI takes the terminal) and still offers to persist the entered secret into the keychain.
- A keychain secret exists but the keystore is locked at launch → the OS unlock prompt occurs before the TUI starts; a denied/failed unlock is a clear error, not an empty secret.
- A `cmd` source whose command exits non-zero, times out, or prints nothing → clear error, never an empty secret.

## Requirements *(mandatory)*

### Functional Requirements

#### Credential sources

- **FR-001**: The system MUST support exactly two credential sources for a non-anonymous user: the OS keychain and an external command (`cmd`).
- **FR-002**: A non-anonymous user MUST declare exactly one of the two sources; declaring zero or both MUST be a validation error with a clear message.
- **FR-003**: The system MUST remove the inline `secretAccessKey` source entirely (both the literal and the `${ENV}` form) — the field is not part of the accepted, env-resolved, or validated schema.
- **FR-004**: The system MUST remove `${ENV}` reference resolution **for the secret** — there is no env-reference step in secret resolution; the secret comes only from the keychain or the cmd's stdout. (Env resolution for non-secret fields, if any, is out of scope of this change.)
- **FR-005**: The system MUST remove the `awsProfile` source and the inline `sessionToken` field entirely from the schema, resolver, and validation.
- **FR-006**: The system MUST preserve the anonymous user (no secret) — it is not a secret method and is unaffected by this change.
- **FR-007**: The system MUST preserve the no-echo interactive prompt fallback, including its offer to persist the entered secret into the keychain, as the onboarding ramp.
- **FR-008**: A stored secret MUST NOT be written to any s3s config file in plaintext under any of the supported sources.
- **FR-008a**: The `cmd` source MUST yield a single secret value (its trimmed stdout); it MUST NOT carry a session token in this feature. STS/temporary-credential support is out of scope (deferred to a separate credential_process-JSON enhancement).

#### Removal posture (no migration)

- **FR-009**: Because s3s is pre-release with no users, the system MUST NOT provide a migration command, migration messaging, or backward-compatible loading for the removed sources — they are simply deleted.
- **FR-010**: A non-anonymous user that declares none of the two supported sources MUST fail validation via the standard "missing credentials — declare keychain or cmd" path (no special migration error). A removed field present in a config is treated as an unknown/ignored field, leaving the user with no source and thus failing FR-002.
- **FR-011**: An `apiVersion` change is OPTIONAL and informational only; it MUST NOT gate loading on migration logic (there is nothing to migrate).

#### Config-path override

- **FR-012**: The system MUST accept a `--config <path>` command-line flag that selects the config file for the invocation.
- **FR-013**: The system MUST accept an `S3S_CONFIG` environment variable that selects the config file.
- **FR-014**: The config-path precedence MUST be: `--config` flag > `S3S_CONFIG` env > default XDG path (`~/.config/s3s/config.yaml`).
- **FR-015**: The selected config path MUST apply consistently across the TUI launch, the `cred` subcommands, and config init/generate — all read and write the same selected file.
- **FR-016**: The selected config path MUST compose with the existing active-context precedence (`--context` flag > `S3S_CONTEXT` env > `current-context`), which resolves against the selected config.
- **FR-017**: Selecting a non-existent explicit config path MUST produce a clear "config not found" error (the empty/first-run state applies to the default path only).
- **FR-018**: The owner-only config-permissions gate that guards the `cmd` source MUST apply to whichever config file is selected.
- **FR-018a**: Config selection MUST be a launch-time concern only — switching the active config file from inside the running TUI is out of scope; a user relaunches with a different `--config`/`S3S_CONFIG`.

#### Cross-platform keychain

- **FR-019**: The keychain source MUST work on macOS (login Keychain), Windows (Credential Manager), and Linux/BSD desktop (Secret Service via GNOME Keyring / KWallet) through the same single `keychain: true` field.
- **FR-020**: When the OS keystore is unavailable (e.g. headless Linux with no Secret Service), keychain resolution MUST emit a clear, actionable error pointing the user at the `cmd` source, and MUST NOT fall back to reading or writing any plaintext secret.
- **FR-020a**: The keychain account MUST be namespaced by a config-identity component derived from the resolved config path (in addition to the context name), so two configs selected via `--config`/`S3S_CONFIG` that share a context name do not collide on a single keychain entry. The `s3s cred set|rotate|rm` lifecycle MUST operate on the same namespaced account for the selected config.

#### Documentation

- **FR-021**: Documentation MUST cover the keychain source with a per-OS backend table, the `s3s cred set|rotate|rm <context>` lifecycle, and the headless-Linux caveat.
- **FR-022**: Documentation MUST cover the `cmd` source contract (stdout is the secret, argv never a shell, owner-only config-perms gate, bounded timeout) with ready recipes for HashiCorp Vault, 1Password (`op`), `pass`, `sops`, `secret-tool`, and the macOS `security` command.

### Key Entities

- **User (credential)**: A named credential in the config. After this change, a non-anonymous user references exactly one of: `keychain` (boolean) or `cmd` (command line). The `accessKeyId` (non-secret) remains. The fields `secretAccessKey`, `sessionToken` (inline), and `awsProfile` are removed from the accepted schema.
- **Config selection**: The resolved config file path for an invocation, derived from flag > env > default; the single file all config reads/writes target.
- **Keychain entry**: A secret held in the OS keystore, identified by an account that combines a config-identity component (derived from the resolved config path) with the context name, so entries from different configs never collide.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A non-anonymous user can be configured with a secret using only one of two sources (keychain or cmd); no third secret source is selectable anywhere in the product.
- **SC-002**: The removed sources (inline `secretAccessKey`/`${ENV}`, inline `sessionToken`, `awsProfile`) are absent from the schema, wizard, resolver, and validation; a non-anonymous user without a `keychain`/`cmd` source fails the standard missing-credentials validation. No migration code path exists.
- **SC-003**: The same `keychain: true` configuration results in a working secret on macOS, Windows, and Linux-desktop without any per-OS field differences.
- **SC-004**: On a headless box with no keystore, the user receives a single clear error naming the `cmd` alternative, and no plaintext secret is ever read or written.
- **SC-005**: A user can run s3s against an alternate config via `--config` or `S3S_CONFIG`, and the default config remains byte-for-byte unchanged after the session.
- **SC-006**: Flag-over-env-over-default precedence for the config path is observable and deterministic in 100% of combinations.
- **SC-007**: Every supported source has copy-pasteable documentation; a new user can configure either source from the docs without reading source code.
- **SC-008**: The structural read-only guard remains green and no secret appears in logs or on-screen output (existing safety invariants preserved).

## Assumptions

- The OS keychain library in use exposes the three desktop backends (macOS Keychain, Windows Credential Manager, Linux Secret Service) and has no encrypted-file fallback on headless Linux — hence the loud-error requirement rather than a silent fallback.
- "Remove" (выпилить) means the fields are deleted from the schema, resolver, and validation — not merely de-emphasized; the goal is one clear scheme.
- s3s is pre-release with **no users**, so no migration path, migration command, or backward-compatible loading is needed for the removed sources (confirmed in Clarifications). A removed field left in a hand-written config is simply an unknown/ignored field, leaving that user with no valid source and failing the standard missing-credentials validation.
- The anonymous user and the no-echo prompt fallback are not "secret sources" in the sense being reduced and are therefore retained.
- The config-path override is a launch/CLI concern; switching configs at runtime from inside the TUI is out of scope for this feature (a user relaunches with a different `--config`).
- An `apiVersion` change, if made, is informational only; there is no requirement to load both old and new schema versions.
- The `cmd` source yields a single secret value only; session-token / STS support is deferred (a later credential_process-JSON enhancement).
- The existing `cred` subcommand, owner-only gate, argv-not-shell execution, and bounded command timeout are retained as-is for the `cmd` source.

## Out of Scope

- Runtime switching of the active config file from within the TUI (relaunch-based only).
- An encrypted-file keystore fallback for headless Linux (the `cmd` source is the headless answer).
- Re-introducing AWS `credential_process` JSON parsing for `cmd` (tracked separately in ROADMAP as an optional enhancement; may be revisited if the US2 clarification favors it for the awsProfile migration).
- Any change to the four read-only storage methods or the structural read-only guard.
