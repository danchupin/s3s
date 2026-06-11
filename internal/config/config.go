// Package config loads and validates the kubectl-style YAML config
// (~/.config/s3s/config.yaml): clusters, users, contexts, and current-context.
// The secret is never stored in the config (014 FR-008) — a non-anonymous user names
// the OS keychain or an external command. A ${ENV} reference in the non-secret
// accessKeyId is resolved at load.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	yaml "go.yaml.in/yaml/v3"
)

// Error sentinels (all messages are secret-free).
var (
	ErrNotFound = errors.New("config: file not found")
	ErrInvalid  = errors.New("config: invalid")
	ErrEnvUnset = errors.New("config: referenced environment variable not set")
)

// Config is the top-level config file root.
type Config struct {
	APIVersion     string    `yaml:"apiVersion"`
	Clusters       []Cluster `yaml:"clusters"`
	Users          []User    `yaml:"users"`
	Contexts       []Context `yaml:"contexts"`
	CurrentContext string    `yaml:"current-context"`
	DownloadDir    string    `yaml:"downloadDir,omitempty"` // default local download dir (005 FR-007)
	// UsageScanBudget caps the ambient (hover) usage scan in enumerated objects
	// (017 US1/FR-006). nil ⇒ DefaultUsageScanBudget; 0 ⇒ ambient scanning off
	// (explicit-only); negative ⇒ invalid.
	UsageScanBudget *int `yaml:"usageScanBudget,omitempty"`
	// Health-card small-object warning knobs (017 US4/FR-023): the warning fires when
	// more than HealthSmallObjectShare of enumerated objects fall below
	// HealthSmallObjectKiB. Zero values resolve to the 128 KiB / 0.5 defaults.
	HealthSmallObjectKiB   int     `yaml:"healthSmallObjectKiB,omitempty"`
	HealthSmallObjectShare float64 `yaml:"healthSmallObjectShare,omitempty"`

	path string // source file path (set by Load) — for the cmd-source owner-only gate
}

// ResolvedHealthSmallObjectKiB applies the 128 KiB default (017 D8).
func (c *Config) ResolvedHealthSmallObjectKiB() int {
	if c.HealthSmallObjectKiB == 0 {
		return 128
	}
	return c.HealthSmallObjectKiB
}

// ResolvedHealthSmallObjectShare applies the 0.5 default (017 D8).
func (c *Config) ResolvedHealthSmallObjectShare() float64 {
	if c.HealthSmallObjectShare == 0 {
		return 0.5
	}
	return c.HealthSmallObjectShare
}

// DefaultUsageScanBudget is the ambient usage-scan cap when usageScanBudget is absent
// (≈20 listing pages — 017 research D2).
const DefaultUsageScanBudget = 20000

// ResolvedUsageScanBudget applies the default for an absent usageScanBudget.
func (c *Config) ResolvedUsageScanBudget() int {
	if c.UsageScanBudget == nil {
		return DefaultUsageScanBudget
	}
	return *c.UsageScanBudget
}

// Path returns the file this config was loaded from (empty for an in-memory config).
func (c *Config) Path() string { return c.path }

// InsecurePerms reports whether the file at path is group/world readable — a config
// holding or referencing secrets should be owner-only (005 FR-040). Missing/unstattable
// files are treated as not-insecure (other code reports the real error).
func InsecurePerms(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().Perm()&0o077 != 0
}

// Cluster is a named S3-compatible endpoint.
type Cluster struct {
	Name          string   `yaml:"name"`
	Endpoint      string   `yaml:"endpoint"`
	Region        string   `yaml:"region,omitempty"`
	PathStyle     bool     `yaml:"pathStyle"`
	TLSSkipVerify bool     `yaml:"tlsSkipVerify,omitempty"`
	Buckets       []string `yaml:"buckets,omitempty"` // pinned bucket names (010) — empty ⇒ list-all
}

// User is a named credential. A non-anonymous user names exactly ONE credential
// source (014 FR-002): the OS keychain or an external command. The secret itself is
// never stored in the config (FR-008) — it lives in the keystore or the command's
// stdout.
type User struct {
	Name        string `yaml:"name"`
	Anonymous   bool   `yaml:"anonymous,omitempty"`
	AccessKeyID string `yaml:"accessKeyId,omitempty"`
	Keychain    bool   `yaml:"keychain,omitempty"` // OS keystore source (014)
	Command     string `yaml:"cmd,omitempty"`      // external-command source (014)
}

// sourceCount reports how many credential sources the user declares. Exactly one is
// required for a non-anonymous user (014 FR-002).
func (u User) sourceCount() int {
	n := 0
	if u.Keychain {
		n++
	}
	if u.Command != "" {
		n++
	}
	return n
}

// Context binds a cluster and a user under a selectable name. ReadOnly marks the
// context as protected: it refuses all mutations even under the global --write
// switch (FR-002).
type Context struct {
	Name     string `yaml:"name"`
	Cluster  string `yaml:"cluster"`
	User     string `yaml:"user"`
	ReadOnly bool   `yaml:"readonly,omitempty"`
}

// DefaultRegion is used when a cluster omits region.
const DefaultRegion = "us-east-1"

// envRef matches a whole-string ${VAR} reference.
var envRef = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

// DefaultPath resolves the config path via XDG, falling back to ~/.config.
func DefaultPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "s3s", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "s3s", "config.yaml")
	}
	return filepath.Join(home, ".config", "s3s", "config.yaml")
}

// Empty returns a valid, connection-less config bound to path — the "no connections yet"
// state (009 first-run). AddConnection writes the first triple to path (creating the parent
// dir), so a fresh install needs no `config init` before adding a connection in-app.
func Empty(path string) *Config { return &Config{path: path} }

// Load reads, env-resolves, and validates the config at path.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("%w: parse error", ErrInvalid)
	}
	if err := c.resolveEnv(); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	c.path = path
	return &c, nil
}

// resolveEnv replaces a ${VAR} reference in the non-secret accessKeyId with its
// environment value. A referenced-but-unset var is an error. The secret itself is
// never an inline/${ENV} value (014 FR-004) — it comes from the keychain or a command.
func (c *Config) resolveEnv() error {
	for i := range c.Users {
		u := &c.Users[i]
		ak, err := resolveRef(u.AccessKeyID)
		if err != nil {
			return fmt.Errorf("user %q accessKeyId: %w", u.Name, err)
		}
		u.AccessKeyID = ak
	}
	return nil
}

// resolveRef returns env value for a ${VAR} reference, the input unchanged for an
// inline value, and an error if a referenced var is unset.
func resolveRef(v string) (string, error) {
	m := envRef.FindStringSubmatch(v)
	if m == nil {
		return v, nil // inline value
	}
	val, ok := os.LookupEnv(m[1])
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrEnvUnset, m[1])
	}
	return val, nil
}

// Validate enforces structural and cross-reference rules (contracts/config-schema.md).
func (c *Config) Validate() error {
	// A fully-empty config is the valid "no connections yet" state (007 US5: the last
	// connection is deletable, leaving the app in its add-connection state). A PARTIAL
	// set is still invalid. The cross-reference + dangling-current-context checks below
	// STILL run for the empty case (a non-empty current-context over zero contexts is
	// caught there, review #5).
	allEmpty := len(c.Clusters) == 0 && len(c.Users) == 0 && len(c.Contexts) == 0
	if !allEmpty && (len(c.Clusters) == 0 || len(c.Users) == 0 || len(c.Contexts) == 0) {
		return fmt.Errorf("%w: need at least one cluster, user, and context", ErrInvalid)
	}

	if c.UsageScanBudget != nil && *c.UsageScanBudget < 0 {
		return fmt.Errorf("%w: usageScanBudget must be >= 0", ErrInvalid)
	}
	if c.HealthSmallObjectKiB < 0 {
		return fmt.Errorf("%w: healthSmallObjectKiB must be >= 0", ErrInvalid)
	}
	if c.HealthSmallObjectShare < 0 || c.HealthSmallObjectShare > 1 {
		return fmt.Errorf("%w: healthSmallObjectShare must be within (0,1]", ErrInvalid)
	}

	clusters := map[string]bool{}
	for _, cl := range c.Clusters {
		if cl.Name == "" {
			return fmt.Errorf("%w: cluster with empty name", ErrInvalid)
		}
		if clusters[cl.Name] {
			return fmt.Errorf("%w: duplicate cluster name %q", ErrInvalid, cl.Name)
		}
		clusters[cl.Name] = true
		u, err := url.Parse(cl.Endpoint)
		if err != nil || !u.IsAbs() || u.Host == "" {
			return fmt.Errorf("%w: cluster %q endpoint is not a valid absolute URL", ErrInvalid, cl.Name)
		}
	}

	users := map[string]bool{}
	for _, us := range c.Users {
		if us.Name == "" {
			return fmt.Errorf("%w: user with empty name", ErrInvalid)
		}
		if users[us.Name] {
			return fmt.Errorf("%w: duplicate user name %q", ErrInvalid, us.Name)
		}
		users[us.Name] = true
		if !us.Anonymous {
			switch us.sourceCount() {
			case 0:
				return fmt.Errorf("%w: user %q is not anonymous but missing credentials — declare keychain or cmd", ErrInvalid, us.Name)
			case 1:
				// Both sources need the (non-secret) accessKeyId from config.
				if us.AccessKeyID == "" {
					return fmt.Errorf("%w: user %q is missing accessKeyId", ErrInvalid, us.Name)
				}
			default:
				return fmt.Errorf("%w: user %q declares more than one credential source — choose exactly one (keychain | cmd)", ErrInvalid, us.Name)
			}
		}
	}

	contexts := map[string]bool{}
	for _, cx := range c.Contexts {
		if cx.Name == "" {
			return fmt.Errorf("%w: context with empty name", ErrInvalid)
		}
		if contexts[cx.Name] {
			return fmt.Errorf("%w: duplicate context name %q", ErrInvalid, cx.Name)
		}
		contexts[cx.Name] = true
		if !clusters[cx.Cluster] {
			return fmt.Errorf("%w: context %q references unknown cluster %q", ErrInvalid, cx.Name, cx.Cluster)
		}
		if !users[cx.User] {
			return fmt.Errorf("%w: context %q references unknown user %q", ErrInvalid, cx.Name, cx.User)
		}
	}

	if c.CurrentContext != "" && !contexts[c.CurrentContext] {
		return fmt.Errorf("%w: current-context %q does not resolve", ErrInvalid, c.CurrentContext)
	}
	return nil
}

// cluster returns the named cluster.
func (c *Config) cluster(name string) (Cluster, bool) {
	for _, cl := range c.Clusters {
		if cl.Name == name {
			return cl, true
		}
	}
	return Cluster{}, false
}

// user returns the named user.
func (c *Config) user(name string) (User, bool) {
	for _, u := range c.Users {
		if u.Name == name {
			return u, true
		}
	}
	return User{}, false
}

// context returns the named context.
func (c *Config) context(name string) (Context, bool) {
	for _, cx := range c.Contexts {
		if cx.Name == name {
			return cx, true
		}
	}
	return Context{}, false
}

// ContextNames lists context names in declaration order (for the switcher).
func (c *Config) ContextNames() []string {
	out := make([]string, 0, len(c.Contexts))
	for _, cx := range c.Contexts {
		out = append(out, cx.Name)
	}
	return out
}
