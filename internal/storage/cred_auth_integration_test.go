//go:build integration

// Closes Constitution IV's credential/auth-flow focus for the 005 non-env sources: a
// secret resolved from the OS keychain (mock-backed) must actually authenticate
// against a real MinIO backend (005 T036a, credential-source-contract C2).
package storage

import (
	"context"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/danchupin/s3s/internal/secret"
)

func TestIntegrationKeychainSourceAuthenticates(t *testing.T) {
	b := startBackend(t)
	b.createBucket(t, "authcheck")

	// Store the backend's secret in a mock keystore, then resolve it via the keychain
	// source exactly as the app would.
	keyring.MockInit()
	if err := secret.StoreKeychain("ctx", rootPass); err != nil {
		t.Fatalf("store keychain: %v", err)
	}
	res, err := secret.Resolve(context.Background(), secret.Request{
		Kind: secret.Keychain, AccessKeyID: rootUser, Ref: "ctx",
	})
	if err != nil {
		t.Fatalf("resolve keychain source: %v", err)
	}

	store, err := New(ClientConfig{
		Endpoint:    b.endpoint,
		Region:      "us-east-1",
		PathStyle:   true,
		AccessKeyID: res.AccessKeyID,
		SecretKey:   res.SecretKey.Reveal(),
	})
	if err != nil {
		t.Fatalf("New with keychain-sourced credential: %v", err)
	}
	buckets, err := store.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets with keychain-sourced credential failed to authenticate: %v", err)
	}
	found := false
	for _, bk := range buckets {
		if bk.Name == "authcheck" {
			found = true
		}
	}
	if !found {
		t.Errorf("authenticated but did not list the seeded bucket; got %v", buckets)
	}
}
