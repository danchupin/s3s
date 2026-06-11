package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/google/shlex"
)

// ExecRunner invokes a plugin as an argv-exec'd subprocess: one JSON request to
// stdin, one JSON response from stdout (read capped at MaxOutput), a deadline
// over the whole exchange. The declared command line is split with POSIX
// shell-words rules and executed directly — never via a shell — and execution
// is refused unless the config file at ConfigPath is owner-only-writable (the
// same "attacker edits the YAML → command runs at launch" defense as the
// credential cmd source). stderr is discarded: never rendered, never logged.
type ExecRunner struct {
	ConfigPath string
}

// Invoke runs one exchange and classifies its outcome. Exactly one slog record
// is emitted per invocation — plugin, capability, target, duration, outcome —
// never the payload and never argv (flags may embed user-specific tokens).
func (r *ExecRunner) Invoke(ctx context.Context, d Decl, req Request) Result {
	start := time.Now()
	res := r.invoke(ctx, d, req)
	res.Duration = time.Since(start)

	attrs := []any{
		"plugin", d.Name,
		"capability", string(req.Capability),
		"connection", req.Connection.Name,
	}
	if req.Target != nil {
		attrs = append(attrs, "bucket", req.Target.Bucket, "key", req.Target.Key)
	}
	attrs = append(attrs, "duration_ms", res.Duration.Milliseconds(), "outcome", string(res.Outcome))
	slog.Info("plugin.invoke", attrs...)
	return res
}

func (r *ExecRunner) invoke(ctx context.Context, d Decl, req Request) Result {
	if err := requireOwnerOnly(r.ConfigPath); err != nil {
		return Result{Outcome: OutcomeExecError, ErrDetail: Sanitize(err.Error(), MaxErrDetail, true)}
	}
	argv, err := shlex.Split(d.Cmd)
	if err != nil || len(argv) == 0 {
		return Result{Outcome: OutcomeExecError, ErrDetail: "cannot parse the plugin command line"}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return Result{Outcome: OutcomeExecError, ErrDetail: "cannot encode the plugin request"}
	}

	timeout := d.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out := &cappedBuffer{max: MaxOutput}
	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	// The deadline must kill the plugin's whole process group — a grandchild
	// (e.g. a shell script's spawned helper) inheriting stdout would otherwise
	// hold the pipe open past the kill. WaitDelay backstops anything that still
	// survives so Wait can never hang.
	setupProcessGroup(cmd)
	cmd.WaitDelay = 500 * time.Millisecond

	runErr := cmd.Run()
	switch {
	case cctx.Err() == context.DeadlineExceeded:
		return Result{Outcome: OutcomeTimeout, ErrDetail: "timeout"}
	case runErr != nil:
		res := Result{Outcome: OutcomeExecError, ErrDetail: Sanitize(runErr.Error(), MaxErrDetail, true)}
		var execErr *exec.Error
		if errors.As(runErr, &execErr) || errors.Is(runErr, os.ErrNotExist) {
			res.Unavailable = true
			res.ErrDetail = "executable not found"
		}
		return res
	case out.overflow:
		return Result{Outcome: OutcomeInvalidOutput, ErrDetail: fmt.Sprintf("output exceeds %d bytes", MaxOutput)}
	}

	resp, outcome := DecodeResponse(bytes.TrimSpace(out.buf.Bytes()), req.Capability)
	switch outcome {
	case OutcomeOK:
		return Result{Outcome: OutcomeOK, Buckets: resp.Buckets, Fields: resp.Fields}
	case OutcomeContractError:
		return Result{Outcome: OutcomeContractError, ErrDetail: Sanitize(resp.Error, MaxErrDetail, true)}
	case OutcomeIncompatible:
		return Result{
			Outcome:      OutcomeIncompatible,
			Incompatible: resp.ContractVersion,
			ErrDetail:    fmt.Sprintf("plugin speaks contract v%d, this build speaks v%d", resp.ContractVersion, ContractVersion),
		}
	default:
		return Result{Outcome: OutcomeInvalidOutput, ErrDetail: "unparsable or invalid response"}
	}
}

// cappedBuffer accepts writes up to max bytes and flags (without storing) any
// excess, so a runaway producer is bounded before the JSON parse.
type cappedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room > 0 {
		if len(p) > room {
			b.buf.Write(p[:room])
			b.overflow = true
		} else {
			b.buf.Write(p)
		}
	} else if len(p) > 0 {
		b.overflow = true
	}
	return len(p), nil // swallow everything; overflow is the signal
}

// requireOwnerOnly refuses when the config file is group/world writable or not
// owned by the running user — mirrors the credential cmd source defense.
func requireOwnerOnly(path string) error {
	if path == "" {
		return errors.New("plugins need the config path for their security check")
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat config: %w", err)
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("refusing to run plugins — config %s is group/world-writable (chmod 600)", path)
	}
	return ownedByCurrentUser(fi)
}
