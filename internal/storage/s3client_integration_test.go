//go:build integration

// Integration tests run against a real MinIO via testcontainers (Constitution IV).
// They are gated behind the `integration` build tag and t.Skip when the Docker
// provider is unreachable, so `go test ./...` stays green without Docker.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go/modules/minio"
)

const (
	minioImage = "minio/minio:RELEASE.2024-01-16T16-07-38Z"
	rootUser   = "minioadmin"
	rootPass   = "minioadmin"
)

// testBackend holds a running MinIO and both a read-only Storage (under test) and
// a raw seeding client (test-only write path, allowed inside internal/storage).
type testBackend struct {
	endpoint string
	store    Storage
	seed     *s3.Client
}

func startBackend(t *testing.T) *testBackend {
	t.Helper()
	ctx := context.Background()

	container, err := minio.Run(ctx, minioImage,
		minio.WithUsername(rootUser), minio.WithPassword(rootPass))
	if err != nil {
		t.Skipf("skipping integration test, Docker/MinIO unavailable: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	conn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	endpoint := "http://" + conn

	store, err := New(ClientConfig{
		Endpoint:    endpoint,
		Region:      "us-east-1",
		PathStyle:   true,
		AccessKeyID: rootUser,
		SecretKey:   rootPass,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rawCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(rootUser, rootPass, "")),
	)
	if err != nil {
		t.Fatalf("seed config: %v", err)
	}
	seed := s3.NewFromConfig(rawCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &testBackend{endpoint: endpoint, store: store, seed: seed}
}

func (b *testBackend) createBucket(t *testing.T, name string) {
	t.Helper()
	_, err := b.seed.CreateBucket(context.Background(), &s3.CreateBucketInput{Bucket: aws.String(name)})
	if err != nil {
		t.Fatalf("create bucket %q: %v", name, err)
	}
}

func (b *testBackend) put(t *testing.T, bucket, key, body, contentType string) {
	t.Helper()
	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if _, err := b.seed.PutObject(context.Background(), in); err != nil {
		t.Fatalf("put %q/%q: %v", bucket, key, err)
	}
}

// TestIntegrationCreateFolder creates an empty folder on a writable real backend
// and verifies it shows up as a common prefix after a re-list (FR-009, SC-006).
func TestIntegrationCreateFolder(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "wbucket")

	mut, ok := b.store.(Mutator)
	if !ok {
		t.Fatal("real client must satisfy Mutator")
	}
	if err := mut.CreateFolder(context.Background(), "wbucket", "reports"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	page, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "wbucket"})
	if err != nil {
		t.Fatalf("ListLevel: %v", err)
	}
	found := false
	for _, d := range page.Dirs {
		if d == "reports/" {
			found = true
		}
	}
	if !found {
		t.Errorf("created folder not listed; dirs=%v", page.Dirs)
	}
}

// TestIntegrationGuardRefuses verifies the read-only guard refuses a mutation
// against a real backend without changing anything (SC-002, SC-007, FR-012).
func TestIntegrationGuardRefuses(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "robucket")

	ro := Guard(b.store, false).(Mutator)
	if err := ro.CreateFolder(context.Background(), "robucket", "nope"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("guarded CreateFolder = %v, want ErrReadOnly", err)
	}

	page, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "robucket"})
	if err != nil {
		t.Fatalf("ListLevel: %v", err)
	}
	if len(page.Dirs) != 0 || len(page.Objects) != 0 {
		t.Errorf("guard mutated storage: dirs=%v objects=%v", page.Dirs, page.Objects)
	}
}

func TestIntegrationListBuckets(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "alpha")
	b.createBucket(t, "beta")

	got, err := b.store.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	names := map[string]bool{}
	for _, x := range got {
		names[x.Name] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Fatalf("buckets = %+v, want alpha+beta", got)
	}
}

func TestIntegrationListBucketsAnonymous(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "pub")

	anon, err := New(ClientConfig{
		Endpoint:  b.endpoint,
		Region:    "us-east-1",
		PathStyle: true,
		Anonymous: true,
	})
	if err != nil {
		t.Fatalf("New anonymous: %v", err)
	}
	// Anonymous ListBuckets is denied by default MinIO policy — assert it is
	// classified, not that it succeeds (auth path exercised).
	if _, err := anon.ListBuckets(context.Background()); err == nil {
		t.Log("anonymous ListBuckets unexpectedly allowed (policy permits) — OK")
	} else if !errors.Is(err, ErrAccessDenied) && !errors.Is(err, ErrUnreachable) {
		t.Fatalf("anonymous error not classified: %v", err)
	}
}

func TestIntegrationListLevelDelimiter(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "tree")
	b.put(t, "tree", "top.txt", "x", "text/plain")
	b.put(t, "tree", "docs/a.md", "x", "text/plain")
	b.put(t, "tree", "docs/deep/b.md", "x", "text/plain")
	b.put(t, "tree", "img/logo.png", "x", "image/png")

	page, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "tree"})
	if err != nil {
		t.Fatalf("ListLevel root: %v", err)
	}
	if len(page.Dirs) != 2 {
		t.Errorf("root dirs = %v, want docs/+img/", page.Dirs)
	}
	if len(page.Objects) != 1 || page.Objects[0].Key != "top.txt" {
		t.Errorf("root objects = %v, want [top.txt]", page.Objects)
	}

	sub, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "tree", Prefix: "docs/"})
	if err != nil {
		t.Fatalf("ListLevel docs/: %v", err)
	}
	if len(sub.Dirs) != 1 || sub.Dirs[0] != "docs/deep/" {
		t.Errorf("docs dirs = %v, want [docs/deep/]", sub.Dirs)
	}
	if len(sub.Objects) != 1 || sub.Objects[0].Key != "docs/a.md" {
		t.Errorf("docs objects = %v, want [docs/a.md]", sub.Objects)
	}
}

func TestIntegrationListLevelPaginationOver1000(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "big")
	const total = 1100
	for i := 0; i < total; i++ {
		b.put(t, "big", fmt.Sprintf("k%05d", i), "x", "")
	}

	q := LevelQuery{Bucket: "big", MaxKeys: 1000}
	count := 0
	pages := 0
	for {
		page, err := b.store.ListLevel(context.Background(), q)
		if err != nil {
			t.Fatalf("page %d: %v", pages, err)
		}
		pages++
		count += len(page.Objects)
		if page.NextToken == nil {
			break
		}
		q.Token = page.NextToken
		if pages > 5 {
			t.Fatal("pagination did not terminate")
		}
	}
	if count != total {
		t.Errorf("listed %d objects across %d pages, want %d", count, pages, total)
	}
	if pages < 2 {
		t.Errorf("expected >1 page for %d keys, got %d", total, pages)
	}
}

func TestIntegrationListLevelEmptyPrefix(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "empty")
	page, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "empty"})
	if err != nil {
		t.Fatalf("ListLevel empty: %v", err)
	}
	if len(page.Dirs) != 0 || len(page.Objects) != 0 || page.NextToken != nil {
		t.Errorf("empty bucket should yield empty page, got %+v", page)
	}
}

func TestIntegrationListLevelSearch(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "search")
	b.put(t, "search", "apple", "x", "")
	b.put(t, "search", "apricot", "x", "")
	b.put(t, "search", "banana", "x", "")

	page, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "search", Search: "ap"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(page.Objects) != 2 {
		t.Errorf("search 'ap' = %v, want apple+apricot", page.Objects)
	}

	none, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "search", Search: "zzz"})
	if err != nil {
		t.Fatalf("no-match search: %v", err)
	}
	if len(none.Objects) != 0 || len(none.Dirs) != 0 {
		t.Errorf("no-match should be empty, got %+v", none)
	}
}

func TestIntegrationHeadObject(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "meta")
	body := "hello metadata"
	b.put(t, "meta", "file.txt", body, "text/plain")

	md, err := b.store.HeadObject(context.Background(), "meta", "file.txt")
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if md.Size != int64(len(body)) {
		t.Errorf("Size = %d, want %d", md.Size, len(body))
	}
	if md.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", md.ContentType)
	}
	if md.ETag == "" || md.LastModified.IsZero() {
		t.Errorf("missing etag/last-modified: %+v", md)
	}

	if _, err := b.store.HeadObject(context.Background(), "meta", "absent"); !errors.Is(err, ErrNotFound) {
		t.Errorf("absent key: want ErrNotFound, got %v", err)
	}
}

func TestIntegrationGetObjectRange(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "data")

	small := "0123456789"
	b.put(t, "data", "small.txt", small, "text/plain")

	// Ranged read bounded to first 5 bytes.
	rc, err := b.store.GetObjectRange(context.Background(), "data", "small.txt", 0, 4)
	if err != nil {
		t.Fatalf("GetObjectRange: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(got) != "01234" {
		t.Errorf("range 0-4 = %q, want 01234", got)
	}

	// Object smaller than the range returns its full bytes.
	rc2, err := b.store.GetObjectRange(context.Background(), "data", "small.txt", 0, PreviewLimit-1)
	if err != nil {
		t.Fatalf("GetObjectRange full: %v", err)
	}
	full, _ := io.ReadAll(rc2)
	_ = rc2.Close()
	if string(full) != small {
		t.Errorf("full read = %q, want %q", full, small)
	}

	// Large object: ranged read returns at most PreviewLimit bytes.
	big := strings.Repeat("a", int(PreviewLimit)+1024)
	b.put(t, "data", "big.bin", big, "application/octet-stream")
	rc3, err := b.store.GetObjectRange(context.Background(), "data", "big.bin", 0, PreviewLimit-1)
	if err != nil {
		t.Fatalf("GetObjectRange big: %v", err)
	}
	n, _ := io.Copy(io.Discard, rc3)
	_ = rc3.Close()
	if n != PreviewLimit {
		t.Errorf("ranged big read = %d bytes, want %d", n, PreviewLimit)
	}
}
