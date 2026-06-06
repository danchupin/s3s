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

// review #2: an invalid draft (e.g. missing access key id, which the keychain source
// requires) must NOT mutate the live config — otherwise the leftover invalid entry would
// block every future add and corrupt the resolver's shared config.
func TestAddConnectionInvalidDoesNotCorrupt(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t)
	beforeCtx, beforeUsers, beforeClusters := len(cfg.Contexts), len(cfg.Users), len(cfg.Clusters)

	if _, err := cfg.AddConnection(NewConnection{Name: "bad", Endpoint: "http://h:9000"}, "SK"); err == nil {
		t.Fatal("an empty access key id should fail validation (keychain source)")
	}
	if len(cfg.Contexts) != beforeCtx || len(cfg.Users) != beforeUsers || len(cfg.Clusters) != beforeClusters {
		t.Fatalf("a failed add must not mutate the live config: ctx %d->%d users %d->%d clusters %d->%d",
			beforeCtx, len(cfg.Contexts), beforeUsers, len(cfg.Users), beforeClusters, len(cfg.Clusters))
	}
	// A subsequent VALID add still works (no corruption left behind).
	if _, err := cfg.AddConnection(NewConnection{Name: "good", Endpoint: "http://h:9000", AccessKeyID: "AK"}, "SK"); err != nil {
		t.Errorf("a valid add after a failed one should succeed: %v", err)
	}
}
