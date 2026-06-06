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

// TestKeychainMissing: a missing entry yields a clear, actionable error (FR-043).
func TestKeychainMissing(t *testing.T) {
	keyring.MockInit()
	_, err := GetKeychain("absent")
	if err == nil || !strings.Contains(err.Error(), "no keystore entry") {
		t.Errorf("want a clear missing-entry error, got %v", err)
	}
}
