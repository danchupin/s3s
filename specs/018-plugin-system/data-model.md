# Data Model: Plugin System for External Capability Providers

**Feature**: 018-plugin-system | **Date**: 2026-06-11

## Entities

### PluginDecl (config, `internal/config`)

One declared plugin. Source of truth: YAML `plugins:` list.

| Field | Type | Rules |
|-------|------|-------|
| `Name` | string | required; unique across the list; shown in UI and logs |
| `Capability` | string enum | required; `bucket-discovery` \| `object-metadata` |
| `Cmd` | string | required non-empty; shlex-split at invocation; never `sh -c` |
| `Timeout` | duration | optional; default `5s`; must be > 0 |
| `Enabled` | *bool | optional; default `true`; persisted by the in-app toggle |
| `Connections` | []string | discovery only: required, ≥ 1 connection name |
| `Match` | *MatchRule | metadata only: required |

### MatchRule (config)

Scope filter deciding which objects trigger an enrichment invocation.

| Field | Type | Rules |
|-------|------|-------|
| `Connections` | []string | required, ≥ 1 |
| `Buckets` | []string | optional globs; empty ⇒ any bucket |
| `KeyPattern` | string | optional RE2; must compile at config load; empty ⇒ any key |

An object `(connection, bucket, key)` matches iff connection ∈ Connections AND
(Buckets empty OR any glob matches bucket) AND (KeyPattern empty OR it matches key).

### Request envelope (`internal/plugin`)

Written to plugin stdin as a single JSON object.

| Field | Type | Notes |
|-------|------|-------|
| `ContractVersion` | int | always `1` |
| `Capability` | string | the declared capability being invoked |
| `Connection.Name` | string | context name |
| `Connection.Endpoint` | string | cluster endpoint URL |
| `Connection.UserLabel` | string | config user name (label, not credential) |
| `Connection.AccessKeyID` | string | public identifier; **secret key never present** (clarified 2026-06-11) |
| `Target.Bucket` | string | object-metadata only |
| `Target.Key` | string | object-metadata only |

### Response envelope (`internal/plugin`)

Read from plugin stdout (≤ 1 MiB), single JSON object.

| Field | Type | Notes |
|-------|------|-------|
| `ContractVersion` | int | must equal 1, else `incompatible` |
| `Buckets` | []string | bucket-discovery success payload |
| `Fields` | []Field `{Name, Value string}` | object-metadata success payload; order preserved |
| `Error` | string | soft failure; mutually exclusive with payload |

Validation: exactly one of payload/`Error` present for the declared capability;
anything else ⇒ `invalid_output`.

### Invocation (runtime, `internal/plugin`)

One request/response exchange.

| Field | Type |
|-------|------|
| `Plugin` | string |
| `Capability` | string |
| `Target` | connection / bucket / key |
| `Started`, `Duration` | time.Time / time.Duration |
| `Outcome` | enum: `ok` \| `timeout` \| `exec_error` \| `contract_error` \| `invalid_output` |
| `ErrDetail` | string — sanitized, ≤ 200 chars retained |

### PluginStatus (runtime, UI model)

Last-known operational state per declared plugin; feeds the status surface.

| Field | Type |
|-------|------|
| `Decl` | PluginDecl summary (name, capability, scope text) |
| `Enabled` | bool (live, persisted via Connector) |
| `Availability` | enum: `ready` \| `unavailable` (missing executable / unknown connection) \| `incompatible` (version mismatch) |
| `Last` | *Invocation (nil until first run) |

### DiscoveryResult (runtime)

| Field | Type | Notes |
|-------|------|-------|
| `Names` | []string | sanitized, validated, ≤ 5 000 (cap indicated) |
| `Discarded` | int | invalid names dropped (FR-019), shown in notice |

Merged for display: `union(pinned, listed?, discovered…)`, dedup, sort — pure function,
unit-tested.

### EnrichmentResult (runtime)

| Field | Type | Notes |
|-------|------|-------|
| `Plugin` | string | group attribution (`From <plugin>`) |
| `Fields` | []Field | sanitized; ≤ 64 fields, values ≤ 4 096 B, truncation marked |

## Cache keys (session, App-owned maps)

- discovery: `(contextName, pluginName)` → DiscoveryResult
- enrichment: `(contextName, pluginName, bucket, key)` → EnrichmentResult

Invalidation: manual refresh (`r`) for the affected view; context switch clears that
context's entries. No TTL. Stale in-flight results dropped by generation check before
the cache is ever written.

## State machine (per plugin, drives status surface)

```text
            ┌─────────────┐ config load: missing exe / unknown connection
            │ unavailable │◄────────────────────────────────┐
            └─────────────┘                                  │
declared ──► ready ──invoke──► running ──► ok ───────────────┤ (Last updated)
   │            ▲                  │  └──► failed(timeout/exec/invalid) ─► ready
   │            │                  └─────► incompatible (version) ─► disabled-like terminal
   └─ enabled=false ─► disabled ◄─ toggle (space, persisted) ─► ready
```

- `disabled` and `unavailable` plugins are never invoked (no scope match evaluation even).
- `incompatible` requires a config/plugin fix; re-checked on config reload (app restart).
- All transitions logged (D12); UI states text-distinct (NO_COLOR-safe).

## Validation summary (config load)

1. Unique `Name` across `plugins:` — duplicate ⇒ load error.
2. Known `Capability` — unknown ⇒ load error.
3. Non-empty `Cmd`; shlex-splittable (checked lazily at invocation like the credential
   cmd source — parse failure ⇒ `exec_error`).
4. Discovery ⇒ `Connections` non-empty; metadata ⇒ `Match.Connections` non-empty —
   violation ⇒ load error.
5. `KeyPattern` compiles ⇒ else load error.
6. Scope referencing an unknown connection name ⇒ warning → `unavailable` status (not
   fatal: shared configs across machines).
7. `Timeout` > 0 ⇒ else load error.
