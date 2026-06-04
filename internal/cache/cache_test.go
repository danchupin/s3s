package cache

import "testing"

type level struct {
	dirs int
}

func TestCacheHitMiss(t *testing.T) {
	c := New[level]()
	k := Key{Context: "ctx", Bucket: "b", Prefix: "p/", Search: ""}

	if _, ok := c.Get(k); ok {
		t.Fatal("empty cache should miss")
	}
	c.Put(k, level{dirs: 3})
	v, ok := c.Get(k)
	if !ok || v.dirs != 3 {
		t.Fatalf("expected hit dirs=3, got %v ok=%v", v, ok)
	}
}

func TestCacheKeyDistinctBySearch(t *testing.T) {
	c := New[level]()
	base := Key{Context: "ctx", Bucket: "b", Prefix: "p/"}
	searched := base
	searched.Search = "ab"

	c.Put(base, level{dirs: 1})
	c.Put(searched, level{dirs: 2})

	if v, _ := c.Get(base); v.dirs != 1 {
		t.Errorf("base level overwritten: %v", v)
	}
	if v, _ := c.Get(searched); v.dirs != 2 {
		t.Errorf("searched level wrong: %v", v)
	}
}

func TestCacheInvalidateForcesMiss(t *testing.T) {
	c := New[level]()
	k := Key{Context: "ctx", Bucket: "b"}
	c.Put(k, level{dirs: 5})
	c.Invalidate(k)
	if _, ok := c.Get(k); ok {
		t.Fatal("invalidated key should miss (FR-011a)")
	}
}

func TestCacheInvalidateBucket(t *testing.T) {
	c := New[level]()
	c.Put(Key{Context: "ctx", Bucket: "b", Prefix: ""}, level{})
	c.Put(Key{Context: "ctx", Bucket: "b", Prefix: "x/"}, level{})
	c.Put(Key{Context: "ctx", Bucket: "other"}, level{})

	c.InvalidateBucket("ctx", "b")
	if c.Len() != 1 {
		t.Fatalf("InvalidateBucket should leave 1 entry, got %d", c.Len())
	}
	if _, ok := c.Get(Key{Context: "ctx", Bucket: "other"}); !ok {
		t.Error("unrelated bucket should survive")
	}
}

func TestCacheClear(t *testing.T) {
	c := New[level]()
	c.Put(Key{Bucket: "a"}, level{})
	c.Put(Key{Bucket: "b"}, level{})
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("Clear should empty cache, got %d", c.Len())
	}
}
