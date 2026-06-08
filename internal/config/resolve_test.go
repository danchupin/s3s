package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/danchupin/s3s/internal/secret"
)

func TestActiveContextPrecedence(t *testing.T) {
	cases := []struct {
		flag, env, current, want string
	}{
		{"flagctx", "envctx", "curctx", "flagctx"}, // flag wins
		{"", "envctx", "curctx", "envctx"},         // env over current
		{"", "", "curctx", "curctx"},               // current fallback
		{"", "", "", ""},                           // nothing
	}
	for _, c := range cases {
		if got := ActiveContextName(c.flag, c.env, c.current); got != c.want {
			t.Errorf("ActiveContextName(%q,%q,%q) = %q, want %q", c.flag, c.env, c.current, got, c.want)
		}
	}
}

func TestResolveClientConfig(t *testing.T) {
	keyring.MockInit()
	path := writeConfig(t, validYAML)
	if err := secret.StoreKeychain(keychainAccount(path, "dev"), "k"); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	cc, err := c.ClientConfig(context.Background(), "local")
	if err != nil {
		t.Fatal(err)
	}
	if cc.Endpoint != "http://127.0.0.1:9000" || !cc.PathStyle || cc.Region != "us-east-1" {
		t.Errorf("client config wrong: %+v", cc)
	}
	if cc.Anonymous || cc.AccessKeyID != "admin" || cc.SecretKey != "k" {
		t.Errorf("creds wrong: %+v", cc)
	}

	pub, err := c.ClientConfig(context.Background(), "pub")
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Anonymous {
		t.Errorf("pub context should be anonymous: %+v", pub)
	}
}

func TestResolveUnknownContext(t *testing.T) {
	c, _ := Load(writeConfig(t, validYAML))
	if _, _, err := c.Resolve("ghost"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown context: want ErrInvalid, got %v", err)
	}
}

// TestConfigPathPrecedence: --config flag > S3S_CONFIG env > DefaultPath() (014 FR-014).
func TestConfigPathPrecedence(t *testing.T) {
	if got := ResolvePath("/flag.yaml", "/env.yaml"); got != "/flag.yaml" {
		t.Errorf("flag must win, got %q", got)
	}
	if got := ResolvePath("", "/env.yaml"); got != "/env.yaml" {
		t.Errorf("env over default, got %q", got)
	}
	if got := ResolvePath("", ""); got != DefaultPath() {
		t.Errorf("neither set → DefaultPath(), got %q want %q", got, DefaultPath())
	}
}

// TestKeychainAccountIsolation: two configs at different paths that each define a user
// named "dev" must map to DISTINCT keystore accounts, so a secret stored under one is
// invisible to the other (014 FR-020a).
func TestKeychainAccountIsolation(t *testing.T) {
	keyring.MockInit()
	dirA := writeConfig(t, validYAML)
	dirB := filepath.Join(t.TempDir(), "other.yaml")
	if err := os.WriteFile(dirB, []byte(validYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	accA := keychainAccount(dirA, "dev")
	accB := keychainAccount(dirB, "dev")
	if accA == accB {
		t.Fatalf("same user name under different configs must not share an account: %q", accA)
	}
	if err := secret.StoreKeychain(accA, "secret-A"); err != nil {
		t.Fatal(err)
	}
	// Config B resolves: its account has no entry → a clear error, never config A's secret.
	cB, err := Load(dirB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cB.ClientConfig(context.Background(), "local"); err == nil {
		t.Error("config B must NOT resolve config A's secret (accounts are namespaced)")
	}
}

// TestCredAndResolutionAgreeOnAccount: the account `s3s cred` / offerSaveToKeychain write
// to (Config.KeychainAccount) is the SAME account runtime resolution reads from
// (secretRequest). This is what makes "save the prompted secret for next time" (FR-007)
// actually work after namespacing (FR-020a).
func TestCredAndResolutionAgreeOnAccount(t *testing.T) {
	keyring.MockInit()
	path := writeConfig(t, validYAML)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	account, err := c.KeychainAccount("local") // what cred/offer use
	if err != nil {
		t.Fatal(err)
	}
	if err := secret.StoreKeychain(account, "saved-secret"); err != nil {
		t.Fatal(err)
	}
	cc, err := c.ClientConfig(context.Background(), "local") // what resolution reads
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if cc.SecretKey != "saved-secret" {
		t.Errorf("cred/offer and resolution must agree on the keystore account; got %q", cc.SecretKey)
	}
}

// TestActiveContextResolvesAgainstSelectedConfig: the active-context precedence resolves
// against whichever config was selected, so an alt config's current-context wins (FR-016).
func TestActiveContextResolvesAgainstSelectedConfig(t *testing.T) {
	altBody := `
apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: https://alt.example
users:
  - name: pubu
    anonymous: true
contexts:
  - name: altctx
    cluster: c
    user: pubu
current-context: altctx
`
	alt := writeConfig(t, altBody)
	c, err := Load(ResolvePath(alt, ""))
	if err != nil {
		t.Fatal(err)
	}
	active := ActiveContextName("", "", c.CurrentContext)
	if active != "altctx" {
		t.Fatalf("active context should come from the selected config, got %q", active)
	}
	if _, _, err := c.Resolve(active); err != nil {
		t.Errorf("selected config's active context must resolve: %v", err)
	}
}

func TestResolveDefaultsRegion(t *testing.T) {
	body := `
apiVersion: s3s/v1
clusters:
  - name: c
    endpoint: https://example.com
    pathStyle: false
users:
  - name: public
    anonymous: true
contexts:
  - name: x
    cluster: c
    user: public
current-context: x
`
	c, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatal(err)
	}
	cl, _, err := c.Resolve("x")
	if err != nil {
		t.Fatal(err)
	}
	if cl.Region != DefaultRegion {
		t.Errorf("region = %q, want default %q", cl.Region, DefaultRegion)
	}
}
