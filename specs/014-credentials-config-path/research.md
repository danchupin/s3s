# Research: Credential Sources Simplification & Config-Path Override

Phase 0 design decisions. Each is grounded in the current code (file:line verified) and the
40+ tool survey recorded in `ROADMAP.md` → "Credentials & security".

## R1 — Which sources stay, which go

**Decision**: Keep `keychain` (default) + `cmd`. Delete inline `secretAccessKey` (literal +
`${ENV}`), inline `sessionToken`, and `awsProfile`. Keep anonymous + the no-echo prompt fallback.

**Rationale**: The tool landscape converged on keychain-as-default + external-command-as-hatch
(Cyberduck, TablePlus, gh, pgcli; AWS `credential_process`/git `credential.helper`/docker
`credsStore`/kubectl `exec` are one shared contract). Plaintext/`${ENV}`/profile is the legacy,
de-recommended camp. Confirmed by the survey synthesis (workflow, 41 verified tools).

**Alternatives rejected**: Keeping `awsProfile` — narrowest audience, security-neutral
(`~/.aws/credentials` is itself plaintext), and `credential_process` is already out of scope for
it (`internal/secret/awsprofile.go:24-26`), so it only covers the static-keys slice, reachable via
`cmd`. Keeping `${ENV}` — leaks via `/proc`, child procs, shell history, CI logs.

## R2 — Inline Kind is fully removable

**Decision**: Delete `secret.Inline` and `secret.AWSProfile` from the `Kind` enum and `Resolve()`.

**Rationale**: The only paths that produced `Inline` were `secretRequest`(resolve.go:82, deleted)
and config inline secrets (deleted). The interactive prompt fallback uses
`ClientConfigWithSecret(name, sec string)` (resolve.go:91-106), which sets `SecretKey: sec`
directly and **never** builds a `secret.Request{Kind: Inline}`. Verified by reading resolve.go +
main.go:153-164. So removing `Inline` breaks no runtime path.

**Note**: Removing enum members shifts iota values (`Keychain` becomes 0). Values are never
persisted or serialized (they exist only at resolution time), so the shift is safe.

## R3 — Keychain account namespacing scheme

**Decision**: `keychainAccount(configPath, userName) = configIdentity(configPath) + ":" + userName`,
where `configIdentity(path) = base64url(sha256(filepath.Abs(path)))[:8]`.

**Rationale**: Two configs (selected via `--config`/`S3S_CONFIG`) may each define a context/user
named `prod`. Today the keystore account is the bare user name (`resolve.go:77` `Ref: u.Name`;
`resolve.go:110` returns `cx.User`; `connection.go:70` `StoreKeychain(nc.Name, …)`), so the two
would share one keystore entry. Prefixing with a path-derived id isolates them. SHA-256 of the
**absolute** path is deterministic and portable; an 8-char base64url prefix is short, collision-safe
for realistic config counts, and keeps the `security`/Credential-Manager UI readable
(`a1b2c3d4:prod`). The `:` separator never appears in a sha256-base64url id, so parsing is
unambiguous.

**Alternatives rejected**: basename (collides across dirs); full absolute path (long, leaks dir
structure into the keystore label); a user-declared per-config id (extra schema field — rejected in
clarify Q2 in favor of automatic derivation).

**Critical consistency**: the SAME helper MUST be applied at **all five** keystore call sites or a
secret stored via one path is invisible to another. The codebase map listed three; two more were
found by direct read:

| # | Call site | File:line | Today | After |
|---|-----------|-----------|-------|-------|
| 1 | resolution (read) | `resolve.go:77` | `Ref: u.Name` | `Ref: keychainAccount(configPath, u.Name)` |
| 2 | `cred`/offer (read+write+remove) | `resolve.go:110` | returns `cx.User` | returns `keychainAccount(c.path, cx.User)` |
| 3 | add (write) | `connection.go:70` | `StoreKeychain(nc.Name, …)` | namespaced |
| 4 | remove (delete) | `connection.go:180` | `RemoveKeychain(name)` | namespaced |
| 5 | wizard (write) | `generate.go:162` | `StoreKeychain(userName, …)` | namespaced |

`Config.path` is set by `Load`/`Empty` (config.go:148/127) and reachable in every Config-method
site; `secretRequest`(2 is a `User` method) and `RunInit`(5) already receive the path as a param.
**No public signature changes** — `secret.{Get,Store,Remove}Keychain` keep `account string`
(preserves Constitution I: the `secret` package stays config-agnostic).

## R4 — Config-path override precedence + explicit-not-found

**Decision**: `config.ConfigPath(flag, env) string` = flag > env > `DefaultPath()`. Add
`const EnvConfig = "S3S_CONFIG"`. A non-existent **explicit** path (flag or env non-empty) is a hard
error; only the **default** path keeps the first-run empty-config behavior (FR-017).

**Rationale**: `--config` already exists (main.go:53, cred.go:20, runConfigInit:197); only
`S3S_CONFIG` + precedence are new. The active-context precedence (`ActiveContextName`,
resolve.go:16) is the established mirror — `ConfigPath` follows the same flag>env>default shape and
lives beside it. Today `run()` (main.go:67-75) treats `ErrNotFound` as first-run for ANY path;
that's wrong once a user explicitly names a missing file — they want an error, not a silent empty
config. Gate with `explicit := flag != "" || env != ""`.

**Alternatives rejected**: separate `LoadExplicit`/`Load` functions (more surface; the boolean at
the single call site is simpler). A `S3S_CONFIG_PATH` alias (spec says `S3S_CONFIG`).

**Threading**: all three entrypoints (`run`, `runConfigInit`, `runCred`) call `ConfigPath` →
`Load`/`Empty` sets `c.path` → every downstream keystore/owner-gate use is automatically the chosen
path (`secretRequest(c.path)` resolve.go:138; namespacing from `c.path`).

## R5 — Headless keychain loud-fail

**Decision**: `GetKeychain` (keychain.go:20) already returns `ErrNoKeystore` when the OS keystore is
unavailable; enrich that message to name the `cmd` source as the remedy (FR-020). No silent
plaintext fallback exists or is added.

**Rationale**: `zalando/go-keyring` v0.2.8 has no file fallback — on headless Linux (no Secret
Service D-Bus) it errors (`keyring_unix.go` build-tag `linux`; `keyring_fallback.go:9`
`ErrUnsupportedPlatform`). The prompt fallback is correctly TTY-gated (main.go:149), so a headless
launch surfaces the error rather than hanging. Wording: point to a `cmd` source (and `${ENV}` is
gone, so don't suggest it).

## R6 — No migration

**Decision**: No migration command, no compat loading, no apiVersion gate. Removed fields just
disappear from the struct.

**Rationale**: clarify Q1 — s3s is pre-release with no users. YAML unmarshal ignores unknown fields,
so a stale `awsProfile:`/`secretAccessKey:` leaves the user with no source → standard
missing-credentials validation error (config.go Validate). Zero migration code to maintain.

## R7 — Cleanup fallout

**Decision**: After removing the secret fields, audit and drop now-unused imports/helpers:
- `EnvVarName()` (generate.go:101-116) — only caller was the deleted env branch → delete.
- `logging` import in `generate.go` — only use was `logging.Secret("${…}")` in the env branch → drop.
- `logging` import in `config.go` — was used by `SecretAccessKey`/`SessionToken logging.Secret`
  fields; after removal, confirm no other use (resolveEnv built `logging.Secret(sk)` for the secret;
  that block goes) and drop if unused. `go build`/`go vet` will flag any miss.
- The `${ENV}` machinery (`envRef` regex, `resolveRef`) **stays** — still used to resolve a
  non-secret `accessKeyId: ${VAR}` (FR-004 scopes the removal to the secret only).
- `storage.ClientConfig.SessionToken` field **stays** (storage layer; harmless zero value now).

## R8 — TDD order (Constitution III)

Write failing tests first, per user story:
- **US1/US2**: `secret_test.go` delete Inline tests; `source_test.go` update exact-one-source to
  keychain|cmd, delete sessionToken test; `config_test.go` validYAML → keychain (mock keyring),
  delete env-unset test; `generate_test.go` delete env/EnvVarName tests, add keychain-default test.
- **US3**: new `resolve_test.go` cases — `ConfigPath` precedence (flag>env>default); `run` explicit
  non-existent path errors while default first-runs.
- **US4**: new namespacing test — two temp config paths + same context name → distinct keystore
  accounts (mock keyring); `keychain_test.go` account uses namespaced value.
- Delete `awsprofile_test.go` entirely.
- Keep green: `command_test.go`, `keychain_test.go` (roundtrip), `cred_test.go`,
  `cred_auth_integration_test.go`, `connection_integration_test.go` (update asserted account to
  namespaced).
