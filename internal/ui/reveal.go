package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Reveal/inspect popup: a read-only centered overlay showing the FULL
// identifier of the current selection (bucket / object key / folder / breadcrumb path) so a
// value too long to wrap in place is still fully visible, plus a best-effort OSC52 clipboard
// copy. Reuses the confirmPopupView surface for design-system consistency.

type revealKind int

const (
	revealBucket revealKind = iota
	revealObject
	revealFolder
	revealPath
)

func (k revealKind) label() string {
	switch k {
	case revealObject:
		return "object key"
	case revealFolder:
		return "folder"
	case revealPath:
		return "path"
	default:
		return "bucket"
	}
}

// revealState is the active reveal popup payload (nil = closed).
type revealState struct {
	kind   revealKind
	value  string
	detail string // optional extra line (e.g. the full storage class for a marked object)
}

// openReveal captures the full identifier of the current selection and opens the reveal
// popup, emitting a best-effort clipboard copy. No-op when nothing is selected.
func (m App) openReveal() (tea.Model, tea.Cmd) {
	rs := m.revealTarget()
	if rs == nil {
		return m, nil
	}
	m.reveal = rs
	return m, tea.SetClipboard(rs.value)
}

// revealTarget derives the reveal payload from the current mode/focus + selection.
func (m App) revealTarget() *revealState {
	switch {
	case m.mode == modeTree || (m.mode == modeBuckets && m.focusZone == zoneObjects):
		e := m.selected()
		if e == nil {
			return nil
		}
		if e.isDir {
			return &revealState{kind: revealFolder, value: e.full}
		}
		rs := &revealState{kind: revealObject, value: e.full}
		if e.obj != nil && !isStandardClass(e.obj.StorageClass) {
			rs.detail = "class: " + e.obj.StorageClass // full, lossless class (016 US5)
		}
		return rs
	case m.mode == modeBuckets:
		fb := m.filteredBuckets()
		if m.bucketSel < 0 || m.bucketSel >= len(fb) {
			return nil
		}
		return &revealState{kind: revealBucket, value: fb[m.bucketSel].Name}
	}
	return nil
}

// revealView renders the reveal popup centered over the (w × h) body, reusing popupBoxStyle +
// lipgloss.Place (mirroring confirmPopupView). The full value is wrapped so it is never
// truncated; copy is best-effort (OSC52), so the hint says so. NO_COLOR-safe (text + border).
func (m App) revealView(w, h int) string {
	rs := m.reveal
	capW := max(10, min(w-8, 72))
	var body string
	for i, l := range wrapValue(sanitizeLabel(rs.value), capW, max(1, h-6)) {
		if i > 0 {
			body += "\n"
		}
		body += objCellStyle.Render(l)
	}
	if rs.detail != "" {
		body += "\n\n" + dimCellStyle.Render(sanitizeLabel(rs.detail))
	}
	inner := titleStyle.Render("reveal "+rs.kind.label()) + "\n\n" +
		body + "\n\n" +
		dimCellStyle.Render("copied (best-effort) · any key to close")
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popupBoxStyle.Render(inner))
}
