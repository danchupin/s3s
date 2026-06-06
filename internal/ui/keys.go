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
	Analyze     []string // du analytics on folder/level/bucket (006 US1) — read
	Download    []string // download the selected object / marked set (006 US1) — read
	NewFolder   []string // create an empty folder (write mode)
	Delete      []string // delete the selected object (write mode)
	Upload      []string // upload a local file into the current level (write mode)
	Copy        []string // copy the selected object to a new key (write mode)
	Move        []string // move/rename the selected object (write mode)
	DeleteAll   []string // recursively delete the selected folder/prefix (write mode)
	WriteToggle []string // arm/disarm write at runtime (005 US5)
	Mark        []string // mark/unmark an object for multi-select (005 US3)
	Sort        []string // cycle the sort column (005 US4)
	SortDir     []string // toggle the sort direction (005 US4)
	Command     []string // open the `:` command bar (006 US3)
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
		Analyze:     []string{"a"}, // analyze (du) — frees the old menu key (006 US1)
		Download:    []string{"d"}, // download — read, works read-only (006 US1)
		NewFolder:   []string{"+"},
		Delete:      []string{"x"}, // k9s-style delete; frees "d" for download (006 US1)
		Upload:      []string{"u"},
		Copy:        []string{"y"}, // "yank"; "c" is taken by context switch
		Move:        []string{"m"},
		DeleteAll:   []string{"X"},          // recursive delete — matches the "x" family (006 US1)
		WriteToggle: []string{"w"},          // arm/disarm write at runtime (005 US5)
		Mark:        []string{" ", "space"}, // multi-select (005 US3)
		Sort:        []string{"s"},          // cycle sort column (005 US4)
		SortDir:     []string{"S"},          // toggle sort direction (005 US4)
		Command:     []string{":"},          // open the `:` command bar (006 US3)
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
}

func glyph(k string) string {
	if g, ok := keyGlyph[k]; ok {
		return g
	}
	return k
}

// formatKeys renders all aliases of an action as "↑/k", "→/l/Enter", etc. (FR-014).
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

// helpLines is the content of the categorized help surface (FR-010..FR-014c). Sections:
// Navigation / Search & View / Actions (the `a` menu) / Context / Global / Connection.
// The key column is derived from defaultKeys() so help can never drift from bindings.
func (m App) helpLines() []string {
	k := m.keys
	sec := func(s string) string { return titleStyle.Render(s) }
	row := func(keys, desc string) string {
		return "  " + accentStyle.Render(pad(keys, 17)) + dimCellStyle.Render(desc)
	}
	conn := func(label, val string, st lipgloss.Style) string {
		if val == "" {
			val = "—"
		}
		return "  " + dimCellStyle.Render(pad(label, 14)) + st.Render(val)
	}

	// Write-capability tag for the Actions menu items (FR-013/H4).
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
		row(formatKeys(k.Mark), "mark/unmark an object (multi-select → d/x/y act on the set)"),
		row(formatKeys(k.Sort)+", "+formatKeys(k.SortDir), "cycle sort column · toggle direction"),
		"",
		sec("Actions") + dimCellStyle.Render("  (single key on the selection — no menu)"),
		row(formatKeys(k.Download), "download the selected object / marked set"),
		row(formatKeys(k.Analyze), "analyze (du) a bucket / folder / level"),
		row(formatKeys(k.Refresh), "reload the current list"),
		row(formatKeys(k.NewFolder), "create a folder") + wtag,
		row(formatKeys(k.Delete), "delete the selected object / marked set") + wtag,
		row(formatKeys(k.Upload), "upload a local file") + wtag,
		row(formatKeys(k.Copy), "copy the selected object / marked set") + wtag,
		row(formatKeys(k.Move), "move/rename the selected object") + wtag,
		row(formatKeys(k.DeleteAll), "recursively delete the selected folder") + wtag,
		"",
		sec("Context"),
		row(formatKeys(k.Context), "switch context"),
		row("1-9", "switch to context by number"),
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
