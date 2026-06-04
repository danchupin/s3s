# Contract: Storage Interface (read-only)

**Package**: `internal/storage` | **Feature**: 001-s3-readonly-browser

This is the sole boundary between the UI and S3. It exposes **only read operations** — by
construction there is no way to mutate storage (Constitution V, FR-019, SC-009). `internal/storage`
is the only package permitted to import `aws-sdk-go-v2/service/s3`; a CI guard forbids
`Put*`/`Delete*`/`Create*`/`Copy*` symbols elsewhere.

## Go interface

```go
package storage

import (
    "context"
    "io"
    "time"
)

// Storage is a read-only view of one S3-compatible backend (bound to one context).
type Storage interface {
    // ListBuckets returns buckets visible to the active credentials. FR-006.
    ListBuckets(ctx context.Context) ([]Bucket, error)

    // ListLevel returns one page of a tree node: child common-prefixes ("dirs") and
    // objects at (bucket, prefix) using "/" as delimiter. FR-007, FR-010, FR-017.
    // A nil/empty q.Token starts at the first page; the returned Page.NextToken
    // (nil when complete) is passed back to fetch the next page.
    ListLevel(ctx context.Context, q LevelQuery) (Page, error)

    // HeadObject returns object metadata without fetching content. FR-013.
    HeadObject(ctx context.Context, bucket, key string) (ObjectMetadata, error)

    // GetObjectRange streams at most (end-start+1) bytes; callers cap at 5 MiB for
    // preview and close the reader. FR-014, FR-016.
    GetObjectRange(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, error)
}

type Bucket struct {
    Name         string
    CreationDate time.Time
}

type LevelQuery struct {
    Bucket string
    Prefix string // "" = bucket root; nested levels end with "/"
    Search string // FR-017: server-side prefix narrowing within the current level
    Token  *string
    MaxKeys int32 // page size hint (default 1000)
}

type Page struct {
    Dirs      []string     // common prefixes (child directories)
    Objects   []ObjectRef
    NextToken *string      // nil => complete
}

type ObjectRef struct {
    Key          string
    Size         int64
    LastModified time.Time
    StorageClass string
}

type ObjectMetadata struct {
    Key          string
    Size         int64
    LastModified time.Time
    ContentType  string
    StorageClass string
    ETag         string
    UserMetadata map[string]string
}
```

## Behavior contract

- **Delimiter**: `ListLevel` always uses `/`. `Dirs` are common prefixes; `Objects` are keys at the
  level (no further `/`). Empty bucket/prefix → empty `Dirs` and `Objects`, `NextToken=nil`.
- **Search**: when `LevelQuery.Search != ""`, the effective S3 prefix is `Prefix + Search` (still
  delimited), so narrowing is server-side and complete, not limited to loaded entries (FR-017).
- **Pagination**: one backend listing request per `ListLevel` call (SC-003). Caller fetches the next
  page only on demand by re-calling with `Token = previous NextToken` (FR-010).
- **Ranged read**: `GetObjectRange(ctx, b, k, 0, 5*1024*1024-1)` for preview; an object smaller than
  the range returns its full bytes; larger objects are truncated by the caller's cap (FR-016).
- **Errors**: implementations MUST classify and wrap so the UI can render distinct states for:
  `ErrNotFound`, `ErrAccessDenied` (401/403), `ErrUnreachable`/timeout, `ErrInvalidConfig`. Errors
  MUST NOT embed secret values (FR-005, FR-021).
- **Read-only**: the interface has no mutating method. Any future write capability requires a new,
  explicitly-reviewed interface — it cannot be added accidentally (FR-019).

## Construction

```go
// Build from a resolved context (cluster + user). Anonymous when user.Anonymous.
func New(cfg ClientConfig) (Storage, error)

type ClientConfig struct {
    Endpoint      string
    Region        string
    PathStyle     bool
    TLSSkipVerify bool
    Anonymous     bool
    AccessKeyID   string
    SecretKey     string // never logged
    SessionToken  string // optional
}
```

Maps to aws-sdk-go-v2: `BaseEndpoint`, `UsePathStyle`, `aws.AnonymousCredentials{}` when
`Anonymous`, else `credentials.NewStaticCredentialsProvider(AccessKeyID, SecretKey, SessionToken)`.

## Test contract

- **Unit**: a `fake.go` in-memory `Storage` (seeded with buckets/keys) drives UI and core unit
  tests; tests assert tree shaping, pagination boundaries, search narrowing, and error mapping.
- **Integration** (`//go:build integration`): the real impl runs against a testcontainers MinIO —
  asserts auth (anonymous + static), delimiter listing, pagination across >1000 keys, ranged reads,
  and error codes (Constitution IV).
