package ui

import (
	tea "charm.land/bubbletea/v2"
)

// startSearch opens the search/filter input for the current view.
func (m App) startSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.searchInput = ""
	return m, nil
}

// onSearchKey handles keystrokes while the search/filter input is active. In the
// bucket list it filters names locally (no backend call); in the object tree it
// runs a debounced server-side prefix search.
func (m App) onSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		if m.mode == modeBuckets {
			m.bucketFilter = ""
			m.bucketSel = 0
			return m, nil
		}
		if m.search != "" {
			m.search = "" // clear active filter, restore full level (FR-018)
			return m.enterLevel()
		}
		return m, nil
	case "enter":
		m.searching = false // close input; bucket filter / search term stay applied
		if m.mode == modeBuckets {
			return m, nil
		}
		return m.runSearch(m.searchInput, false)
	case "backspace":
		if r := []rune(m.searchInput); len(r) > 0 {
			m.searchInput = string(r[:len(r)-1])
		}
		return m.afterFilterEdit()
	default:
		if msg.Text != "" {
			m.searchInput += msg.Text
			return m.afterFilterEdit()
		}
	}
	return m, nil
}

// afterFilterEdit applies a filter keystroke: live local filtering for buckets,
// debounced server-side search for the object tree.
func (m App) afterFilterEdit() (tea.Model, tea.Cmd) {
	if m.mode == modeBuckets {
		m.bucketFilter = m.searchInput
		m.bucketSel = 0
		return m, nil
	}
	return m.scheduleSearch()
}

// scheduleSearch arms a debounce; only the latest keystroke's timer survives the
// searchGen check, coalescing bursts into ≤1 in-flight request (FR-017a).
func (m App) scheduleSearch() (tea.Model, tea.Cmd) {
	m.searchGen++
	return m, debounceSearch(m.searchGen, m.searchInput)
}

// onSearchFire applies a debounced search if it is still the latest keystroke and
// the input is still open (live narrowing while typing).
func (m App) onSearchFire(msg searchFireMsg) (tea.Model, tea.Cmd) {
	if msg.searchGen != m.searchGen || !m.searching {
		return m, nil // superseded keystroke or input already closed
	}
	return m.runSearch(msg.term, true) // keep input open for further typing
}

// runSearch sets the active search term and reloads the level (via the cancel/
// generation path). keepOpen controls whether the input stays focused.
func (m App) runSearch(term string, keepOpen bool) (tea.Model, tea.Cmd) {
	m.search = term
	model, cmd := m.enterLevel()
	app := model.(App)
	app.searching = keepOpen
	return app, cmd
}
