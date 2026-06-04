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
}

// FakeBucket is one seeded bucket.
type FakeBucket struct {
	CreationDate time.Time
	Objects      map[string]FakeObject // full key -> object
}

// FakeObject is one seeded object.
type FakeObject struct {
	Data         []byte
	ContentType  string
	StorageClass string
	ETag         string
	LastModified time.Time
	UserMetadata map[string]string
	AccessDenied bool // simulate a 403 on Head/Get
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
	if err := ctx.Err(); err != nil {
		return nil, err
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
	if err := ctx.Err(); err != nil {
		return Page{}, err
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
			Size:         int64(len(o.Data)),
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
		Size:         int64(len(o.Data)),
		LastModified: o.LastModified,
		ContentType:  o.ContentType,
		StorageClass: o.StorageClass,
		ETag:         o.ETag,
		UserMetadata: o.UserMetadata,
	}, nil
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
