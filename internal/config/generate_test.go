package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/danchupin/s3s/internal/secret"
)

func TestUpsertReplacesByName(t *testing.T) {
	c := &Config{APIVersion: "s3s/v1"}
	c.Upsert(
		Cluster{Name: "c", Endpoint: "http://a"},
		User{Name: "u", Anonymous: true},
		Context{Name: "x", Cluster: "c", User: "u"},
		true,
	)
	// Re-upsert same names with a changed endpoint → replace, not duplicate.
	c.Upsert(
		Cluster{Name: "c", Endpoint: "http://b"},
		User{Name: "u", Anonymous: true},
		Context{Name: "x", Cluster: "c", User: "u"},
		false,
	)
	if len(c.Clusters) != 1 || c.Clusters[0].Endpoint != "http://b" {
		t.Fatalf("cluster should be replaced in place: %+v", c.Clusters)
	}
	if len(c.Contexts) != 1 || c.CurrentContext != "x" {
		t.Fatalf("context state wrong: %+v current=%q", c.Contexts, c.CurrentContext)
	}
}

// TestRunInitKeychainDefault: pressing Enter at the source prompt selects keychain (the
// blessed default); the secret goes to the OS keystore under the namespaced account and
// never to disk (014 US1/FR-008). promptSecret is stubbed because the real one needs a TTY.
func TestRunInitKeychainDefault(t *testing.T) {
	keyring.MockInit()
	orig := promptSecret
	promptSecret = func(string) (string, error) { return "kc-secret", nil }
	defer func() { promptSecret = orig }()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	in := strings.NewReader(strings.Join([]string{
		"local",                 // context
		"",                      // cluster default
		"http://127.0.0.1:9000", // endpoint
		"",                      // region default
		"",                      // path-style default yes
		"",                      // tls skip default no
		"n",                     // anonymous no
		"",                      // credential source: Enter → keychain (the default)
		"AKIAEXAMPLE",           // access key id
		"",                      // current context default yes
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := RunInit(in, &out, path); err != nil {
		t.Fatalf("RunInit: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "keychain: true") || !strings.Contains(string(body), "AKIAEXAMPLE") {
		t.Errorf("config should declare keychain + accessKeyId:\n%s", body)
	}
	if strings.Contains(string(body), "kc-secret") {
		t.Errorf("secret must NOT be written to disk:\n%s", body)
	}
	got, err := secret.GetKeychain(keychainAccount(path, "local"))
	if err != nil || got != "kc-secret" {
		t.Errorf("secret should be in the keystore under the namespaced account: got %q err %v", got, err)
	}
}

// TestRunInitCmdSource: choosing cmd writes a cmd source (no prompt, no secret on disk).
func TestRunInitCmdSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	in := strings.NewReader(strings.Join([]string{
		"local", "", "http://127.0.0.1:9000", "", "", "", "n",
		"cmd",                                 // credential source
		"AKIAEXAMPLE",                         // access key id
		"vault kv get -field=secret s3/local", // command
		"",                                    // current default
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := RunInit(in, &out, path); err != nil {
		t.Fatalf("RunInit cmd: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "cmd: vault kv get -field=secret s3/local") {
		t.Errorf("config should declare the cmd source:\n%s", body)
	}
	if strings.Contains(string(body), "keychain") {
		t.Errorf("cmd source must not set keychain:\n%s", body)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("written cmd config does not load: %v", err)
	}
	if u, _ := cfg.user("local"); u.Command == "" {
		t.Errorf("loaded user should carry the cmd: %+v", u)
	}
}

func TestRunInitAnonymous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	in := strings.NewReader(strings.Join([]string{
		"pub",                 // context
		"rgw",                 // cluster
		"https://rgw.example", // endpoint
		"us-east-1",           // region
		"n",                   // path-style no
		"",                    // tls skip default no
		"y",                   // anonymous yes
		"",                    // current default yes
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := RunInit(in, &out, path); err != nil {
		t.Fatalf("RunInit anonymous: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cc, err := cfg.ClientConfig(context.Background(), "pub")
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if !cc.Anonymous {
		t.Errorf("anonymous user not set: %+v", cc)
	}
	if strings.Contains(out.String(), "export") {
		t.Errorf("anonymous flow should not ask to export a secret:\n%s", out.String())
	}
}

func TestRunInitMergesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	existing := `apiVersion: s3s/v1
clusters:
  - name: old
    endpoint: http://old:9000
    pathStyle: true
users:
  - name: olduser
    anonymous: true
contexts:
  - name: oldctx
    cluster: old
    user: olduser
current-context: oldctx
`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	in := strings.NewReader(strings.Join([]string{
		"newctx", "newcl", "http://new:9000", "", "", "", "y", "n", // anonymous yes, current=no
	}, "\n") + "\n")
	var out bytes.Buffer
	if err := RunInit(in, &out, path); err != nil {
		t.Fatalf("RunInit merge: %v", err)
	}

	cfg, err := loadRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Contexts) != 2 {
		t.Fatalf("merge should keep both contexts, got %+v", cfg.Contexts)
	}
	if cfg.CurrentContext != "oldctx" {
		t.Errorf("current-context should stay oldctx (setCurrent=no), got %q", cfg.CurrentContext)
	}
}
