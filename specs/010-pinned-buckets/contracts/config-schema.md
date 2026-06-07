# Contract: Config schema — pinned buckets

Covers FR-001, FR-006, FR-007, FR-012.

## Schema
- `Cluster.Buckets []string \`yaml:"buckets,omitempty"\`` — ordered, normalized pinned bucket names.
- Absent/empty ⇒ list-all behavior (unchanged). Present ⇒ pinned model.

## Round-trip guarantees
- A config **without** `buckets:` loads with `Buckets == nil` and re-marshals byte-identical
  (`omitempty`). No migration, no `Validate()` change.
- A config **with** `buckets:` loads the list in order; `Marshal`→`Save`→`Load` preserves order and
  membership.

## `AppendBucket(ctxName, bucket string) ([]string, error)`
- Resolves `ctxName` → its cluster; appends the normalized `bucket` to `cluster.Buckets`.
- Normalization: trim; reject empty (`ErrInvalid`); **idempotent** (existing name ⇒ no dup, returns
  current list, `nil` error).
- Unknown context ⇒ `ErrNotFound`.
- Trial-validate-persist: build trial `*Config` (`slices.Clone`), `Validate()`, `Marshal`/`Save`,
  then commit live. A `Save` failure leaves the live config and disk unchanged.
- Logs `slog.Info("connection.bucket-add", "context", ctxName, "bucket", bucket, "outcome", "ok")` —
  no secret.
- Returns the cluster's updated bucket list.

## Test assertions (config unit, temp-file)
1. Load YAML with `buckets: [a, b]` → `cluster.Buckets == ["a","b"]`.
2. `AddConnection(NewConnection{..., Buckets: ["a","b"]}, secret)` → on-disk YAML contains both; no
   plaintext secret; reload → present.
3. `AppendBucket(ctx, "c")` → list `["a","b","c"]` on disk + live.
4. `AppendBucket(ctx, "b")` (dup) → list unchanged, no error.
5. `AppendBucket(ctx, "  ")` → `ErrInvalid`, config untouched.
6. `AppendBucket("nope", "x")` → `ErrNotFound`.
7. Config without `buckets:` re-marshals identically (byte compare).
