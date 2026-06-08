package secret

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService namespaces s3s entries in the OS keystore. The account is the
// context name (005 R9).
const keyringService = "s3s"

// ErrNoKeystore reports that the OS keystore is unavailable (e.g. headless Linux with
// no Secret Service). There is NO plaintext fallback — the actionable remedy is a `cmd`
// credential source (FR-020).
var ErrNoKeystore = errors.New("secret: OS keystore unavailable — use a cmd credential source (e.g. cmd: \"vault kv get -field=secret …\") on this machine")

// GetKeychain fetches a stored secret for account. A missing entry or an unavailable
// keystore both yield a clear, actionable error — never an empty secret (FR-020/043).
func GetKeychain(account string) (string, error) {
	s, err := keyring.Get(keyringService, account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("secret: no keystore entry for %q (run: s3s cred set, or use a cmd source)", account)
		}
		return "", fmt.Errorf("%w: %v", ErrNoKeystore, err)
	}
	if s == "" {
		return "", fmt.Errorf("secret: empty keystore entry for %q", account)
	}
	return s, nil
}

// StoreKeychain writes (or replaces) the secret for account in the OS keystore only —
// never the config file (005 FR-037).
func StoreKeychain(account, secret string) error {
	if err := keyring.Set(keyringService, account, secret); err != nil {
		return fmt.Errorf("%w: %v", ErrNoKeystore, err)
	}
	return nil
}

// RemoveKeychain deletes the keystore entry for account.
func RemoveKeychain(account string) error {
	if err := keyring.Delete(keyringService, account); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil // already gone — benign
		}
		return fmt.Errorf("%w: %v", ErrNoKeystore, err)
	}
	return nil
}
