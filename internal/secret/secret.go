// Package secret resolves a context's S3 secret from one of several pluggable,
// secure sources (OS keychain, an external command, an AWS shared profile, an
// env-resolved inline value) so the secret never needs to live in the shell
// environment or on disk in plaintext (005 US6). It is UI- and SDK-agnostic
// (Constitution I) and never imports internal/config (no import cycle): callers pass
// primitive inputs and receive a redacting Resolved credential.
package secret

import (
	"context"
	"fmt"

	"github.com/danchupin/s3s/internal/logging"
)

// Kind selects a credential source. Inline/Keychain/Command carry a non-secret
// AccessKeyID from config; AWSProfile supplies both keys itself.
type Kind int

const (
	Inline     Kind = iota // env-resolved literal secret
	Keychain               // OS keystore entry
	Command                // external command whose stdout is the secret
	AWSProfile             // ~/.aws/credentials profile (static keys)
)

// Resolved is the outcome of resolving a source. The secret values are held only as
// redacting logging.Secret and are never persisted to an s3s file (005 FR-035/FR-039).
type Resolved struct {
	AccessKeyID  string
	SecretKey    logging.Secret
	SessionToken logging.Secret
}

// Request describes one source to resolve.
type Request struct {
	Kind        Kind
	AccessKeyID string // non-secret, from config; empty for AWSProfile
	Ref         string // inline secret | keychain account | command line | profile name
	ConfigPath  string // config file path — used by the Command owner-only perms gate
}

// Resolve produces the credential for the requested source, or a clear error when the
// source is unavailable (never an empty secret — 005 FR-043).
func Resolve(ctx context.Context, r Request) (Resolved, error) {
	switch r.Kind {
	case Inline:
		if r.Ref == "" {
			return Resolved{}, fmt.Errorf("secret: empty inline secret")
		}
		return Resolved{AccessKeyID: r.AccessKeyID, SecretKey: logging.Secret(r.Ref)}, nil
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
	case AWSProfile:
		ak, sk, tok, err := awsProfile(r.Ref)
		if err != nil {
			return Resolved{}, err
		}
		return Resolved{AccessKeyID: ak, SecretKey: logging.Secret(sk), SessionToken: logging.Secret(tok)}, nil
	default:
		return Resolved{}, fmt.Errorf("secret: unknown source kind %d", r.Kind)
	}
}
