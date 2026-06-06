package ui

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// buildApp constructs a test App with explicit arm/lock state.
func buildApp(f *storage.Fake, armed, readonly bool) App {
	return New(
		Backend{Store: f, Cluster: "c", User: "u", Endpoint: "x", Writable: armed, ReadOnly: readonly},
		"ctx", []string{"ctx"}, nil, preview.ProtoNone,
	)
}

// TestWriteToggleArmDisarm: RO + toggle → confirm prompt; confirm → writable; second
// toggle → instant RO (no confirm) — write-toggle-contract C2 (005 FR-025/FR-026).
func TestWriteToggleArmDisarm(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := buildApp(f, false, false)

	if m.writable() {
		t.Fatal("fresh app should start read-only")
	}
	m = press(m, "w")
	if !m.armConfirm {
		t.Fatal("toggle from RO should open an arm confirmation")
	}
	if m.writable() {
		t.Fatal("must not be writable before confirming")
	}
	m = press(m, "y")
	if m.armConfirm || !m.writable() {
		t.Fatalf("after confirm: armConfirm=%v writable=%v, want false/true", m.armConfirm, m.writable())
	}
	// Disarm is instant — no confirmation.
	m = press(m, "w")
	if m.armConfirm {
		t.Error("disarm must not prompt")
	}
	if m.writable() {
		t.Error("after disarm toggle, must be read-only")
	}
}

// TestWriteToggleCancel: declining the arm confirmation stays read-only.
func TestWriteToggleCancel(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := buildApp(f, false, false)
	m = press(m, "w")
	m = press(m, "n")
	if m.armConfirm || m.writable() {
		t.Errorf("declining arm: armConfirm=%v writable=%v, want false/false", m.armConfirm, m.writable())
	}
}

// TestWriteToggleReadOnlyContextRefused: a readonly:true context refuses to arm and
// stays read-only (005 FR-028); --write initial starts armed (FR-031).
func TestWriteToggleReadOnlyContextRefused(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")

	locked := buildApp(f, true, true) // --write intent AND readonly:true
	if locked.writable() {
		t.Fatal("readonly:true context must never be writable, even with --write")
	}
	locked = press(locked, "w")
	if locked.armConfirm {
		t.Error("toggle on a locked context must not open a confirmation")
	}
	if locked.writable() {
		t.Error("locked context stayed/ became writable after toggle")
	}
	if !strings.Contains(locked.notice, "read-only") {
		t.Errorf("expected a read-only refusal notice, got %q", locked.notice)
	}

	armed := buildApp(f, true, false) // --write on a writable context
	if !armed.writable() {
		t.Error("--write on a writable context should start armed")
	}
}

// TestWriteToggleContextSwitchDerives: switching to a readonly:true context forces RO;
// switching to a writable context preserves the arm intent (005 FR-029).
func TestWriteToggleContextSwitchDerives(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	resolve := func(name string) (Backend, error) {
		ro := name == "locked"
		return Backend{Store: f, Cluster: "c2", User: "u", Endpoint: "x", ReadOnly: ro}, nil
	}
	m := buildApp(f, true, false) // armed, writable context
	m.resolve = resolve
	m.contexts = []string{"ctx", "locked", "open"}
	if !m.writable() {
		t.Fatal("precondition: should be armed+writable")
	}

	mm, _ := m.applyContext("locked")
	m = finishSwitch(mm.(App), resolve, "locked")
	if m.writable() {
		t.Error("after switching to a readonly:true context, must be read-only")
	}
	if !m.armed {
		t.Error("arm intent should be preserved across the switch")
	}

	mm, _ = m.applyContext("open")
	m = finishSwitch(mm.(App), resolve, "open")
	if !m.writable() {
		t.Error("switching back to a writable context should restore writable (armed preserved)")
	}
}

// TestWriteBadgeOnEveryScreen: the loud [RW] marker shows on list, menu, and help
// while armed, and [RO] while disarmed, including a narrow width (005 FR-027).
func TestWriteBadgeOnEveryScreen(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "b/x.txt")

	armed := buildApp(f, true, false)
	if !strings.Contains(viewOf(armed), "[RW]") {
		t.Error("armed list view missing [RW] badge")
	}
	if menu := viewOf(press(armed, "a")); !strings.Contains(menu, "[RW]") {
		t.Error("armed action-menu overlay missing [RW] badge")
	}
	if help := viewOf(press(armed, "?")); !strings.Contains(help, "[RW]") {
		t.Error("armed help overlay missing [RW] badge")
	}

	// Narrow width: the badge must still be present.
	narrow := deliver(armed, tea.WindowSizeMsg{Width: 24, Height: 20})
	if !strings.Contains(viewOf(narrow), "[RW]") {
		t.Error("armed narrow view dropped the [RW] badge")
	}

	ro := buildApp(f, false, false)
	if !strings.Contains(viewOf(ro), "[RO]") {
		t.Error("disarmed view missing [RO] marker")
	}
}

// TestWriteToggleLogged: each transition emits a security-relevant slog record with
// the new state + context (005 FR-032, write-toggle-contract C5).
func TestWriteToggleLogged(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(old)

	f := storage.NewFake()
	f.Seed("b")
	m := buildApp(f, false, false)
	m = press(m, "w")
	m = press(m, "y") // arm
	_ = press(m, "w") // disarm

	out := buf.String()
	if !strings.Contains(out, "write.toggle") {
		t.Fatalf("no write.toggle log record: %s", out)
	}
	if !strings.Contains(out, `"state":"write"`) || !strings.Contains(out, `"state":"read-only"`) {
		t.Errorf("expected both arm and disarm states logged; got %s", out)
	}
	if !strings.Contains(out, `"context":"ctx"`) {
		t.Errorf("transition should log the context; got %s", out)
	}
}
