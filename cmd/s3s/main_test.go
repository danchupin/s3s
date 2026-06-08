package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danchupin/s3s/internal/config"
)

const launchConfig = `apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: http://127.0.0.1:9000
users:
  - name: u
    anonymous: true
contexts:
  - name: ctx
    cluster: c
    user: u
current-context: ctx
`

// TestLoadForLaunchExplicitMissingErrors: an explicitly named missing config (via flag or
// S3S_CONFIG env) is a hard error, not the first-run state (014 FR-017).
func TestLoadForLaunchExplicitMissingErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	if _, _, _, err := loadForLaunch(missing, ""); err == nil {
		t.Error("explicit --config missing should error")
	}
	if _, _, _, err := loadForLaunch("", missing); err == nil {
		t.Error("explicit S3S_CONFIG missing should error")
	}
}

// TestLoadForLaunchDefaultFirstRun: a missing DEFAULT config first-runs (no error) so the
// TUI opens the add-connection form (009 / 014 FR-017).
func TestLoadForLaunchDefaultFirstRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // default path resolves under an empty temp dir
	cfg, _, firstRun, err := loadForLaunch("", "")
	if err != nil {
		t.Fatalf("default missing config should first-run, not error: %v", err)
	}
	if !firstRun || cfg == nil {
		t.Errorf("expected first-run with an empty config; firstRun=%v cfg=%v", firstRun, cfg)
	}
}

// TestLoadForLaunchPrecedenceAndAltUntouchedDefault: the flag wins over env, and launching
// against an explicit alt config leaves the default config byte-for-byte unchanged (014
// FR-014, SC-005).
func TestLoadForLaunchPrecedenceAndAltUntouchedDefault(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	defPath := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defPath, []byte(launchConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(defPath)

	alt := filepath.Join(t.TempDir(), "alt.yaml")
	if err := os.WriteFile(alt, []byte(launchConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(t.TempDir(), "env.yaml")
	if err := os.WriteFile(env, []byte(launchConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	// Flag wins over env; the resolved path is the flag's.
	if _, path, _, err := loadForLaunch(alt, env); err != nil || path != alt {
		t.Fatalf("flag should win over env; path=%q err=%v", path, err)
	}

	after, _ := os.ReadFile(defPath)
	if string(before) != string(after) {
		t.Error("default config must be byte-for-byte unchanged when running against an alt config")
	}
}
