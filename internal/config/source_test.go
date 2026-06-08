package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cfgWithUser(u User) *Config {
	return &Config{
		APIVersion: "s3s/v1",
		Clusters:   []Cluster{{Name: "c", Endpoint: "http://x"}},
		Users:      []User{u},
		Contexts:   []Context{{Name: "x", Cluster: "c", User: u.Name}},
	}
}

// TestValidateExactlyOneSource: a non-anonymous user must declare exactly one of the two
// sources — keychain or cmd (014 FR-002); anonymous is unaffected.
func TestValidateExactlyOneSource(t *testing.T) {
	if err := cfgWithUser(User{Name: "u", AccessKeyID: "AK", Keychain: true}).Validate(); err != nil {
		t.Errorf("keychain-only should validate: %v", err)
	}
	if err := cfgWithUser(User{Name: "u", AccessKeyID: "AK", Command: "vault kv get -field=secret s3/u"}).Validate(); err != nil {
		t.Errorf("cmd-only should validate: %v", err)
	}
	if err := cfgWithUser(User{Name: "u", Anonymous: true}).Validate(); err != nil {
		t.Errorf("anonymous should validate: %v", err)
	}
	if err := cfgWithUser(User{Name: "u", Keychain: true}).Validate(); err == nil {
		t.Error("keychain without accessKeyId should fail")
	}
	err := cfgWithUser(User{Name: "u", AccessKeyID: "AK", Keychain: true, Command: "echo x"}).Validate()
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("two sources should fail with the one-source error, got %v", err)
	}
	zeroErr := cfgWithUser(User{Name: "u"}).Validate()
	if zeroErr == nil || !strings.Contains(zeroErr.Error(), "keychain or cmd") {
		t.Errorf("zero sources (non-anon) should fail naming keychain/cmd, got %v", zeroErr)
	}
}

// TestInsecurePerms: a group/world-readable config is flagged (005 FR-040).
func TestInsecurePerms(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(p, 0o600)
	if InsecurePerms(p) {
		t.Error("0600 should be secure")
	}
	_ = os.Chmod(p, 0o644)
	if !InsecurePerms(p) {
		t.Error("0644 (group/world-readable) should be insecure")
	}
}
