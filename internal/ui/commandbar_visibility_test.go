package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// finding: every advertised-in-help key must also be discoverable in the
// always-visible command bar — nobody reads docs to find `Y`/`H`/`i`/`:` (constitution
// VI/VII: the capability map is part of the UI, not the manual).

// TestCommandBarAdvertisesReadActions: bucket list and level view both surface the full
// read-action set: detail, full scan, health, share, reveal, refresh + the `:` bar.
func TestCommandBarAdvertisesReadActions(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x.txt", storage.FakeObject{Data: []byte("y")})

	bucketList := withBuckets(f, []string{"ctx"}, nil)
	bucketList = deliver(bucketList, tea.WindowSizeMsg{Width: 160, Height: 45})
	v := stripANSI(viewOf(bucketList))
	for _, want := range []string{"a  detail", "A  full scan", "H  health", "Y  share", "i  reveal", "r  refresh", ": cmds"} {
		if !strings.Contains(v, want) {
			t.Errorf("bucket-list command bar missing %q:\n%s", want, v)
		}
	}

	level := treeApp(f, false)
	level = deliver(level, tea.WindowSizeMsg{Width: 160, Height: 45})
	lv := stripANSI(viewOf(level))
	for _, want := range []string{"d  download", "a  detail", "A  full scan", "H  health", "Y  share", "i  reveal", "Space  mark"} {
		if !strings.Contains(lv, want) {
			t.Errorf("level command bar missing %q:\n%s", want, lv)
		}
	}
}

// TestObjectViewAdvertisesToggle: the JSON preview names its pretty↔raw toggle inline.
func TestObjectViewAdvertisesToggle(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "d.json", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	m.mode = modeObject
	md := storage.ObjectMetadata{Key: "d.json"}
	m.meta = &md
	pl := preview.Build("d.json", "application/json", []byte(`{"a":1}`), false)
	m.prev = &pl
	if v := stripANSI(viewOf(m)); !strings.Contains(v, "p raw") {
		t.Errorf("object view must advertise the raw toggle:\n%s", v)
	}
}
