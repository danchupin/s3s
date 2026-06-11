package ui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Per-field copy (017 US2/FR-012): a field-select overlay over the focused object's
// metadata rows. Enter copies the FULL untruncated value (best-effort OSC52 — the same
// clipboard path reveal uses) and confirms in the footer; Esc cancels. The rows come
// from the SAME metaGroups source the pane renders, so the copyable set cannot drift
// from the visible one. The user-facing entry point is the copy menu (US3); the
// dispatcher is directly invokable for tests and future bindings.

// fieldCopyState is the active field-select overlay (nil = closed).
type fieldCopyState struct {
	key  string      // object key the rows belong to
	rows []metaField // label + FULL value (+ user-metadata rows)
	sel  int
}

// fieldCopyMeta resolves the metadata backing the field-copy rows for the current
// focus: the full-screen object view's load, or the details pane's debounced load.
func (m App) fieldCopyMeta() (*storage.ObjectMetadata, string) {
	if m.mode == modeObject && m.meta != nil {
		return m.meta, m.meta.Key
	}
	if e := m.selected(); e != nil && !e.isDir && m.paneMeta != nil && m.paneSelKey == e.full {
		return m.paneMeta, e.full
	}
	return nil, ""
}

// startFieldCopy opens the field-select overlay for the focused object's loaded
// metadata. No-op when no metadata is loaded yet (nothing to copy).
func (m App) startFieldCopy() (tea.Model, tea.Cmd) {
	md, key := m.fieldCopyMeta()
	if md == nil {
		return m, nil
	}
	var rows []metaField
	for _, g := range metaGroups(*md, m.now()) {
		rows = append(rows, g.fields...)
	}
	for _, k := range sortedKeys(md.UserMetadata) {
		rows = append(rows, metaField{label: k, value: md.UserMetadata[k]})
	}
	m.fieldCopy = &fieldCopyState{key: key, rows: rows}
	return m, nil
}

// onFieldCopyKey owns input while the field-select overlay is open.
func (m App) onFieldCopyKey(key string) (tea.Model, tea.Cmd) {
	fc := m.fieldCopy
	switch {
	case matches(key, m.keys.Up):
		if fc.sel > 0 {
			fc.sel--
		}
	case matches(key, m.keys.Down):
		if fc.sel < len(fc.rows)-1 {
			fc.sel++
		}
	case matches(key, m.keys.Enter):
		if fc.sel >= 0 && fc.sel < len(fc.rows) {
			f := fc.rows[fc.sel]
			m.fieldCopy = nil
			m.notice = "copied " + f.label + " — " + truncate(f.value, 40)
			return m, tea.SetClipboard(f.value)
		}
	case matches(key, m.keys.Back):
		m.fieldCopy = nil
	}
	return m, nil
}

// fieldCopyView renders the field-select overlay centered over the (w × h) body,
// reusing the popup surface (constitution VII).
func (m App) fieldCopyView(w, h int) string {
	fc := m.fieldCopy
	capW := max(10, min(w-8, 72))
	var body string
	for i, f := range fc.rows {
		line := pad(f.label, metaKeyWidth) + truncate(f.value, max(1, capW-metaKeyWidth-2))
		if i == fc.sel {
			body += selRowStyle.Render("▸ " + line)
		} else {
			body += objCellStyle.Render("  " + line)
		}
		body += "\n"
	}
	inner := titleStyle.Render("copy a field") + "\n\n" +
		strings.TrimRight(body, "\n") + "\n\n" +
		dimCellStyle.Render(keyHint(m.keys.Enter, "copy")+sepDot+keyHint(m.keys.Back, "cancel"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, popupBoxStyle.Render(inner))
}

// sortedKeys returns the map's keys sorted (stable row order).
func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
