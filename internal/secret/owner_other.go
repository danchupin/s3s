//go:build !unix

package secret

import "os"

// ownedByCurrentUser is a no-op on platforms without POSIX file ownership (e.g.
// Windows); the perms-bit check in requireOwnerOnly still applies.
func ownedByCurrentUser(os.FileInfo) error { return nil }
