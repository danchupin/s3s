// Package storage is the sole boundary between the UI and S3-compatible backends.
//
// It exposes ONLY read operations — by construction there is no way to mutate
// storage (Constitution V, FR-019, SC-009). This package is the only one
// permitted to import aws-sdk-go-v2/service/s3; a CI guard (scripts/check-readonly.sh)
// forbids mutating S3 symbols elsewhere.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// Error sentinels. Implementations classify and wrap backend failures into these
// so the UI can render distinct states. Wrapped errors MUST NOT embed secrets
// (FR-005, FR-021).
var (
	// ErrNotFound: bucket/key/prefix does not exist (404, NoSuchKey, NoSuchBucket).
	ErrNotFound = errors.New("storage: not found")
	// ErrAccessDenied: authentication/authorization failure (401/403).
	ErrAccessDenied = errors.New("storage: access denied")
	// ErrUnreachable: endpoint unreachable, connection refused, or timeout.
	ErrUnreachable = errors.New("storage: backend unreachable")
	// ErrInvalidConfig: client could not be constructed from the given config.
	ErrInvalidConfig = errors.New("storage: invalid configuration")
	// ErrReadOnly: a mutation was attempted on a read-only backend (the context is
	// readonly, or --write was not passed). Returned by readOnlyGuard before any
	// network call, so storage is provably unchanged (FR-003, FR-011, FR-012).
	ErrReadOnly = errors.New("storage: backend is read-only")
	// ErrInvalidName: a create-folder target failed validation (empty/whitespace or
	// control characters). Returned before any network call (FR-010).
	ErrInvalidName = errors.New("storage: invalid name")
)

// Mutator adds write capability on top of Storage. The real client, the in-memory
// Fake, and readOnlyGuard all implement it; the read-only guard returns ErrReadOnly
// without contacting the backend. Mutating S3 calls live ONLY in this package
// (scripts/check-readonly.sh enforces it).
type Mutator interface {
	// CreateFolder creates an empty folder at (bucket, prefix) by putting a
	// zero-length object whose key is prefix normalised to exactly one trailing
	// "/". Returns ErrReadOnly (no network call) when the backend is read-only and
	// ErrInvalidName when the prefix is empty/whitespace or has control chars.
	// FR-009, FR-010.
	CreateFolder(ctx context.Context, bucket, prefix string) error
}

// Storage is a read-only view of one S3-compatible backend (bound to one context).
type Storage interface {
	// ListBuckets returns buckets visible to the active credentials. FR-006.
	ListBuckets(ctx context.Context) ([]Bucket, error)

	// ListLevel returns one page of a tree node: child common-prefixes ("dirs")
	// and objects at (bucket, prefix) using "/" as delimiter. FR-007, FR-010, FR-017.
	// A nil/empty q.Token starts at the first page; the returned Page.NextToken
	// (nil when complete) is passed back to fetch the next page.
	ListLevel(ctx context.Context, q LevelQuery) (Page, error)

	// HeadObject returns object metadata without fetching content. FR-013.
	HeadObject(ctx context.Context, bucket, key string) (ObjectMetadata, error)

	// GetObjectRange streams at most (end-start+1) bytes; callers cap at 5 MiB
	// for preview and close the reader. FR-014, FR-016.
	GetObjectRange(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, error)
}

// Bucket is a top-level container listed for the active context.
type Bucket struct {
	Name         string
	CreationDate time.Time
}

// LevelQuery selects one tree node (one page) to list.
type LevelQuery struct {
	Bucket  string
	Prefix  string  // "" = bucket root; nested levels end with "/"
	Search  string  // FR-017: server-side prefix narrowing within the current level
	Token   *string // continuation token; nil/empty starts at first page
	MaxKeys int32   // page size hint (default 1000 when zero)
}

// Page is one listing page of a tree level.
type Page struct {
	Dirs      []string    // common prefixes (child directories)
	Objects   []ObjectRef // objects at this level (keys without further "/")
	NextToken *string     // nil => complete
}

// ObjectRef is a lightweight entry shown in a level.
type ObjectRef struct {
	Key          string
	Size         int64
	LastModified time.Time
	StorageClass string
}

// ObjectMetadata is the detailed view from HeadObject (FR-013).
type ObjectMetadata struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	StorageClass string
	ETag         string
	UserMetadata map[string]string
}

// ClientConfig is the resolved (cluster + user) input used to build a Storage.
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

// DefaultMaxKeys is the S3 page-size hint used when LevelQuery.MaxKeys is zero.
const DefaultMaxKeys int32 = 1000

// PreviewLimit is the maximum number of bytes fetched for a preview (FR-014/016).
const PreviewLimit int64 = 5 * 1024 * 1024
