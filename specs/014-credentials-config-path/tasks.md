---
description: "Task list for feature 014 — credential sources simplification + config-path override"
---

# Tasks: Credential Sources Simplification & Config-Path Override

**Input**: Design documents from `specs/014-credentials-config-path/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: INCLUDED — TDD is non-negotiable (Constitution III). Failing tests precede implementation.

**Organization**: By user story. US2 (removal) precedes US1 (only-two outcome); US4 (namespacing)
depends on US3 (config-path). MVP = US1 + US2 (the two-source scheme on a single config).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable (different file, no incomplete dependency)
- **[Story]**: US1..US5 from spec.md
- Exact file paths included

## Path note

Go module `github.com/danchupin/s3s`. Edits live in `internal/config`, `internal/secret`,
`internal/ui`, `cmd/s3s`, plus README/ROADMAP. No new package, no new source file. Tests are
white-box (same package), keystore mocked via `keyring.MockInit()`.

---

## Phase 1: Setup

**Purpose**: baseline + reuse patterns.

- [x] T001 Confirm baseline `make test` is green on branch `014-credentials-config-path`, and capture the `keyring.MockInit()` + `StoreKeychain` test pattern from `internal/config/connection_test.go` for reuse in rewritten tests.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: simplify the `internal/secret` package — the Go whole-package compile root that the
resolver and validation build against. MUST complete before US1/US2/US4 compile.

- [x] T002 [P] Delete `internal/secret/awsprofile.go` entirely (AWS profile source removed).
- [x] T003 [P] Delete `internal/secret/awsprofile_test.go` entirely (its `TestAWSProfile` exercises the removed `awsProfile` function).
- [x] T004 In `internal/secret/secret.go`: remove the `Inline` and `AWSProfile` constants from the `Kind` enum, delete their `case`s in `Resolve()`, delete the `SessionToken` field from `Resolved`, and update the package/`Request`/`Resolved` doc comments to name only `Keychain`/`Command`.
- [x] T005 In `internal/secret/secret_test.go`: delete `TestResolveInlineRedacts` and `TestResolveEmptyInline` (they assert the removed `Inline` kind); keep command/keychain coverage.
- [x] T006 In `internal/secret/keychain.go`: enrich the unavailable-keystore error in `GetKeychain` (around line 26) to name the `cmd` source as the actionable remedy (FR-020 headless loud-fail); keep `Get/Store/RemoveKeychain` signatures (`account string`) unchanged (Constitution I).

**Checkpoint**: `go build ./internal/secret/...` green; `secret` package exposes only Keychain + Command.

> **Build-coupling note (Go whole-package compile)**: deleting `secret.Inline`/`AWSProfile` in T004 leaves `internal/config/resolve.go` referencing those constants (and `config.go` referencing the deleted `User` fields) until US2 lands. So `go build ./...` (whole module) is EXPECTED RED from T004 until T008 **and** T011 complete — they form one atomic compile unit. Treat Foundational + US2's T008/T010/T011 as a single landing; only the scoped `./internal/secret/...` build is green at this checkpoint. Do not run a full-module build between them and expect green.

---

## Phase 3: US2 — Removed sources gone from the schema/resolver/validation (Priority: P1)

**Goal**: inline `secretAccessKey` (literal + `${ENV}`), inline `sessionToken`, and `awsProfile`
are absent from the config struct, env-resolution, resolver, and validation.

**Independent test**: a non-anonymous user with neither `keychain` nor `cmd` fails the standard
missing-credentials validation; the removed fields are not part of the accepted schema.

- [x] T007 [P] [US2] In `internal/config/source_test.go`: in `TestValidateExactlyOneSource` drop the `awsProfile` and inline `secretAccessKey` cases and add a `cmd` case; delete `TestSessionTokenPreservedNonProfile`. (TDD: encodes the new validation surface — red until T008/T009.)
- [x] T008 [US2] In `internal/config/config.go`: delete `User.SecretAccessKey` (line 70), `User.SessionToken` (71), `User.AWSProfile` (74); update the struct doc comment (62-65) to name only keychain/cmd.
- [x] T009 [US2] In `internal/config/config.go`: update `sourceCount()` (79) to count only `Keychain`+`Command`; update `Validate()` (193) to require exactly one of keychain|cmd, drop the awsProfile `accessKeyId` exemption (235), and rewrite the error message to list `keychain | cmd`.
- [x] T010 [US2] In `internal/config/config.go`: update `resolveEnv()` (154) to remove `${ENV}` resolution for the deleted secret/sessionToken fields while keeping it for `accessKeyId`; drop the now-unused `logging` import if `go build` flags it.
- [x] T011 [US2] In `internal/config/resolve.go`: in `secretRequest()` (74) delete the `AWSProfile` (80-81) and inline `SecretAccessKey` (82-83) cases; in `ClientConfig()` (123) delete the sessionToken-preservation block (148-154); in `ClientConfigWithSecret()` (91) drop the `SessionToken.Reveal()` (104). **Closes the build-coupling window** opened by T004/T008 — land together with T008/T010 so `go build ./...` returns green.
- [x] T012 [P] [US2] In `internal/config/config_test.go`: rewrite `validYAML` (line ~23) from `secretAccessKey: ${S3S_TEST_SECRET}` to `keychain: true` + `accessKeyId`; add `keyring.MockInit()` + `StoreKeychain` setup in `TestLoadValid`; delete `TestLoadEnvUnset`; adapt `TestSecretRedactedInConfig` to the new fixture.
- [x] T013 [P] [US2] In `internal/config/resolve_test.go`: rewrite `TestResolveClientConfig` to use a mock keyring + stored secret instead of the env source.
- [x] T013a [P] [US2] Add a regression test (in `internal/config/resolve_test.go` or `config_test.go`) asserting an **anonymous** user still resolves a working (no-secret) `ClientConfig` after the removal — FR-006 (anonymous preserved).
- [x] T013b [US2] Add a regression test asserting a config carrying a **stale removed field** (e.g. `awsProfile: x` or `secretAccessKey: y`) and no `keychain`/`cmd` fails with the standard missing-credentials error (unknown field ignored → FR-010 edge case).

**Checkpoint**: `go build ./...` green; `go test ./internal/config/... ./internal/secret/...` green; removed fields no longer exist; anonymous + missing-source paths verified.

---

## Phase 4: US1 — Only two sources; wizard default keychain (Priority: P1)

**Goal**: the wizard offers only keychain (the pre-selected default) and cmd; both authenticate.

**Independent test**: run `s3s config init`, press Enter at the source prompt → user written with
`keychain: true`; a keychain context and a cmd context each resolve a working backend.

**Depends on**: US2 (the removed branches must be gone).

- [x] T014 [P] [US1] In `internal/config/generate_test.go`: delete `TestRunInitWritesEnvRefNotSecret` and `TestEnvVarName`; add `TestRunInitKeychainDefault` (Enter → `keychain: true`, secret stored in mock keyring) and a cmd-source case. (TDD red.)
- [x] T015 [US1] In `internal/config/generate.go`: change the source prompt (154) to `Credential source [keychain/cmd]` default `keychain`; delete the `awsprofile` and `env` branches; make keychain the `default:` branch (invalid/empty input → keychain); delete `EnvVarName()` (101-116), the `envVar` variable, and the `export S3S_*_SECRET` hint block; drop the now-unused `logging` import.
- [x] T016 [US1] In `internal/ui/connections.go`: simplify the secret-field hint in `connFieldHint` (line ~572) from "stored in your OS keychain · env var / cmd / AWS profile via config file" to keychain-only wording (note `cmd` as the alternative source).
- [x] T017 [P] [US1] In `internal/ui/connections_test.go`: update `TestConnFormSecretGuidance` to the simplified hint text.
- [x] T017a [US1] Add a regression test in `cmd/s3s/cred_test.go` asserting the no-echo prompt fallback + `offerSaveToKeychain` path stays wired: with a mock keyring, `offerSaveToKeychain` stores under the account returned by `cfg.KeychainAccount` (FR-007 preserved; pairs with the namespacing in T025). The interactive `secret.Prompt` itself stays manual-verify (main.go `run()` is TTY-bound).

**Checkpoint**: wizard offers exactly keychain/cmd with keychain default; prompt-fallback save path verified; `go test ./internal/config/... ./internal/ui/... ./cmd/...` green. **MVP (US1+US2) deliverable.**

---

## Phase 5: US3 — Config-path override (Priority: P2)

**Goal**: `--config` flag > `S3S_CONFIG` env > default XDG path, applied to TUI launch, `cred`, and
`config init`; an explicitly named missing file errors.

**Independent test**: launch and run `cred`/`init` against an alternate config via flag and env;
the default config stays untouched; flag beats env beats default.

- [x] T018 [P] [US3] In `internal/config/resolve_test.go`: add a `TestConfigPathPrecedence` table asserting flag > env > `DefaultPath()`. (TDD red.)
- [x] T019 [US3] In `internal/config/resolve.go`: add `const EnvConfig = "S3S_CONFIG"` and `func ConfigPath(flag, env string) string` returning flag > env > `DefaultPath()`.
- [x] T020 [US3] In `cmd/s3s/main.go` `run()`: at the path block (61-63), FIRST capture `explicit := cfgPath != "" || os.Getenv(config.EnvConfig) != ""` from the RAW flag/env **before** any reassignment (the flag var is `cfgPath`, not `cfgFlag`; `ConfigPath` overwrites it so `explicit` MUST be read first or it is always true). THEN `cfgPath = config.ConfigPath(cfgPath, os.Getenv(config.EnvConfig))`. On `config.ErrNotFound`: return a hard error when `explicit`; keep the first-run empty-config path only when NOT explicit (default path) — FR-017. Log line at 98 already records the resolved `config`.
- [x] T021 [P] [US3] In `cmd/s3s/main.go` `runConfigInit()` (201-204) and `cmd/s3s/cred.go` `runCred()` (30-33): apply the same `config.ConfigPath(rawFlag, os.Getenv(config.EnvConfig))` resolution, capturing the raw flag value before reassignment (same ordering rule as T020). `config init` writes the file, so it needs no explicit-not-found gate; `cred` loads it, so a non-existent explicit path errors via the normal `config.Load` failure.
- [x] T022 [P] [US3] Add a test (`cmd/s3s` or `internal/config`) asserting an explicit non-existent `--config`/`S3S_CONFIG` errors while the default path first-runs (FR-017).
- [x] T022a [P] [US3] Add a test asserting the active-context precedence (`--context` > `S3S_CONTEXT` > `current-context`) resolves against the **selected** config (load an alt config via path, confirm its `current-context`/overrides win) — FR-016.
- [x] T022b [P] [US3] Add a test asserting that running against an alternate `--config`/`S3S_CONFIG` leaves the default config file **byte-for-byte unchanged** (hash before/after, or assert no write to the default path) — SC-005.

**Checkpoint**: all three entrypoints honor the override; active-context resolves against the chosen config; default config untouched; `go test ./cmd/... ./internal/config/...` green.

---

## Phase 6: US4 — Cross-platform keychain, headless loud-fail, namespacing isolation (Priority: P2)

**Goal**: same `keychain: true` works on macOS/Windows/Linux-desktop; headless fails loudly toward
`cmd`; multiple configs with same-named contexts keep isolated keychain entries.

**Independent test**: store a secret for context `prod` under config A; loading config B (also with a
`prod` context) does not see A's secret — distinct namespaced accounts.

**Depends on**: US3 (the resolved config path is the namespace input).

- [x] T023 [P] [US4] Add `TestKeychainAccountIsolation` (in `internal/config/resolve_test.go` or `connection_test.go`): two temp config paths + same user/context name → distinct keychain accounts via mock keyring. (TDD red.)
- [x] T024 [US4] In `internal/config/resolve.go`: add unexported `configIdentity(path string) string` = `base64url(sha256(filepath.Abs(path)))[:8]` and `keychainAccount(configPath, userName string) string` = `configIdentity(path) + ":" + userName`.
- [x] T025 [US4] Apply `keychainAccount` at all FIVE keystore sites: `secretRequest` Ref (`resolve.go:77`), `KeychainAccount` (`resolve.go:110`), `AddConnection` (`connection.go:70`), `RemoveConnection` (`connection.go:180`), and the wizard keychain branch (`generate.go:162`). Verify all derive from the same resolved config path.
- [x] T026 [P] [US4] Update namespaced-account assertions in `internal/config/connection_test.go` (`TestAddConnectionMapsTripleNoPlaintext`, `TestRemoveConnection*`) and `cmd/s3s/cred_test.go` (`TestCredRemoveKeystoreOnly`).
- [x] T027 [P] [US4] Update `cmd/s3s/connection_integration_test.go` (asserted `keyring.Get` account) and `internal/storage/cred_auth_integration_test.go` to store/read under the namespaced account so the kept keychain auth flow stays green.
- [x] T028 [US4] Add a unit test asserting `GetKeychain`'s unavailable-store error message names the `cmd` remedy (FR-020), or assert the wording on the existing `ErrNoKeystore` path.

**Checkpoint**: multi-config isolation proven; `go test ./...` green; integration (keychain auth) green.

---

## Phase 7: US5 — Documentation (Priority: P3)

**Goal**: first-class docs for the two kept sources + the config-path override.

**Independent test**: a new user can configure keychain or cmd, and run multiple configs, from the
docs alone.

- [x] T029 [P] [US5] Rewrite the `README.md` "Credential sources" section + config examples: 4 sources → 2 (keychain default + cmd), with a per-OS keychain backend table, the `s3s cred set|rotate|rm` lifecycle, the headless caveat, and cmd recipes (vault/op/pass/sops/secret-tool/security); remove the `awsProfile` and `${ENV}` examples; document `--config`/`S3S_CONFIG` precedence.
- [x] T030 [P] [US5] In `ROADMAP.md`: move the "Credentials & security" items to the Done section referencing feature 014 (the `quickstart.md` already lives under `specs/014-credentials-config-path/`).

---

## Phase 8: Polish & Cross-Cutting

- [x] T031 Run `make fmt vet lint`; resolve unused-import/lint issues (notably `logging` in `config.go`/`generate.go`).
- [x] T032 Run `make check-readonly` — confirm green (no write-capable S3 symbol introduced).
- [x] T033 Run `make test` and `make test-integration` (MinIO keychain auth flow) — all green.
- [x] T034 [P] Final sweep against Success Criteria SC-001..008: two sources only, removed-source validation, cross-OS keychain field, headless error, multi-config isolation, config-path precedence, docs complete, read-only guard + secret-redaction intact. Note: cross-OS keychain (FR-019/SC-003) is **inherited** from `zalando/go-keyring` (one code path, three backends) and verified on the CI OS via the keychain integration test — full tri-OS verification is out of single-OS CI scope; document this in the sweep.

---

## Dependencies & Execution Order

```
Setup (T001)
  └─ Foundational (T002-T006)        # secret-package removal — blocks all
       └─ US2 (T007-T013)            # P1 — removal from config/resolver/validation
            └─ US1 (T014-T017)       # P1 — only-two outcome + wizard default   ── MVP (US1+US2)
                 └─ US3 (T018-T022)  # P2 — config-path override
                      └─ US4 (T023-T028)  # P2 — namespacing isolation (needs resolved path) + headless
                           └─ US5 (T029-T030)  # P3 — docs
                                └─ Polish (T031-T034)
```

- **US2 → US1**: the removed branches must be gone before the "only two" outcome + tests.
- **US3 → US4**: keychain namespacing keys on the resolved config path.
- **US5/Polish** last.

## Parallel Opportunities

- Foundational: T002 + T003 (delete two independent files).
- US2: T012 + T013 + T013a (different test files); T007 in parallel with T008-T011 once written (red-first); T013b sequential (shares config_test.go).
- US1: T014 + T017 (test files) parallel to the impl edits; T017a after T015.
- US3: T021 + T022 + T022a + T022b parallel after T019/T020 (independent test files).
- US4: T026 + T027 (different test files) parallel after T024/T025.
- US5: T029 + T030 (README vs ROADMAP).

## Implementation Strategy

**MVP = US1 + US2** (P1): the two-source scheme on a single (default) config — keychain default +
cmd, all removed sources gone. Shippable on its own. Keychain account stays the bare user name at
MVP; US4 introduces config-identity namespacing (which only matters once US3's multi-config override
lands). Because s3s is pre-release with no users, the MVP→US4 account-key change needs no migration.

Deliver incrementally: MVP (US1+US2) → US3 (multi-config) → US4 (isolation + headless) → US5 (docs).
