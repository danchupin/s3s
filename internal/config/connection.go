package config

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

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
	Buckets     []string // pinned bucket names (010 US3) — stored on the cluster
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

	cl := Cluster{Name: nc.Name, Endpoint: nc.Endpoint, Region: nc.Region, PathStyle: nc.PathStyle, Buckets: nc.Buckets}
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

// AppendBucket adds a pinned bucket name to the cluster bound by the named context (010).
// It mirrors AddConnection's trial-validate-persist-commit discipline: a trial copy is
// validated and written to disk BEFORE the live config is mutated, so a failure leaves
// cfg/UI untouched. The append is idempotent — a name already pinned is a no-op that still
// succeeds — and the name is trimmed (empty is rejected). Returns the cluster's updated
// bucket list. This is a CONFIG mutation (not an S3 write); it never touches the keychain.
func (c *Config) AppendBucket(ctxName, bucket string) ([]string, error) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return nil, fmt.Errorf("%w: empty bucket name", ErrInvalid)
	}
	cx, ok := c.context(ctxName)
	if !ok {
		return nil, fmt.Errorf("%w: no context named %q", ErrNotFound, ctxName)
	}

	trial := *c
	trial.Clusters = slices.Clone(c.Clusters)
	idx := slices.IndexFunc(trial.Clusters, func(cl Cluster) bool { return cl.Name == cx.Cluster })
	if idx < 0 {
		return nil, fmt.Errorf("%w: context %q references unknown cluster %q", ErrInvalid, ctxName, cx.Cluster)
	}
	existing := trial.Clusters[idx].Buckets
	if slices.Contains(existing, bucket) {
		return slices.Clone(existing), nil // already pinned — idempotent no-op
	}
	updated := append(slices.Clone(existing), bucket)
	trial.Clusters[idx].Buckets = updated
	if err := trial.Validate(); err != nil {
		return nil, err
	}
	data, err := Marshal(&trial)
	if err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	if err := Save(c.Path(), data); err != nil {
		return nil, err
	}
	c.Clusters = trial.Clusters
	slog.Info("connection.bucket-add", "context", ctxName, "bucket", bucket, "outcome", "ok")
	return slices.Clone(updated), nil
}

// RemoveConnection deletes the connection named name — the cluster + user + context
// triple — and its keychain secret (007 US5 / FR-031). It is the symmetric inverse of
// AddConnection: it validates a TRIAL copy with the triple removed before mutating the
// live config, deletes the keychain secret best-effort (a missing secret never blocks
// removal), persists, then commits to the live config (FR-033). Returns the updated
// context-name list.
//
// The SESSION's active context is guarded by the UI (it refuses deleting the live
// m.ctxName); this config layer guards CONFIG integrity instead: if the deleted context
// is the persisted current-context, current-context is re-pointed to a remaining context
// (or cleared when none remain) so the on-disk config never dangles. (Using
// CurrentContext to mean "the live active context" was wrong — it is not updated on an
// in-session switch, so the two notions could disagree; review #2.)
func (c *Config) RemoveConnection(name string) ([]string, error) {
	// Build a trial copy with every triple member named `name` dropped (slices.DeleteFunc
	// on clones — never mutates the live config's backing arrays).
	trial := *c
	trial.Clusters = slices.DeleteFunc(slices.Clone(c.Clusters), func(x Cluster) bool { return x.Name == name })
	trial.Users = slices.DeleteFunc(slices.Clone(c.Users), func(x User) bool { return x.Name == name })
	trial.Contexts = slices.DeleteFunc(slices.Clone(c.Contexts), func(x Context) bool { return x.Name == name })
	if len(trial.Contexts) == len(c.Contexts) {
		return nil, fmt.Errorf("%w: no context named %q", ErrNotFound, name)
	}
	// Re-point current-context if we just removed it, so the config never dangles.
	if trial.CurrentContext == name {
		if len(trial.Contexts) > 0 {
			trial.CurrentContext = trial.Contexts[0].Name
		} else {
			trial.CurrentContext = ""
		}
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
	c.Clusters, c.Users, c.Contexts, c.CurrentContext = trial.Clusters, trial.Users, trial.Contexts, trial.CurrentContext

	slog.Info("connection.delete", "name", name, "outcome", "ok")
	return c.ContextNames(), nil
}
