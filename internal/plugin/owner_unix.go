//go:build unix

package plugin

import (
	"fmt"
	"os"
	"syscall"
)

// ownedByCurrentUser refuses a config file not owned by the running uid, so a
// foreign-owned config can never make s3s execute declared commands.
func ownedByCurrentUser(fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil // unknown stat shape — skip the owner check
	}
	if int(st.Uid) != os.Getuid() {
		return fmt.Errorf("refusing to run plugins — config is not owned by the current user")
	}
	return nil
}
