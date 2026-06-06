package config

import (
	"context"
	"fmt"

	"github.com/danchupin/s3s/internal/secret"
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

// secretRequest builds the single-source resolution request for a non-anonymous user
// (005 FR-041). configPath feeds the command source's owner-only perms gate.
func (u User) secretRequest(configPath string) (secret.Request, error) {
	switch {
	case u.Keychain:
		return secret.Request{Kind: secret.Keychain, AccessKeyID: u.AccessKeyID, Ref: u.Name}, nil
	case u.Command != "":
		return secret.Request{Kind: secret.Command, AccessKeyID: u.AccessKeyID, Ref: u.Command, ConfigPath: configPath}, nil
	case u.AWSProfile != "":
		return secret.Request{Kind: secret.AWSProfile, Ref: u.AWSProfile}, nil
	case !u.SecretAccessKey.IsEmpty():
		return secret.Request{Kind: secret.Inline, AccessKeyID: u.AccessKeyID, Ref: u.SecretAccessKey.Reveal()}, nil
	default:
		return secret.Request{}, fmt.Errorf("%w: user %q has no credential source", ErrInvalid, u.Name)
	}
}

// ClientConfigWithSecret builds a ClientConfig using an explicitly provided secret —
// the interactive startup prompt fallback (005 FR-038) — bypassing source resolution.
func (c *Config) ClientConfigWithSecret(name, sec string) (storage.ClientConfig, error) {
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
		SecretKey:     sec,
	}, nil
}

// KeychainAccount returns the keystore account for the named context's user (the user
// name), so `s3s cred` and runtime resolution agree on the key (005 R9).
func (c *Config) KeychainAccount(name string) (string, error) {
	cx, ok := c.context(name)
	if !ok {
		return "", fmt.Errorf("%w: no such context %q", ErrInvalid, name)
	}
	return cx.User, nil
}

// ClientConfig builds a storage.ClientConfig for the named context, resolving the
// user's single credential source (keychain / command / aws-profile / inline) via
// internal/secret. This is the single trust boundary where the secret is revealed (to
// construct the client); it is never written to an s3s file (005 FR-035). A resolution
// failure surfaces a clear error and never connects with an empty secret (FR-043).
func (c *Config) ClientConfig(ctx context.Context, name string) (storage.ClientConfig, error) {
	cl, u, err := c.Resolve(name)
	if err != nil {
		return storage.ClientConfig{}, err
	}
	cc := storage.ClientConfig{
		Endpoint:      cl.Endpoint,
		Region:        cl.Region,
		PathStyle:     cl.PathStyle,
		TLSSkipVerify: cl.TLSSkipVerify,
		Anonymous:     u.Anonymous,
	}
	if u.Anonymous {
		return cc, nil
	}
	req, err := u.secretRequest(c.path)
	if err != nil {
		return storage.ClientConfig{}, err
	}
	res, rerr := secret.Resolve(ctx, req)
	if rerr != nil {
		return storage.ClientConfig{}, rerr
	}
	cc.AccessKeyID = res.AccessKeyID
	cc.SecretKey = res.SecretKey.Reveal()
	cc.SessionToken = res.SessionToken.Reveal()
	// An inline source may also carry an explicit sessionToken from config.
	if req.Kind == secret.Inline && cc.SessionToken == "" {
		cc.SessionToken = u.SessionToken.Reveal()
	}
	return cc, nil
}
