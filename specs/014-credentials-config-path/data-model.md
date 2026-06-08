# Data Model: Credential Sources Simplification & Config-Path Override

The config schema is kubectl-style YAML (`clusters` / `users` / `contexts` / `current-context`).
This feature changes the **User** entity and introduces two derived concepts.

## User (credential) — AFTER

A named credential. A non-anonymous user declares **exactly one** of two sources.

| Field | Type | Status | Notes |
|-------|------|--------|-------|
| `name` | string | KEPT | unique; also the keystore account base (pre-namespace) |
| `anonymous` | bool | KEPT | no secret; not a "source" |
| `accessKeyId` | string | KEPT | non-secret; `${ENV}` still resolved (FR-004 scopes removal to the secret) |
| `keychain` | bool | KEPT | source #1 — secret in OS keystore |
| `cmd` | string | KEPT | source #2 — argv; stdout is the secret |
| ~~`secretAccessKey`~~ | ~~Secret~~ | **DELETED** | inline literal + `${ENV}` |
| ~~`sessionToken`~~ | ~~Secret~~ | **DELETED** | inline STS token |
| ~~`awsProfile`~~ | ~~string~~ | **DELETED** | `~/.aws/credentials` profile |

**Validation (`Config.Validate` → `sourceCount`)**: for a non-anonymous user, exactly one of
`keychain | cmd`. Zero → "missing credentials — declare keychain or cmd". Both → "more than one
source". `accessKeyId` required (no source supplies its own key anymore, so the awsProfile
exemption at config.go:235 is removed). Error message lists only `keychain | cmd`.

**Example (after)**:
```yaml
users:
  - name: prod            # keychain (default)
    accessKeyId: AKIAPROD
    keychain: true
  - name: vault           # external command
    accessKeyId: AKIAVLT
    cmd: "vault kv get -field=secret s3/prod"
  - name: public
    anonymous: true
```

## Config selection — NEW (derived, not persisted)

The resolved config file path for one invocation.

| Property | Rule |
|----------|------|
| Source precedence | `--config` flag > `S3S_CONFIG` env > `DefaultPath()` (XDG `~/.config/s3s/config.yaml`) |
| Explicit | `flag != "" || env != ""` — when true, a missing file is a hard error (FR-017) |
| Default | when neither set — a missing file is the first-run empty-config state |
| Scope | applies identically to TUI launch, `cred` subcommands, and `config init` |
| Carrier | `Config.path` (set by `Load`/`Empty`) — downstream owner-gate + keychain namespacing read it |

Composes with the unchanged active-context precedence: `--context` > `S3S_CONTEXT` >
`current-context`, resolved against the selected config.

## Keychain entry — namespaced key

A secret held in the OS keystore under service `"s3s"`, account =
`configIdentity(configPath) + ":" + userName`.

| Component | Derivation |
|-----------|------------|
| service | constant `"s3s"` (unchanged, `keychain.go:12`) |
| `configIdentity` | `base64url(sha256(filepath.Abs(configPath)))[:8]` |
| `userName` | the user's `name` (== context name == connection name in the add-connection triple) |

**Invariant**: the same `(configPath, userName)` MUST map to the same account at all five call
sites (read on resolve, read/write/remove via `cred`+`offer`, write on add, delete on remove, write
on wizard). A mismatch silently hides a stored secret.

**Cross-platform backing** (same `keychain: true`, one code path via `zalando/go-keyring`):

| OS | Store |
|----|-------|
| macOS | login Keychain (`/usr/bin/security`) |
| Windows | Credential Manager (`danieljoos/wincred`) |
| Linux/BSD | Secret Service over D-Bus (GNOME Keyring / KWallet) |
| headless / no keystore | unavailable → loud error toward `cmd` (no plaintext fallback) |

## secret.Kind enum — AFTER

| Value | Status |
|-------|--------|
| ~~`Inline`~~ | DELETED |
| `Keychain` | KEPT (now iota 0) |
| `Command` | KEPT |
| ~~`AWSProfile`~~ | DELETED |

`Resolved.SessionToken` field DELETED (only `awsProfile` populated it). `storage.ClientConfig`
retains its `SessionToken` field (storage-layer; now always zero from the config path).
