# Contract: In-app connection manager (US4)

New `modeConnections` (list) and `modeConnForm` (add-form). Reaches existing
config/secret/storage primitives only through an injected `Connector` seam — no
S3 or config-marshalling logic enters `internal/ui` (Constitution I).

## CM1 — Seam (injected by `cmd/s3s/main.go`)

```go
type ConnDraft struct {
    Name, Endpoint, Region, AccessKeyID string
    Secret   logging.Secret
    ReadOnly bool
}
type Connector interface {
    Test(ctx context.Context, d ConnDraft) error            // reachability via storage.New + ListBuckets
    Save(ctx context.Context, d ConnDraft) ([]string, error) // persist; returns updated context-name list
}
```

`ui.New` gains a `Connector` param (nil disables the feature, like a nil
`Resolver`). All calls run in `tea.Cmd`s; results arrive as `connTestedMsg` /
`connSavedMsg` (Constitution II).

## CM2 — Form fields & validation (FR-021)

Fields: display name, endpoint, region, access key id, secret access key
(no-echo), read-only flag. Pre-save validation:
- `Name`, `Endpoint` required; `Endpoint` MUST parse as an absolute URL.
- Derived context/cluster/user names MUST NOT collide with existing entries
  (FR-024) — reject without overwrite.
- Field-level error messages; save blocked until valid.

## CM3 — Test + override (FR-025a)

- On save, `Test` runs first (off-loop). Result shown.
- Test **success** → proceed to save.
- Test **failure** → the form offers an explicit "save anyway"; it MUST NOT
  silently persist a failing connection, and MUST NOT block a deliberate offline
  save. Failure is reported secret-free.

## CM4 — Save mapping & order (FR-022/FR-022a/FR-023/FR-026/FR-027)

`Save` (in `internal/config`/`main` closure, UI-agnostic):
1. Derive `Cluster{Name,Endpoint,Region}`, `User{Name,AccessKeyID,Keychain:true}`,
   `Context{Name,Cluster,User,ReadOnly}` from the draft (schema unchanged).
2. `secret.StoreKeychain(KeychainAccount, secret)` **first**. If it fails → abort
   before touching config (no context points at a missing secret).
3. `config.Upsert(cl,u,cx,false)` then `config.Save(path, Marshal(cfg))`. Preserve
   all existing clusters/users/contexts/settings (no data loss).
4. Return the updated context-name slice.

Invariants:
- The secret access key is **never** written to config in plaintext — only
  `keychain: true` (FR-022/FR-005, constitution secret rule).
- On any failure, report a clear secret-free error; never claim success. A partial
  state (keychain set, config not) is harmless (orphan entry, overwritten on
  retry); the reverse is prevented by ordering.

## CM5 — Session refresh (FR-025)

On `connSavedMsg{names}` the UI replaces `m.contexts` with `names`. Because
`main.go`'s `resolve` closure captures the same mutated `*config.Config`, switching
to the new context resolves immediately — no restart.

**Tests** (white-box UI + config/secret units + one integration):
- form validation blocks empty name/endpoint and bad URL; duplicate name rejected.
- test-failure path surfaces "save anyway"; choosing it persists.
- `Save` writes a `keychain:true` user and NO plaintext secret (assert on marshaled
  config); existing contexts preserved.
- keychain store called with the right account (mock keyring).
- new context appears in `m.contexts` and is switchable in-session.
- integration: add against MinIO → `Test` passes → switch → buckets list.
