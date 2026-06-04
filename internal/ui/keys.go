package ui

// keyMap maps logical actions to the key strings produced by KeyPressMsg.String().
// Several keys may bind to one action. Matching is done via matches().
type keyMap struct {
	Up       []string
	Down     []string
	Top      []string
	Bottom   []string
	Enter    []string // drill in / open
	Back     []string // up a level
	Search   []string
	Metadata []string
	Preview  []string
	Refresh  []string
	Context  []string
	Cancel   []string // cancel in-flight load
	Quit     []string
	Help     []string
}

// defaultKeys is the keybinding contract (contracts/tui-contract.md).
func defaultKeys() keyMap {
	return keyMap{
		Up:       []string{"up", "k"},
		Down:     []string{"down", "j"},
		Top:      []string{"g", "home"},
		Bottom:   []string{"G", "end"},
		Enter:    []string{"enter", "right", "l"},
		Back:     []string{"esc", "left", "h"},
		Search:   []string{"/"},
		Metadata: []string{"i"},
		Preview:  []string{"p", "space", " "},
		Refresh:  []string{"r"},
		Context:  []string{"c"},
		Cancel:   []string{"x"},
		Quit:     []string{"ctrl+c", "q"},
		Help:     []string{"?"},
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

// helpLines is the content of the help overlay (FR / tui-contract).
func helpLines() []string {
	return []string{
		"s3s — read-only S3 browser",
		"",
		"  ↑/k, ↓/j      move selection",
		"  →/l/Enter     enter bucket or directory",
		"  ←/h/Esc       back to parent",
		"  g / G         jump to top / bottom",
		"  /             search current level (prefix)",
		"  i             object metadata",
		"  p / Space     preview object",
		"  r             refresh current level",
		"  c             switch context",
		"  x             cancel in-flight load",
		"  ?             toggle this help",
		"  q / Ctrl+C    quit",
		"",
		"  press any key to close help",
	}
}
