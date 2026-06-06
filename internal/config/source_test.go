package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/logging"
)

func cfgWithUser(u User) *Config {
	return &Config{
		APIVersion: "s3s/v1",
		Clusters:   []Cluster{{Name: "c", Endpoint: "http://x"}},
		Users:      []User{u},
		Contexts:   []Context{{Name: "x", Cluster: "c", User: u.Name}},
	}
}

// TestValidateExactlyOneSource: a non-anonymous user must declare exactly one source
// (005 FR-041); inline/${ENV} still works; anonymous is unaffected.
func TestValidateExactlyOneSource(t *testing.T) {
	if err := cfgWithUser(User{Name: "u", AccessKeyID: "AK", Keychain: true}).Validate(); err != nil {
		t.Errorf("keychain-only should validate: %v", err)
	}
	if err := cfgWithUser(User{Name: "u", AWSProfile: "prod"}).Validate(); err != nil {
		t.Errorf("awsProfile-only (no accessKeyId) should validate: %v", err)
	}
	if err := cfgWithUser(User{Name: "u", AccessKeyID: "AK", SecretAccessKey: logging.Secret("s")}).Validate(); err != nil {
		t.Errorf("inline should validate: %v", err)
	}
	if err := cfgWithUser(User{Name: "u", Anonymous: true}).Validate(); err != nil {
		t.Errorf("anonymous should validate: %v", err)
	}
	err := cfgWithUser(User{Name: "u", AccessKeyID: "AK", Keychain: true, Command: "echo x"}).Validate()
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("two sources should fail with the one-source error, got %v", err)
	}
	if err := cfgWithUser(User{Name: "u"}).Validate(); err == nil {
		t.Error("zero sources (non-anon) should fail")
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
