# Contract: Storage Read-View Extension (017)

Changes to `storage.Storage` (`internal/storage/storage.go:106-143`). All additions are reads;
`make check-readonly` MUST stay green.

## 1. `UsageOf` signature change

```go
UsageOf(ctx context.Context, bucket, prefix string, maxObjects int,
        onProgress func(UsageProgress)) (UsageReport, error)
```

- `maxObjects == 0` → unlimited (full scan). `maxObjects > 0` → stop within one listing page
  after enumerating ≥ maxObjects objects; report `Bounded=true, Complete=false`.
- NEW report fields: `Bounded`, `ScanStart`, `AgeDist [6]DistBucket`, `SizeDist
  [6]DistBucket`, `ClassDist map[string]DistBucket` — accumulated in the SAME pass (no extra
  requests). Boundaries per data-model.md §2.
- Invariants: Σ dist counts == TotalCount; Σ dist sizes == TotalSize; cancelled ctx still
  returns partial report + ctx.Err() (unchanged).
- Callers updated: UI ambient (budget), UI full scan (0), Fake, integration tests.

## 2. `ListIncompleteUploads` (new)

```go
ListIncompleteUploads(ctx context.Context, bucket, prefix string) (IncompleteUploads, error)
```

- Implementation: paginate `ListMultipartUploads` (prefix-scoped, key+upload-id markers);
  for the FIRST 100 uploads, sum part sizes via `ListParts` (paginated) into
  `TotalSize`/`SizedCount`. Beyond 100: count + `OldestInitiated` only.
- Classification (mirrors 016 bucket-config): success+empty → `State=ConfigNone, Count=0`
  (honest zero); AccessDenied → `ConfigDenied`; NotImplemented/501/MethodNotAllowed →
  `ConfigUnsupported` (via `classify` + `ErrUnsupported`); other errors → error return
  (sentinel-classified), UI shows the footer error, never zero.
- Cancelled ctx: return what was accumulated + ctx.Err() (card may show partial MPU info).

## 3. `PresignGet` (new)

```go
PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (url, warn string, err error)
```

- Client-side only: `s3.NewPresignClient(...).PresignGetObject` — MUST NOT issue any network
  request (integration test asserts the URL works via plain `http.Get`, and a unit asserts no
  `s3API` call is recorded by the Fake).
- `ttl` MUST be one of 15m/1h/24h/7d; anything else → `ErrInvalidConfig`-classified error.
- `warn` non-empty when resolved credentials `CanExpire && Expires.Before(now.Add(ttl))`.
- Logging: the implementation MUST log `{op:"presign", bucket, key, ttl}` and MUST NOT include
  `url` in any log/error string (bearer capability; constitution V).

## Read-only guard analysis

Guard regex (`scripts/check-readonly.sh:43-45`): verb ∈
`Put|Delete|Create|Copy|Upload|Restore|Write` fused to entity ∈
`Object|Bucket|MultipartUpload|Part|Tagging|Acl|Policy|Cors|Lifecycle|Replication|Encryption|Versioning|Website|Notification`.

| New symbol (UI-visible) | Verb | Match? |
|---|---|---|
| `PresignGet` / `PresignGetObject` (storage-only) | Presign | no |
| `ListIncompleteUploads` | List | no |
| `IncompleteUploads` type | (no `\b`-anchored banned verb; `Upload` mid-word) | no |
| `ListMultipartUploads`, `ListParts` (storage-internal) | List | no |
| `CreateMultipartUpload`, `UploadPart` (integration seeder ONLY) | banned | inside `internal/storage` — excluded path (`check-readonly.sh:21-27`) |

UI/test code outside `internal/storage` MUST reference only `ListIncompleteUploads`,
`IncompleteUploads`, `PresignGet` — never SDK operation names.

## Fake obligations

- `Fake.UsageOf`: honor cap + record pages-listed counter (budget tests); deterministic
  distributions from seeded `FakeObject{Size, LastModified, StorageClass}`.
- `Fake` MPU: per-bucket seeded `[]FakeIncompleteUpload{Key, Initiated, PartSizes}`; toggles
  `FailListUploads error` (denied) and `UnsupportedListUploads bool`.
- `Fake.PresignGet`: deterministic `https://fake.presign/<bucket>/<key>?ttl=<s>` + configurable
  cred-expiry for `warn` tests; records zero backend calls.

## MinIO integration matrix (constitution IV)

| Case | Seed | Assert |
|---|---|---|
| cap honored | budget+N objects | `Bounded`, totals ≥ budget, ≤ cap+1 pages |
| distributions | objects with known sizes/dates/classes | exact bucket counts/bytes |
| MPU listing | `CreateMultipartUpload`+`UploadPart`, no complete | Count, OldestInitiated, TotalSize==Σ seeded parts |
| MPU honest zero | none | `State==none, Count==0` |
| MPU denied | deny policy (016 harness) | `State==denied`, never 0-as-clean |
| presign valid | object | plain `http.Get(url)` returns bytes |
| presign expired | ttl elapsed (short synthetic — sign with 15m, fetch with clock skew unavailable → assert signature params instead) | backend rejects / URL carries correct `X-Amz-Expires` |

`unsupported` for MPU: Fake + `classify` units only (MinIO implements the API) — same split as
016.

**MinIO quirk (discovered in T052)**: MinIO's `ListMultipartUploads` returns an upload only
when the request prefix matches the EXACT object key — bucket-/prefix-wide listings come back
empty (unlike Ceph RGW/AWS, which honor arbitrary prefixes). The integration test therefore
exercises the exact-key path against MinIO; prefix-wide semantics are covered by the Fake
units and the RGW manual validation (quickstart §Manual). On MinIO deployments the health
card's MPU block may honestly report "none" for a prefix even when uploads dangle below it.
