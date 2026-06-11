package config

import (
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"slices"
	"time"

	"github.com/danchupin/s3s/internal/plugin"
)

// PluginDecl is one declared external capability provider (a user-opted-in
// executable). Declarations live in the top-level plugins: list; an absent
// section keeps the feature fully dormant.
type PluginDecl struct {
	Name        string         `yaml:"name"`
	Capability  string         `yaml:"capability"`
	Cmd         string         `yaml:"cmd"`
	Timeout     *time.Duration `yaml:"timeout,omitempty"`
	Enabled     *bool          `yaml:"enabled,omitempty"`
	Connections []string       `yaml:"connections,omitempty"` // bucket-discovery scope
	Match       *MatchRule     `yaml:"match,omitempty"`       // object-metadata scope
}

// IsEnabled applies the enabled-by-default rule (the flag exists so the in-app
// toggle can persist a disable without deleting the declaration).
func (d PluginDecl) IsEnabled() bool { return d.Enabled == nil || *d.Enabled }

// ResolvedTimeout applies the 5 s default for an absent per-plugin timeout.
func (d PluginDecl) ResolvedTimeout() time.Duration {
	if d.Timeout == nil {
		return plugin.DefaultTimeout
	}
	return *d.Timeout
}

// ScopeConnections is the connection-name scope regardless of capability.
func (d PluginDecl) ScopeConnections() []string {
	if d.Match != nil {
		return d.Match.Connections
	}
	return d.Connections
}

// MatchRule scopes an object-metadata plugin: the object must belong to one of
// the named connections, match a bucket glob (empty ⇒ any bucket), and match
// the RE2 key pattern (empty ⇒ any key).
type MatchRule struct {
	Connections []string `yaml:"connections"`
	Buckets     []string `yaml:"buckets,omitempty"`
	KeyPattern  string   `yaml:"keyPattern,omitempty"`

	keyRE *regexp.Regexp // compiled at validation; lazily on a fresh copy
}

// Matches reports whether (connection, bucket, key) is inside this scope.
func (r *MatchRule) Matches(connection, bucket, key string) bool {
	if !slices.Contains(r.Connections, connection) {
		return false
	}
	if len(r.Buckets) > 0 {
		hit := false
		for _, g := range r.Buckets {
			if ok, err := path.Match(g, bucket); err == nil && ok {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if r.KeyPattern != "" {
		if r.keyRE == nil {
			re, err := regexp.Compile(r.KeyPattern)
			if err != nil {
				return false // validation rejects this at load; defensive here
			}
			r.keyRE = re
		}
		if !r.keyRE.MatchString(key) {
			return false
		}
	}
	return true
}

// validatePlugins enforces the plugins: section rules. A scope referencing an
// unknown connection name is a WARNING (the plugin surfaces as unavailable),
// not a load error — one shared config must stay usable across machines.
func (c *Config) validatePlugins() error {
	names := map[string]bool{}
	known := map[string]bool{}
	for _, cx := range c.Contexts {
		known[cx.Name] = true
	}
	for i := range c.Plugins {
		d := &c.Plugins[i]
		if d.Name == "" {
			return fmt.Errorf("%w: plugin with empty name", ErrInvalid)
		}
		if names[d.Name] {
			return fmt.Errorf("%w: duplicate plugin name %q", ErrInvalid, d.Name)
		}
		names[d.Name] = true
		if !plugin.ValidCapability(d.Capability) {
			return fmt.Errorf("%w: plugin %q has unknown capability %q", ErrInvalid, d.Name, d.Capability)
		}
		if d.Cmd == "" {
			return fmt.Errorf("%w: plugin %q is missing cmd", ErrInvalid, d.Name)
		}
		if d.Timeout != nil && *d.Timeout <= 0 {
			return fmt.Errorf("%w: plugin %q timeout must be > 0", ErrInvalid, d.Name)
		}
		switch plugin.Capability(d.Capability) {
		case plugin.BucketDiscovery:
			if len(d.Connections) == 0 {
				return fmt.Errorf("%w: discovery plugin %q needs connections", ErrInvalid, d.Name)
			}
		case plugin.ObjectMetadata:
			if d.Match == nil || len(d.Match.Connections) == 0 {
				return fmt.Errorf("%w: metadata plugin %q needs match.connections", ErrInvalid, d.Name)
			}
			if d.Match.KeyPattern != "" {
				re, err := regexp.Compile(d.Match.KeyPattern)
				if err != nil {
					return fmt.Errorf("%w: plugin %q keyPattern does not compile: %v", ErrInvalid, d.Name, err)
				}
				d.Match.keyRE = re
			}
		}
		for _, conn := range d.ScopeConnections() {
			if !known[conn] {
				slog.Warn("plugin scope references an unknown connection",
					"plugin", d.Name, "connection", conn)
			}
		}
	}
	return nil
}

// SetPluginEnabled flips the enabled flag of the named plugin declaration and
// persists the config, following the trial-validate-persist-commit discipline
// of every config mutation (atomic write; a failure leaves the live config
// untouched). Idempotent: re-setting the current state still succeeds.
func (c *Config) SetPluginEnabled(name string, enabled bool) error {
	idx := slices.IndexFunc(c.Plugins, func(d PluginDecl) bool { return d.Name == name })
	if idx < 0 {
		return fmt.Errorf("%w: no plugin named %q", ErrNotFound, name)
	}
	trial := *c
	trial.Plugins = slices.Clone(c.Plugins)
	v := enabled
	trial.Plugins[idx].Enabled = &v
	if err := trial.Validate(); err != nil {
		return err
	}
	data, err := Marshal(&trial)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := Save(c.Path(), data); err != nil {
		return err
	}
	c.Plugins = trial.Plugins
	slog.Info("plugin.toggle", "plugin", name, "enabled", enabled, "outcome", "ok")
	return nil
}
