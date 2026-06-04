package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
    secretAccessKey: ${S3S_TEST_SECRET}
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
	t.Setenv("S3S_TEST_SECRET", "s3cr3t")
	c, err := Load(writeConfig(t, validYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.CurrentContext != "local" {
		t.Errorf("current-context = %q", c.CurrentContext)
	}
	u, ok := c.user("dev")
	if !ok {
		t.Fatal("user dev missing")
	}
	if u.SecretAccessKey.Reveal() != "s3cr3t" {
		t.Errorf("secret not resolved from env: %q", u.SecretAccessKey.Reveal())
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

func TestLoadEnvUnset(t *testing.T) {
	_ = os.Unsetenv("S3S_TEST_SECRET")
	_, err := Load(writeConfig(t, validYAML))
	if !errors.Is(err, ErrEnvUnset) {
		t.Fatalf("unset env ref: want ErrEnvUnset, got %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file: want ErrNotFound, got %v", err)
	}
}

func TestSecretRedactedInConfig(t *testing.T) {
	t.Setenv("S3S_TEST_SECRET", "leak-me")
	c, _ := Load(writeConfig(t, validYAML))
	u, _ := c.user("dev")
	if strings.Contains(u.SecretAccessKey.String(), "leak-me") {
		t.Error("secret leaked through String()")
	}
}
