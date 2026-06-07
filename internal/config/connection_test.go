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

func TestRemoveConnectionDropsTripleAndSecret(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t)
	// Add a second connection so the active "base" is not the only one.
	if _, err := cfg.AddConnection(NewConnection{
		Name: "extra", Endpoint: "http://h:9000", AccessKeyID: "AKID",
	}, "SECRET"); err != nil {
		t.Fatalf("AddConnection: %v", err)
	}
	// The keychain secret for "extra" exists.
	if _, err := keyring.Get("s3s", "extra"); err != nil {
		t.Fatalf("precondition: keychain secret for extra missing: %v", err)
	}

	names, err := cfg.RemoveConnection("extra")
	if err != nil {
		t.Fatalf("RemoveConnection: %v", err)
	}
	if len(names) != 1 || names[0] != "base" {
		t.Errorf("after delete, contexts = %v, want [base]", names)
	}
	// Triple is gone from the live config and on disk.
	if _, ok := cfg.context("extra"); ok {
		t.Error("context extra should be gone from the live config")
	}
	raw, _ := os.ReadFile(cfg.Path())
	if strings.Contains(string(raw), "extra") {
		t.Errorf("on-disk config still mentions extra:\n%s", raw)
	}
	// Keychain secret is removed.
	if _, err := keyring.Get("s3s", "extra"); err == nil {
		t.Error("keychain secret for extra should be removed")
	}
}

func TestRemoveConnectionRepointsCurrentContext(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t) // current-context = base
	if _, err := cfg.AddConnection(NewConnection{
		Name: "extra", Endpoint: "http://h:9000", AccessKeyID: "AKID",
	}, "SECRET"); err != nil {
		t.Fatalf("AddConnection: %v", err)
	}
	// Deleting the persisted current-context re-points it to a remaining context so the
	// on-disk config never dangles (the live-session active guard lives in the UI).
	if _, err := cfg.RemoveConnection("base"); err != nil {
		t.Fatalf("RemoveConnection(base) = %v, want nil (re-point, not refuse)", err)
	}
	if _, ok := cfg.context("base"); ok {
		t.Error("base context should be gone")
	}
	if cfg.CurrentContext != "extra" {
		t.Errorf("current-context should re-point to extra; got %q", cfg.CurrentContext)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("config must stay valid after re-point; got %v", err)
	}
}

func TestRemoveConnectionMissingSecretIsBenign(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t)
	// Add a second connection, then wipe its keychain secret to simulate an absent one.
	if _, err := cfg.AddConnection(NewConnection{
		Name: "extra", Endpoint: "http://h:9000", AccessKeyID: "AKID",
	}, "SECRET"); err != nil {
		t.Fatalf("AddConnection: %v", err)
	}
	_ = keyring.Delete("s3s", "extra")
	if _, err := cfg.RemoveConnection("extra"); err != nil {
		t.Fatalf("RemoveConnection with a missing secret must be benign; got %v", err)
	}
}

func TestRemoveLastConnectionLeavesEmptyConfig(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t)
	// Switch the active context away so "base" is deletable (active is non-deletable).
	cfg.CurrentContext = ""
	names, err := cfg.RemoveConnection("base")
	if err != nil {
		t.Fatalf("removing the last connection should be allowed; got %v", err)
	}
	if len(names) != 0 {
		t.Errorf("after removing the last connection, contexts = %v, want empty", names)
	}
	// An empty config is valid (the no-connection state).
	if err := cfg.Validate(); err != nil {
		t.Errorf("empty config should be valid; got %v", err)
	}
}
