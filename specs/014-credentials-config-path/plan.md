# Implementation Plan: Credential Sources Simplification & Config-Path Override

**Branch**: `014-credentials-config-path` | **Date**: 2026-06-08 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/014-credentials-config-path/spec.md`

## Summary

Reduce the four credential sources to **two** — OS keychain (blessed default) and an
external command (`cmd`) — by deleting the inline `secretAccessKey` (literal + `${ENV}`),
inline `sessionToken`, and `awsProfile` sources from the schema, resolver, validation,
and wizard. No migration is provided (pre-release, no users). Add a **config-path
override** (`--config` flag > `S3S_CONFIG` env > default XDG path) that all entrypoints
(TUI, `cred`, `config init`) honor, and **namespace keychain accounts by config identity**
so multiple configs with same-named contexts never collide. Flip the wizard default from
`env` to `keychain`, and make keychain unavailability (headless Linux) fail loudly toward
`cmd` instead of any plaintext fallback. Constitution v1.2.0 already encodes this posture.

## Technical Context

**Language/Version**: Go 1.25 (per go.mod)

**Primary Dependencies**: `charm.land/bubbletea/v2` + `lipgloss/v2` (TUI), `zalando/go-keyring`
v0.2.8 (OS keystore: macOS Keychain via `/usr/bin/security`, Windows Credential Manager via
`danieljoos/wincred`, Linux/BSD Secret Service via `godbus/dbus`), `go.yaml.in/yaml/v3`,
`aws-sdk-go-v2/service/s3` (isolated in `internal/storage`).

**Storage**: Local YAML config (kubectl-style) + OS keystore for secrets. No DB.

**Testing**: `go test` white-box unit tests with `keyring.MockInit()` for keystore;
`//go:build integration` MinIO testcontainers for the credential/auth flow.

**Target Platform**: macOS, Windows, Linux/BSD desktops (keychain); headless Linux (cmd path).

**Project Type**: Single-binary CLI/TUI (Go module `github.com/danchupin/s3s`).

**Performance Goals**: N/A (config-load + one keystore/exec call at launch; not hot-path).

**Constraints**: Secret never written to the s3s config in plaintext; secrets redacted in
logs; TUI owns the terminal so any interactive unlock/prompt happens **before** the program
starts; `check-readonly` guard stays green (no write-S3 symbol added).

**Scale/Scope**: ~10 source files edited, 2 deleted; ~12 test files touched. All changes in
`internal/config`, `internal/secret`, `cmd/s3s`, plus doc updates (README, ROADMAP) and one
UI hint string (`internal/ui/connections.go`).

## Constitution Check

*GATE: re-checked after design below. Constitution v1.2.0.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Core/UI Separation | ✅ PASS | All logic stays in `internal/config` + `internal/secret` (UI-agnostic). The `secret` package stays config-agnostic: the **config layer** computes the namespaced account string and passes it in; `secret.{Get,Store,Remove}Keychain` keep their `account string` parameter unchanged (no new config import). |
| II. Non-Blocking TUI | ✅ PASS | Credential resolution already runs pre-TUI (`main.go:147`) or off-loop on context switch. No new in-loop I/O. |
| III. Test-First | ✅ PASS (required) | Every change is TDD: write failing tests first (namespacing isolation, config-path precedence, explicit-not-found error, wizard keychain default, removed-source rejection), then implement. See Phase notes. |
| IV. Integration Testing | ✅ PASS | The kept keychain auth flow is already covered by `internal/storage/cred_auth_integration_test.go::TestIntegrationKeychainSourceAuthenticates`. The auth **mechanism** is unchanged (keychain→secret→client); only the account **key** gains a namespace, which is a unit-level concern. No storage-contract change → no new integration test mandated; the existing one must stay green. |
| V. Observability & Safe Operations | ✅ PASS (strengthened) | Removes plaintext/`${ENV}`/profile sources → fewer on-disk-secret paths. Headless keychain failure becomes a loud actionable error (FR-020). Secrets stay redacted; logging stays file-only. |
| VI. UI Legibility | ✅ N/A | One hint-string simplification (`connections.go:572`); no layout/visibility change. |
| VII. UI Consistency | ✅ N/A | No new prompt/label patterns or hues. |

**Constitution amendment**: already done this iteration — Technology & Security Constraints
credential bullet rewritten (env/AWS-profile/prompt → keychain/cmd/prompt; never plaintext in
config; headless loud-fail toward cmd), version **1.1.0 → 1.2.0**. No further amendment needed.

**Read-only guard**: `scripts/check-readonly.sh` stays green — this feature only **removes**
credential code and adds CLI/keystore-account logic; no write-capable S3 symbol is introduced.

No violations → **Complexity Tracking is empty.**

## Project Structure

### Documentation (this feature)

```text
specs/014-credentials-config-path/
├── plan.md              # This file
├── spec.md              # Feature spec (clarified)
├── research.md          # Phase 0 — design decisions
├── data-model.md        # Phase 1 — entities after the change
├── quickstart.md        # Phase 1 — keychain + cmd + multi-config usage
├── contracts/           # Phase 1 — CLI/behavior contracts
│   ├── config-path-resolution.md
│   ├── credential-sources.md
│   ├── keychain-account-namespacing.md
│   └── cred-and-wizard.md
└── checklists/requirements.md   # 16/16 (from /speckit-specify + /speckit-clarify)
```

### Source Code (repository root)

```text
internal/config/
├── config.go        # User struct: DELETE SecretAccessKey(70), SessionToken(71), AWSProfile(74);
│                    #   sourceCount()(79) → count keychain+cmd; Validate()(193) error msg → "keychain | cmd";
│                    #   resolveEnv()(154) → drop secret/sessionToken ${ENV} (keep accessKeyId); drop AWSProfile accessKeyId exemption(235)
├── resolve.go       # NEW EnvConfig const + ConfigPath()/ConfigPathExplicit() helper; NEW keychainAccount()/configIdentity();
│                    #   secretRequest()(74) → drop AWSProfile(80)+Inline(82) cases, namespace keychain Ref(77);
│                    #   KeychainAccount()(110) → namespaced; ClientConfig()(123) → drop sessionToken block(148-154);
│                    #   ClientConfigWithSecret()(91) → drop SessionToken.Reveal()(104)
├── connection.go    # AddConnection StoreKeychain(70) + RemoveConnection RemoveKeychain(180) → namespaced account
├── generate.go      # RunInit wizard(154): default env→keychain, options [keychain/cmd], DELETE awsprofile+env cases;
│                    #   namespace StoreKeychain(162); DELETE EnvVarName()(101) + export-hint block + envVar
internal/secret/
├── secret.go        # Kind enum: DELETE Inline(22)+AWSProfile(25); Resolve()(46) drop those cases; drop Resolved.SessionToken(33)
├── awsprofile.go        # DELETE entire file
├── awsprofile_test.go   # DELETE entire file
├── keychain.go      # GetKeychain unavailable-keystore error(26) → point at cmd source (FR-020); funcs otherwise unchanged
cmd/s3s/
├── main.go          # run()(61-63) + runConfigInit()(201-204): config path via ConfigPath(flag, env); explicit-not-found → hard error (FR-017)
├── cred.go          # runCred()(30-33): same ConfigPath resolution
README.md, ROADMAP.md  # docs: 4 sources → 2; ROADMAP "Credentials & security" → Done
internal/ui/connections.go  # connFieldHint(572): drop "env var / cmd / AWS profile" → keychain-only wording
```

**Structure Decision**: Single Go module, existing package layout. No new package, no new file
(except spec docs). The namespacing helper and config-path resolver live in
`internal/config/resolve.go` next to the existing `ActiveContextName`/`KeychainAccount`.

## Key design decisions (detail in research.md)

1. **Keychain account namespacing without signature churn.** `Config.path` is already set by
   `Load`/`Empty`, so all keystore call sites already have the resolved path. Add a package-level
   `keychainAccount(configPath, userName) = configIdentity(configPath) + ":" + userName` and apply
   it at **all five** keystore call sites (the codebase map found three; two more exist):
   `secretRequest`(resolve.go:77), `KeychainAccount`(resolve.go:110), `AddConnection`(connection.go:70),
   `RemoveConnection`(connection.go:180), and the **wizard** keychain branch (generate.go:162).
   `configIdentity = base64url(sha256(filepath.Abs(path)))[:8]` — deterministic, portable, short.
2. **`Inline` Kind is fully removable.** The interactive prompt fallback uses
   `ClientConfigWithSecret(name, sec string)` (resolve.go:91), which injects the raw secret string
   directly — it never constructs `secret.Request{Kind: Inline}`. So deleting `Inline` breaks nothing.
3. **Config-path precedence + explicit-not-found.** `config.ConfigPath(flag, env)` returns
   flag > env > `DefaultPath()`. `run()` must treat a non-existent **explicit** path (flag or env
   set) as a hard error, while only the **default** path keeps the first-run empty-config behavior
   (FR-017). Tracked via an `explicit := flag != "" || env != ""` boolean at the call site.
4. **No migration code.** Removed fields vanish from the struct; YAML unmarshal silently ignores
   them, leaving the user with no source → the standard missing-credentials validation error fires.
   No `s3s config migrate`, no version gate.
5. **Headless loud-fail.** `GetKeychain`'s keystore-unavailable branch (keychain.go:26) gains an
   actionable message naming the `cmd` source. The prompt fallback stays TTY-gated.

## Phase 0: Research → `research.md`

Resolved unknowns: config-identity derivation scheme; namespacing call-site inventory (5);
Inline-removability proof; config-path explicit-vs-default error model; headless error wording;
unused-import cleanup (`logging` in config.go/generate.go after secret fields go).

## Phase 1: Design & Contracts

- `data-model.md`: the post-change `User` entity (keychain|cmd|anonymous), config-selection, and
  the namespaced keychain-entry key.
- `contracts/`: config-path resolution, credential-sources validation, keychain-account
  namespacing, and the `cred`/wizard behavior.
- `quickstart.md`: set up keychain + cmd; run multiple configs via `--config`/`S3S_CONFIG`.
- Agent context: update the `<!-- SPECKIT -->` block in `CLAUDE.md` to point at this plan.

## Complexity Tracking

*No constitution violations — section intentionally empty.*
