package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// Runtime read-only↔write toggle with loud, always-on signalling (005 US5). The
// guard is dynamic (App.activeStore); this file owns the arm/disarm UX and logging.
// Arming takes a deliberate confirm; disarming is instant — asymmetric friction so
// it is easy to get safe and harder to get dangerous (FR-026).

// toggleWrite handles the write-toggle key. On a readonly:true context it refuses
// (FR-028). While armed it disarms instantly (no confirm). While disarmed it opens a
// simple arm confirmation (FR-025/FR-026).
func (m App) toggleWrite() (tea.Model, tea.Cmd) {
	switch {
	case m.ctxReadOnly:
		m.armConfirm = false
		m.notice = "context is read-only — cannot arm write"
		return m, nil
	case m.armed:
		m.armed = false
		m.armConfirm = false
		logWriteState(false, m.ctxName)
		m.notice = "write disarmed — read-only"
		return m, nil
	default:
		m.armConfirm = true // await deliberate confirmation
		return m, nil
	}
}

// onArmConfirmKey resolves the pending arm confirmation: y/Enter arms write, anything
// else cancels and stays read-only (FR-026).
func (m App) onArmConfirmKey(key string) (tea.Model, tea.Cmd) {
	m.armConfirm = false
	switch key {
	case "y", "Y", "enter":
		m.armed = true
		logWriteState(true, m.ctxName)
		m.notice = "write ARMED — mutations enabled"
	default:
		m.notice = "stayed read-only"
	}
	return m, nil
}

// armConfirmLine renders the arm confirmation prompt in the status line (FR-026).
func (m App) armConfirmLine() string {
	return accentStyle.Render("arm WRITE mode? mutations will be enabled") +
		dimCellStyle.Render("  (y / N)")
}

// logWriteState records a read-only↔write transition as a security-relevant event
// (005 FR-032). Secrets are never involved here.
func logWriteState(armed bool, ctx string) {
	state := "read-only"
	if armed {
		state = "write"
	}
	slog.Info("write.toggle", "state", state, "context", ctx)
}
