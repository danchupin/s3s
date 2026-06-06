package secret

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/shlex"
)

// commandTimeout bounds a hung credential command.
const commandTimeout = 10 * time.Second

// commandSecret runs the configured command and returns its stdout (trailing newline
// trimmed) as the secret. It refuses to run unless the config file is owner-only,
// blocking "attacker edits the YAML -> command runs at launch" (005 FR-036, R10). The
// command line is split with POSIX shell-words rules (quotes honored, no shell
// expansion) and executed as argv — never via `sh -c`.
func commandSecret(ctx context.Context, cmdline, configPath string) (string, error) {
	if err := requireOwnerOnly(configPath); err != nil {
		return "", err
	}
	argv, err := shlex.Split(cmdline)
	if err != nil || len(argv) == 0 {
		return "", fmt.Errorf("secret: cannot parse cmd source")
	}
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, argv[0], argv[1:]...).Output()
	if err != nil {
		return "", fmt.Errorf("secret: cmd source failed: %w", err)
	}
	s := strings.TrimRight(string(out), "\r\n")
	if s == "" {
		return "", fmt.Errorf("secret: cmd source produced empty output")
	}
	return s, nil
}

// requireOwnerOnly refuses when the config file is group/world writable or not owned by
// the running user (005 FR-036).
func requireOwnerOnly(path string) error {
	if path == "" {
		return fmt.Errorf("secret: cmd source needs the config path for its security check")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("secret: stat config: %w", err)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("secret: refusing to run cmd source — config %s is group/world-writable (chmod 600)", path)
	}
	return ownedByCurrentUser(fi)
}
