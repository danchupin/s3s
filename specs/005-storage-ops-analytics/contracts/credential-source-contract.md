# Contract: Credential sources & secret handling (US6)

Resolution lives in `internal/secret` (UI- and SDK-agnostic). Config declares exactly one source
per user; the secret is resolved to build the client and is never persisted to an s3s file.

## C1 — One source per user (FR-041)

A `config.User` MUST declare **exactly one** of:
`secretAccessKey` (inline / `${ENV}`), `keychain: true`, `cmd: "<argv>"`, `awsProfile: "<name>"`.

- Zero of these on a non-anonymous user → existing "missing credentials" validation error.
- Two or more → **new** `config.Validate` error: "choose exactly one credential source" (FR-041).
- Anonymous users declare none (unchanged).

**Tests**: each single-source config validates; any two-source combo fails with the one-source
error; anonymous unaffected; `${ENV}` inline still validates + resolves (FR-042).

## C2 — Resolution semantics

`ResolveSecret(ctx, src, accessKeyID) (ResolvedCredential, error)`:

| Source | Behavior |
|--------|----------|
| inline | passthrough the (already env-resolved) secret; `${ENV}` works as today (FR-042) |
| keychain | `go-keyring.Get(service="s3s", account=<context>)`; absent keystore → clear error (FR-043) |
| cmd | **perms-gate first** (C3), then `exec` argv, stdout (trimmed) = secret (FR-036) |
| awsProfile | INI-parse `~/.aws/credentials` (`AWS_SHARED_CREDENTIALS_FILE` honored) → static keys (+ token) |

- A resolved secret is wrapped in `logging.Secret` and **MUST NOT** be written to any s3s file
  (FR-035) or appear in logs/UI/errors (FR-039).
- Unavailable/empty result → actionable error; **MUST NOT** connect with an empty secret (FR-043).
- The secure prompt (C4) is the implicit fallback only when a configured source resolves nothing.

**Tests**: fake keystore returns/omits a secret; command resolver captures stdout + trims newline;
aws-profile parser reads fixture files (present/missing/static-less profile); every path keeps the
secret redacted (assert `logging.Secret` String()/log output).

**Integration**: a non-env source (keychain/command/awsProfile) resolves to a `ClientConfig` that
authenticates against a real MinIO backend (`//go:build integration`) — Constitution IV.

## C3 — `cmd:` owner-only gate (FR-036)

Before executing a `cmd:` source, stat the config file; **refuse** (clear message, no exec) if it
is group/world writable OR not owned by the running uid. Run argv directly (not via `sh -c`).
A short `ctx` timeout bounds a hung command. The `cmd:` string is split into argv with POSIX
shell-words rules (quotes/escapes honored, no shell expansion); an unparseable string is a config
error.

**Tests**: 0600 owner-owned config → command runs; 0666 / group-writable → refused with the
insecure-perms reason; the command line is never logged as secret material; a quoted-argument
command (e.g. `op read "op://vault/s3 prod/secret"`) splits into the correct argv.

## C4 — Secure prompt & keystore management

- Prompt uses no-echo (`x/term.ReadPassword`) and runs **before** the TUI starts (Constitution V —
  the TUI owns the terminal). After a successful prompt, offer to save into the keystore (FR-038).
- In-TUI **context switch** resolves non-interactive sources only; a switch needing a prompt shows
  a "relaunch to enter this context's secret" notice (no terminal corruption).
- `s3s cred set|rotate|rm <context>` writes/removes the secret in the keystore **only** — never the
  config file (FR-037).
- The keychain wizard path stores the secret in the keystore instead of printing an `export` line
  (FR-041a).

**Tests**: `cred set/rotate/rm` round-trip against a fake keystore; wizard keychain path stores +
references without writing the secret to the YAML; config-perms warning fires on a loose-perms
file (FR-040).
