//go:build unix

package plugin

import (
	"os/exec"
	"syscall"
)

// setupProcessGroup makes the plugin lead its own process group and points the
// context cancel at the whole group, so a deadline kill takes the plugin's
// children down with it (a surviving grandchild would otherwise keep the
// stdout pipe open and stall Wait until WaitDelay).
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
