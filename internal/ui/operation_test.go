package ui

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// writableTreeApp returns a writable App parked in the tree view of an empty bucket.
func writableTreeApp() App {
	f := storage.NewFake()
	f.Seed("b")
	m := newApp(f, nil, nil)
	m.writable = true
	m.mode = modeTree
	m.bucket = "b"
	m.level = &levelState{complete: true}
	m.loading = false
	return m
}

// TestCreateFolderRefusedReadOnly: on a read-only context, "+" issues no command,
// starts no op, and shows the read-only hint — never a silent no-op (FR-003).
func TestCreateFolderRefusedReadOnly(t *testing.T) {
	m := writableTreeApp()
	m.writable = false
	m2, cmd := pressCmd(m, "+")
	m = m2
	if cmd != nil {
		t.Error("read-only create must not dispatch a command")
	}
	if m.op != nil {
		t.Error("read-only create must not start an operation")
	}
	if !errors.Is(m.err, storage.ErrReadOnly) {
		t.Errorf("want ErrReadOnly hint, got %v", m.err)
	}
	if !strings.Contains(viewOf(m), "read-only") {
		t.Errorf("view should show the read-only hint:\n%s", viewOf(m))
	}
}

// TestCreateFolderSimpleConfirmFlow: + → name → Enter → simple confirm → y dispatches;
// n aborts. Covers the vertical slice's interaction (US2, SC-001).
func TestCreateFolderSimpleConfirmFlow(t *testing.T) {
	m := writableTreeApp()

	m = press(m, "+")
	if m.op == nil || m.op.phase != phaseName {
		t.Fatalf("want name phase after +, op=%+v", m.op)
	}
	m = typeStr(m, "reports")
	if m.op.name != "reports" {
		t.Fatalf("name not captured: %q", m.op.name)
	}
	m = press(m, "enter")
	if m.op == nil || m.op.phase != phaseConfirm {
		t.Fatalf("want confirm phase after Enter, op=%+v", m.op)
	}
	if !strings.Contains(viewOf(m), "create folder reports/") {
		t.Errorf("confirm prompt missing:\n%s", viewOf(m))
	}

	// n aborts with no command.
	if mc, cmd := pressCmd(m, "n"); cmd != nil || mc.op != nil {
		t.Errorf("n must abort: cmd=%v op=%+v", cmd, mc.op)
	}

	// y dispatches the create-folder command.
	m2, cmd := pressCmd(m, "y")
	m = m2
	if cmd == nil {
		t.Error("y must dispatch a create-folder command")
	}
	if m.op == nil || m.op.phase != phaseRunning {
		t.Errorf("y should move op to running; op=%+v", m.op)
	}
}

// TestCreateFolderInvalidNameStays: a blank name keeps the name phase and shows a hint.
func TestCreateFolderInvalidNameStays(t *testing.T) {
	m := writableTreeApp()
	m = press(m, "+")
	m = typeStr(m, "   ")
	m = press(m, "enter")
	if m.op == nil || m.op.phase != phaseName {
		t.Errorf("blank name should stay in the name phase; op=%+v", m.op)
	}
	if !errors.Is(m.err, storage.ErrInvalidName) {
		t.Errorf("want ErrInvalidName hint, got %v", m.err)
	}
	if !strings.Contains(viewOf(m), "Invalid folder name") {
		t.Errorf("invalid-name hint must be visible, not silent:\n%s", viewOf(m))
	}
}

// TestCreateFolderSuccessRefreshesLevel: a successful op invalidates + re-fetches the
// level so the new folder becomes visible (SC-006).
func TestCreateFolderSuccessRefreshesLevel(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := newApp(f, nil, nil)
	m.writable = true
	m.mode = modeTree
	m.bucket = "b"
	m.level = &levelState{complete: true}
	gen := m.gen

	// Simulate the create having landed on the backend, then deliver the outcome.
	_ = f.CreateFolder(context.Background(), "b", "reports")
	m2, cmd := m.Update(operationDoneMsg{gen: gen})
	m = m2.(App)
	if cmd == nil {
		t.Fatal("success should trigger a level refresh")
	}
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
	if !strings.Contains(viewOf(m), "reports") {
		t.Errorf("new folder not visible after refresh:\n%s", viewOf(m))
	}
}

// TestCreateFolderCancelNotSuccess: a cancelled in-flight op is never an error/success;
// it triggers a refresh to show ground truth (FR-007).
func TestCreateFolderCancelNotSuccess(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := newApp(f, nil, nil)
	m.writable = true
	m.mode = modeTree
	m.bucket = "b"
	m.level = &levelState{complete: true}
	gen := m.gen

	m2, cmd := m.Update(operationDoneMsg{gen: gen, err: context.Canceled})
	m = m2.(App)
	if cmd == nil {
		t.Error("cancel should still refresh to reflect ground truth")
	}
	if m.errorText() != "" {
		t.Errorf("cancel must not surface an error; got %q", m.errorText())
	}
	if m.op != nil {
		t.Error("op should be cleared after cancel")
	}
}

// TestStaleOperationDropped: an outcome from a superseded generation is ignored.
func TestStaleOperationDropped(t *testing.T) {
	m := writableTreeApp()
	stale := m.gen - 1
	m2, cmd := m.Update(operationDoneMsg{gen: stale, err: errors.New("boom")})
	m = m2.(App)
	if cmd != nil || m.err != nil {
		t.Errorf("stale outcome must be dropped: cmd=%v err=%v", cmd, m.err)
	}
}

// TestFooterWriteTag: the footer identity line reflects write mode — [RW] when
// writable, [RO] otherwise.
func TestFooterWriteTag(t *testing.T) {
	if got := footerIdentityLine(80, "ctx", "cl", "u", true); !strings.Contains(got, "[RW]") {
		t.Errorf("writable footer should show [RW]; got %q", got)
	}
	if got := footerIdentityLine(80, "ctx", "cl", "u", false); !strings.Contains(got, "[RO]") {
		t.Errorf("read-only footer should show [RO]; got %q", got)
	}
}

// TestFooterHintWritable: the "+ folder" hint appears only in write mode.
func TestFooterHintWritable(t *testing.T) {
	if got := footerHintsLine(120, true); !strings.Contains(got, "folder") {
		t.Errorf("writable hints should include the + folder hint; got %q", got)
	}
	if got := footerHintsLine(120, false); strings.Contains(got, "folder") {
		t.Errorf("read-only hints must not include the + folder hint; got %q", got)
	}
}

// TestMutationLogging: start is logged before done, the outcome is recorded, and no
// secret appears (FR-008, SC-005).
func TestMutationLogging(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	logMutationStart("create_folder", "bucket", "b", "key", "reports/", "context", "ctx")
	logMutationDone("create_folder", nil, "bucket", "b", "key", "reports/")

	out := buf.String()
	start := strings.Index(out, "mutation.start")
	done := strings.Index(out, "mutation.done")
	switch {
	case start < 0:
		t.Error("missing mutation.start record")
	case done < 0:
		t.Error("missing mutation.done record")
	case start > done:
		t.Error("mutation.start must be logged before mutation.done")
	}
	if !strings.Contains(out, `"outcome":"ok"`) {
		t.Errorf("missing ok outcome:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "secret") || strings.Contains(out, "AKIA") {
		t.Errorf("log must not contain secrets:\n%s", out)
	}
}
