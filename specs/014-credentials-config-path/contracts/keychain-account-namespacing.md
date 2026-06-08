# Contract: Keychain Account Namespacing

## API

```go
// internal/config (unexported helpers)
func configIdentity(configPath string) string   // base64url(sha256(filepath.Abs(path)))[:8]
func keychainAccount(configPath, userName string) string // configIdentity(path) + ":" + userName
```

`secret.{Get,Store,Remove}Keychain(account string, …)` signatures are UNCHANGED — the config layer
passes the already-namespaced account (Constitution I: `secret` stays config-agnostic).

## The five call sites (MUST all use the same helper)

| # | Function | File:line | Account expression |
|---|----------|-----------|--------------------|
| 1 | `User.secretRequest` | resolve.go:77 | `keychainAccount(configPath, u.Name)` as `Ref` |
| 2 | `Config.KeychainAccount` | resolve.go:110 | `keychainAccount(c.path, cx.User)` |
| 3 | `Config.AddConnection` | connection.go:70 | `keychainAccount(c.path, nc.Name)` |
| 4 | `Config.RemoveConnection` | connection.go:180 | `keychainAccount(c.path, name)` |
| 5 | `RunInit` wizard | generate.go:162 | `keychainAccount(path, userName)` |

Site 2 is the one `cred set|rotate|rm` and `offerSaveToKeychain` use, so those agree automatically.

## Invariants

- Deterministic: same `(abs configPath, userName)` → same account, across processes and runs.
- Isolation: two configs at different paths with the same `userName` → different accounts.
- Readability: account renders as `a1b2c3d4:prod` in the OS keystore UI.
- Separator safety: `:` cannot appear in the base64url id, so the boundary is unambiguous.

## Acceptance

1. Store secret for context `prod` under config A; load config B (same `prod` context name) → B's
   resolution does NOT find A's secret (distinct accounts).
2. `s3s cred set prod --config A` then launch `s3s --config A` → resolves the stored secret.
3. `RemoveConnection` deletes exactly the namespaced entry it created via `AddConnection`.
