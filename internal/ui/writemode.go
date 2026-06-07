package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Runtime read-only↔write toggle with loud, always-on signalling. The
// guard is dynamic (App.activeStore); this file owns the arm/disarm UX and logging.
// Arming takes a deliberate confirm; disarming is instant — asymmetric friction so
// it is easy to get safe and harder to get dangerous.

// toggleWrite handles the write-toggle key. On a readonly:true context it refuses
// While armed it disarms instantly (no confirm). While disarmed it opens a
// simple arm confirmation.
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
// else cancels and stays read-only.
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

// armConfirmPopupView renders the write-arm confirmation as a PROMINENT centered popup (012
// US4, FR-014), reusing the shared confirm-popup surface so it is impossible to miss — not a
// faint status line. The badge stays visible; cancel is the default; disarming never
// reaches this surface (it is instant, FR-016).
func (m App) armConfirmPopupView(w, h int) string {
	inner := writeBadge(m.writable()) + "  " + titleStyle.Render("arm WRITE mode") + "\n\n" +
		objCellStyle.Render("mutations will be enabled") + "\n\n" +
		accentStyle.Render("y") + dimCellStyle.Render(" arm · ") +
		accentStyle.Render("n/Esc") + dimCellStyle.Render(" cancel (default)")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popupBoxStyle.Render(inner))
}

// logWriteState records a read-only↔write transition as a security-relevant event
// Secrets are never involved here.
func logWriteState(armed bool, ctx string) {
	state := "read-only"
	if armed {
		state = "write"
	}
	slog.Info("write.toggle", "state", state, "context", ctx)
}
