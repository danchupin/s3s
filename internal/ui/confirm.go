package ui

import (
	tea "charm.land/bubbletea/v2"
)

// onConfirmKey handles the confirmation overlay for the active operation. The
// simple tier takes y/Enter to confirm and n/Esc to abort; the typed tier requires
// a byte-for-byte exact entry of the target identifier (no trim, no case-fold —
// FR-005/SC-003) and aborts on any mismatch.
func (m App) onConfirmKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	op := m.op
	if op.tier == confirmTyped {
		switch key {
		case "esc":
			m.op = nil
			return m, nil
		case "enter":
			if op.input == op.expect { // byte-for-byte exact
				return m.dispatchOp()
			}
			// Mismatch aborts with no action and no command (SC-003).
			m.op = nil
			m.err = errConfirmMismatch
			return m, nil
		case "backspace":
			if n := len(op.input); n > 0 {
				op.input = op.input[:n-1]
			}
			return m, nil
		default:
			if msg.Text != "" {
				op.input += msg.Text
			}
			return m, nil
		}
	}

	// Simple tier (reversible): only y/Enter confirms; n/Esc aborts; other keys are
	// ignored so a stray press neither confirms nor cancels.
	switch key {
	case "y", "Y", "enter":
		return m.dispatchOp()
	case "n", "N", "esc":
		m.op = nil
		return m, nil
	default:
		return m, nil
	}
}
