package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// wideTree builds a writable tree app on a wide terminal so the pane is shown.
func wideTree(f *storage.Fake) App {
	m := treeApp(f, true)
	m.width, m.height = 120, 30
	return m
}

// --- US2: split layout (FR-008/FR-013) ---

func TestPaneSplitWideShowsDetails(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := wideTree(f)
	selectObject(&m, "a.txt")
	v := viewOf(m)
	if !strings.Contains(v, "details") {
		t.Errorf("wide terminal should render a details pane box:\n%s", v)
	}
	if !strings.Contains(v, "a.txt") {
		t.Errorf("pane should show the selected key:\n%s", v)
	}
}

func TestPaneNarrowCollapses(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.width, m.height = 80, 24 // below paneSplitMin
	selectObject(&m, "a.txt")
	v := viewOf(m)
	if strings.Contains(v, "details") {
		t.Errorf("narrow terminal must collapse the pane (no details box):\n%s", v)
	}
	// Hint bar + footer must still be present and unclipped at 80×24.
	for _, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line exceeds width 80 (=%d): %q", w, line)
		}
	}
	if !strings.Contains(v, "quit") {
		t.Errorf("hint bar (quit) must stay visible at 80×24:\n%s", v)
	}
}

func TestPaneFolderShowsSummary(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := wideTree(f)
	selectDir(&m, "docs/")
	if p := m.paneView(40, 20); !strings.Contains(p, "Folder") || !strings.Contains(p, "analyze") {
		t.Errorf("folder pane should show a summary + analyze hint; got:\n%s", p)
	}
}

// --- US2: debounced load + supersede (FR-009/FR-012) ---

func TestPaneScheduleOnMove(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "c.txt")
	m := wideTree(f)
	m.treeSel = 0
	genBefore := m.paneGen
	m, cmd := pressCmd(m, "down") // move onto an object
	if cmd == nil {
		t.Fatal("moving the selection should schedule a debounced pane load (non-nil cmd)")
	}
	if m.paneGen != genBefore+1 {
		t.Fatalf("pane generation should bump on move: %d -> %d", genBefore, m.paneGen)
	}
	if e := m.selected(); e != nil && m.paneSelKey != e.full {
		t.Errorf("paneSelKey = %q, want %q", m.paneSelKey, e.full)
	}
}

func TestPaneStaleMetaDropped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := wideTree(f)
	selectObject(&m, "a.txt")
	m.paneGen = 5
	m.paneSelKey = "a.txt"

	// Stale gen → dropped.
	m = deliver(m, paneMetaMsg{gen: 4, key: "a.txt", md: storage.ObjectMetadata{ContentType: "text/plain"}})
	if m.paneMeta != nil {
		t.Error("a paneMetaMsg with a stale gen must be dropped")
	}
	// Current gen + key → applied.
	m = deliver(m, paneMetaMsg{gen: 5, key: "a.txt", md: storage.ObjectMetadata{ContentType: "text/plain"}})
	if m.paneMeta == nil || m.paneMeta.ContentType != "text/plain" {
		t.Errorf("current paneMetaMsg should fill the pane; got %+v", m.paneMeta)
	}
	if p := m.paneView(40, 20); !strings.Contains(p, "Type") {
		t.Errorf("pane should show the debounced Type once meta arrives:\n%s", p)
	}
}

func TestPaneTickStaleDropped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := wideTree(f)
	selectObject(&m, "a.txt")
	m.paneGen = 7
	m.paneSelKey = "a.txt"
	// A tick for an old generation (scrolled-past row) must fire no load.
	_, cmd := m.onPaneTick(paneTickMsg{gen: 6, key: "a.txt"})
	if cmd != nil {
		t.Error("a stale pane tick must not trigger a load")
	}
	// The current tick on an object fires the load.
	_, cmd = m.onPaneTick(paneTickMsg{gen: 7, key: "a.txt"})
	if cmd == nil {
		t.Error("the current pane tick on an object should trigger a load")
	}
}

func TestPanePreviewFills(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := wideTree(f)
	selectObject(&m, "a.txt")
	m.paneGen = 3
	m.paneSelKey = "a.txt"
	pl := preview.Build("a.txt", "text/plain", []byte("hello world"), false)
	m = deliver(m, panePreviewMsg{gen: 3, key: "a.txt", payload: pl})
	if m.panePrev == nil {
		t.Fatal("panePreviewMsg should populate the pane preview")
	}
	if p := m.paneView(40, 20); !strings.Contains(p, "Preview") {
		t.Errorf("pane should show a Preview section once the payload arrives:\n%s", p)
	}
}

// guard against an accidental modeObject flip from pane messages.
func TestPaneMessagesDoNotFlipMode(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := wideTree(f)
	selectObject(&m, "a.txt")
	m.paneGen = 1
	m.paneSelKey = "a.txt"
	var model tea.Model = m
	model, _ = model.Update(paneMetaMsg{gen: 1, key: "a.txt", md: storage.ObjectMetadata{}})
	if model.(App).mode != modeTree {
		t.Errorf("paneMetaMsg must not change mode; got %v", model.(App).mode)
	}
}
