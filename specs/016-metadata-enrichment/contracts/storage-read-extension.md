# Contract: Storage Read Extension (US4 — FR-011/FR-012/FR-013/FR-014 + Integration)

**Surface**: `storage.Storage` read-view interface (`storage.go:100-128`) gains two read
methods; `s3Client` (`s3client.go`) and `Fake` (`fake.go`) implement them. Constitution IV
requires real-MinIO coverage (contract change). Framing: `internal/storage` already holds a
write surface (`Mutator`, `storage.go:54-98`; `s3API` Put/Delete/Copy/DeleteBucket,
`s3client.go:28-31`) — read-only is a guard-enforced posture; this extends only the READ
view.

## Methods

```
GetObjectTagging(ctx, bucket, key string) (ObjectTags, error)
GetBucketConfiguration(ctx, bucket string) (BucketConfig, error)
```

### GetObjectTagging
- Returns `{ObjectKey: key, Tags: map[string]string}` (values, not just count).
- Empty tag set (200 with no tags, or `NoSuchTagSet`) → empty map, nil error = "none".
- Errors classified to `ErrNotFound` (object absent) / `ErrAccessDenied` (denied) /
  `ErrUnreachable`.

### GetBucketConfiguration
- Returns `BucketConfig` with one `ConfigItem` per sub-resource (Versioning, Encryption,
  Lifecycle, Replication, PublicAccessBlock, Location).
- Each sub-resource fetched and classified **independently**: one denied/unsupported
  sub-resource MUST NOT fail the whole call (FR-012/FR-013 graceful degradation; edge cases
  `spec.md:251-253, 269-270`).
- `ConfigItem.State ∈ {configured, none, denied, unsupported}` with `Reason`
  `nil | ErrAccessDenied | ErrUnsupported`.

## Classification (the FR-013 three-way split — unambiguous)

Three DISTINCT buckets (data-model §3/§4):
- **`none`** — the `*NotFound`/`*NotConfiguration` family (`NoSuchTagSet`,
  `ServerSideEncryptionConfigurationNotFoundError`, `NoSuchLifecycleConfiguration`,
  `ReplicationConfigurationNotFoundError`, `NoSuchPublicAccessBlockConfiguration`) AND a
  200-with-empty-tagset. Mapped in the per-sub-resource caller (NOT generic `classify`).
- **`unsupported`** (`ErrUnsupported`) — a GENUINELY different signal: `smithy.APIError`
  code `NotImplemented`/`MethodNotAllowed`, or HTTP 501/405. Added to `classify`
  (`s3client.go:231-283`) BEFORE the `ErrUnreachable` fallback.
- **`denied`** (`ErrAccessDenied`) — the existing 401/403/`AccessDenied`/`Forbidden`
  mapping (`s3client.go:254-256, 265-266`).
The `*NotFound` family is NEVER mapped to `unsupported` — that distinction is exactly what
FR-013/SC-004 require (a bucket with no lifecycle vs a backend that can't do lifecycle).
`Reason`/`Detail` carry codes/summaries only — never SDK response bodies or secrets
(constitution V; `classify` logs only `code`/`status`/`message`, `s3client.go:275-281`).

## Read-only safety (FR-014)

Every new symbol is `Get*`. The guard bans only
`(Put|Delete|Create|Copy|Upload|Restore|Write)(Object|Bucket|…|Tagging|…)`
(`scripts/check-readonly.sh:43-45`); `Get*` never matches, so the methods are safe even
where UI code references them, and the SDK import stays inside `internal/storage` (excluded
at `check-readonly.sh:21`). `make check-readonly` stays green.

## Invariants

- I1 (FR-013/SC-004): the four states are distinct and each maps to a distinct UI label.
- I2 (constitution IV — MinIO): integration tests assert tag KV pairs, a `configured`
  versioning state, a `none` for an unconfigured sub-resource (MinIO yields the `*NotFound`
  family → none), a `denied` for a policy-denied read, and partial success when one
  sub-resource fails. **`unsupported` is NOT MinIO-testable** (MinIO implements every
  sub-resource); it is covered by (a) a `Fake` unit via `UnsupportedGetConfigs`, and (b) a
  `classify` unit feeding synthetic `NotImplemented`/`501`/`405` (asserting
  `ErrUnsupported`) and a `NoSuchLifecycleConfiguration` (asserting it does NOT map to
  `ErrUnsupported`).
- I3 (II/FR-016): both methods honor `ctx` cancellation; the UI calls them in a `tea.Cmd`
  carrying `detailGen`; stale results are dropped.

## Fake (fake.go)

`FakeObject` gains optional-metadata fields + per-field deny flags; `FakeBucket` gains a
`BucketConfig` plus `FailGetTags` and `UnsupportedGetConfigs map[string]bool` (per
sub-resource) so unit tests seed configured/none/denied/unsupported deterministically —
the `unsupported` branch's only executable home for the UI/storage unit suite.
