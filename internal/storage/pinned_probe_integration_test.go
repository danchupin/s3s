//go:build integration

// 010 pinned buckets (C1): the scoped-connection reachability probe uses a bounded ListLevel
// on a named bucket instead of ListBuckets. This exercises that exact call shape against a real
// MinIO backend, closing Constitution IV's credential/auth-flow focus for the probe path.
//
// NOTE: fully simulating bucket-scoped credentials (ListBuckets → 403 while the named bucket is
// reachable) requires a MinIO admin policy attached to a non-root user, which is out of scope
// for this harness; that half is covered by the white-box UI tests with storage.Fake
// (FailListBuckets + AccessDeniedBuckets) and by manual verification (quickstart.md).
package storage

import (
	"context"
	"testing"
)

func TestIntegrationListLevelProbeReachesBucket(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "probe-bucket")

	store, err := New(ClientConfig{
		Endpoint:    b.endpoint,
		Region:      "us-east-1",
		PathStyle:   true,
		AccessKeyID: rootUser,
		SecretKey:   rootPass,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// The probe: a bounded ListLevel on the named bucket must succeed (the connection-test
	// path for a pinned/scoped connection — cmd/s3s/connection.go connSeam.Test).
	if _, err := store.ListLevel(context.Background(), LevelQuery{Bucket: "probe-bucket", MaxKeys: 1}); err != nil {
		t.Fatalf("ListLevel probe on a reachable bucket failed: %v", err)
	}

	// A non-existent bucket is classified as not-found (distinct from unreachable), so the UI
	// reports the real reason rather than a blanket "unreachable".
	if _, err := store.ListLevel(context.Background(), LevelQuery{Bucket: "no-such-bucket-xyz", MaxKeys: 1}); err == nil {
		t.Error("ListLevel on a missing bucket should error")
	}
}
