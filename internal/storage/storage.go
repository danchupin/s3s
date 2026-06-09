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
	// ErrUnsupported: the backend does not implement this call (HTTP 501/405,
	// NotImplemented/MethodNotAllowed). Distinct from "not configured" (which is a
	// successful empty result) — used by GetBucketConfiguration so the UI can show
	// "unsupported" vs "none" vs "denied" (016 FR-013).
	ErrUnsupported = errors.New("storage: backend does not support this operation")
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

	// GetObjectTagging returns the object's tag key/value pairs (values, not just the
	// count). An empty Tags map = "no tags". A read. 016 US4/FR-011.
	GetObjectTagging(ctx context.Context, bucket, key string) (ObjectTags, error)

	// GetBucketConfiguration fetches each governance sub-resource (versioning,
	// encryption, lifecycle, replication, public-access-block, location) INDEPENDENTLY
	// and classifies each as configured/none/denied/unsupported, so one failed
	// sub-resource never fails the whole call. A read. 016 US4/FR-012/FR-013.
	GetBucketConfiguration(ctx context.Context, bucket string) (BucketConfig, error)
}

// ObjectTags is the tag set of one object (values, not just a count) — 016 US4/FR-011.
type ObjectTags struct {
	ObjectKey string
	Tags      map[string]string
}

// ConfigState is the tri-state (+unsupported) of one bucket-config sub-resource
// (016 FR-013): a value MUST distinguish "configured" / "none" / "denied" / "unsupported".
type ConfigState string

// The four tri-state (+unsupported) values a bucket-config sub-resource can take (FR-013).
const (
	ConfigConfigured  ConfigState = "configured"  // set; Detail summarises it
	ConfigNone        ConfigState = "none"        // call succeeded, nothing configured
	ConfigDenied      ConfigState = "denied"      // caller lacks read permission
	ConfigUnsupported ConfigState = "unsupported" // backend does not implement the call
)

// ConfigItem is one bucket-config sub-resource result. Detail/Reason carry only
// summaries/codes — never SDK bodies or secrets (constitution V).
type ConfigItem struct {
	State  ConfigState
	Detail string // human summary when configured (e.g. "Enabled", "SSE-KMS", "3 rules")
	Reason error  // nil | ErrAccessDenied | ErrUnsupported
}

// BucketConfig aggregates the independently-classified governance sub-resources of a
// bucket (016 US4). Bucket policy / policy-public status is intentionally OUT of scope.
type BucketConfig struct {
	Bucket            string
	Versioning        ConfigItem
	Encryption        ConfigItem
	Lifecycle         ConfigItem
	Replication       ConfigItem
	PublicAccessBlock ConfigItem
	Location          ConfigItem
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

// ObjectMetadata is the detailed view from HeadObject (FR-013). The block below the
// core fields is populated from the SAME HeadObject response (016 FR-001/FR-002 — no
// extra round-trip). Optional fields are "" / zero when the response omits them;
// the permission-gated ObjectLock* fields are "" both when unset AND when the caller
// lacks the retention/legal-hold read permission (the header is simply absent), so the
// UI renders them as "unknown" rather than "none" (016 FR-003).
type ObjectMetadata struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	StorageClass string
	ETag         string
	UserMetadata map[string]string

	// 016 enriched fields (all from the existing HeadObject response).
	VersionID           string    // x-amz-version-id
	DeleteMarker        bool      // current version is a delete marker
	SSEAlgorithm        string    // server-side-encryption: AES256 | aws:kms | …
	SSEKMSKeyID         string    // KMS key ARN (long — revealable)
	ReplicationStatus   string    // COMPLETE | PENDING | FAILED | REPLICA
	RestoreStatus       string    // parsed from x-amz-restore (ongoing/expiry)
	ObjectLockMode      string    // GOVERNANCE | COMPLIANCE — permission-gated
	ObjectLockRetainTil time.Time // retain-until date
	ObjectLockLegalHold string    // ON | OFF — permission-gated
	LifecycleExpiration string    // x-amz-expiration (lifecycle delete schedule)
	ContentEncoding     string
	CacheControl        string
	ContentDisposition  string
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
