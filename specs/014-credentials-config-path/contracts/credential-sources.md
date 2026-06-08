# Contract: Credential Sources (two only)

## Accepted schema (non-anonymous user)

Exactly one of:
- `keychain: true` (+ `accessKeyId`) — secret in the OS keystore.
- `cmd: "<command>"` (+ `accessKeyId`) — argv executed, trimmed stdout is the secret.

Plus `anonymous: true` (no secret). The fields `secretAccessKey`, `sessionToken`, `awsProfile`
are NOT part of the schema.

## Validation

| Case | Outcome |
|------|---------|
| keychain only | valid |
| cmd only | valid |
| neither (non-anonymous) | error: "user %q missing credentials — declare keychain or cmd" |
| both keychain and cmd | error: "more than one credential source — choose exactly one (keychain | cmd)" |
| anonymous | valid (no source) |
| keychain/cmd without accessKeyId | error: "user %q is missing accessKeyId" |
| stale `secretAccessKey`/`awsProfile`/`sessionToken` present | unknown field ignored → falls to "missing credentials" |

## Resolution (`secret.Resolve`)

| Source | Mechanism | Session token |
|--------|-----------|---------------|
| keychain | `GetKeychain(namespacedAccount)` | none |
| cmd | argv (never `sh -c`), owner-only config gate, 10s timeout, trimmed stdout | none |
| prompt fallback | no-echo `secret.Prompt` pre-TUI; offers save-to-keychain | none |

`cmd` yields a single secret only (FR-008a). No source writes a secret to the s3s config.

## Keychain availability (FR-020)

When the OS keystore is unavailable (e.g. headless Linux, no Secret Service): `GetKeychain` returns
a clear error naming the `cmd` source as the remedy. No plaintext fallback. The prompt is TTY-gated.

## Wizard (`config init`)

Prompt: `Credential source [keychain/cmd]`, default **keychain**. Invalid/empty input → keychain.
The `env` and `awsProfile` options are removed. No `export S3S_*_SECRET` hint is printed.
