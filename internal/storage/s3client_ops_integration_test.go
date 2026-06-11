//go:build integration

// Integration coverage for the 005 read methods (GetObject full-stream, UsageOf
// recursive aggregation) against a real MinIO (Constitution IV).
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// TestIntegrationGetObjectFull downloads a multi-megabyte object and verifies the
// full body length + content, and that cancellation surfaces ctx.Err() (005 FR-001,
// storage-read-ops-contract C1).
func TestIntegrationGetObjectFull(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "dlbucket")

	// ~3 MiB body (larger than the 5 MiB preview cap is unnecessary; this proves the
	// full stream is returned, not a bounded slice).
	body := bytes.Repeat([]byte("s3s-download-payload-"), 150_000) // ~3.15 MiB
	b.put(t, "dlbucket", "big/data.bin", string(body), "application/octet-stream")

	rc, err := b.store.GetObject(context.Background(), "dlbucket", "big/data.bin")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(got) != len(body) || !bytes.Equal(got, body) {
		t.Errorf("GetObject body len=%d, want %d (content match=%v)", len(got), len(body), bytes.Equal(got, body))
	}

	// Cancellation mid-flight surfaces ctx.Err().
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.store.GetObject(ctx, "dlbucket", "big/data.bin"); err == nil {
		t.Error("GetObject with cancelled ctx returned nil error")
	}
}

// TestIntegrationUsageOfPagination seeds more than one ListObjectsV2 page worth of
// keys across nested prefixes and verifies totals + ranked children survive the
// pagination boundary; a cancelled scan returns Complete=false (005 FR-008/FR-011).
func TestIntegrationUsageOfPagination(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "usage")

	// 1200 keys (> the 1000 default page size) split across two sub-prefixes, plus a
	// couple of direct objects, so the immediate-child breakdown crosses a page edge.
	const n = 1200
	for i := 0; i < n; i++ {
		sub := "alpha"
		if i%2 == 0 {
			sub = "beta"
		}
		b.put(t, "usage", fmt.Sprintf("%s/deep/%04d.txt", sub, i), "x", "")
	}
	b.put(t, "usage", "top.bin", "0123456789", "") // 10 bytes, direct object

	rep, err := b.store.UsageOf(context.Background(), "usage", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf: %v", err)
	}
	if rep.TotalCount != n+1 {
		t.Errorf("TotalCount = %d, want %d (pagination lost keys)", rep.TotalCount, n+1)
	}
	// 3 immediate children: alpha/, beta/, top.bin.
	if len(rep.Children) != 3 {
		t.Fatalf("children = %d, want 3 (%+v)", len(rep.Children), rep.Children)
	}
	for _, c := range rep.Children {
		if c.IsDir && c.Count != n/2 {
			t.Errorf("child %q count = %d, want %d", c.Name, c.Count, n/2)
		}
	}
	if !rep.Complete {
		t.Error("Complete = false on a finished scan")
	}

	// Cancelled scan: partial + Complete=false.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	crep, cerr := b.store.UsageOf(ctx, "usage", "", 0, nil)
	if cerr == nil {
		t.Error("cancelled UsageOf returned nil error")
	}
	if crep.Complete {
		t.Error("cancelled UsageOf Complete = true, want false")
	}
}

// TestIntegrationIncompleteUploads seeds REAL in-progress multipart uploads (create +
// upload-part WITHOUT complete — the seeder stays inside internal/storage) and verifies
// count / oldest / part-size totals plus the honest zero (017 US4/FR-021/FR-022, T043).
func TestIntegrationIncompleteUploads(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "mpu")

	part := bytes.Repeat([]byte("p"), 5*1024*1024) // MinIO min part size for non-final parts
	for i, key := range []string{"stale/a.bin", "stale/b.bin"} {
		create, err := b.seed.CreateMultipartUpload(context.Background(), &s3.CreateMultipartUploadInput{
			Bucket: aws.String("mpu"), Key: aws.String(key),
		})
		if err != nil {
			t.Fatalf("create MPU %d: %v", i, err)
		}
		if _, err := b.seed.UploadPart(context.Background(), &s3.UploadPartInput{
			Bucket: aws.String("mpu"), Key: aws.String(key),
			UploadId: create.UploadId, PartNumber: aws.Int32(1),
			Body: bytes.NewReader(part),
		}); err != nil {
			t.Fatalf("upload part %d: %v", i, err)
		}
		// NO CompleteMultipartUpload — the uploads dangle on purpose.
	}

	// MinIO quirk: ListMultipartUploads returns an upload only when the prefix matches
	// the EXACT object key (bucket-/prefix-wide listing comes back empty) — unlike Ceph
	// RGW/AWS, which honor arbitrary prefixes. The exact-key query below validates the
	// real pagination + ListParts sizing path against MinIO; the prefix-wide semantics
	// are covered by the Fake units and manual validation on RGW (quickstart §Manual).
	got, err := b.store.ListIncompleteUploads(context.Background(), "mpu", "stale/a.bin")
	if err != nil {
		t.Fatalf("ListIncompleteUploads: %v", err)
	}
	if got.State != ConfigConfigured || got.Count != 1 {
		t.Errorf("got %+v, want the exact-key in-progress upload", got)
	}
	if got.SizedCount != 1 || got.TotalSize != int64(len(part)) {
		t.Errorf("sizing: SizedCount=%d TotalSize=%d, want 1/%d", got.SizedCount, got.TotalSize, len(part))
	}
	if got.OldestInitiated.IsZero() {
		t.Error("OldestInitiated must be set")
	}

	clean, err := b.store.ListIncompleteUploads(context.Background(), "mpu", "other/")
	if err != nil {
		t.Fatalf("ListIncompleteUploads(clean): %v", err)
	}
	if clean.State != ConfigNone || clean.Count != 0 {
		t.Errorf("clean prefix = %+v, want honest none/0", clean)
	}
}

// TestIntegrationUsageDistributions verifies the same-pass histograms against seeded
// sizes on a real backend (017 US4/FR-020, T043).
func TestIntegrationUsageDistributions(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "dist")

	b.put(t, "dist", "small.txt", "tiny", "")                                   // <128KiB
	b.put(t, "dist", "mid.bin", string(bytes.Repeat([]byte("m"), 200<<10)), "") // 128KiB–1MiB
	b.put(t, "dist", "big.bin", string(bytes.Repeat([]byte("b"), 2<<20)), "")   // 1–16MiB

	rep, err := b.store.UsageOf(context.Background(), "dist", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf: %v", err)
	}
	if rep.SizeDist[0].Count != 1 || rep.SizeDist[1].Count != 1 || rep.SizeDist[2].Count != 1 {
		t.Errorf("SizeDist = %+v, want one object per seeded bucket", rep.SizeDist)
	}
	// Everything was written seconds ago → the whole count lands in the <1d age bucket.
	if rep.AgeDist[0].Count != 3 {
		t.Errorf("AgeDist[<1d].Count = %d, want 3", rep.AgeDist[0].Count)
	}
	if len(rep.ClassDist) == 0 {
		t.Error("ClassDist must carry at least the default class")
	}
	var cc int
	for _, b := range rep.ClassDist {
		cc += b.Count
	}
	if cc != rep.TotalCount {
		t.Errorf("ClassDist Σ = %d, want %d", cc, rep.TotalCount)
	}
}

// TestIntegrationPresignGetFetchable mints a presigned link against the real MinIO and
// fetches it with PLAIN http.Get (no SDK on the consumer side) — the link must deliver
// the object bytes and carry the chosen expiry (017 US3/FR-015, T034).
func TestIntegrationPresignGetFetchable(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "presign")
	b.put(t, "presign", "share/доклад v+1.txt", "presigned-payload", "text/plain")

	u, warn, err := b.store.PresignGet(context.Background(), "presign", "share/доклад v+1.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if warn != "" {
		t.Errorf("warn = %q, want empty for static creds", warn)
	}
	parsed, perr := url.Parse(u)
	if perr != nil || parsed.Query().Get("X-Amz-Expires") != "900" {
		t.Errorf("URL expiry: parse=%v X-Amz-Expires=%q, want 900", perr, parsed.Query().Get("X-Amz-Expires"))
	}

	resp, herr := http.Get(u) //nolint:gosec // the URL is minted above, not user input
	if herr != nil {
		t.Fatalf("plain http.Get(presigned): %v", herr)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("presigned GET status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "presigned-payload" {
		t.Errorf("presigned body = %q", body)
	}

	// A tampered signature must be rejected — proves the backend validates the link.
	bad := u[:len(u)-4] + "0000"
	if resp2, err2 := http.Get(bad); err2 == nil { //nolint:gosec
		_ = resp2.Body.Close()
		if resp2.StatusCode == http.StatusOK {
			t.Error("tampered signature was accepted")
		}
	}
}

// TestIntegrationUsageOfBudgetCap seeds more keys than the cap and verifies the real
// client stops within one ListObjectsV2 page of maxObjects, reporting an honest lower
// bound — and that the uncapped scan of the same bucket is exact (017 US1/FR-001, T017).
func TestIntegrationUsageOfBudgetCap(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "usagecap")

	const n = 1200 // > one 1000-key page
	for i := 0; i < n; i++ {
		b.put(t, "usagecap", fmt.Sprintf("k/%04d.txt", i), "x", "")
	}

	rep, err := b.store.UsageOf(context.Background(), "usagecap", "", 1000, nil)
	if err != nil {
		t.Fatalf("UsageOf(capped): %v", err)
	}
	if !rep.Bounded || rep.Complete {
		t.Errorf("capped scan: Bounded=%v Complete=%v, want Bounded=true Complete=false", rep.Bounded, rep.Complete)
	}
	// Stop within one page of the cap: the first 1000-key page reaches the cap.
	if rep.TotalCount != 1000 {
		t.Errorf("capped TotalCount = %d, want exactly one page (1000)", rep.TotalCount)
	}

	full, err := b.store.UsageOf(context.Background(), "usagecap", "", 0, nil)
	if err != nil {
		t.Fatalf("UsageOf(full): %v", err)
	}
	if !full.Complete || full.Bounded || full.TotalCount != n {
		t.Errorf("full scan = %+v, want exact %d objects", full, n)
	}
}
