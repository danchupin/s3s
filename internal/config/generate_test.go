package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvVarName(t *testing.T) {
	cases := map[string]string{
		"local":       "S3S_LOCAL_SECRET",
		"prod-public": "S3S_PROD_PUBLIC_SECRET",
		"a.b/c":       "S3S_A_B_C_SECRET",
	}
	for in, want := range cases {
		if got := EnvVarName(in); got != want {
			t.Errorf("EnvVarName(%q) = %q, want %q", in, got, want)
		}
	}
}

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

func TestRunInitWritesEnvRefNotSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Scripted answers: context, cluster(default), endpoint, region(default),
	// pathStyle(default y), tlsSkip(default n), anonymous=n, accessKey, current(default y).
	in := strings.NewReader(strings.Join([]string{
		"local",                 // context
		"",                      // cluster (default = local)
		"http://127.0.0.1:9000", // endpoint
		"",                      // region default
		"",                      // path-style default yes
		"",                      // tls skip default no
		"n",                     // anonymous no
		"env",                   // credential source (env / ${ENV})
		"AKIAEXAMPLE",           // access key id
		"",                      // current context default yes
	}, "\n") + "\n")

	var out bytes.Buffer
	if err := RunInit(in, &out, path); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	body := string(data)
	if strings.Contains(body, "AKIAEXAMPLE") == false {
		t.Errorf("access key id should be written:\n%s", body)
	}
	if !strings.Contains(body, "${S3S_LOCAL_SECRET}") {
		t.Errorf("secret should be an env ref:\n%s", body)
	}
	// Output guides the user to export the secret.
	if !strings.Contains(out.String(), "export S3S_LOCAL_SECRET=") {
		t.Errorf("output should hint export:\n%s", out.String())
	}

	// File perms must be 0600.
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("config perms = %o, want 600", info.Mode().Perm())
	}

	// The written config loads and resolves once the env var is set.
	t.Setenv("S3S_LOCAL_SECRET", "real-secret")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("written config does not load: %v", err)
	}
	cc, err := cfg.ClientConfig(context.Background(), "local")
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if cc.SecretKey != "real-secret" || cc.AccessKeyID != "AKIAEXAMPLE" || !cc.PathStyle {
		t.Errorf("resolved client config wrong: %+v", cc)
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
