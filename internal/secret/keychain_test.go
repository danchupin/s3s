package secret

import (
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestKeychainRoundTrip: store/fetch/remove against a mock keystore (005 FR-037, C2).
func TestKeychainRoundTrip(t *testing.T) {
	keyring.MockInit()
	if err := StoreKeychain("ctx", "s3cr3t"); err != nil {
		t.Fatalf("store: %v", err)
	}
	got, err := GetKeychain("ctx")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("get = %q, want s3cr3t", got)
	}
	if err := RemoveKeychain("ctx"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := GetKeychain("ctx"); err == nil {
		t.Error("get after remove should error")
	}
}

// TestKeychainMissing: a missing entry yields a clear, actionable error pointing at
// `s3s cred set` or a cmd source (FR-020/043).
func TestKeychainMissing(t *testing.T) {
	keyring.MockInit()
	_, err := GetKeychain("absent")
	if err == nil || !strings.Contains(err.Error(), "no keystore entry") {
		t.Errorf("want a clear missing-entry error, got %v", err)
	}
	if !strings.Contains(err.Error(), "cmd") {
		t.Errorf("missing-entry error should mention the cmd alternative, got %v", err)
	}
}

// TestNoKeystoreErrorNamesCmd: the unavailable-keystore sentinel (headless Linux, no
// Secret Service) names the cmd source as the remedy and never implies a plaintext
// fallback (FR-020).
func TestNoKeystoreErrorNamesCmd(t *testing.T) {
	if !strings.Contains(ErrNoKeystore.Error(), "cmd") {
		t.Errorf("ErrNoKeystore should point at a cmd source, got %q", ErrNoKeystore.Error())
	}
}
