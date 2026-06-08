# Contract: Config-Path Resolution

## API

```go
// internal/config
const EnvConfig = "S3S_CONFIG"

// ConfigPath applies the precedence: --config flag > S3S_CONFIG env > DefaultPath().
func ConfigPath(flag, env string) string
```

Call sites (all three entrypoints):
- `cmd/s3s/main.go` `run()` — replaces the `if cfgPath == "" { cfgPath = DefaultPath() }` block.
- `cmd/s3s/main.go` `runConfigInit()` — same.
- `cmd/s3s/cred.go` `runCred()` — same.

Each computes `explicit := flag != "" || env != ""` for the not-found rule below.

## Behavior

| Given | flag | S3S_CONFIG | Result path |
|-------|------|------------|-------------|
| flag only | `/a.yaml` | — | `/a.yaml` |
| env only | "" | `/b.yaml` | `/b.yaml` |
| both | `/a.yaml` | `/b.yaml` | `/a.yaml` (flag wins) |
| neither | "" | "" | `DefaultPath()` |

## Not-found rule (FR-017)

| Path origin | File missing → |
|-------------|----------------|
| explicit (flag or env set) | hard error `config: file not found: <path>` (no first-run) |
| default (neither set) | first-run empty-config state (in-app add-connection) |

## Acceptance

1. `s3s --config /x` with `/x` absent → exits with "config not found", no TUI.
2. `S3S_CONFIG=/y s3s` with `/y` present → loads `/y`; default config untouched.
3. `--config` and `S3S_CONFIG` both set, different files → flag's file loaded.
4. `s3s cred set prod --config /z` and `s3s config init --config /z` operate on `/z`.
5. Active context resolves against the selected config: `--context` > `S3S_CONTEXT` > its `current-context`.
