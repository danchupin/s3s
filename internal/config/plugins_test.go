package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pluginsYAML wraps a plugins: section in a minimal valid config (one anonymous
// connection named prod-rgw).
func pluginsYAML(section string) string {
	return `apiVersion: s3s/v1
clusters:
  - name: prod-rgw
    endpoint: http://localhost:9000
users:
  - name: prod-rgw
    anonymous: true
contexts:
  - name: prod-rgw
    cluster: prod-rgw
    user: prod-rgw
current-context: prod-rgw
` + section
}

func loadPlugins(t *testing.T, yaml string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestPluginsParseDefaults(t *testing.T) {
	cfg, err := loadPlugins(t, pluginsYAML(`plugins:
  - name: disco
    capability: bucket-discovery
    cmd: "discover --cluster prod"
    connections: [prod-rgw]
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Plugins) != 1 {
		t.Fatalf("plugins = %d, want 1", len(cfg.Plugins))
	}
	d := cfg.Plugins[0]
	if d.Name != "disco" || d.Capability != "bucket-discovery" || d.Cmd != "discover --cluster prod" {
		t.Errorf("decl = %+v", d)
	}
	if !d.IsEnabled() {
		t.Error("enabled must default to true")
	}
	if d.ResolvedTimeout() != 5*time.Second {
		t.Errorf("timeout default = %v, want 5s", d.ResolvedTimeout())
	}
}

func TestPluginsParseExplicit(t *testing.T) {
	cfg, err := loadPlugins(t, pluginsYAML(`plugins:
  - name: meta
    capability: object-metadata
    cmd: "meta-cmd"
    timeout: 3s
    enabled: false
    match:
      connections: [prod-rgw]
      buckets: ["images-*"]
      keyPattern: "^[0-9a-f]{32}"
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	d := cfg.Plugins[0]
	if d.IsEnabled() {
		t.Error("enabled: false must parse")
	}
	if d.ResolvedTimeout() != 3*time.Second {
		t.Errorf("timeout = %v, want 3s", d.ResolvedTimeout())
	}
	if d.Match == nil || len(d.Match.Buckets) != 1 || d.Match.KeyPattern == "" {
		t.Errorf("match = %+v", d.Match)
	}
}

func TestPluginsValidationMatrix(t *testing.T) {
	cases := []struct {
		name    string
		section string
	}{
		{"duplicate name", `plugins:
  - {name: p, capability: bucket-discovery, cmd: c, connections: [prod-rgw]}
  - {name: p, capability: bucket-discovery, cmd: c, connections: [prod-rgw]}
`},
		{"empty name", `plugins:
  - {name: "", capability: bucket-discovery, cmd: c, connections: [prod-rgw]}
`},
		{"unknown capability", `plugins:
  - {name: p, capability: actions, cmd: c, connections: [prod-rgw]}
`},
		{"empty cmd", `plugins:
  - {name: p, capability: bucket-discovery, cmd: "", connections: [prod-rgw]}
`},
		{"zero timeout", `plugins:
  - {name: p, capability: bucket-discovery, cmd: c, timeout: 0s, connections: [prod-rgw]}
`},
		{"negative timeout", `plugins:
  - {name: p, capability: bucket-discovery, cmd: c, timeout: -1s, connections: [prod-rgw]}
`},
		{"discovery without connections", `plugins:
  - {name: p, capability: bucket-discovery, cmd: c}
`},
		{"metadata without match", `plugins:
  - {name: p, capability: object-metadata, cmd: c}
`},
		{"metadata without match connections", `plugins:
  - name: p
    capability: object-metadata
    cmd: c
    match:
      buckets: ["x-*"]
`},
		{"non-compiling keyPattern", `plugins:
  - name: p
    capability: object-metadata
    cmd: c
    match:
      connections: [prod-rgw]
      keyPattern: "([unclosed"
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadPlugins(t, pluginsYAML(tc.section))
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("want ErrInvalid, got %v", err)
			}
		})
	}
}

func TestPluginsUnknownConnectionIsWarningOnly(t *testing.T) {
	cfg, err := loadPlugins(t, pluginsYAML(`plugins:
  - {name: p, capability: bucket-discovery, cmd: c, connections: [other-machine]}
`))
	if err != nil {
		t.Fatalf("unknown connection must not fail the load: %v", err)
	}
	if len(cfg.Plugins) != 1 {
		t.Fatal("declaration must survive the load")
	}
}

func TestMatchRuleMatches(t *testing.T) {
	cases := []struct {
		name string
		rule MatchRule
		conn, bucket, key string
		want bool
	}{
		{"full match", MatchRule{Connections: []string{"prod"}, Buckets: []string{"images-*"}, KeyPattern: "^[0-9a-f]{4}"},
			"prod", "images-prod", "0af3.jpg", true},
		{"wrong connection", MatchRule{Connections: []string{"prod"}, Buckets: []string{"images-*"}},
			"stage", "images-prod", "k", false},
		{"glob miss", MatchRule{Connections: []string{"prod"}, Buckets: []string{"images-*"}},
			"prod", "logs", "k", false},
		{"key pattern miss", MatchRule{Connections: []string{"prod"}, KeyPattern: "^[0-9a-f]{4}"},
			"prod", "any", "not-hex.jpg", false},
		{"empty buckets is wildcard", MatchRule{Connections: []string{"prod"}},
			"prod", "anything", "any-key", true},
		{"empty keyPattern is wildcard", MatchRule{Connections: []string{"prod"}, Buckets: []string{"b*"}},
			"prod", "bx", "whatever", true},
		{"second glob hits", MatchRule{Connections: []string{"prod"}, Buckets: []string{"logs-*", "images-*"}},
			"prod", "images-prod", "k", true},
		{"unanchored pattern searches", MatchRule{Connections: []string{"prod"}, KeyPattern: "thumb"},
			"prod", "b", "photos/thumb/1.jpg", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.rule
			if got := r.Matches(tc.conn, tc.bucket, tc.key); got != tc.want {
				t.Errorf("Matches(%q,%q,%q) = %v, want %v", tc.conn, tc.bucket, tc.key, got, tc.want)
			}
		})
	}
}

func TestPluginsAbsentSectionLoads(t *testing.T) {
	cfg, err := loadPlugins(t, pluginsYAML(""))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Plugins) != 0 {
		t.Errorf("plugins = %v, want none", cfg.Plugins)
	}
}
