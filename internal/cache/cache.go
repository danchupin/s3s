// Package cache holds a per-session, TTL-free cache of loaded tree levels.
//
// Returning to an already-loaded level reads from this cache — no re-fetch
// (FR-011) — until the user forces a manual refresh, which invalidates the entry
// (FR-011a). There is intentionally NO expiry: freshness is user-driven.
package cache

// Key identifies one cached level by its full coordinates (FR-011).
type Key struct {
	Context string
	Bucket  string
	Prefix  string
	Search  string
}

// Cache is a simple in-memory map from Key to a value of type V. It is not safe
// for concurrent use; the Bubble Tea event loop is single-threaded and owns it.
type Cache[V any] struct {
	m map[Key]V
}

// New returns an empty cache.
func New[V any]() *Cache[V] {
	return &Cache[V]{m: make(map[Key]V)}
}

// Get returns the cached value and whether it was present.
func (c *Cache[V]) Get(k Key) (V, bool) {
	v, ok := c.m[k]
	return v, ok
}

// Put stores (or replaces) the value for a key.
func (c *Cache[V]) Put(k Key, v V) {
	c.m[k] = v
}

// Invalidate removes one entry, forcing a miss (and re-fetch) next time (FR-011a).
func (c *Cache[V]) Invalidate(k Key) {
	delete(c.m, k)
}

// InvalidateBucket drops every entry for a (context, bucket) pair — used when a
// refresh should clear all nested levels of a bucket.
func (c *Cache[V]) InvalidateBucket(context, bucket string) {
	for k := range c.m {
		if k.Context == context && k.Bucket == bucket {
			delete(c.m, k)
		}
	}
}

// Clear empties the cache (e.g. on context switch).
func (c *Cache[V]) Clear() {
	c.m = make(map[Key]V)
}

// Len reports the number of cached entries.
func (c *Cache[V]) Len() int { return len(c.m) }
