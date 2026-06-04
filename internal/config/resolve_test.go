package config

import (
	"errors"
	"testing"
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
	t.Setenv("S3S_TEST_SECRET", "k")
	c, err := Load(writeConfig(t, validYAML))
	if err != nil {
		t.Fatal(err)
	}

	cc, err := c.ClientConfig("local")
	if err != nil {
		t.Fatal(err)
	}
	if cc.Endpoint != "http://127.0.0.1:9000" || !cc.PathStyle || cc.Region != "us-east-1" {
		t.Errorf("client config wrong: %+v", cc)
	}
	if cc.Anonymous || cc.AccessKeyID != "admin" || cc.SecretKey != "k" {
		t.Errorf("creds wrong: %+v", cc)
	}

	pub, err := c.ClientConfig("pub")
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Anonymous {
		t.Errorf("pub context should be anonymous: %+v", pub)
	}
}

func TestResolveUnknownContext(t *testing.T) {
	t.Setenv("S3S_TEST_SECRET", "k")
	c, _ := Load(writeConfig(t, validYAML))
	if _, _, err := c.Resolve("ghost"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown context: want ErrInvalid, got %v", err)
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
