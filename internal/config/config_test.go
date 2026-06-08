package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/danchupin/s3s/internal/secret"
)

const validYAML = `
apiVersion: s3s/v1
clusters:
  - name: minio-local
    endpoint: http://127.0.0.1:9000
    region: us-east-1
    pathStyle: true
users:
  - name: dev
    accessKeyId: admin
    keychain: true
  - name: public
    anonymous: true
contexts:
  - name: local
    cluster: minio-local
    user: dev
  - name: pub
    cluster: minio-local
    user: public
current-context: local
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadValid(t *testing.T) {
	keyring.MockInit()
	path := writeConfig(t, validYAML)
	if err := secret.StoreKeychain(keychainAccount(path, "dev"), "s3cr3t"); err != nil {
		t.Fatalf("seed keychain: %v", err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.CurrentContext != "local" {
		t.Errorf("current-context = %q", c.CurrentContext)
	}
	u, ok := c.user("dev")
	if !ok || !u.Keychain {
		t.Fatalf("user dev should be a keychain source, got %+v ok=%v", u, ok)
	}
	cc, err := c.ClientConfig(context.Background(), "local")
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if cc.SecretKey != "s3cr3t" || cc.AccessKeyID != "admin" {
		t.Errorf("resolved wrong: secret=%q ak=%q", cc.SecretKey, cc.AccessKeyID)
	}
}

func TestLoadDanglingClusterRef(t *testing.T) {
	body := strings.Replace(validYAML, "cluster: minio-local\n    user: dev", "cluster: nope\n    user: dev", 1)
	t.Setenv("S3S_TEST_SECRET", "x")
	_, err := Load(writeConfig(t, body))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("dangling cluster ref: want ErrInvalid, got %v", err)
	}
}

func TestLoadDanglingUserRef(t *testing.T) {
	body := strings.Replace(validYAML, "cluster: minio-local\n    user: dev", "cluster: minio-local\n    user: ghost", 1)
	t.Setenv("S3S_TEST_SECRET", "x")
	_, err := Load(writeConfig(t, body))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("dangling user ref: want ErrInvalid, got %v", err)
	}
}

func TestLoadAnonymousAccepted(t *testing.T) {
	body := `
apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: https://example.com
users:
  - name: public
    anonymous: true
contexts:
  - name: pub
    cluster: c
    user: public
current-context: pub
`
	if _, err := Load(writeConfig(t, body)); err != nil {
		t.Fatalf("anonymous user should be accepted: %v", err)
	}
}

func TestLoadNonAnonMissingKeysRejected(t *testing.T) {
	body := `
apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: https://example.com
users:
  - name: dev
    accessKeyId: only-access-no-secret
contexts:
  - name: local
    cluster: c
    user: dev
current-context: local
`
	_, err := Load(writeConfig(t, body))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing secret: want ErrInvalid, got %v", err)
	}
}

// TestLoadStaleRemovedFieldRejected: a config carrying a removed source field (e.g.
// awsProfile) and no keychain/cmd is treated as an unknown/ignored field, leaving the
// user with no source → standard missing-credentials validation error (014 FR-010).
func TestLoadStaleRemovedFieldRejected(t *testing.T) {
	body := `
apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: https://example.com
users:
  - name: dev
    accessKeyId: AK
    awsProfile: prod
    secretAccessKey: leftover
contexts:
  - name: local
    cluster: c
    user: dev
current-context: local
`
	_, err := Load(writeConfig(t, body))
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale removed fields must leave no source → ErrInvalid, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "missing credentials") {
		t.Errorf("want missing-credentials error, got %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file: want ErrNotFound, got %v", err)
	}
}

// TestAnonymousResolves: an anonymous user still resolves a working, secret-less
// ClientConfig after the source removal (014 FR-006).
func TestAnonymousResolves(t *testing.T) {
	c, err := Load(writeConfig(t, validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cc, err := c.ClientConfig(context.Background(), "pub")
	if err != nil {
		t.Fatalf("ClientConfig(pub): %v", err)
	}
	if !cc.Anonymous || cc.SecretKey != "" {
		t.Errorf("anonymous should resolve with no secret, got %+v", cc)
	}
}

// 009: Empty is the valid connection-less state bound to a path; AddConnection writes there.
func TestEmptyConfigIsValidAndBound(t *testing.T) {
	c := Empty("/tmp/s3s-x/config.yaml")
	if c.Path() != "/tmp/s3s-x/config.yaml" {
		t.Errorf("Empty must bind the path; got %q", c.Path())
	}
	if len(c.ContextNames()) != 0 {
		t.Errorf("Empty must have no contexts; got %v", c.ContextNames())
	}
	if err := c.Validate(); err != nil {
		t.Errorf("an empty config must be valid (no connections yet); got %v", err)
	}
}

func TestEmptyConfigAddConnectionWrites(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml") // parent dir does not exist yet
	c := Empty(path)
	names, err := c.AddConnection(NewConnection{
		Name: "first", Endpoint: "http://h:9000", AccessKeyID: "AK", PathStyle: true,
	}, "SK")
	if err != nil {
		t.Fatalf("AddConnection on an empty config should create the file + dir; got %v", err)
	}
	if len(names) != 1 || names[0] != "first" {
		t.Errorf("context names after add = %v, want [first]", names)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("config file should be written at %s; %v", path, statErr)
	}
}
