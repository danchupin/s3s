package ui

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// T032: `n` no longer opens the connections manager/form; `c` opens the manager when a
// connector is wired (011 US4 / FR-020 — the standalone add-connection key was removed).
func TestNKeyNoLongerOpensConnections(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m = press(m, "n")
	if m.mode == modeConnections || m.mode == modeConnForm {
		t.Errorf("n must no longer open the connections manager/form, got mode %d", m.mode)
	}
	m = press(m, "c")
	if m.mode != modeConnections {
		t.Errorf("c must open the connections manager (connector wired), got mode %d", m.mode)
	}
}

// T034: with color stripped (NO_COLOR), each advertised key remains distinguishable from its
// label via its leading-token position (011 FR-024). Bold is the color-on cue; position is the
// redundant non-color cue.
func TestKeysDistinguishableNoColor(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "x")
	m := withBuckets(f, nil, nil)
	m.width, m.height = 120, 30
	plain := stripANSI(viewOf(m))
	if !strings.Contains(plain, "q quit") {
		t.Errorf("with color stripped, the key must still precede its label (q quit):\n%s", plain)
	}
}

// T035: navigation and locked global keys are unchanged from the pre-011 keymap (011 FR-022).
func TestKeymapNavLockedUnchanged(t *testing.T) {
	k := defaultKeys()
	want := map[string][]string{
		"Up": {"up", "k"}, "Down": {"down", "j"}, "Top": {"g", "home"}, "Bottom": {"G", "end"},
		"Enter": {"enter", "right", "l"}, "Back": {"esc", "left", "h"},
		"Quit": {"ctrl+c", "q"}, "Help": {"?"}, "Command": {":"}, "Search": {"/"},
		"Mark": {" ", "space"}, "Context": {"c"},
	}
	got := map[string][]string{
		"Up": k.Up, "Down": k.Down, "Top": k.Top, "Bottom": k.Bottom,
		"Enter": k.Enter, "Back": k.Back, "Quit": k.Quit, "Help": k.Help,
		"Command": k.Command, "Search": k.Search, "Mark": k.Mark, "Context": k.Context,
	}
	for name, w := range want {
		if !reflect.DeepEqual(got[name], w) {
			t.Errorf("keymap %s changed: got %v want %v", name, got[name], w)
		}
	}
}
