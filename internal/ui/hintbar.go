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

// action is one invokable item: its keymap binds (single source of truth — the same
// fields help renders, so dispatch and help can never drift), a label, a write-gate, a
// bulk flag, an availability predicate, and the entry point it dispatches to.
type action struct {
	binds     []string // keymap aliases; binds[0] is the displayed key
	label     string
	writeOnly bool
	bulk      bool // routes to the bulk variant + shows a count when a multi-select is active
	// avail receives the precomputed selection kind so each predicate avoids re-deriving it
	// (selKind→selected→treeEntries re-sorts the level; recomputing per action is wasteful).
	avail  func(App, selKind) bool
	invoke func(App) (tea.Model, tea.Cmd)
}

// hasMarks reports an active multi-select.
func (m App) hasMarks() bool { return m.selCount() > 0 }

// actionCatalog returns the ordered actions for the current list mode. Keys come from
// m.keys.* (the same source help uses), so a rebind updates dispatch, the hint bar, and
// help together. Bulk variants take over download/delete/copy when a multi-select is
// active (FR-006); their invoke picks the bulk vs single entry point at dispatch time.
func (m App) actionCatalog() []action {
	k := m.keys
	always := func(App, selKind) bool { return true }
	if m.mode == modeBuckets {
		return []action{
			{binds: k.Analyze, label: "analyze", avail: func(a App, _ selKind) bool { return len(a.filteredBuckets()) > 0 }, invoke: App.startAnalyze},
			{binds: k.Refresh, label: "refresh", avail: always, invoke: App.refreshBuckets},
		}
	}
	objOrMarks := func(a App, kind selKind) bool { return kind == selObject || a.hasMarks() }
	return []action{
		{binds: k.Download, label: "download", bulk: true, avail: objOrMarks, invoke: func(a App) (tea.Model, tea.Cmd) {
			if a.hasMarks() {
				return a.startBulkDownload()
			}
			return a.startDownload()
		}},
		{binds: k.Analyze, label: "analyze", avail: func(_ App, kind selKind) bool { return kind != selObject }, invoke: App.startAnalyze},
		{binds: k.Delete, label: "delete", writeOnly: true, bulk: true, avail: objOrMarks, invoke: func(a App) (tea.Model, tea.Cmd) {
			if a.hasMarks() {
				return a.startBulkDelete()
			}
			return a.startRemoveObject()
		}},
		{binds: k.DeleteAll, label: "rm -r", writeOnly: true, avail: func(_ App, kind selKind) bool { return kind == selFolder }, invoke: App.startRecursiveDelete},
		{binds: k.Copy, label: "copy", writeOnly: true, bulk: true, avail: objOrMarks, invoke: func(a App) (tea.Model, tea.Cmd) {
			if a.hasMarks() {
				return a.startBulkCopy()
			}
			return a.startCopy()
		}},
		{binds: k.Move, label: "move", writeOnly: true, avail: func(_ App, kind selKind) bool { return kind == selObject }, invoke: App.startMove},
		{binds: k.Upload, label: "upload", writeOnly: true, avail: always, invoke: App.startUpload},
		{binds: k.NewFolder, label: "mkdir", writeOnly: true, avail: always, invoke: App.startCreateFolder},
		{binds: k.Refresh, label: "refresh", avail: always, invoke: App.refresh},
	}
}

// availableActions filters the catalog to the actions valid for the current selection
// and capability: an unavailable action is dropped; a write action is dropped when the
// context is not writable (FR-003/FR-004).
func (m App) availableActions() []action {
	kind := m.selKind() // once — predicates take it instead of re-deriving (re-sorting the level)
	var out []action
	for _, a := range m.actionCatalog() {
		if a.writeOnly && !m.writable() {
			continue
		}
		if a.avail != nil && !a.avail(m, kind) {
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
	kind := m.selKind()
	for _, a := range m.actionCatalog() {
		if !matches(key, a.binds) {
			continue
		}
		if a.avail != nil && !a.avail(m, kind) {
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
	if m.hasMarks() && a.bulk {
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
		parts = append(parts, seg(glyph(a.binds[0]), m.actionLabel(a)))
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
