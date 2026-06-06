//go:build integration

// Integration test for the in-app connection manager (006 US4) against a real MinIO via
// testcontainers (Constitution IV). Gated behind the `integration` build tag; it t.Skip
// when the Docker provider is unreachable, so `go test ./...` stays green without Docker.
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/zalando/go-keyring"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/logging"
	"github.com/danchupin/s3s/internal/storage"
	"github.com/danchupin/s3s/internal/ui"
)

const baseConfig = `apiVersion: s3s/v1
clusters:
  - name: base
    endpoint: http://base:9000
users:
  - name: base
    accessKeyId: base-key
    keychain: true
contexts:
  - name: base
    cluster: base
    user: base
current-context: base
`

// TestConnectionAddTestSaveSwitch exercises the full US4 flow against MinIO: a draft is
// reachability-tested, saved (config triple + keychain secret), then the new context is
// resolved and lists buckets — proving an in-session switch works (FR-025/FR-025a).
func TestConnectionAddTestSaveSwitch(t *testing.T) {
	ctx := context.Background()

	container, err := minio.Run(ctx, "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		minio.WithUsername("minioadmin"), minio.WithPassword("minioadmin"))
	if err != nil {
		t.Skipf("skipping integration test, Docker/MinIO unavailable: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	conn, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	endpoint := "http://" + conn

	keyring.MockInit() // keep the secret out of the host keystore

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(baseConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load base config: %v", err)
	}

	seam := connSeam{cfg: cfg}
	draft := ui.ConnDraft{
		Name:        "minio",
		Endpoint:    endpoint,
		Region:      "us-east-1",
		AccessKeyID: "minioadmin",
		Secret:      logging.Secret("minioadmin"),
	}

	// Test: a reachable MinIO must pass.
	if err := seam.Test(ctx, draft); err != nil {
		t.Fatalf("reachability Test against MinIO should pass: %v", err)
	}

	// Save: persists the triple + keychain secret, returns the updated names.
	names, err := seam.Save(ctx, draft)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(names) != 2 || names[1] != "minio" {
		t.Fatalf("context names should include the new connection; got %v", names)
	}

	// Switch: resolve the new context (secret from keychain) and list buckets.
	cc, err := cfg.ClientConfig(ctx, "minio")
	if err != nil {
		t.Fatalf("resolve new context: %v", err)
	}
	st, err := storage.New(cc)
	if err != nil {
		t.Fatalf("storage.New for new context: %v", err)
	}
	if _, err := st.ListBuckets(ctx); err != nil {
		t.Fatalf("the new context should list buckets after switch: %v", err)
	}

	// The on-disk config must carry no plaintext secret (only keychain:true).
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "keychain: true") {
		t.Errorf("new user should use keychain:true; got:\n%s", raw)
	}
	if strings.Contains(string(raw), "secretAccessKey") {
		t.Errorf("config must not contain a plaintext secret:\n%s", raw)
	}
}
