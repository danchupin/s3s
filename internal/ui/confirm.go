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
			if op.input.Value == op.expect { // byte-for-byte exact
				return m.dispatchOp()
			}
			// Mismatch aborts with no action and no command.
			m.op = nil
			m.err = errConfirmMismatch
			return m, nil
		case "left":
			op.input.Left()
			return m, nil
		case "right":
			op.input.Right()
			return m, nil
		case "home":
			op.input.Home()
			return m, nil
		case "end":
			op.input.End()
			return m, nil
		case "backspace":
			op.input.Backspace()
			return m, nil
		case "delete":
			op.input.DeleteFwd()
			return m, nil
		default:
			if msg.Text != "" {
				op.input.Insert(msg.Text)
			}
			return m, nil
		}
	}

	// Simple tier (reversible): y/Enter confirms; n or the Back binding aborts; other keys
	// are ignored so a stray press neither confirms nor cancels. Cancel is sourced from the
	// keymap so a rebind of Back takes effect here too. (The typed tier keeps a
	// literal Esc cancel — its Back aliases left/h are live text-editing keys there.)
	switch {
	case key == "y" || key == "Y" || key == "enter":
		return m.dispatchOp()
	case key == "n" || key == "N" || matches(key, m.keys.Back):
		m.op = nil
		return m, nil
	default:
		return m, nil
	}
}
