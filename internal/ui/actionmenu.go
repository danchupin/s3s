package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// menuItem is one entry in the contextual action menu (FR-023..FR-027). Each item is
// a pure entry point into an EXISTING operation flow (start*/refresh) — the menu adds
// no operation or confirmation behavior of its own (FR-026).
type menuItem struct {
	label     string
	writeOnly bool
	invoke    func(App) (tea.Model, tea.Cmd)
}

// menuItemsFor builds the contextual item list for the current mode/selection/
// capability (contract action-menu C2). Refresh is always present and always last
// (FR-025); write items are omitted in read-only contexts (FR-024).
func (m App) menuItemsFor() []menuItem {
	// Bucket list: analyze the highlighted bucket (read), then Refresh.
	if m.mode == modeBuckets {
		return []menuItem{
			{label: "analyze", invoke: App.startAnalyze},
			{label: "refresh", invoke: App.refreshBuckets},
		}
	}

	var items []menuItem
	// Download is a read — available for an object selection in any context, incl.
	// read-only (005 US1, FR-002). Menu-only: no dedicated top-level key (FR-023).
	if m.selKind() == selObject {
		items = append(items, menuItem{label: "download", invoke: App.startDownload})
	}
	// Analyze (du) is a read — available on a folder selection or the current level,
	// in any context (005 US2, FR-010).
	if m.selKind() != selObject {
		items = append(items, menuItem{label: "analyze", invoke: App.startAnalyze})
	}
	if m.writable() {
		switch m.selKind() {
		case selObject:
			items = append(items,
				menuItem{label: "delete", writeOnly: true, invoke: App.startRemoveObject},
				menuItem{label: "copy", writeOnly: true, invoke: App.startCopy},
				menuItem{label: "move / rename", writeOnly: true, invoke: App.startMove},
			)
		case selFolder:
			items = append(items,
				menuItem{label: "recursive delete", writeOnly: true, invoke: App.startRecursiveDelete},
			)
		}
		// Level-scoped actions apply regardless of selection.
		items = append(items,
			menuItem{label: "upload here", writeOnly: true, invoke: App.startUpload},
			menuItem{label: "new folder", writeOnly: true, invoke: App.startCreateFolder},
		)
	}
	items = append(items, menuItem{label: "refresh", invoke: App.refresh})
	return items
}

// openActionMenu opens the contextual action menu over a list mode (FR-023). It is a
// no-op outside the bucket/tree list modes (handled by the key router, which only
// calls this from those modes).
func (m App) openActionMenu() (tea.Model, tea.Cmd) {
	m.menuItems = m.menuItemsFor()
	m.menuSel = 0
	m.prevMode = m.mode
	m.mode = modeActionMenu
	return m, nil
}

// onMenuKey routes a keypress while the action menu is open. Navigation uses the
// existing keys (↑/↓ + vim); Enter/→ invokes the selected item (restoring the prior
// mode first, so the resulting operation flow renders normally); Esc/← closes
// (FR-027). Esc here closes the menu and never cancels a background load (FR-029
// modal precedence).
func (m App) onMenuKey(key string) (tea.Model, tea.Cmd) {
	n := len(m.menuItems)
	switch {
	case matches(key, m.keys.Up):
		if m.menuSel > 0 {
			m.menuSel--
		}
	case matches(key, m.keys.Down):
		if m.menuSel < n-1 {
			m.menuSel++
		}
	case matches(key, m.keys.Top):
		m.menuSel = 0
	case matches(key, m.keys.Bottom):
		m.menuSel = max(0, n-1)
	case matches(key, m.keys.Enter):
		if m.menuSel < 0 || m.menuSel >= n {
			return m, nil
		}
		item := m.menuItems[m.menuSel]
		m.mode = m.prevMode
		m.menuItems = nil
		return item.invoke(m)
	case matches(key, m.keys.Back):
		m.mode = m.prevMode
		m.menuItems = nil
	}
	return m, nil
}

// actionMenuView renders the menu as a bordered overlay box (k9s-style), titled with
// the current selection and ending with a dismissal hint (FR-027). Nav cues use arrow
// glyphs only (FR-031).
func (m App) actionMenuView(width int) string {
	var b strings.Builder
	for i, it := range m.menuItems {
		marker := "  "
		label := objCellStyle.Render(it.label)
		if i == m.menuSel {
			marker = "▶ "
			label = selRowStyle.Render(it.label)
		}
		b.WriteString(marker + label + "\n")
	}
	b.WriteString("\n" + dimCellStyle.Render("↑/↓ select · ↵ run · esc close"))
	title := "actions"
	if sel := m.selectionName(); sel != "" {
		title = "actions: " + sel
	}
	return boxView(title, "", strings.TrimRight(b.String(), "\n"), width, len(m.menuItems)+2)
}

// refreshBuckets reloads the bucket list (the menu's Refresh item in the bucket list;
// sole refresh entry point now that top-level `r` is removed — FR-025/FR-028).
func (m App) refreshBuckets() (tea.Model, tea.Cmd) {
	ctx := (&m).beginLoad()
	return m, tea.Batch(loadBuckets(ctx, m.activeStore(), m.gen), spinnerTick())
}
