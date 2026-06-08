// Package secret resolves a context's S3 secret from one of two pluggable, secure
// sources (the OS keychain or an external command) so the secret never needs to live
// in the shell environment or on disk in plaintext (014). It is UI- and SDK-agnostic
// (Constitution I) and never imports internal/config (no import cycle): callers pass
// primitive inputs and receive a redacting Resolved credential.
package secret

import (
	"context"
	"fmt"

	"github.com/danchupin/s3s/internal/logging"
)

// Kind selects a credential source. Both carry a non-secret AccessKeyID from config.
type Kind int

// Credential source kinds (014).
const (
	Keychain Kind = iota // OS keystore entry
	Command              // external command whose stdout is the secret
)

// Resolved is the outcome of resolving a source. The secret is held only as a
// redacting logging.Secret and is never persisted to an s3s file (FR-008).
type Resolved struct {
	AccessKeyID string
	SecretKey   logging.Secret
}

// Request describes one source to resolve.
type Request struct {
	Kind        Kind
	AccessKeyID string // non-secret, from config
	Ref         string // keychain account | command line
	ConfigPath  string // config file path — used by the Command owner-only perms gate
}

// Resolve produces the credential for the requested source, or a clear error when the
// source is unavailable (never an empty secret — FR-020/043).
func Resolve(ctx context.Context, r Request) (Resolved, error) {
	switch r.Kind {
	case Keychain:
		s, err := GetKeychain(r.Ref)
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{AccessKeyID: r.AccessKeyID, SecretKey: logging.Secret(s)}, nil
	case Command:
		s, err := commandSecret(ctx, r.Ref, r.ConfigPath)
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{AccessKeyID: r.AccessKeyID, SecretKey: logging.Secret(s)}, nil
	default:
		return Resolved{}, fmt.Errorf("secret: unknown source kind %d", r.Kind)
	}
}
