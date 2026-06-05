package ui

import (
	"errors"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// typedFixture is a test-only ConfirmTyped operation (no production destructive
// action ships in 002 — US3). It exercises the typed tier end-to-end.
func typedFixture() *operation {
	return &operation{kind: "noop_destructive", tier: confirmTyped, expect: "prod", phase: phaseConfirm}
}

func typedApp() App {
	m := newApp(storage.NewFake(), nil, nil)
	m.writable = true
	m.op = typedFixture()
	return m
}

func typeStr(m App, s string) App {
	for _, r := range s {
		m = press(m, string(r))
	}
	return m
}

// TestTypedConfirmExactMatchDispatches: a byte-for-byte exact entry confirms and
// dispatches (FR-005/SC-003).
func TestTypedConfirmExactMatchDispatches(t *testing.T) {
	m := typeStr(typedApp(), "prod")
	if m.op == nil || m.op.input != "prod" {
		t.Fatalf("typed input not captured: %+v", m.op)
	}
	m2, cmd := pressCmd(m, "enter")
	m = m2
	if cmd == nil {
		t.Error("exact match must dispatch a command")
	}
	if m.op == nil || m.op.phase != phaseRunning {
		t.Errorf("exact match should move op to running; op=%+v", m.op)
	}
}

// TestTypedConfirmMismatchAborts: any mismatch (incl. trailing space / wrong case)
// aborts with no command and no change (SC-003).
func TestTypedConfirmMismatchAborts(t *testing.T) {
	for _, entry := range []string{"prdo", "prod ", "PROD"} {
		m := typeStr(typedApp(), entry)
		m2, cmd := pressCmd(m, "enter")
		m = m2
		if cmd != nil {
			t.Errorf("entry %q: mismatch must not dispatch a command", entry)
		}
		if m.op != nil {
			t.Errorf("entry %q: mismatch must abort the op", entry)
		}
		if !errors.Is(m.err, errConfirmMismatch) {
			t.Errorf("entry %q: want errConfirmMismatch, got %v", entry, m.err)
		}
	}
}

// TestTypedConfirmEscAborts: Esc cancels with no command.
func TestTypedConfirmEscAborts(t *testing.T) {
	m, cmd := pressCmd(typedApp(), "esc")
	if cmd != nil {
		t.Error("esc must not dispatch")
	}
	if m.op != nil {
		t.Error("esc must abort the op")
	}
}
