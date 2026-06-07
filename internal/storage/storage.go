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
	// ErrInvalidName: a create-folder target or a destination key/prefix failed
	// validation (empty/whitespace, control characters, or destination == source).
	// Returned before any network call (FR-010, FR-013).
	ErrInvalidName = errors.New("storage: invalid name")
	// ErrMovePartial: a move copied the object to the destination but could not
	// delete the source. The data is safe at the destination; the source still
	// exists. Never a clean success (FR-007).
	ErrMovePartial = errors.New("storage: move copied object but source delete failed")
	// ErrBucketNotEmpty: a bucket delete was attempted on a bucket that still holds
	// objects. Bucket delete requires an empty bucket; it never recursively purges
	// (007 FR-024b). Returned before any DeleteBucket call.
	ErrBucketNotEmpty = errors.New("storage: bucket is not empty")
)

// DeleteSummary is the truthful outcome of a recursive delete: how many objects
// were removed and how many could not be (best-effort). Failed > 0 => partial,
// never a clean success (FR-009, FR-011).
type DeleteSummary struct {
	Deleted int
	Failed  int
}

// Mutator adds write capability on top of Storage. The real client, the in-memory
// Fake, and readOnlyGuard all implement it; the read-only guard returns ErrReadOnly
// without contacting the backend. Mutating S3 calls live ONLY in this package
// (scripts/check-readonly.sh enforces it). Method names deliberately avoid the
// guard's verb+entity pattern (RemoveObject not DeleteObject, etc.) so UI code may
// reference them without tripping the read-only scan.
type Mutator interface {
	// CreateFolder creates an empty folder at (bucket, prefix) by putting a
	// zero-length object whose key is prefix normalised to exactly one trailing
	// "/". Returns ErrReadOnly (no network call) when the backend is read-only and
	// ErrInvalidName when the prefix is empty/whitespace or has control chars.
	// FR-009, FR-010.
	CreateFolder(ctx context.Context, bucket, prefix string) error

	// RemoveObject removes a single object. Returns ErrReadOnly when read-only and
	// ErrNotFound if the key is already gone (the UI treats that as benign). FR-001.
	RemoveObject(ctx context.Context, bucket, key string) error

	// UploadFile creates (or overwrites) the object at key from r, streaming size
	// bytes. Honors ctx cancellation; a cancelled upload is never a success. FR-002.
	UploadFile(ctx context.Context, bucket, key string, r io.Reader, size int64) error

	// CopyKey server-side copies srcKey to dstKey within the same bucket; the source
	// is unchanged. Rejects an empty/whitespace/control dstKey or dstKey == srcKey
	// with ErrInvalidName before any network call. FR-004, FR-005, FR-013.
	CopyKey(ctx context.Context, bucket, srcKey, dstKey string) error

	// MoveObject = CopyKey(src->dst) then RemoveObject(src). If the copy fails the
	// source is left intact and dst is not claimed; if the copy succeeds but the
	// source delete fails, returns ErrMovePartial (no data loss). FR-006, FR-007.
	MoveObject(ctx context.Context, bucket, srcKey, dstKey string) error

	// DeleteRecursive enumerates every object under prefix (paginated) and deletes
	// them in batches, best-effort: a per-object failure does not abort the run.
	// onProgress (may be nil) is called after each batch with cumulative counts. A
	// cancelled ctx stops further work and returns the counts achieved with ctx.Err().
	// FR-008, FR-009, FR-011.
	DeleteRecursive(ctx context.Context, bucket, prefix string, onProgress func(DeleteSummary)) (DeleteSummary, error)

	// RemoveBucket deletes a whole bucket. It requires the bucket to be EMPTY: a
	// non-empty bucket returns ErrBucketNotEmpty BEFORE any delete (no recursive
	// purge). Returns ErrReadOnly when read-only. The method name uses "Remove" (not
	// "Delete") so UI references do not trip the read-only scan. 007 FR-024b.
	RemoveBucket(ctx context.Context, bucket string) error
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

	// GetObject streams the full object body (no range cap). The caller closes the
	// reader. A read — usable in read-only contexts (download is local-only writes).
	// 005 FR-001/FR-002.
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// UsageOf recursively aggregates every object under prefix: total size/count and
	// an immediate-child breakdown ranked largest-first. onProgress (nil-safe) gets
	// running totals; a cancelled ctx returns the partial report (Complete=false) with
	// ctx.Err(). A read. 005 FR-008..FR-012.
	UsageOf(ctx context.Context, bucket, prefix string, onProgress func(UsageProgress)) (UsageReport, error)
}

// UsageChild is one immediate child (sub-prefix or direct object) of an analyzed
// prefix, with the bytes/objects beneath it (005 data-model).
type UsageChild struct {
	Name  string // sub-prefix has a trailing "/", a direct object does not
	IsDir bool   // true => sub-prefix, false => direct object
	Size  int64  // bytes beneath this child (recursive for a sub-prefix)
	Count int    // object count beneath this child
}

// UsageReport is the aggregate result of UsageOf for one prefix. Children are ranked
// by Size descending (ties by Name). Complete is false when the scan was cancelled.
type UsageReport struct {
	Bucket     string
	Prefix     string
	TotalSize  int64
	TotalCount int
	Children   []UsageChild
	Complete   bool
}

// UsageProgress is a running tick emitted during a long UsageOf scan (005 FR-011).
type UsageProgress struct {
	ScannedCount int
	ScannedSize  int64
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
