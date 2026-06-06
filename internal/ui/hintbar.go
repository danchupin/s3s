package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Menu-less direct actions (006 US1). The modal action menu is gone (FR-001): the
// SAME selection/capability gating that built the old menu now drives (a) an always-
// visible hint bar of valid single-key actions and (b) the direct-key dispatch table.
// Each action's invoke is the EXISTING start*/refresh entry point, so confirmations
// and operation flows are unchanged (FR-005).

// action is one invokable item: a single key, a label, a write-gate, an availability
// predicate (mode/selection), and the entry point it dispatches to.
type action struct {
	key       string
	label     string
	writeOnly bool
	avail     func(App) bool
	invoke    func(App) (tea.Model, tea.Cmd)
}

// hasMarks reports an active multi-select.
func (m App) hasMarks() bool { return m.selCount() > 0 }

// actionCatalog returns the ordered actions for the current list mode. Order is the
// hint-bar display order. Bulk variants take over d/x/y when a multi-select is active
// (FR-006); their invoke picks the bulk vs single entry point at dispatch time.
func (m App) actionCatalog() []action {
	if m.mode == modeBuckets {
		return []action{
			{key: "a", label: "analyze", avail: func(a App) bool { return len(a.filteredBuckets()) > 0 }, invoke: App.startAnalyze},
			{key: "r", label: "refresh", avail: func(App) bool { return true }, invoke: App.refreshBuckets},
		}
	}
	objOrMarks := func(a App) bool { return a.selKind() == selObject || a.hasMarks() }
	return []action{
		{key: "d", label: "download", avail: objOrMarks, invoke: func(a App) (tea.Model, tea.Cmd) {
			if a.hasMarks() {
				return a.startBulkDownload()
			}
			return a.startDownload()
		}},
		{key: "a", label: "analyze", avail: func(a App) bool { return a.selKind() != selObject || a.selKind() == selFolder }, invoke: App.startAnalyze},
		{key: "x", label: "delete", writeOnly: true, avail: objOrMarks, invoke: func(a App) (tea.Model, tea.Cmd) {
			if a.hasMarks() {
				return a.startBulkDelete()
			}
			return a.startRemoveObject()
		}},
		{key: "X", label: "rm -r", writeOnly: true, avail: func(a App) bool { return a.selKind() == selFolder }, invoke: App.startRecursiveDelete},
		{key: "y", label: "copy", writeOnly: true, avail: objOrMarks, invoke: func(a App) (tea.Model, tea.Cmd) {
			if a.hasMarks() {
				return a.startBulkCopy()
			}
			return a.startCopy()
		}},
		{key: "m", label: "move", writeOnly: true, avail: func(a App) bool { return a.selKind() == selObject }, invoke: App.startMove},
		{key: "u", label: "upload", writeOnly: true, avail: func(App) bool { return true }, invoke: App.startUpload},
		{key: "+", label: "mkdir", writeOnly: true, avail: func(App) bool { return true }, invoke: App.startCreateFolder},
		{key: "r", label: "refresh", avail: func(App) bool { return true }, invoke: App.refresh},
	}
}

// availableActions filters the catalog to the actions valid for the current selection
// and capability: an unavailable action is dropped; a write action is dropped when the
// context is not writable (FR-003/FR-004).
func (m App) availableActions() []action {
	var out []action
	for _, a := range m.actionCatalog() {
		if a.writeOnly && !m.writable() {
			continue
		}
		if a.avail != nil && !a.avail(m) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// dispatchActionKey runs the action bound to key if it is currently available. Reports
// whether the key matched an action (so the caller can fall through otherwise). A
// write key pressed in a read-only context matches but is a safe no-op + hint (FR-004).
func (m App) dispatchActionKey(key string) (tea.Model, tea.Cmd, bool) {
	for _, a := range m.actionCatalog() {
		if a.key != key {
			continue
		}
		if a.avail != nil && !a.avail(m) {
			return m, nil, true // matched but not applicable to this selection — inert
		}
		if a.writeOnly && !m.writable() {
			m.err = storage.ErrReadOnly
			return m, nil, true
		}
		mm, cmd := a.invoke(m)
		return mm, cmd, true
	}
	return m, nil, false
}

// actionLabel renders a label, substituting the bulk variant + count when marks are set.
func (m App) actionLabel(a action) string {
	if m.hasMarks() && (a.key == "d" || a.key == "x" || a.key == "y") {
		return fmt.Sprintf("%s %d", a.label, m.selCount())
	}
	return a.label
}

// hintBarView renders the always-visible contextual hint line: navigation cues, the
// valid direct actions for the current selection/capability, then the global cues
// (006 US1, FR-003). It is width-fit segment-by-segment, dropping the lowest-priority
// middle items first while keeping the global help/quit cues (FR-040).
func (m App) hintBarView(w int) string {
	seg := func(key, label string) string {
		return hintKeyStyle.Render(key) + " " + hintLabelStyle.Render(label)
	}
	// Leading navigation cues.
	parts := []string{seg("↵", "open")}
	if m.mode == modeBuckets {
		parts = append(parts, seg("/", "filter"))
	} else {
		parts = append(parts, seg("/", "search"))
	}
	// Direct actions for the current selection/capability (the heart of US1).
	for _, a := range m.availableActions() {
		parts = append(parts, seg(a.key, m.actionLabel(a)))
	}
	if len(m.contexts) > 1 {
		parts = append(parts, seg("c", "context"))
	}
	// Trailing global cues — these survive every width drop.
	globals := []string{seg("?", "help"), seg("q", "quit")}

	sep := hintLabelStyle.Render(" · ")
	join := func(mid []string) string { return strings.Join(append(append([]string{}, mid...), globals...), sep) }
	for len(parts) > 0 && lipgloss.Width(join(parts)) > w {
		parts = parts[:len(parts)-1] // drop the lowest-priority (trailing) middle cue
	}
	return join(parts)
}

// refreshBuckets reloads the bucket list (the `r` action in the bucket list).
func (m App) refreshBuckets() (tea.Model, tea.Cmd) {
	ctx := (&m).beginLoad()
	return m, tea.Batch(loadBuckets(ctx, m.activeStore(), m.gen), spinnerTick())
}
