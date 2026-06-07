package ui

import (
	"sort"

	tea "charm.land/bubbletea/v2"
)

// Multi-select. Only OBJECTS are markable (folders/prefixes are not, FR-014);
// the selection is per-level and cleared on navigation. Recursive folder
// deletion stays its own dedicated action — never driven by multi-select.

// toggleMark marks/unmarks the highlighted object.
func (m App) toggleMark() (tea.Model, tea.Cmd) {
	e := m.selected()
	if e == nil || e.isDir {
		return m, nil // objects only
	}
	if m.sel == nil {
		m.sel = map[string]bool{}
	}
	if m.sel[e.full] {
		delete(m.sel, e.full)
	} else {
		m.sel[e.full] = true
	}
	return m, nil
}

// selCount is the number of marked objects.
func (m App) selCount() int { return len(m.sel) }

// selSize is the combined size of the marked objects in the loaded level.
func (m App) selSize() int64 {
	var total int64
	if m.level == nil {
		return 0
	}
	for i := range m.level.objects {
		if m.sel[m.level.objects[i].Key] {
			total += m.level.objects[i].Size
		}
	}
	return total
}

// selectedKeys returns the marked keys in deterministic order.
func (m App) selectedKeys() []string {
	keys := make([]string, 0, len(m.sel))
	for k := range m.sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
