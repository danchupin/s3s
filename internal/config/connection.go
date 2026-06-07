package config

import (
	"fmt"
	"log/slog"

	"github.com/danchupin/s3s/internal/secret"
)

// NewConnection is the UI-agnostic description of a connection to add (006 US4). The
// secret is passed separately and never lands in the config — it goes to the keychain.
type NewConnection struct {
	Name        string
	Endpoint    string
	Region      string
	AccessKeyID string
	PathStyle   bool
	ReadOnly    bool
}

// AddConnection persists a new connection by mapping one draft onto the existing config
// model: a cluster + user + context triple (names derived from Name; on-disk schema
// unchanged, 006 FR-022a). It mutates THIS in-memory config (so a live context switch sees
// the new context without a restart, FR-025) and writes it back to c.Path().
//
// The secret is stored in the OS keychain FIRST; only if that succeeds is the config
// rewritten (FR-026: never a context pointing at a missing secret; a stray keychain entry
// on a later config failure is harmless and overwritten on retry). The config keeps only
// keychain:true — no plaintext secret (FR-022). Returns the updated context-name list.
func (c *Config) AddConnection(nc NewConnection, secretVal string) ([]string, error) {
	// Uniqueness: reject a name that collides with an existing context/cluster/user
	// (FR-024) before touching the keychain or the file.
	for _, cx := range c.Contexts {
		if cx.Name == nc.Name {
			return nil, fmt.Errorf("%w: a context named %q already exists", ErrInvalid, nc.Name)
		}
	}
	for _, cl := range c.Clusters {
		if cl.Name == nc.Name {
			return nil, fmt.Errorf("%w: a cluster named %q already exists", ErrInvalid, nc.Name)
		}
	}
	for _, u := range c.Users {
		if u.Name == nc.Name {
			return nil, fmt.Errorf("%w: a user named %q already exists", ErrInvalid, nc.Name)
		}
	}

	cl := Cluster{Name: nc.Name, Endpoint: nc.Endpoint, Region: nc.Region, PathStyle: nc.PathStyle}
	u := User{Name: nc.Name, AccessKeyID: nc.AccessKeyID, Keychain: true}
	cx := Context{Name: nc.Name, Cluster: nc.Name, User: nc.Name, ReadOnly: nc.ReadOnly}

	// Validate a TRIAL copy before mutating the live config or touching the keychain — so
	// an invalid triple never corrupts the shared *Config (which the resolver closure
	// reuses) and never blocks future adds. The trial appends (names are unique, checked
	// above) and preserves current-context (FR-025: setCurrent stays off).
	trial := *c
	trial.Clusters = append(append([]Cluster{}, c.Clusters...), cl)
	trial.Users = append(append([]User{}, c.Users...), u)
	trial.Contexts = append(append([]Context{}, c.Contexts...), cx)
	if err := trial.Validate(); err != nil {
		return nil, err
	}

	// Keychain first, then disk — only after BOTH succeed is the live config mutated, so a
	// failure leaves cfg/UI untouched (a stray keychain entry is harmless, FR-026).
	if err := secret.StoreKeychain(nc.Name, secretVal); err != nil {
		return nil, err
	}
	data, err := Marshal(&trial)
	if err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	if err := Save(c.Path(), data); err != nil {
		return nil, err
	}
	// Commit to the live config (FR-025: in-session switch) — append, never re-point
	// current-context.
	c.Clusters, c.Users, c.Contexts = trial.Clusters, trial.Users, trial.Contexts

	// Observability (Constitution V): record the add with NON-secret fields only.
	slog.Info("connection.add", "name", nc.Name, "endpoint", nc.Endpoint,
		"region", nc.Region, "readonly", nc.ReadOnly, "outcome", "ok")

	return c.ContextNames(), nil
}

// RemoveConnection deletes the connection named name — the cluster + user + context
// triple — and its keychain secret (007 US5 / FR-031). It is the symmetric inverse of
// AddConnection: it refuses the ACTIVE context (FR-032), validates a TRIAL copy with the
// triple removed before mutating the live config, deletes the keychain secret
// best-effort (a missing secret never blocks removal), persists, then commits to the
// live config (so the change is visible in-session, FR-033). Returns the updated
// context-name list.
func (c *Config) RemoveConnection(name string) ([]string, error) {
	if name == c.CurrentContext {
		return nil, fmt.Errorf("%w: cannot delete the active context %q (switch first)", ErrInvalid, name)
	}
	// Build a trial copy with every triple member named `name` dropped.
	trial := *c
	trial.Clusters = filterNamed(c.Clusters, func(x Cluster) string { return x.Name }, name)
	trial.Users = filterNamed(c.Users, func(x User) string { return x.Name }, name)
	trial.Contexts = filterNamed(c.Contexts, func(x Context) string { return x.Name }, name)
	if len(trial.Contexts) == len(c.Contexts) {
		return nil, fmt.Errorf("%w: no context named %q", ErrNotFound, name)
	}
	// A config with zero contexts is valid (the app falls back to the no-connection
	// state, 007 US5 last-connection edge case); Validate tolerates an empty set.
	if err := trial.Validate(); err != nil {
		return nil, err
	}

	// Persist first, then commit live — a failure leaves cfg/UI untouched.
	data, err := Marshal(&trial)
	if err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	if err := Save(c.Path(), data); err != nil {
		return nil, err
	}
	// Best-effort keychain cleanup: a missing secret MUST NOT fail the removal (FR-031).
	if rmErr := secret.RemoveKeychain(name); rmErr != nil {
		slog.Warn("connection.delete", "name", name, "keychain", "not-removed", "err", rmErr)
	}
	c.Clusters, c.Users, c.Contexts = trial.Clusters, trial.Users, trial.Contexts

	slog.Info("connection.delete", "name", name, "outcome", "ok")
	return c.ContextNames(), nil
}

// filterNamed returns a new slice with every element whose name == drop removed.
func filterNamed[T any](in []T, nameOf func(T) string, drop string) []T {
	out := make([]T, 0, len(in))
	for _, x := range in {
		if nameOf(x) != drop {
			out = append(out, x)
		}
	}
	return out
}
