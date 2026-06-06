package secret

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeCfg(t *testing.T, perm os.FileMode) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("x"), perm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, perm); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCommandSecretOwnerOnly: an owner-only config runs the command; stdout (trimmed)
// is the secret (005 FR-036, C3).
func TestCommandSecretOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("printf/perms differ on windows")
	}
	cfg := writeCfg(t, 0o600)
	s, err := commandSecret(context.Background(), "printf %s my-secret", cfg)
	if err != nil {
		t.Fatalf("owner-only config should run: %v", err)
	}
	if s != "my-secret" {
		t.Errorf("secret = %q, want my-secret", s)
	}
}

// TestCommandSecretRefusesLoosePerms: a group/world-writable config refuses to exec
// the command (FR-036).
func TestCommandSecretRefusesLoosePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("perms model differs on windows")
	}
	cfg := writeCfg(t, 0o666)
	_, err := commandSecret(context.Background(), "printf %s x", cfg)
	if err == nil || !strings.Contains(err.Error(), "group/world-writable") {
		t.Errorf("want refusal on loose perms, got %v", err)
	}
}

// TestCommandSecretQuotedArgs: POSIX shell-words keeps a quoted argument as one token.
func TestCommandSecretQuotedArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("printf differs on windows")
	}
	cfg := writeCfg(t, 0o600)
	s, err := commandSecret(context.Background(), `printf %s "hello world"`, cfg)
	if err != nil {
		t.Fatalf("quoted command: %v", err)
	}
	if s != "hello world" {
		t.Errorf("quoted arg split wrong: %q", s)
	}
}
