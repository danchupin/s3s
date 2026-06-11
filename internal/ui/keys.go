package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// keyMap maps logical actions to the key strings produced by KeyPressMsg.String().
// Several keys may bind to one action. Matching is done via matches().
type keyMap struct {
	Up          []string
	Down        []string
	Top         []string
	Bottom      []string
	Enter       []string // drill in / open object
	Back        []string // up a level
	Search      []string
	Refresh     []string
	Context     []string
	MoreDetail  []string // 016: context-aware "more detail" — usage breakdown / tags / bucket config (was Analyze)
	FullScan    []string // 017: explicit UNCAPPED usage scan — the only unbounded path
	Download    []string // download the selected object / marked set — read
	NewFolder   []string // create an empty folder (write mode)
	Delete      []string // delete the selected object (write mode)
	Upload      []string // upload a local file into the current level (write mode)
	Copy        []string // copy the selected object to a new key (write mode)
	Move        []string // move/rename the selected object (write mode)
	DeleteAll   []string // recursively delete the selected folder/prefix (write mode)
	DeleteChord []string // dangerous-action chord: object/group/recursive/bucket/connection delete
	MoveChord   []string // dangerous-action chord: move/rename (ctrl+m is Enter, so ctrl+o)
	WriteToggle []string // arm/disarm write at runtime
	Mark        []string // mark/unmark an object for multi-select
	Sort        []string // cycle the sort column
	SortDir     []string // toggle the sort direction
	Command     []string // open the `:` command bar
	Reveal      []string // reveal/inspect the full identifier of the selection
	Tab         []string // toggle focus between zones in the two-pane browse
	Quit        []string
	Help        []string
}

// defaultKeys is the keybinding contract (contracts/tui-contract.md).
func defaultKeys() keyMap {
	return keyMap{
		Up:          []string{"up", "k"},
		Down:        []string{"down", "j"},
		Top:         []string{"g", "home"},
		Bottom:      []string{"G", "end"},
		Enter:       []string{"enter", "right", "l"},
		Back:        []string{"esc", "left", "h"},
		Search:      []string{"/"},
		Refresh:     []string{"r"},
		Context:     []string{"c"},
		MoreDetail:  []string{"a"}, // "more detail": usage breakdown / tags / bucket config (was analyze)
		FullScan:    []string{"A"}, // shift-variant of "more detail": the explicit full usage scan (017)
		Download:    []string{"d"}, // download — read, works read-only
		NewFolder:   []string{"+"},
		Delete:      []string{"x"}, // k9s-style delete; frees "d" for download
		Upload:      []string{"u"},
		Copy:        []string{"y"}, // "yank"; "c" is taken by context switch
		Move:        []string{"m"},
		DeleteAll:   []string{"X"},          // recursive delete — matches the "x" family
		DeleteChord: []string{"ctrl+x"},     // dangerous delete chord
		MoveChord:   []string{"ctrl+o"},     // move chord — ctrl+m is Enter (reserved), so ctrl+o
		WriteToggle: []string{"w"},          // arm/disarm write at runtime
		Mark:        []string{" ", "space"}, // multi-select
		Sort:        []string{"s"},          // cycle sort column
		SortDir:     []string{"S"},          // toggle sort direction
		Command:     []string{":"},          // open the `:` command bar
		Reveal:      []string{"i"},          // inspect/reveal full identifier
		Tab:         []string{"tab"},        // focus toggle between zones
		Quit:        []string{"ctrl+c", "q"},
		Help:        []string{"?"},
	}
}

// matches reports whether key (a KeyPressMsg.String()) is bound to any of binds.
func matches(key string, binds []string) bool {
	for _, b := range binds {
		if key == b {
			return true
		}
	}
	return false
}

// keyGlyph maps raw binding strings to their display glyph; vim/letter keys pass
// through unchanged (help is the only surface that advertises the vim aliases, FR-031).
var keyGlyph = map[string]string{
	"up": "↑", "down": "↓", "left": "←", "right": "→",
	"enter": "Enter", "esc": "Esc", "home": "Home", "end": "End", "ctrl+c": "Ctrl+C",
	"ctrl+x": "Ctrl+X", "ctrl+o": "Ctrl+O", "tab": "Tab", " ": "Space",
}

func glyph(k string) string {
	if g, ok := keyGlyph[k]; ok {
		return g
	}
	return k
}

// firstBind returns the first binding of an action, or "" when none is set.
func firstBind(binds []string) string {
	if len(binds) == 0 {
		return ""
	}
	return binds[0]
}

// keyHint renders "glyph label" for a single-key action hint sourced from the keymap, so a
// rebind propagates to every on-screen hint — no hardcoded key literals.
func keyHint(binds []string, label string) string {
	return glyph(firstBind(binds)) + " " + label
}

// formatKeys renders all aliases of an action as "↑/k", "→/l/Enter", etc..
func formatKeys(binds []string) string {
	parts := make([]string, len(binds))
	for i, b := range binds {
		parts[i] = glyph(b)
	}
	return strings.Join(parts, "/")
}

// helpView renders the styled, categorized help overlay (m.helpLines is already styled).
func (m App) helpView() string {
	return strings.Join(m.helpLines(), "\n")
}

// helpLines is the content of the categorized help surface. Sections:
// Navigation / Search & View / Actions (single-key, no menu) / Context / Global /
// Connection. The key column is derived from defaultKeys() so help can never drift.
func (m App) helpLines() []string {
	k := m.keys
	sec := func(s string) string { return titleStyle.Render(s) }
	row := func(keys, desc string) string {
		return "  " + keyStyle.Render(pad(keys, 17)) + dimCellStyle.Render(desc)
	}
	conn := func(label, val string, st lipgloss.Style) string {
		if val == "" {
			val = "—"
		}
		return "  " + dimCellStyle.Render(pad(label, 14)) + st.Render(val)
	}

	// Write-capability tag for the write actions.
	wtag := dimCellStyle.Render("  (write)")
	if !m.writable() {
		wtag = warnStyle.Render("  (needs --write)")
	}

	lines := []string{
		titleStyle.Render("s3s — keyboard-driven S3 browser (read-only by default; --write to mutate)"),
		"",
		sec("Navigation"),
		row(formatKeys(k.Up)+", "+formatKeys(k.Down), "move selection"),
		row(formatKeys(k.Enter), "enter bucket/dir, or open an object"),
		row(formatKeys(k.Back), "back to parent (Esc also cancels an in-flight load)"),
		row(formatKeys(k.Top)+", "+formatKeys(k.Bottom), "jump to top / bottom"),
		"",
		sec("Search & View"),
		row(formatKeys(k.Search), "filter buckets / search a level (prefix)"),
		row(formatKeys(k.Mark), "mark/unmark an object (multi-select → "+glyph(firstBind(k.Download))+"/"+glyph(firstBind(k.Delete))+"/"+glyph(firstBind(k.Copy))+" act on the set)"),
		row(formatKeys(k.Sort)+", "+formatKeys(k.SortDir), "cycle sort column · toggle direction"),
		"",
		sec("Actions") + dimCellStyle.Render("  (single key on the selection — no menu)"),
		row(formatKeys(k.Download), "download the selected object / marked set"),
		row(formatKeys(k.MoreDetail), "more detail: usage totals/breakdown · tags · bucket config"),
		row(formatKeys(k.FullScan), "full usage scan (uncapped) of the focused bucket/folder"),
		row(formatKeys(k.Refresh), "reload the current list"),
		row(formatKeys(k.NewFolder), "create a folder") + wtag,
		row(formatKeys(k.Upload), "upload a local file") + wtag,
		row(formatKeys(k.Copy), "copy the selected object / marked set") + wtag,
		"",
		sec("Dangerous") + dimCellStyle.Render("  (Ctrl chord required — a bare key won't fire)"),
		row(glyph(firstBind(k.DeleteChord)), "delete the selected object / marked set") + wtag,
		row(glyph(firstBind(k.DeleteChord)), "recursively delete a folder · delete a bucket") + wtag,
		row(glyph(firstBind(k.MoveChord)), "move/rename the selected object") + wtag,
		"",
		sec("Context"),
		row(formatKeys(k.Context), "connections: switch · add · delete"),
		row("1-9", "switch to context by number"),
		row(glyph(firstBind(k.DeleteChord))+" (on contexts)", "delete the selected connection"),
		dimCellStyle.Render("  changed in 011: the standalone n add-connection key was removed — c opens the connections manager"),
		"",
		sec("Global"),
		row(formatKeys(k.WriteToggle), "arm/disarm write (confirm to arm; instant to disarm)"),
		row(formatKeys(k.Help), "toggle this help"),
		row(formatKeys(k.Quit), "quit"),
		"",
		sec("Connection"),
		conn("context", m.ctxName, segCtxStyle),
		conn("cluster", m.info.Cluster, segClusterStyle),
		conn("endpoint", m.info.Endpoint, segEndpointStyle),
		conn("region", m.info.Region, segRegionStyle),
		conn("user", m.info.User, segUserStyle),
		conn("s3s ver", Version, dimCellStyle),
		"",
		dimCellStyle.Render("  press any key to close help"),
	}
	return lines
}
