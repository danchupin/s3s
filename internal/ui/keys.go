package ui

// keyMap maps logical actions to the key strings produced by KeyPressMsg.String().
// Several keys may bind to one action. Matching is done via matches().
type keyMap struct {
	Up      []string
	Down    []string
	Top     []string
	Bottom  []string
	Enter   []string // drill in / open object
	Back    []string // up a level
	Search  []string
	Refresh []string
	Context []string
	Cancel  []string // cancel in-flight load
	Quit    []string
	Help    []string
}

// defaultKeys is the keybinding contract (contracts/tui-contract.md).
func defaultKeys() keyMap {
	return keyMap{
		Up:      []string{"up", "k"},
		Down:    []string{"down", "j"},
		Top:     []string{"g", "home"},
		Bottom:  []string{"G", "end"},
		Enter:   []string{"enter", "right", "l"},
		Back:    []string{"esc", "left", "h"},
		Search:  []string{"/"},
		Refresh: []string{"r"},
		Context: []string{"c"},
		Cancel:  []string{"x"},
		Quit:    []string{"ctrl+c", "q"},
		Help:    []string{"?"},
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

// helpView renders the styled help overlay.
func (m App) helpView() string {
	lines := helpLines()
	out := titleStyle.Render(lines[0])
	for _, l := range lines[1:] {
		out += "\n" + dimCellStyle.Render(l)
	}
	return out
}

// helpLines is the content of the help overlay (FR / tui-contract).
func helpLines() []string {
	return []string{
		"s3s — read-only S3 browser",
		"",
		"  ↑/k, ↓/j      move selection",
		"  →/l/Enter     enter bucket/dir, or open an object (metadata + content)",
		"  ←/h/Esc       back to parent",
		"  g / G         jump to top / bottom",
		"  /             filter buckets / search a level (prefix)",
		"  r             refresh current level",
		"  c             switch context",
		"  1-9           switch to context by number",
		"  x             cancel in-flight load",
		"  ?             toggle this help",
		"  q / Ctrl+C    quit",
		"",
		"  press any key to close help",
	}
}
