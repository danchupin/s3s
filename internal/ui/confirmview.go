package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// Confirmation surfaces (007 US4). The surface is chosen per tier, not per action:
//   - BINARY tier (single-object/group delete, move, overwrite): a centered popup
//     dialog (k9s-style), y/N — confirmPopupView, overlaid by View().
//   - TYPED-IDENTIFIER tier (recursive delete → path, bucket → name, connection →
//     name): a PROMINENT INLINE form (not a separate window) with a real editable
//     field — typedConfirmForm, rendered in the footer status zone.
//
// Both surfaces carry the loud write badge and share one visual style (FR-027a), and
// cancel on Esc with no mutation (FR-025). The byte-exact match + y/N logic lives in
// onConfirmKey (unchanged); this file only renders.

// The surface is a pure function of the tier: the typed-identifier tier (confirmTyped)
// renders the prominent inline form (typedConfirmForm, in the footer); every other tier
// renders the centered binary popup (confirmPopupView, by View()). Call sites compare
// op.tier directly — there is no separate surface enum.

// popupBoxStyle is the centered binary-confirm dialog box: a rounded border in the
// caution palette, padded, width-capped — reuses existing tokens (FR-013/FR-027a).
var popupBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colWarn).
	Padding(0, 2)

// confirmPopupView renders the centered binary-confirm dialog over the body area. It
// carries the loud write badge and shares the palette with the inline form (FR-027a).
// It is sized to fit (no clip) and centered in the (w × h) body box (SC-009).
func (m App) confirmPopupView(w, h int) string {
	badge := writeBadge(m.writable())
	prompt := m.simpleConfirmText()
	capW := max(10, min(w-8, 60))
	inner := badge + "  " + titleStyle.Render("confirm") + "\n\n" +
		objCellStyle.Render(truncate(prompt, capW)) + "\n\n" +
		accentStyle.Render("y") + dimCellStyle.Render(" confirm · ") +
		accentStyle.Render("n/Esc") + dimCellStyle.Render(" cancel")
	box := popupBoxStyle.Render(inner)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// typedIdentifierLabel describes what the operator must type for the typed tier.
func typedIdentifierLabel(kind string) (verb, noun string) {
	switch kind {
	case "delete_recursive":
		return "recursively delete", "path"
	case "delete_bucket":
		return "delete bucket", "bucket name"
	case "delete_connection":
		return "delete connection", "connection name"
	default:
		return "confirm", "value"
	}
}

// typedConfirmForm renders the PROMINENT inline typed-identifier form in the footer (not
// a separate window). A left accent bar marks it; it carries the badge, names what must
// be typed, and shows the editable input with a cursor. The input scrolls horizontally
// so a long path / bucket name stays legible (FR-023a). Returns a multi-line block.
func (m App) typedConfirmForm(w int) string {
	op := m.op
	verb, noun := typedIdentifierLabel(op.kind)
	bar := warnStyle.Render("▌ ")
	badge := writeBadge(m.writable())

	head := bar + badge + " " + titleStyle.Render("type to confirm")
	// Window the input to the available width so a long identifier scrolls into view —
	// rune/width-aware (never cuts mid-rune; review #4).
	avail := max(8, w-lipgloss.Width(noun)-18)
	field := objCellStyle.Render(op.input.Render(avail, false))
	prompt := bar + dimCellStyle.Render(fmt.Sprintf("%s — type %s ", verb, noun)) +
		accentStyle.Render(sanitizeLabel(op.expect)) + dimCellStyle.Render(": ") + field
	hint := bar + dimCellStyle.Render("Enter confirm · Esc cancel")
	if op.input.Value != "" && op.input.Value != op.expect {
		hint = bar + warnStyle.Render("does not match yet") + dimCellStyle.Render(" · Enter confirm · Esc cancel")
	}
	return head + "\n" + prompt + "\n" + hint
}
