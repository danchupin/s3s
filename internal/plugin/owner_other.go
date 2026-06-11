//go:build !unix

package plugin

import "os"

// ownedByCurrentUser is a no-op on platforms without POSIX file ownership; the
// perms-bit check in requireOwnerOnly still applies.
func ownedByCurrentUser(os.FileInfo) error { return nil }
