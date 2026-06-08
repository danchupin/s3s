# Contract: `cred` Subcommand & Wizard

## `s3s cred <set|rotate|rm> <context> [--config <path>]`

- Resolves the config path via `ConfigPath(flag, S3S_CONFIG)`.
- Loads the config, computes the namespaced keystore account via `cfg.KeychainAccount(context)`.
- `set`/`rotate`: no-echo prompt → `StoreKeychain(account, secret)`.
- `rm`: `RemoveKeychain(account)`.
- Operates on the OS keystore ONLY — never the config file (read-only guarantee intact).

## `offerSaveToKeychain` (post interactive prompt at launch)

- TTY-gated. Uses `cfg.KeychainAccount(active)` → same namespaced account as `cred` and resolution.
- `[y/N]` prompt; best-effort store; failures reported, non-fatal.

## `s3s config init [--config <path>]` (wizard)

- Resolves the write path via `ConfigPath`.
- Credential-source prompt: `[keychain/cmd]`, default **keychain**.
  - keychain → ask `accessKeyId`, no-echo secret, `StoreKeychain(keychainAccount(path, user), sec)`.
  - cmd → ask `accessKeyId`, ask command line; stored as `cmd:` in config.
- No `env`/`awsProfile` branches; no `export S3S_*_SECRET` hint; `EnvVarName` helper deleted.

## Acceptance

1. `s3s config init` with no source answer (Enter) → user written with `keychain: true`.
2. `s3s config init` answering `cmd` → user written with `cmd:`; no secret on disk.
3. `s3s cred set prod --config /x` then `s3s --context prod --config /x` → resolves without prompt.
4. Launch with empty keystore on a TTY → prompt → answer `y` → secret saved under the namespaced
   account → next launch resolves silently.
