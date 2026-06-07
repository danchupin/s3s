package ui

import (
	tea "charm.land/bubbletea/v2"
)

// filterIsBucketList reports whether the filter targets the bucket LIST (instant local
// filter) rather than an object level (server-side current-prefix search) — the focus-aware
// scope split.
func (m App) filterIsBucketList() bool {
	return m.mode == modeBuckets && m.focusZone == zoneBuckets
}

// committedFilterTerm is the filter term currently applied to the focused pane.
func (m App) committedFilterTerm() string {
	if m.filterIsBucketList() {
		return m.bucketFilter
	}
	return m.search
}

// startSearch opens the filter input for the focused pane, PRE-FILLED with the committed term
// so it re-opens for refinement; the committed term is remembered for Esc-revert.
func (m App) startSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.filterBefore = m.committedFilterTerm()
	m.searchInput = m.filterBefore
	return m, nil
}

// onSearchKey handles keystrokes while the filter input is active. The bucket list filters
// names locally (no backend call); an object level (full-screen tree OR the objects zone) runs
// a debounced server-side prefix search. Enter commits; Esc reverts to the committed term.
func (m App) onSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel the in-progress input → revert to the last committed state: a
		// committed filter is preserved; clearing it is the separate Back/clear path.
		m.searching = false
		if m.filterIsBucketList() {
			m.bucketFilter = m.filterBefore
			m.bucketSel = 0
			return m, nil
		}
		if m.search != m.filterBefore {
			return m.runSearch(m.filterBefore, false)
		}
		return m, nil
	case "enter":
		// Commit: close the input; the filter stays applied and focus stays on the filtered pane.
		m.searching = false
		if m.filterIsBucketList() {
			m.bucketFilter = m.searchInput
			m.bucketSel = 0
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

// afterFilterEdit applies a filter keystroke: live local filtering for the bucket list,
// debounced live server-side search for an object level.
func (m App) afterFilterEdit() (tea.Model, tea.Cmd) {
	if m.filterIsBucketList() {
		m.bucketFilter = m.searchInput
		m.bucketSel = 0
		return m, nil
	}
	return m.scheduleSearch()
}

// scheduleSearch arms a debounce; only the latest keystroke's timer survives the
// searchGen check, coalescing bursts into ≤1 in-flight request.
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

// runSearch sets the active search term and reloads the current object level. In the objects
// zone it reloads via loadObjectsLevel so the two-pane browse is preserved; in the
// full-screen tree it reloads via enterLevel. keepOpen controls whether the input stays focused.
func (m App) runSearch(term string, keepOpen bool) (tea.Model, tea.Cmd) {
	var model tea.Model
	var cmd tea.Cmd
	if m.mode == modeBuckets && m.focusZone == zoneObjects {
		model, cmd = m.loadObjectsLevel(m.bucket, m.prefix, term) // stays multi-pane
	} else {
		m.search = term
		model, cmd = m.enterLevel()
	}
	app := model.(App)
	app.searching = keepOpen
	return app, cmd
}
