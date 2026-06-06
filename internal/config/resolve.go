package config

import (
	"fmt"

	"github.com/danchupin/s3s/internal/storage"
)

// EnvContext is the environment variable that selects the active context.
const EnvContext = "S3S_CONTEXT"

// ActiveContextName applies the precedence: explicit --context flag >
// S3S_CONTEXT env > current-context in config (FR-002).
func ActiveContextName(flag, env, current string) string {
	switch {
	case flag != "":
		return flag
	case env != "":
		return env
	default:
		return current
	}
}

// Resolve returns the cluster and user bound by the named context, with the
// cluster's region defaulted when empty.
func (c *Config) Resolve(name string) (Cluster, User, error) {
	if name == "" {
		return Cluster{}, User{}, fmt.Errorf("%w: no active context selected", ErrInvalid)
	}
	cx, ok := c.context(name)
	if !ok {
		return Cluster{}, User{}, fmt.Errorf("%w: no such context %q", ErrInvalid, name)
	}
	cl, ok := c.cluster(cx.Cluster)
	if !ok {
		return Cluster{}, User{}, fmt.Errorf("%w: context %q references unknown cluster %q", ErrInvalid, name, cx.Cluster)
	}
	u, ok := c.user(cx.User)
	if !ok {
		return Cluster{}, User{}, fmt.Errorf("%w: context %q references unknown user %q", ErrInvalid, name, cx.User)
	}
	if cl.Region == "" {
		cl.Region = DefaultRegion
	}
	return cl, u, nil
}

// WriteMode is the resolved write capability for one context. ReadOnly reports the
// context's hard lock (readonly: true), independent of the runtime arm state — the UI
// needs it to refuse arming a locked context (005 FR-028).
type WriteMode struct {
	Writable bool
	ReadOnly bool
}

// WriteModeFor returns the write policy for the named context: writable only when
// the global --write switch is on AND the context is not marked readonly (read-only
// always wins — FR-001/FR-002, opt-out model). ReadOnly carries the context lock so
// the UI can keep it absolute under the runtime toggle. The existing Resolve/
// ClientConfig methods are intentionally left unchanged so their callers do not break.
func (c *Config) WriteModeFor(name string, writeFlag bool) (WriteMode, error) {
	cx, ok := c.context(name)
	if !ok {
		return WriteMode{}, fmt.Errorf("%w: no such context %q", ErrInvalid, name)
	}
	return WriteMode{Writable: writeFlag && !cx.ReadOnly, ReadOnly: cx.ReadOnly}, nil
}

// ClientConfig builds a storage.ClientConfig for the named context. This is the
// single trust boundary where secrets are revealed (to construct the client).
func (c *Config) ClientConfig(name string) (storage.ClientConfig, error) {
	cl, u, err := c.Resolve(name)
	if err != nil {
		return storage.ClientConfig{}, err
	}
	return storage.ClientConfig{
		Endpoint:      cl.Endpoint,
		Region:        cl.Region,
		PathStyle:     cl.PathStyle,
		TLSSkipVerify: cl.TLSSkipVerify,
		Anonymous:     u.Anonymous,
		AccessKeyID:   u.AccessKeyID,
		SecretKey:     u.SecretAccessKey.Reveal(),
		SessionToken:  u.SessionToken.Reveal(),
	}, nil
}
