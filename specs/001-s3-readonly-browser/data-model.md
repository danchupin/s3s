# Phase 1 Data Model: Read-Only S3 Browser (TUI)

**Feature**: 001-s3-readonly-browser | **Date**: 2026-06-04

Entities are derived from the spec's Key Entities and Functional Requirements. All are in-memory or
config-file structures; there is no persistent application datastore.

---

## Config domain (kubectl-style, persisted in `~/.config/s3s/config.yaml`)

### Config

Top-level file root.

| Field | Type | Notes |
|-------|------|-------|
| `apiVersion` | string | e.g. `s3s/v1`; forward-compat marker |
| `clusters` | []Cluster | named endpoints |
| `users` | []User | named credentials |
| `contexts` | []Context | named cluster+user bindings |
| `current-context` | string | name of the default Context |

**Validation**: `current-context` MUST reference an existing Context; each Context's `cluster`/
`user` MUST reference existing entries; names unique within their list.

### Cluster

| Field | Type | Notes |
|-------|------|-------|
| `name` | string | unique key |
| `endpoint` | string | URL of Ceph RGW / MinIO (scheme + host[:port]) |
| `region` | string | default `us-east-1` if empty |
| `pathStyle` | bool | true=path-style, false=virtual-host/domain-style (FR-003) |
| `tlsSkipVerify` | bool | default false; explicit opt-in only (FR-004) |

**Validation**: `endpoint` MUST be a valid absolute URL. `tlsSkipVerify=true` only meaningful for
`https` endpoints.

### User (Credential)

| Field | Type | Notes |
|-------|------|-------|
| `name` | string | unique key |
| `anonymous` | bool | if true, no signing; ignore key fields (FR-005a) |
| `accessKeyId` | string | inline or `${ENV_VAR}` reference |
| `secretAccessKey` | string | inline or `${ENV_VAR}` reference; never logged (FR-005) |
| `sessionToken` | string | optional session/STS token (FR-005a) |

**Validation**: either `anonymous: true` OR (`accessKeyId` AND `secretAccessKey`) present.
Environment values override inline values (FR-005). Secrets are redacted in all logs/displays.

### Context

| Field | Type | Notes |
|-------|------|-------|
| `name` | string | unique key; selectable via flag/env/in-app (FR-002) |
| `cluster` | string | references Cluster.name |
| `user` | string | references User.name |

**Precedence for active context (FR-002)**: explicit `--context` flag > `S3S_CONTEXT` env >
`current-context` in config. In-app switcher changes the active context at runtime (FR-002, SC-005).

---

## Browsing domain (runtime, in-memory)

### Bucket

| Field | Type | Notes |
|-------|------|-------|
| `Name` | string | listed for active context (FR-006) |
| `CreationDate` | time.Time | optional, when provided by backend |

### Level

A loaded view of one tree node = the listing of a (bucket, prefix) at the current depth using
delimiter `/`.

| Field | Type | Notes |
|-------|------|-------|
| `Bucket` | string | |
| `Prefix` | string | "" at bucket root; ends with `/` for sub-levels |
| `Dirs` | []string | common prefixes (child directories) (FR-007) |
| `Objects` | []ObjectRef | objects at this level (keys without further `/`) |
| `NextToken` | *string | continuation token; nil when fully loaded (FR-010) |
| `Complete` | bool | true when no more pages |
| `LoadedAt` | time.Time | for display; cache has no TTL (FR-011) |

**State / lifecycle**:
- `empty` → first page requested → `partial` (NextToken != nil) → more pages on demand →
  `complete`.
- Manual refresh (FR-011a) discards the Level and re-fetches from `empty`.
- Search replaces the Level with a prefix-narrowed query (FR-017); clearing search restores the
  unfiltered Level (FR-018).

### ObjectRef

Lightweight entry shown in a level (from ListObjectsV2 contents).

| Field | Type | Notes |
|-------|------|-------|
| `Key` | string | full key |
| `Size` | int64 | bytes |
| `LastModified` | time.Time | |
| `StorageClass` | string | optional |

### ObjectMetadata

Detailed view from HeadObject (FR-013).

| Field | Type | Notes |
|-------|------|-------|
| `Key` | string | |
| `Size` | int64 | |
| `LastModified` | time.Time | |
| `ContentType` | string | |
| `StorageClass` | string | |
| `ETag` | string | |
| `UserMetadata` | map[string]string | user-defined metadata |

### PreviewPayload

Bounded content for preview (FR-014/015/016).

| Field | Type | Notes |
|-------|------|-------|
| `Key` | string | |
| `ContentType` | string | drives text vs image rendering |
| `Data` | []byte | at most first 5 MiB (ranged read) |
| `Truncated` | bool | true if object exceeds the 5 MiB bound |
| `Kind` | enum {Text, Image, Binary} | classification for the renderer |

**Validation**: `len(Data) <= 5 MiB`. `Kind=Image` triggers half-block/protocol render; `Binary`
shows a safe summary; oversized → `Truncated=true` with a user-visible notice.

---

## Navigation state (UI model, transient)

| Field | Type | Notes |
|-------|------|-------|
| `ActiveContext` | string | current context name |
| `Path` | []string | breadcrumb: bucket + prefix segments (FR-009) |
| `Cache` | map[levelKey]*Level | per-session level cache (FR-011) |
| `Search` | string | active prefix filter at current level (FR-017) |
| `LoadGen` | int | generation id; stale-load drop (Constitution II) |
| `Loading` | bool | spinner + cancel while in-flight (FR-012) |

`levelKey = (context, bucket, prefix, search)`.

---

## Relationships

```text
Config 1──* Cluster
Config 1──* User
Config 1──* Context ──1 Cluster
                    └─1 User
Context (active) ──> Storage client ──> Bucket * ──> Level (tree) ──* Level (nested)
Level ──* ObjectRef ──(HeadObject)──> ObjectMetadata
ObjectRef ──(ranged GetObject)──> PreviewPayload
```
