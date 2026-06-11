package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Fake is an in-memory Storage implementation for unit tests. It shapes the
// delimiter tree, paginates, narrows by search, and maps errors exactly like the
// real client's contract, with NO network or SDK dependency.
type Fake struct {
	Buckets map[string]*FakeBucket
	// FailDelete marks full keys whose deletion fails (returns ErrAccessDenied),
	// for exercising best-effort recursive-delete partials and move's no-data-loss
	// branch. Key format: "<bucket>/<key>".
	FailDelete map[string]bool
	// FailListBuckets makes ListBuckets return ErrAccessDenied — simulates bucket-scoped
	// credentials that lack s3:ListAllMyBuckets (010 pinned buckets, R9).
	FailListBuckets bool
	// AccessDeniedBuckets[bucket]=true makes ListLevel on that bucket return ErrAccessDenied
	// — simulates a bucket the creds cannot reach (010, distinct from ErrNotFound).
	AccessDeniedBuckets map[string]bool
	// Call counters (test assertions, e.g. "0 ListBuckets calls for a pinned connection").
	ListBucketsCalls int
	ListLevelCalls   int

	// 017 US1: UsageOf models pagination in chunks of DefaultMaxKeys; UsagePages
	// counts simulated pages so budget tests can assert the enumeration stopped
	// within one page of the cap.
	UsagePages int
	// UsageScanStart pins the age-histogram reference (zero ⇒ time.Now()) so
	// distribution tests are deterministic (017 US4).
	UsageScanStart time.Time
}

// FakeBucket is one seeded bucket.
type FakeBucket struct {
	CreationDate time.Time
	Objects      map[string]FakeObject // full key -> object

	// 016 US4: bucket configuration the Fake returns. A zero-value ConfigItem (State
	// == "") is normalised to ConfigNone. UnsupportedGetConfigs[sub]=true forces that
	// sub-resource to "unsupported" (sub ∈ versioning|encryption|lifecycle|replication|
	// publicaccessblock|location) — MinIO can't produce this, so it is unit-tested here.
	BucketConfig          BucketConfig
	UnsupportedGetConfigs map[string]bool
}

// FakeObject is one seeded object.
type FakeObject struct {
	Data []byte
	// Size overrides the effective object size when > 0 (len(Data) otherwise), so
	// tests can model multi-GiB objects without allocating (017 distribution tests).
	Size         int64
	ContentType  string
	StorageClass string
	ETag         string
	LastModified time.Time
	UserMetadata map[string]string
	AccessDenied bool // simulate a 403 on Head/Get

	// 016 enriched HeadObject fields (leave a permission-gated field "" to simulate
	// "header absent" → rendered "unknown").
	VersionID           string
	DeleteMarker        bool
	SSEAlgorithm        string
	SSEKMSKeyID         string
	ReplicationStatus   string
	RestoreStatus       string
	ObjectLockMode      string
	ObjectLockRetainTil time.Time
	ObjectLockLegalHold string
	LifecycleExpiration string
	ContentEncoding     string
	CacheControl        string
	ContentDisposition  string

	// 016 US4: object tags (values). TagsDenied simulates a 403 on GetObjectTagging.
	Tags       map[string]string
	TagsDenied bool
}

// NewFake returns an empty Fake ready to seed.
func NewFake() *Fake { return &Fake{Buckets: map[string]*FakeBucket{}} }

// Seed adds (or replaces) a bucket with the given keys (zero-value objects).
func (f *Fake) Seed(bucket string, keys ...string) {
	b, ok := f.Buckets[bucket]
	if !ok {
		b = &FakeBucket{Objects: map[string]FakeObject{}}
		f.Buckets[bucket] = b
	}
	for _, k := range keys {
		b.Objects[k] = FakeObject{}
	}
}

// SeedObject adds (or replaces) one fully-specified object.
func (f *Fake) SeedObject(bucket, key string, obj FakeObject) {
	b, ok := f.Buckets[bucket]
	if !ok {
		b = &FakeBucket{Objects: map[string]FakeObject{}}
		f.Buckets[bucket] = b
	}
	b.Objects[key] = obj
}

var _ Storage = (*Fake)(nil)

// ListBuckets returns seeded buckets sorted by name.
func (f *Fake) ListBuckets(ctx context.Context) ([]Bucket, error) {
	f.ListBucketsCalls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.FailListBuckets {
		return nil, fmt.Errorf("list buckets: %w", ErrAccessDenied)
	}
	out := make([]Bucket, 0, len(f.Buckets))
	for name, b := range f.Buckets {
		out = append(out, Bucket{Name: name, CreationDate: b.CreationDate})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListLevel pages one tree node using "/" as delimiter.
func (f *Fake) ListLevel(ctx context.Context, q LevelQuery) (Page, error) {
	f.ListLevelCalls++
	if err := ctx.Err(); err != nil {
		return Page{}, err
	}
	if f.AccessDeniedBuckets[q.Bucket] {
		return Page{}, fmt.Errorf("list %q: %w", q.Bucket, ErrAccessDenied)
	}
	b, ok := f.Buckets[q.Bucket]
	if !ok {
		return Page{}, fmt.Errorf("list %q: %w", q.Bucket, ErrNotFound)
	}
	maxKeys := q.MaxKeys
	if maxKeys <= 0 {
		maxKeys = DefaultMaxKeys
	}
	effPrefix := q.Prefix + q.Search

	keys := make([]string, 0, len(b.Objects))
	for k := range b.Objects {
		if strings.HasPrefix(k, effPrefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	start := 0
	if q.Token != nil && *q.Token != "" {
		if n, err := strconv.Atoi(*q.Token); err == nil && n >= 0 {
			start = n
		}
	}

	var (
		dirs    []string
		objs    []ObjectRef
		seenDir = map[string]bool{}
		count   int32
		i       = start
	)
	for i < len(keys) && count < maxKeys {
		k := keys[i]
		rest := k[len(effPrefix):]
		if idx := strings.IndexByte(rest, '/'); idx >= 0 {
			dir := effPrefix + rest[:idx+1] // common prefix incl trailing "/"
			if !seenDir[dir] {
				seenDir[dir] = true
				dirs = append(dirs, dir)
				count++
			}
			// roll up: skip every key under this common prefix (counts as one).
			for i < len(keys) && strings.HasPrefix(keys[i], dir) {
				i++
			}
			continue
		}
		o := b.Objects[k]
		objs = append(objs, ObjectRef{
			Key:          k,
			Size:         o.effSize(),
			LastModified: o.LastModified,
			StorageClass: o.StorageClass,
		})
		count++
		i++
	}

	var next *string
	if i < len(keys) {
		t := strconv.Itoa(i)
		next = &t
	}
	return Page{Dirs: dirs, Objects: objs, NextToken: next}, nil
}

// HeadObject returns metadata for a seeded object.
func (f *Fake) HeadObject(ctx context.Context, bucket, key string) (ObjectMetadata, error) {
	if err := ctx.Err(); err != nil {
		return ObjectMetadata{}, err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return ObjectMetadata{}, fmt.Errorf("head %q: %w", bucket, ErrNotFound)
	}
	o, ok := b.Objects[key]
	if !ok {
		return ObjectMetadata{}, fmt.Errorf("head %q/%q: %w", bucket, key, ErrNotFound)
	}
	if o.AccessDenied {
		return ObjectMetadata{}, fmt.Errorf("head %q/%q: %w", bucket, key, ErrAccessDenied)
	}
	return ObjectMetadata{
		Key:          key,
		Size:         o.effSize(),
		LastModified: o.LastModified,
		ContentType:  o.ContentType,
		StorageClass: o.StorageClass,
		ETag:         o.ETag,
		UserMetadata: o.UserMetadata,

		VersionID:           o.VersionID,
		DeleteMarker:        o.DeleteMarker,
		SSEAlgorithm:        o.SSEAlgorithm,
		SSEKMSKeyID:         o.SSEKMSKeyID,
		ReplicationStatus:   o.ReplicationStatus,
		RestoreStatus:       o.RestoreStatus,
		ObjectLockMode:      o.ObjectLockMode,
		ObjectLockRetainTil: o.ObjectLockRetainTil,
		ObjectLockLegalHold: o.ObjectLockLegalHold,
		LifecycleExpiration: o.LifecycleExpiration,
		ContentEncoding:     o.ContentEncoding,
		CacheControl:        o.CacheControl,
		ContentDisposition:  o.ContentDisposition,
	}, nil
}

// CreateFolder creates a zero-length object at the normalised "<prefix>/" key in
// the in-memory bucket, mirroring the real client (FR-009/FR-010).
func (f *Fake) CreateFolder(ctx context.Context, bucket, prefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := normalizeFolderKey(prefix)
	if err != nil {
		return err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return fmt.Errorf("create folder %q: %w", bucket, ErrNotFound)
	}
	b.Objects[key] = FakeObject{}
	return nil
}

var _ Mutator = (*Fake)(nil)

// RemoveObject deletes one key from the in-memory bucket (FR-001). A key marked in
// FailDelete returns ErrAccessDenied (simulating a backend refusal); a missing key
// returns ErrNotFound.
func (f *Fake) RemoveObject(ctx context.Context, bucket, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return fmt.Errorf("remove %q: %w", bucket, ErrNotFound)
	}
	if f.FailDelete[bucket+"/"+key] {
		return fmt.Errorf("remove %q/%q: %w", bucket, key, ErrAccessDenied)
	}
	if _, ok := b.Objects[key]; !ok {
		return fmt.Errorf("remove %q/%q: %w", bucket, key, ErrNotFound)
	}
	delete(b.Objects, key)
	return nil
}

// UploadFile stores the bytes read from r at key (FR-002).
func (f *Fake) UploadFile(ctx context.Context, bucket, key string, r io.Reader, _ int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return fmt.Errorf("upload %q: %w", bucket, ErrNotFound)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	b.Objects[key] = FakeObject{Data: data}
	return nil
}

// CopyKey duplicates srcKey to dstKey within one bucket; the source is unchanged
// (FR-004). Rejects an invalid or identical destination (FR-013).
func (f *Fake) CopyKey(ctx context.Context, bucket, srcKey, dstKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateDestKey(srcKey, dstKey); err != nil {
		return err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return fmt.Errorf("copy %q: %w", bucket, ErrNotFound)
	}
	src, ok := b.Objects[srcKey]
	if !ok {
		return fmt.Errorf("copy %q/%q: %w", bucket, srcKey, ErrNotFound)
	}
	b.Objects[dstKey] = src
	return nil
}

// MoveObject = CopyKey then RemoveObject(src), with the no-data-loss guarantee
// (FR-006/FR-007): copy failure leaves the source intact; a copy-ok/delete-fail
// returns ErrMovePartial with both keys present.
func (f *Fake) MoveObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	if err := f.CopyKey(ctx, bucket, srcKey, dstKey); err != nil {
		return err
	}
	if err := f.RemoveObject(ctx, bucket, srcKey); err != nil {
		return ErrMovePartial
	}
	return nil
}

// DeleteRecursive removes every key under prefix, best-effort: a key in FailDelete
// is counted as Failed and left in place; the run continues (FR-009). onProgress is
// invoked per batch; a cancelled ctx returns the partial counts with ctx.Err().
func (f *Fake) DeleteRecursive(ctx context.Context, bucket, prefix string, onProgress func(DeleteSummary)) (DeleteSummary, error) {
	var sum DeleteSummary
	b, ok := f.Buckets[bucket]
	if !ok {
		return sum, fmt.Errorf("delete %q: %w", bucket, ErrNotFound)
	}
	keys := make([]string, 0, len(b.Objects))
	for k := range b.Objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for i, k := range keys {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		if f.FailDelete[bucket+"/"+k] {
			sum.Failed++
		} else {
			delete(b.Objects, k)
			sum.Deleted++
		}
		// Report periodically so the UI can render running progress.
		if onProgress != nil && ((i+1)%progressEvery == 0 || i == len(keys)-1) {
			onProgress(sum)
		}
	}
	return sum, nil
}

// RemoveBucket deletes an empty bucket; a non-empty bucket returns ErrBucketNotEmpty
// without removing anything (007 FR-024b). A missing bucket returns ErrNotFound.
func (f *Fake) RemoveBucket(ctx context.Context, bucket string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return fmt.Errorf("remove bucket %q: %w", bucket, ErrNotFound)
	}
	if len(b.Objects) > 0 {
		return ErrBucketNotEmpty
	}
	delete(f.Buckets, bucket)
	return nil
}

// GetObject returns a reader over the full object body (005 FR-001).
func (f *Fake) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("get %q: %w", bucket, ErrNotFound)
	}
	o, ok := b.Objects[key]
	if !ok {
		return nil, fmt.Errorf("get %q/%q: %w", bucket, key, ErrNotFound)
	}
	if o.AccessDenied {
		return nil, fmt.Errorf("get %q/%q: %w", bucket, key, ErrAccessDenied)
	}
	return io.NopCloser(bytes.NewReader(o.Data)), nil
}

// effSize is the effective object size: the Size override when set, else len(Data).
func (o FakeObject) effSize() int64 {
	if o.Size > 0 {
		return o.Size
	}
	return int64(len(o.Data))
}

// UsageOf recursively aggregates objects under prefix with the shared usageAgg, so
// the Fake ranks children and buckets distributions identically to the real client
// (005 FR-008/FR-009, 017 US1/US4). Pagination is modelled in chunks of
// DefaultMaxKeys: UsagePages counts pages, and the maxObjects cap is checked at page
// boundaries exactly like the real client (truncation first — a cap reached on the
// final page is exact).
func (f *Fake) UsageOf(ctx context.Context, bucket, prefix string, maxObjects int, onProgress func(UsageProgress)) (UsageReport, error) {
	scanStart := f.UsageScanStart
	if scanStart.IsZero() {
		scanStart = time.Now()
	}
	agg := newUsageAgg(bucket, prefix, scanStart)
	b, ok := f.Buckets[bucket]
	if !ok {
		return agg.report(false, false), fmt.Errorf("usage %q: %w", bucket, ErrNotFound)
	}
	keys := make([]string, 0, len(b.Objects))
	for k := range b.Objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	pageSize := int(DefaultMaxKeys)
	for start := 0; start < len(keys); start += pageSize {
		end := start + pageSize
		if end > len(keys) {
			end = len(keys)
		}
		f.UsagePages++
		for _, k := range keys[start:end] {
			if err := ctx.Err(); err != nil {
				return agg.report(false, false), err
			}
			o := b.Objects[k]
			agg.add(k, o.effSize(), o.LastModified, o.StorageClass)
		}
		if onProgress != nil {
			onProgress(UsageProgress{ScannedCount: agg.totalCount, ScannedSize: agg.totalSize})
		}
		if end == len(keys) {
			break // final page — exact even when the cap was just reached
		}
		if maxObjects > 0 && agg.totalCount >= maxObjects {
			return agg.report(false, true), nil
		}
	}
	return agg.report(true, false), nil
}

// GetObjectTagging returns the seeded tags for an object (016 US4). TagsDenied → 403.
func (f *Fake) GetObjectTagging(ctx context.Context, bucket, key string) (ObjectTags, error) {
	if err := ctx.Err(); err != nil {
		return ObjectTags{}, err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return ObjectTags{}, fmt.Errorf("tags %q: %w", bucket, ErrNotFound)
	}
	o, ok := b.Objects[key]
	if !ok {
		return ObjectTags{}, fmt.Errorf("tags %q/%q: %w", bucket, key, ErrNotFound)
	}
	if o.TagsDenied {
		return ObjectTags{}, fmt.Errorf("tags %q/%q: %w", bucket, key, ErrAccessDenied)
	}
	return ObjectTags{ObjectKey: key, Tags: o.Tags}, nil
}

// GetBucketConfiguration returns the seeded BucketConfig with empty sub-resources
// normalised to "none" and UnsupportedGetConfigs entries forced to "unsupported"
// (016 US4/FR-013).
func (f *Fake) GetBucketConfiguration(ctx context.Context, bucket string) (BucketConfig, error) {
	if err := ctx.Err(); err != nil {
		return BucketConfig{}, err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return BucketConfig{Bucket: bucket}, fmt.Errorf("config %q: %w", bucket, ErrNotFound)
	}
	cfg := b.BucketConfig
	cfg.Bucket = bucket
	cfg.Versioning = defConfigState(cfg.Versioning)
	cfg.Encryption = defConfigState(cfg.Encryption)
	cfg.Lifecycle = defConfigState(cfg.Lifecycle)
	cfg.Replication = defConfigState(cfg.Replication)
	cfg.PublicAccessBlock = defConfigState(cfg.PublicAccessBlock)
	cfg.Location = defConfigState(cfg.Location)
	for sub := range b.UnsupportedGetConfigs {
		item := ConfigItem{State: ConfigUnsupported, Reason: ErrUnsupported}
		switch sub {
		case "versioning":
			cfg.Versioning = item
		case "encryption":
			cfg.Encryption = item
		case "lifecycle":
			cfg.Lifecycle = item
		case "replication":
			cfg.Replication = item
		case "publicaccessblock", "pab":
			cfg.PublicAccessBlock = item
		case "location":
			cfg.Location = item
		}
	}
	return cfg, nil
}

// defConfigState normalises a zero-value ConfigItem (State == "") to ConfigNone.
func defConfigState(it ConfigItem) ConfigItem {
	if it.State == "" {
		return ConfigItem{State: ConfigNone}
	}
	return it
}

// GetObjectRange returns a reader over Data[start : end+1] (clamped).
func (f *Fake) GetObjectRange(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, ok := f.Buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("get %q: %w", bucket, ErrNotFound)
	}
	o, ok := b.Objects[key]
	if !ok {
		return nil, fmt.Errorf("get %q/%q: %w", bucket, key, ErrNotFound)
	}
	if o.AccessDenied {
		return nil, fmt.Errorf("get %q/%q: %w", bucket, key, ErrAccessDenied)
	}
	if start < 0 {
		start = 0
	}
	if start > int64(len(o.Data)) {
		start = int64(len(o.Data))
	}
	stop := end + 1
	if stop > int64(len(o.Data)) || stop < start {
		stop = int64(len(o.Data))
	}
	return io.NopCloser(bytes.NewReader(o.Data[start:stop])), nil
}
