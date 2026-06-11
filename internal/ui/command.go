package ui

import (
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Command bar. `:` opens a one-line input (modeCommand) that jumps to a view
// or runs an action, mirroring k9s. The registry is the long-tail companion to the
// single-key direct actions: frequent actions are keys; everything is reachable
// (and discoverable) here. `:` and `/` are distinct modes and never coexist.

// command is one entry in the `:` registry.
type command struct {
	name    string
	aliases []string
	invoke  func(App) (tea.Model, tea.Cmd)
}

// commandRegistry returns the available commands.
func commandRegistry() []command {
	return []command{
		{name: "buckets", invoke: func(m App) (tea.Model, tea.Cmd) {
			m.mode = modeBuckets
			m.level = nil
			m.sel = nil
			return m, nil
		}},
		{name: "contexts", aliases: []string{"ctx"}, invoke: App.openContextSwitch},
		{name: "connect", aliases: []string{"conn"}, invoke: App.openConnections},
		{name: "detail", aliases: []string{"info", "du"}, invoke: App.startMoreDetail},
		{name: "scan", invoke: App.startFullScan},
		{name: "copy", aliases: []string{"share"}, invoke: App.openCopyMenu},
		{name: "health", aliases: []string{"card"}, invoke: App.openHealth},
		{name: "refresh", invoke: func(m App) (tea.Model, tea.Cmd) {
			if m.mode == modeBuckets {
				return m.refreshBuckets()
			}
			return m.refresh()
		}},
		{name: "help", invoke: func(m App) (tea.Model, tea.Cmd) {
			m.prevMode = m.mode
			m.mode = modeHelp
			return m, nil
		}},
		{name: "quit", aliases: []string{"q"}, invoke: func(m App) (tea.Model, tea.Cmd) {
			(&m).cancelLoad()
			return m, tea.Quit
		}},
	}
}

// canOpenCommand reports whether the `:` bar may open in the current mode (not while a
// search, op prompt, form, or another bar already owns input — those are handled before
// this point in onKey).
func (m App) canOpenCommand() bool {
	switch m.mode {
	case modeBuckets, modeTree, modeObject, modeContextSwitch, modeHealth:
		return true
	}
	return false
}

// startCommand opens the command bar.
func (m App) startCommand() (tea.Model, tea.Cmd) {
	m.prevMode = m.mode
	m.mode = modeCommand
	m.cmdInput = ""
	return m, nil
}

// onCommandKey edits/dispatches the command bar input. Esc cancels with no effect; Enter
// dispatches; every other printable key is text.
func (m App) onCommandKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.mode = m.prevMode
		m.cmdInput = ""
		return m, nil
	case "enter":
		return m.dispatchCommand()
	case "backspace":
		if n := len(m.cmdInput); n > 0 {
			m.cmdInput = m.cmdInput[:n-1]
		}
		return m, nil
	default:
		if msg.Text != "" {
			m.cmdInput += msg.Text
		}
		return m, nil
	}
}

// dispatchCommand resolves the typed input to a command (exact name/alias, else a unique
// prefix) and invokes it; an unknown/ambiguous command yields a non-destructive notice and
// takes no action.
func (m App) dispatchCommand() (tea.Model, tea.Cmd) {
	in := strings.TrimSpace(strings.ToLower(m.cmdInput))
	m.mode = m.prevMode
	m.cmdInput = ""
	if in == "" {
		return m, nil
	}
	reg := commandRegistry()
	// Exact name/alias.
	for _, c := range reg {
		if c.name == in {
			return c.invoke(m)
		}
		for _, a := range c.aliases {
			if a == in {
				return c.invoke(m)
			}
		}
	}
	// Unique prefix of a canonical name.
	var match *command
	for i := range reg {
		if strings.HasPrefix(reg[i].name, in) {
			if match != nil {
				match = nil // ambiguous
				break
			}
			match = &reg[i]
		}
	}
	if match != nil {
		return match.invoke(m)
	}
	m.notice = "unknown command: " + in
	return m, nil
}

// commandCandidates returns the canonical names matching the current input (for the
// discovery hint shown beside the bar).
func (m App) commandCandidates() []string {
	in := strings.TrimSpace(strings.ToLower(m.cmdInput))
	var out []string
	for _, c := range commandRegistry() {
		if in == "" || strings.HasPrefix(c.name, in) {
			out = append(out, c.name)
		}
	}
	sort.Strings(out)
	return out
}

// commandLine renders the command bar in the footer status line.
func (m App) commandLine(w int) string {
	cands := strings.Join(m.commandCandidates(), " ")
	suffix := dimCellStyle.Render("▏  " + truncate(cands, max(1, w-len(m.cmdInput)-12)))
	return accentStyle.Render(":") + objCellStyle.Render(m.cmdInput) + " " + suffix
}
