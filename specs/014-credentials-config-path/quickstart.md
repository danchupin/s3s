# Quickstart: Secrets & Multiple Configs

s3s stores a context's S3 secret in exactly one of two places. The secret is **never** written
to the config file.

## Option 1 — OS keychain (default, recommended)

The convenient secure default. Same `keychain: true` everywhere; the OS provides the store:

| OS | Backed by |
|----|-----------|
| macOS | login Keychain |
| Windows | Credential Manager |
| Linux desktop | Secret Service (GNOME Keyring / KWallet) |

```yaml
users:
  - name: prod
    accessKeyId: AKIAPROD
    keychain: true
```

Manage the secret:
```bash
s3s cred set prod      # store / first time   (no-echo prompt)
s3s cred rotate prod   # replace
s3s cred rm prod       # delete
```
Or just launch — if the keystore has no entry, s3s prompts once (no echo) and offers to save it.

> **Headless Linux** (no Secret Service / D-Bus): the keychain is unavailable and s3s tells you to
> use a `cmd` source. It never falls back to a plaintext secret.

## Option 2 — External command (`cmd`)

The escape hatch for headless boxes and anyone who already runs a secret manager. The command's
**stdout** is the secret (argv, never a shell; the config must be `chmod 600`; 10s timeout):

```yaml
users:
  - name: prod
    accessKeyId: AKIAPROD
    cmd: "vault kv get -field=secret s3/prod"
```

Ready recipes:
```bash
vault kv get -field=secret s3/prod          # HashiCorp Vault
op read "op://Private/s3-prod/secret"        # 1Password CLI
pass show s3/prod                            # pass
sops -d --extract '["secret"]' creds.yaml    # sops
secret-tool lookup service s3s account prod  # libsecret
security find-generic-password -w -s s3s -a prod   # macOS
```

## Multiple configs

Keep separate configs (work vs personal, prod vs staging) and pick one per launch:

```bash
s3s --config ~/.config/s3s/work.yaml
S3S_CONFIG=~/.config/s3s/staging.yaml s3s
s3s cred set prod --config ~/.config/s3s/work.yaml
s3s config init --config ~/.config/s3s/personal.yaml
```

Precedence: `--config` flag > `S3S_CONFIG` env > default `~/.config/s3s/config.yaml`. An explicitly
named missing file is an error (the empty first-run state is only for the default path). Keychain
entries are isolated per config, so two configs that both define a `prod` context never share a
secret. Switching configs is relaunch-only.
