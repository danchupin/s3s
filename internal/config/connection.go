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

	// Keychain first — abort before touching config if it fails (FR-026).
	if err := secret.StoreKeychain(nc.Name, secretVal); err != nil {
		return nil, err
	}

	cl := Cluster{Name: nc.Name, Endpoint: nc.Endpoint, Region: nc.Region, PathStyle: true}
	u := User{Name: nc.Name, AccessKeyID: nc.AccessKeyID, Keychain: true}
	cx := Context{Name: nc.Name, Cluster: nc.Name, User: nc.Name, ReadOnly: nc.ReadOnly}
	c.Upsert(cl, u, cx, false) // mutate the live config (FR-025)

	if err := c.Validate(); err != nil {
		return nil, err
	}
	data, err := Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	if err := Save(c.Path(), data); err != nil {
		return nil, err
	}

	// Observability (Constitution V): record the add with NON-secret fields only.
	slog.Info("connection.add", "name", nc.Name, "endpoint", nc.Endpoint,
		"region", nc.Region, "readonly", nc.ReadOnly, "outcome", "ok")

	return c.ContextNames(), nil
}
