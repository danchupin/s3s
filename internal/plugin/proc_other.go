//go:build !unix

package plugin

import "os/exec"

// setupProcessGroup is a no-op on platforms without POSIX process groups; the
// default CommandContext kill plus WaitDelay still bound the exchange.
func setupProcessGroup(*exec.Cmd) {}
