package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// --- test helpers ---

func deliver(m App, msg tea.Msg) App {
	mm, _ := m.Update(msg)
	return mm.(App)
}

func keyMsgFor(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

func press(m App, s string) App {
	return deliver(m, keyMsgFor(s))
}

func pressCmd(m App, s string) (App, tea.Cmd) {
	mm, cmd := m.Update(keyMsgFor(s))
	return mm.(App), cmd
}

// withBuckets builds an App and delivers the initial bucket list for the fake.
func withBuckets(f *storage.Fake, contexts []string, switchFn ContextSwitcher) App {
	m := New(f, "ctx", contexts, switchFn)
	bs, _ := f.ListBuckets(context.Background())
	return deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
}

func viewOf(m App) string { return m.View().Content }

// --- tests ---

func TestInitialLoadingState(t *testing.T) {
	f := storage.NewFake()
	m := New(f, "ctx", []string{"ctx"}, nil)
	if !m.loading {
		t.Fatal("New should arm an initial load (loading=true)")
	}
	if m.gen != 1 {
		t.Fatalf("initial gen = %d, want 1", m.gen)
	}
	if !strings.Contains(viewOf(m), "loading") {
		t.Errorf("initial view should show loading; got:\n%s", viewOf(m))
	}
}

func TestBucketsRender(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	m := withBuckets(f, []string{"ctx"}, nil)

	if m.loading {
		t.Error("loading should clear after buckets arrive")
	}
	v := viewOf(m)
	if !strings.Contains(v, "alpha") || !strings.Contains(v, "beta") {
		t.Errorf("bucket names missing from view:\n%s", v)
	}
	if !strings.Contains(v, "> alpha") {
		t.Errorf("cursor should mark first bucket:\n%s", v)
	}
}

func TestBucketsNavigation(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	m := withBuckets(f, []string{"ctx"}, nil)

	m = press(m, "down")
	if m.bucketSel != 1 {
		t.Fatalf("after down, bucketSel = %d, want 1", m.bucketSel)
	}
	if !strings.Contains(viewOf(m), "> beta") {
		t.Errorf("cursor should be on beta:\n%s", viewOf(m))
	}
	m = press(m, "up")
	if m.bucketSel != 0 {
		t.Fatalf("after up, bucketSel = %d, want 0", m.bucketSel)
	}
}

func TestStaleBucketsDropped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	m := withBuckets(f, []string{"ctx"}, nil)
	before := len(m.buckets)

	m = deliver(m, bucketsMsg{gen: 999, buckets: []storage.Bucket{{Name: "ghost"}}})
	if len(m.buckets) != before {
		t.Errorf("stale bucketsMsg should be dropped; buckets changed to %+v", m.buckets)
	}
}

func TestErrorState(t *testing.T) {
	f := storage.NewFake()
	m := New(f, "ctx", []string{"ctx"}, nil)
	m = deliver(m, errMsg{gen: m.gen, err: storage.ErrAccessDenied})

	if m.loading {
		t.Error("loading should clear on error")
	}
	if !strings.Contains(m.errorText(), "Access denied") {
		t.Errorf("errorText = %q, want access-denied copy", m.errorText())
	}
	if !strings.Contains(viewOf(m), "error:") {
		t.Errorf("view footer should show error:\n%s", viewOf(m))
	}
}

func TestContextSwitchChangesContext(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	other := storage.NewFake()
	other.Seed("gamma")

	switched := false
	switchFn := func(name string) (storage.Storage, error) {
		if name == "other" {
			switched = true
			return other, nil
		}
		return f, nil
	}

	m := withBuckets(f, []string{"ctx", "other"}, switchFn)

	m = press(m, "c")
	if m.mode != modeContextSwitch {
		t.Fatalf("'c' should open context switcher, mode = %v", m.mode)
	}
	if !strings.Contains(viewOf(m), "Switch context") {
		t.Errorf("context switcher view missing:\n%s", viewOf(m))
	}

	m = press(m, "down") // select "other"
	if m.ctxSel != 1 {
		t.Fatalf("ctxSel = %d, want 1", m.ctxSel)
	}
	m = press(m, "enter") // apply

	if !switched {
		t.Error("switchFn was not called for 'other'")
	}
	if m.ctxName != "other" {
		t.Errorf("active context = %q, want other", m.ctxName)
	}
	if m.mode != modeBuckets {
		t.Errorf("after switch, mode = %v, want buckets", m.mode)
	}
	if !m.loading {
		t.Error("switching contexts should arm a fresh bucket load")
	}
}

func TestHelpOverlay(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	m := withBuckets(f, []string{"ctx"}, nil)

	m = press(m, "?")
	if m.mode != modeHelp {
		t.Fatalf("'?' should open help, mode = %v", m.mode)
	}
	if !strings.Contains(viewOf(m), "read-only S3 browser") {
		t.Errorf("help overlay content missing:\n%s", viewOf(m))
	}
	m = press(m, "x") // any key closes
	if m.mode != modeBuckets {
		t.Errorf("any key should close help, mode = %v", m.mode)
	}
}

func TestQuit(t *testing.T) {
	f := storage.NewFake()
	m := New(f, "ctx", []string{"ctx"}, nil)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("'q' should return a command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("quit command produced no message")
	}
}

func TestResizeClampsSelection(t *testing.T) {
	f := storage.NewFake()
	f.Seed("a")
	f.Seed("b")
	f.Seed("c")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, "down")
	m = press(m, "down") // sel=2
	m = deliver(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	if m.width != 100 || m.height != 40 {
		t.Errorf("size not applied: %dx%d", m.width, m.height)
	}
	if m.bucketSel != 2 {
		t.Errorf("selection should be preserved across resize, got %d", m.bucketSel)
	}
}
