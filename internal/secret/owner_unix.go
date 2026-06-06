//go:build unix

package secret

import (
	"fmt"
	"os"
	"syscall"
)

// ownedByCurrentUser refuses a config file not owned by the running uid (005 FR-036).
func ownedByCurrentUser(fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil // unknown stat shape — skip the owner check
	}
	if int(st.Uid) != os.Getuid() {
		return fmt.Errorf("secret: refusing to run cmd source — config is not owned by the current user")
	}
	return nil
}
