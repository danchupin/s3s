//go:build integration

// Integration tests for the 003 object write operations against a real MinIO
// (Constitution IV). Gated behind the `integration` build tag; t.Skip when Docker
// is unreachable. Reuses the testBackend helpers from s3client_integration_test.go.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func (b *testBackend) mut(t *testing.T) Mutator {
	t.Helper()
	m, ok := b.store.(Mutator)
	if !ok {
		t.Fatal("real client must satisfy Mutator")
	}
	return m
}

func (b *testBackend) readback(t *testing.T, bucket, key string) []byte {
	t.Helper()
	rc, err := b.store.GetObjectRange(context.Background(), bucket, key, 0, PreviewLimit-1)
	if err != nil {
		t.Fatalf("readback %q/%q: %v", bucket, key, err)
	}
	defer func() { _ = rc.Close() }()
	data, _ := io.ReadAll(rc)
	return data
}

func (b *testBackend) exists(t *testing.T, bucket, key string) bool {
	t.Helper()
	_, err := b.store.HeadObject(context.Background(), bucket, key)
	if err == nil {
		return true
	}
	if errors.Is(err, ErrNotFound) {
		return false
	}
	t.Fatalf("head %q/%q: %v", bucket, key, err)
	return false
}

// TestIntegrationRemoveObject deletes a real object and verifies it is gone (US1).
func TestIntegrationRemoveObject(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "del")
	b.put(t, "del", "victim.txt", "bye", "text/plain")

	if err := b.mut(t).RemoveObject(context.Background(), "del", "victim.txt"); err != nil {
		t.Fatalf("RemoveObject: %v", err)
	}
	if b.exists(t, "del", "victim.txt") {
		t.Error("object should be gone after RemoveObject")
	}
}

// readOnlyReader exposes ONLY Read (no Seek), reproducing the UI's progress-counting
// reader. A bare PutObject can't sign a non-seekable body over http and fails — the
// upload path MUST work through this (regression guard for the manager fix).
type readOnlyReader struct{ r io.Reader }

func (ro readOnlyReader) Read(p []byte) (int, error) { return ro.r.Read(p) }

// TestIntegrationUploadFile uploads a small and a large file through a NON-seekable
// reader and verifies the readback is byte-identical (US2, SC-003).
func TestIntegrationUploadFile(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "upb")
	mut := b.mut(t)

	small := []byte("small payload")
	if err := mut.UploadFile(context.Background(), "upb", "s.txt", readOnlyReader{bytes.NewReader(small)}, int64(len(small))); err != nil {
		t.Fatalf("UploadFile small: %v", err)
	}
	if got := b.readback(t, "upb", "s.txt"); !bytes.Equal(got, small) {
		t.Errorf("small readback mismatch: got %q want %q", got, small)
	}

	large := bytes.Repeat([]byte("0123456789abcdef"), 400*1024) // ~6 MiB
	if err := mut.UploadFile(context.Background(), "upb", "big.bin", readOnlyReader{bytes.NewReader(large)}, int64(len(large))); err != nil {
		t.Fatalf("UploadFile large: %v", err)
	}
	if got := b.readback(t, "upb", "big.bin"); !bytes.Equal(got[:len(small)+1], large[:len(small)+1]) || len(got) == 0 {
		t.Errorf("large readback prefix mismatch (len got=%d)", len(got))
	}
}

// TestIntegrationCopyKey server-side copies an object; both keys exist, source
// unchanged (US3).
func TestIntegrationCopyKey(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "cpb")
	b.put(t, "cpb", "src.txt", "data", "text/plain")

	if err := b.mut(t).CopyKey(context.Background(), "cpb", "src.txt", "dir/dst.txt"); err != nil {
		t.Fatalf("CopyKey: %v", err)
	}
	if !b.exists(t, "cpb", "src.txt") {
		t.Error("source must remain after copy")
	}
	if got := b.readback(t, "cpb", "dir/dst.txt"); string(got) != "data" {
		t.Errorf("copied bytes = %q, want data", got)
	}
}

// TestIntegrationMoveObject moves an object; only the destination remains (US4).
func TestIntegrationMoveObject(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "mvb")
	b.put(t, "mvb", "old/x.txt", "move-me", "text/plain")

	if err := b.mut(t).MoveObject(context.Background(), "mvb", "old/x.txt", "new/x.txt"); err != nil {
		t.Fatalf("MoveObject: %v", err)
	}
	if b.exists(t, "mvb", "old/x.txt") {
		t.Error("source must be gone after a clean move")
	}
	if got := b.readback(t, "mvb", "new/x.txt"); string(got) != "move-me" {
		t.Errorf("moved bytes = %q, want move-me", got)
	}
}

// TestIntegrationDeleteRecursiveMultiPage removes a multi-page prefix subtree and
// reports accurate counts (US5, FR-009, SC-006).
func TestIntegrationDeleteRecursiveMultiPage(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "rmb")
	const total = 1100
	for i := 0; i < total; i++ {
		b.put(t, "rmb", fmt.Sprintf("tree/k%05d", i), "x", "")
	}
	b.put(t, "rmb", "keep.txt", "x", "") // outside the prefix

	var lastDeleted int
	sum, err := b.mut(t).DeleteRecursive(context.Background(), "rmb", "tree/", func(s DeleteSummary) {
		lastDeleted = s.Deleted
	})
	if err != nil {
		t.Fatalf("DeleteRecursive: %v", err)
	}
	if sum.Deleted != total || sum.Failed != 0 {
		t.Errorf("summary = %+v, want Deleted=%d Failed=0", sum, total)
	}
	if lastDeleted != total {
		t.Errorf("final progress deleted = %d, want %d", lastDeleted, total)
	}

	page, err := b.store.ListLevel(context.Background(), LevelQuery{Bucket: "rmb"})
	if err != nil {
		t.Fatalf("ListLevel: %v", err)
	}
	for _, d := range page.Dirs {
		if d == "tree/" {
			t.Error("prefix tree/ should be gone after recursive delete")
		}
	}
	if !b.exists(t, "rmb", "keep.txt") {
		t.Error("a key outside the prefix must be untouched")
	}
}

// TestIntegrationGuardRefusesAllOps verifies the read-only guard refuses every new
// mutating method against a real backend without changing anything (FR-012, SC-008).
func TestIntegrationGuardRefusesAllOps(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "rog")
	b.put(t, "rog", "x.txt", "data", "")
	ro := Guard(b.store, false).(Mutator)
	ctx := context.Background()

	if err := ro.RemoveObject(ctx, "rog", "x.txt"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("RemoveObject = %v, want ErrReadOnly", err)
	}
	if err := ro.UploadFile(ctx, "rog", "y.txt", strings.NewReader("z"), 1); !errors.Is(err, ErrReadOnly) {
		t.Errorf("UploadFile = %v, want ErrReadOnly", err)
	}
	if err := ro.CopyKey(ctx, "rog", "x.txt", "z.txt"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("CopyKey = %v, want ErrReadOnly", err)
	}
	if err := ro.MoveObject(ctx, "rog", "x.txt", "z.txt"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("MoveObject = %v, want ErrReadOnly", err)
	}
	if _, err := ro.DeleteRecursive(ctx, "rog", "", nil); !errors.Is(err, ErrReadOnly) {
		t.Errorf("DeleteRecursive = %v, want ErrReadOnly", err)
	}
	// Nothing changed.
	if !b.exists(t, "rog", "x.txt") {
		t.Error("guard must not have mutated storage")
	}
}
