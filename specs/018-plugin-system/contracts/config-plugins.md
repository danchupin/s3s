# Contract: Config Extension — `plugins:` Section

**Feature**: 018-plugin-system

## YAML schema

```yaml
plugins:                                  # optional; absent ⇒ feature dormant
  - name: avito-bucket-discovery          # required, unique
    capability: bucket-discovery          # required: bucket-discovery | object-metadata
    cmd: "s3s-avito-discovery --cluster prod"   # required; shlex rules, argv exec
    timeout: 5s                           # optional; default 5s; > 0
    enabled: true                         # optional; default true
    connections: [prod-rgw, stage-rgw]    # discovery: required, ≥1 context name

  - name: image-storage-meta
    capability: object-metadata
    cmd: "s3s-image-meta"
    timeout: 3s
    match:                                # metadata: required
      connections: [prod-rgw]             # required, ≥1
      buckets: ["images-*"]               # optional globs; empty ⇒ any
      keyPattern: "^[0-9a-f]{32}"         # optional RE2; must compile
```

## Validation matrix (config load)

| Rule | Violation handling |
|------|--------------------|
| `name` unique, non-empty | load error |
| `capability` ∈ {bucket-discovery, object-metadata} | load error |
| `cmd` non-empty | load error |
| `timeout` parseable, > 0 | load error |
| discovery ⇒ `connections` ≥ 1; metadata ⇒ `match.connections` ≥ 1 | load error |
| `keyPattern` compiles (RE2) | load error |
| scope names an unknown connection | **warning** → plugin `unavailable` (config shared across machines must stay loadable) |
| executable missing at invocation time | runtime → `unavailable` status, no crash |

## Defaults

| Field | Default |
|-------|---------|
| `timeout` | `5s` |
| `enabled` | `true` |
| `match.buckets` | any bucket |
| `match.keyPattern` | any key |

## Persistence operations

The in-app toggle (status surface, `space`) persists through the `Connector` port:

```go
// SetPluginEnabled flips the enabled flag of the named plugin declaration in the
// config file and returns the refreshed plugin declarations. Config mutation only —
// no keychain access, no storage calls. Idempotent.
SetPluginEnabled(ctx context.Context, name string, enabled bool) ([]config.PluginDecl, error)
```

Mirrors `AddBucket` (010): executed off the event loop in a `tea.Cmd`, optimistic UI
update on the success message, transient error notice on failure, atomic config write
(temp + rename) like every existing config mutation.

## Security invariants

- Declared commands run only when the config file passes the owner-only-writable check
  (same gate as the credential `cmd` source). Group/world-writable config ⇒ plugins
  refuse to run with a clear status reason.
- s3s adds no credential material to the child environment or argv; the request JSON
  contains the access key ID only (public identifier).
- Config continues to never store secrets (unchanged 014 invariant).
