# Contract: Config & Flag Delta (enable writes)

**Packages**: `internal/config`, `cmd/s3s` | **Feature**: 002-write-foundation

The only operator-facing surface for enabling writes. Implements the clarified
opt-out model (Session 2026-06-05): global `--write` switch, per-context `readonly`
override.

## `--write` flag

```
s3s            # read-only (default; today's behaviour, unchanged)
s3s --write    # writes enabled for contexts not marked readonly
```

- Default `false`. Boolean, no value. Ephemeral (not persisted) — a deliberate,
  per-invocation safety gate (FR-001).
- Threaded from `main.go` into the resolver closure as `writeFlag`.
- Independent of `--context`/`--config`/`--version`.

## `readonly` context field

```yaml
contexts:
  - name: prod
    cluster: ceph-prod
    user: ro
    readonly: true        # NEW: refuses all mutations even under --write (FR-002)
  - name: local
    cluster: minio-local
    user: dev
    # readonly absent ⇒ false ⇒ writable when --write is passed
```

- Optional `bool`, yaml key `readonly`, default `false`.
- Placed on the **context** (not cluster/user) so the same cluster can be writable
  under one context and protected under another.
- No other config change. `apiVersion`, clusters, users, precedence
  (`--context` > `S3S_CONTEXT` > `current-context`) all unchanged.

## Resolution

The existing `Resolve(name) (Cluster, User, error)` and `ClientConfig(name)
(storage.ClientConfig, error)` are **unchanged**. Write policy is added as a
**separate** method so no current caller breaks:

```go
type WritePolicy struct { Writable bool }

// WritePolicyFor returns the write policy for the named context.
// Writable == writeFlag && !context.ReadOnly.
func (c *Config) WritePolicyFor(name string, writeFlag bool) (WritePolicy, error)
```

`main.go` threads the `--write` flag into the resolver closure, which calls
`WritePolicyFor` and wraps the backend with `storage.Guard(backend, policy)`.

Truth table:

| `--write` | context `readonly` | `Writable` | Backend |
|:---:|:---:|:---:|---|
| off | (any) | `false` | read-only guard |
| on | `false`/absent | `true` | writable |
| on | `true` | `false` | read-only guard |

## Behaviour

- A mutation attempt when `Writable == false` yields the read-only hint /
  `ErrReadOnly` (FR-003) — never a silent no-op.
- Switching context re-runs resolution; the resolver re-wraps (or unwraps) the new
  backend accordingly, so per-context read-only is honoured live (FR-002).

## Test contract

- **Unit**: `WritePolicyFor` truth table (all four rows); absent `readonly` ⇒
  `false`; malformed `readonly` value ⇒ validation error at load.
- **Unit**: a context marked `readonly: true` yields `Writable == false` even with
  `writeFlag == true` (SC-002).
- **Integration/UI**: launching without `--write` makes every context refuse
  mutations; with `--write`, a non-readonly context permits create-folder and a
  `readonly` context still refuses.
