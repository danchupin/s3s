package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
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

func loadBase(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(baseConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load base config: %v", err)
	}
	return cfg
}

func TestAddConnectionMapsTripleNoPlaintext(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t)

	names, err := cfg.AddConnection(NewConnection{
		Name: "newc", Endpoint: "http://h:9000", Region: "us-east-1", AccessKeyID: "AKID",
	}, "SUPER_SECRET")
	if err != nil {
		t.Fatalf("AddConnection: %v", err)
	}
	if len(names) != 2 || names[1] != "newc" {
		t.Errorf("context names should include newc; got %v", names)
	}

	// The on-disk config gained the triple, keychain:true, and NO plaintext secret.
	raw, err := os.ReadFile(cfg.Path())
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"name: newc", "endpoint: http://h:9000", "keychain: true", "accessKeyId: AKID"} {
		if !strings.Contains(s, want) {
			t.Errorf("config should contain %q; got:\n%s", want, s)
		}
	}
	if strings.Contains(s, "SUPER_SECRET") {
		t.Errorf("config MUST NOT contain the plaintext secret:\n%s", s)
	}

	// The secret is retrievable from the keychain under the connection name.
	got, err := keyring.Get("s3s", "newc")
	if err != nil || got != "SUPER_SECRET" {
		t.Errorf("keychain should hold the secret; got %q err %v", got, err)
	}

	// Re-loading the saved config resolves (the triple is internally consistent).
	if _, err := Load(cfg.Path()); err != nil {
		t.Errorf("saved config should re-load cleanly: %v", err)
	}
}

func TestAddConnectionRejectsDuplicate(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t)
	if _, err := cfg.AddConnection(NewConnection{Name: "base", Endpoint: "http://h:9000"}, "x"); err == nil {
		t.Error("a duplicate context name must be rejected")
	}
}
