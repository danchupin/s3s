package storage

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestFakeListBucketsSorted(t *testing.T) {
	f := NewFake()
	f.Seed("zeta")
	f.Seed("alpha")
	got, err := f.ListBuckets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Fatalf("buckets not sorted: %+v", got)
	}
}

func TestFakeListLevelTreeShaping(t *testing.T) {
	f := NewFake()
	f.Seed("b",
		"a.txt",
		"docs/readme.md",
		"docs/deep/x.txt",
		"images/logo.png",
		"top.bin",
	)
	page, err := f.ListLevel(context.Background(), LevelQuery{Bucket: "b"})
	if err != nil {
		t.Fatal(err)
	}
	wantDirs := map[string]bool{"docs/": true, "images/": true}
	if len(page.Dirs) != 2 {
		t.Fatalf("dirs = %v, want 2 common prefixes", page.Dirs)
	}
	for _, d := range page.Dirs {
		if !wantDirs[d] {
			t.Errorf("unexpected dir %q", d)
		}
	}
	wantObjs := map[string]bool{"a.txt": true, "top.bin": true}
	if len(page.Objects) != 2 {
		t.Fatalf("objects = %v, want 2", page.Objects)
	}
	for _, o := range page.Objects {
		if !wantObjs[o.Key] {
			t.Errorf("unexpected object %q", o.Key)
		}
	}
	if page.NextToken != nil {
		t.Errorf("NextToken = %v, want nil (complete)", *page.NextToken)
	}
}

func TestFakeListLevelNestedPrefix(t *testing.T) {
	f := NewFake()
	f.Seed("b", "docs/readme.md", "docs/deep/x.txt", "docs/deep/y.txt")
	page, err := f.ListLevel(context.Background(), LevelQuery{Bucket: "b", Prefix: "docs/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Dirs) != 1 || page.Dirs[0] != "docs/deep/" {
		t.Fatalf("dirs = %v, want [docs/deep/]", page.Dirs)
	}
	if len(page.Objects) != 1 || page.Objects[0].Key != "docs/readme.md" {
		t.Fatalf("objects = %v, want [docs/readme.md]", page.Objects)
	}
}

func TestFakeListLevelPaginationBoundary(t *testing.T) {
	f := NewFake()
	f.Seed("b", "a", "b", "c", "d", "e")
	q := LevelQuery{Bucket: "b", MaxKeys: 2}
	var got []string
	pages := 0
	for {
		page, err := f.ListLevel(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, o := range page.Objects {
			got = append(got, o.Key)
		}
		if page.NextToken == nil {
			break
		}
		q.Token = page.NextToken
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}
	if pages != 3 {
		t.Errorf("pages = %d, want 3 (2+2+1)", pages)
	}
	if len(got) != 5 {
		t.Errorf("collected %d objects, want 5: %v", len(got), got)
	}
}

func TestFakeListLevelSearchNarrowing(t *testing.T) {
	f := NewFake()
	f.Seed("b", "apple", "apricot", "banana")
	page, err := f.ListLevel(context.Background(), LevelQuery{Bucket: "b", Search: "ap"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Objects) != 2 {
		t.Fatalf("search 'ap' = %v, want apple+apricot", page.Objects)
	}
	// no-match
	empty, err := f.ListLevel(context.Background(), LevelQuery{Bucket: "b", Search: "zzz"})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Objects) != 0 || len(empty.Dirs) != 0 {
		t.Fatalf("no-match should be empty, got %+v", empty)
	}
}

func TestFakeErrorMapping(t *testing.T) {
	f := NewFake()
	f.Seed("b", "k")
	if _, err := f.ListLevel(context.Background(), LevelQuery{Bucket: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing bucket: want ErrNotFound, got %v", err)
	}
	if _, err := f.HeadObject(context.Background(), "b", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing key: want ErrNotFound, got %v", err)
	}
	f.SeedObject("b", "secret", FakeObject{AccessDenied: true})
	if _, err := f.HeadObject(context.Background(), "b", "secret"); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("denied key: want ErrAccessDenied, got %v", err)
	}
}

func TestFakeGetObjectRange(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "k", FakeObject{Data: []byte("0123456789")})
	rc, err := f.GetObjectRange(context.Background(), "b", "k", 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	data, _ := io.ReadAll(rc)
	if string(data) != "01234" {
		t.Fatalf("range 0-4 = %q, want 01234", data)
	}
	// range past end clamps
	rc2, _ := f.GetObjectRange(context.Background(), "b", "k", 8, 999)
	data2, _ := io.ReadAll(rc2)
	_ = rc2.Close()
	if string(data2) != "89" {
		t.Fatalf("range 8-999 = %q, want 89", data2)
	}
}
