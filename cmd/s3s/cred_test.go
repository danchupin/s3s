package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/secret"
)

const credTestConfig = `apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: http://127.0.0.1:9000
    pathStyle: true
users:
  - name: u
    accessKeyId: AK
    keychain: true
contexts:
  - name: ctx
    cluster: c
    user: u
current-context: ctx
`

// TestCredRemoveKeystoreOnly: `s3s cred rm <ctx>` removes the keystore entry and never
// touches the config file (005 FR-037, credential-source-contract C4).
func TestCredRemoveKeystoreOnly(t *testing.T) {
	keyring.MockInit()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(credTestConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	// Seed under the SAME namespaced account that `cred` targets (014 FR-020a).
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	account, err := cfg.KeychainAccount("ctx")
	if err != nil {
		t.Fatal(err)
	}
	if err := secret.StoreKeychain(account, "s3cr3t"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	// Flags must precede positionals for the flag package.
	if err := runCred([]string{"--config", path, "rm", "ctx"}); err != nil {
		t.Fatalf("cred rm: %v", err)
	}
	if _, err := secret.GetKeychain(account); err == nil {
		t.Error("cred rm should remove the keystore entry")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("cred rm must not modify the config file")
	}
}

// TestCredUnknownAction: an unknown action is a clear error.
func TestCredUnknownAction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(credTestConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCred([]string{"--config", path, "frobnicate", "ctx"}); err == nil {
		t.Error("unknown cred action should error")
	}
}
