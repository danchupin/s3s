// Package config loads and validates the kubectl-style YAML config
// (~/.config/s3s/config.yaml): clusters, users, contexts, and current-context.
// Secrets are held as logging.Secret so they never leak through logs or display
// (FR-005). ${ENV} references are resolved at load (FR-005, contracts/config-schema.md).
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/danchupin/s3s/internal/logging"
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
}

// Cluster is a named S3-compatible endpoint.
type Cluster struct {
	Name          string `yaml:"name"`
	Endpoint      string `yaml:"endpoint"`
	Region        string `yaml:"region"`
	PathStyle     bool   `yaml:"pathStyle"`
	TLSSkipVerify bool   `yaml:"tlsSkipVerify"`
}

// User is a named credential. Secrets are redacted everywhere (FR-005).
type User struct {
	Name            string         `yaml:"name"`
	Anonymous       bool           `yaml:"anonymous"`
	AccessKeyID     string         `yaml:"accessKeyId"`
	SecretAccessKey logging.Secret `yaml:"secretAccessKey"`
	SessionToken    logging.Secret `yaml:"sessionToken"`
}

// Context binds a cluster and a user under a selectable name.
type Context struct {
	Name    string `yaml:"name"`
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
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
	return &c, nil
}

// resolveEnv replaces ${VAR} references in credential fields with environment
// values (env-over-inline precedence). A referenced-but-unset var is an error.
func (c *Config) resolveEnv() error {
	for i := range c.Users {
		u := &c.Users[i]
		ak, err := resolveRef(u.AccessKeyID)
		if err != nil {
			return fmt.Errorf("user %q accessKeyId: %w", u.Name, err)
		}
		u.AccessKeyID = ak

		sk, err := resolveRef(string(u.SecretAccessKey))
		if err != nil {
			return fmt.Errorf("user %q secretAccessKey: %w", u.Name, err)
		}
		u.SecretAccessKey = logging.Secret(sk)

		st, err := resolveRef(string(u.SessionToken))
		if err != nil {
			return fmt.Errorf("user %q sessionToken: %w", u.Name, err)
		}
		u.SessionToken = logging.Secret(st)
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
	if len(c.Clusters) == 0 || len(c.Users) == 0 || len(c.Contexts) == 0 {
		return fmt.Errorf("%w: need at least one cluster, user, and context", ErrInvalid)
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
			if us.AccessKeyID == "" || us.SecretAccessKey.IsEmpty() {
				return fmt.Errorf("%w: user %q is not anonymous but missing credentials", ErrInvalid, us.Name)
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
