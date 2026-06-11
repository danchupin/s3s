package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// writeFixture writes an executable /bin/sh script and returns its path.
func writeFixture(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// ownerOnlyConfig creates a chmod-600 stand-in config file for the exec gate.
func ownerOnlyConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("apiVersion: s3s/v1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func discoveryReq() Request {
	return Request{
		ContractVersion: ContractVersion,
		Capability:      BucketDiscovery,
		Connection:      Connection{Name: "ctx", Endpoint: "http://x", UserLabel: "u", AccessKeyID: "AK"},
	}
}

func TestRunnerHappyPath(t *testing.T) {
	script := writeFixture(t, "ok.sh", `cat >/dev/null
echo '{"contractVersion":1,"buckets":["alpha","beta"]}'`)
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeOK {
		t.Fatalf("outcome = %s (%s), want ok", res.Outcome, res.ErrDetail)
	}
	if len(res.Buckets) != 2 || res.Buckets[0] != "alpha" {
		t.Errorf("buckets = %v", res.Buckets)
	}
	if res.Duration <= 0 {
		t.Error("duration must be recorded")
	}
}

func TestRunnerTimeoutKillsProcess(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "alive")
	script := writeFixture(t, "slow.sh", fmt.Sprintf(`cat >/dev/null
sleep 2
touch %s
echo '{"contractVersion":1,"buckets":[]}'`, marker))
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	start := time.Now()
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script, Timeout: 100 * time.Millisecond}, discoveryReq())
	if res.Outcome != OutcomeTimeout {
		t.Fatalf("outcome = %s, want timeout", res.Outcome)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("invoke took %v — the deadline did not kill the process", elapsed)
	}
	time.Sleep(150 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("process survived the deadline (marker file written)")
	}
}

func TestRunnerNonzeroExit(t *testing.T) {
	script := writeFixture(t, "fail.sh", `cat >/dev/null
echo '{"contractVersion":1,"buckets":["ignored"]}'
exit 3`)
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeExecError {
		t.Fatalf("outcome = %s, want exec_error", res.Outcome)
	}
	if len(res.Buckets) != 0 {
		t.Errorf("stdout of a failed process must be ignored: %v", res.Buckets)
	}
}

func TestRunnerGarbageStdout(t *testing.T) {
	script := writeFixture(t, "garbage.sh", `cat >/dev/null
echo 'this is not json'`)
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeInvalidOutput {
		t.Fatalf("outcome = %s, want invalid_output", res.Outcome)
	}
}

func TestRunnerHugeStdout(t *testing.T) {
	// ~2 MiB of output: past the 1 MiB read cap ⇒ invalid_output.
	script := writeFixture(t, "huge.sh", `cat >/dev/null
i=0
while [ $i -lt 2048 ]; do
  printf '%01024d' 0
  i=$((i+1))
done
i=0
while [ $i -lt 1024 ]; do
  printf '%01024d' 0
  i=$((i+1))
done`)
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeInvalidOutput {
		t.Fatalf("outcome = %s, want invalid_output (read cap)", res.Outcome)
	}
}

func TestRunnerSoftError(t *testing.T) {
	long := strings.Repeat("e", 400)
	script := writeFixture(t, "soft.sh", fmt.Sprintf(`cat >/dev/null
echo '{"contractVersion":1,"error":"%s"}'`, long))
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeContractError {
		t.Fatalf("outcome = %s, want contract_error", res.Outcome)
	}
	if len(res.ErrDetail) > MaxErrDetail+len(TruncationMarker) {
		t.Errorf("reason not capped: %d bytes", len(res.ErrDetail))
	}
	if !strings.HasPrefix(res.ErrDetail, "eee") {
		t.Errorf("reason lost: %q", res.ErrDetail)
	}
}

func TestRunnerVersionMismatch(t *testing.T) {
	script := writeFixture(t, "v2.sh", `cat >/dev/null
echo '{"contractVersion":2,"buckets":["a"]}'`)
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeIncompatible {
		t.Fatalf("outcome = %s, want incompatible", res.Outcome)
	}
	if res.Incompatible != 2 {
		t.Errorf("Incompatible = %d, want 2", res.Incompatible)
	}
}

func TestRunnerMissingExecutable(t *testing.T) {
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: filepath.Join(t.TempDir(), "nope")}, discoveryReq())
	if res.Outcome != OutcomeExecError {
		t.Fatalf("outcome = %s, want exec_error", res.Outcome)
	}
	if !res.Unavailable {
		t.Error("a missing executable must be classified for the unavailable status")
	}
}

func TestRunnerOwnerOnlyGate(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	script := writeFixture(t, "gate.sh", fmt.Sprintf(`touch %s
cat >/dev/null
echo '{"contractVersion":1,"buckets":[]}'`, marker))
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfg, 0o666); err != nil { // explicit chmod — WriteFile perms pass through umask
		t.Fatal(err)
	}
	r := &ExecRunner{ConfigPath: cfg}
	res := r.Invoke(context.Background(), Decl{Name: "p", Cmd: script}, discoveryReq())
	if res.Outcome != OutcomeExecError {
		t.Fatalf("outcome = %s, want exec_error (gate refusal)", res.Outcome)
	}
	if !strings.Contains(res.ErrDetail, "writable") {
		t.Errorf("refusal reason must name the perms problem: %q", res.ErrDetail)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the gate must refuse BEFORE spawning the process")
	}
}

// recordCollector captures slog records emitted during an invocation.
type recordCollector struct {
	mu      sync.Mutex
	records []map[string]string
}

func (c *recordCollector) Enabled(context.Context, slog.Level) bool { return true }
func (c *recordCollector) Handle(_ context.Context, r slog.Record) error {
	attrs := map[string]string{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	c.mu.Lock()
	c.records = append(c.records, attrs)
	c.mu.Unlock()
	return nil
}
func (c *recordCollector) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *recordCollector) WithGroup(string) slog.Handler      { return c }

func TestRunnerLogsInvocationFactsOnly(t *testing.T) {
	col := &recordCollector{}
	prev := slog.Default()
	slog.SetDefault(slog.New(col))
	defer slog.SetDefault(prev)

	script := writeFixture(t, "ok.sh", `cat >/dev/null
echo '{"contractVersion":1,"fields":[{"name":"Image ID","value":"sEcReTpAyLoAd123"}]}'`)
	r := &ExecRunner{ConfigPath: ownerOnlyConfig(t)}
	req := Request{
		ContractVersion: ContractVersion,
		Capability:      ObjectMetadata,
		Connection:      Connection{Name: "ctx", Endpoint: "http://x", UserLabel: "u", AccessKeyID: "AK"},
		Target:          &Target{Bucket: "images", Key: "0af3.jpg"},
	}
	res := r.Invoke(context.Background(), Decl{Name: "meta-plugin", Cmd: script + " --token=tok-XYZZY"}, req)
	if res.Outcome != OutcomeOK {
		t.Fatalf("outcome = %s (%s)", res.Outcome, res.ErrDetail)
	}

	if len(col.records) != 1 {
		t.Fatalf("invocation must emit exactly one record, got %d", len(col.records))
	}
	rec := col.records[0]
	for k, want := range map[string]string{
		"plugin": "meta-plugin", "capability": "object-metadata",
		"connection": "ctx", "bucket": "images", "key": "0af3.jpg",
		"outcome": "ok",
	} {
		if rec[k] != want {
			t.Errorf("record[%s] = %q, want %q", k, rec[k], want)
		}
	}
	if _, ok := rec["duration_ms"]; !ok {
		t.Error("record must carry duration_ms")
	}
	flat := fmt.Sprintf("%v", rec)
	if strings.Contains(flat, "sEcReTpAyLoAd123") {
		t.Error("record must not contain the response payload")
	}
	if strings.Contains(flat, "tok-XYZZY") || strings.Contains(flat, script) {
		t.Error("record must not contain argv")
	}
}
