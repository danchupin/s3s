package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// --- test helpers ---

// newApp builds a test App with a fake store and no image protocol.
func newApp(f *storage.Fake, contexts []string, resolve Resolver) App {
	return New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x"},
		"ctx", contexts, resolve, preview.ProtoNone)
}

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
func withBuckets(f *storage.Fake, contexts []string, resolve Resolver) App {
	m := newApp(f, contexts, resolve)
	bs, _ := f.ListBuckets(context.Background())
	return deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
}

func viewOf(m App) string { return m.View().Content }

// --- tests ---

func TestInitialLoadingState(t *testing.T) {
	f := storage.NewFake()
	m := newApp(f, []string{"ctx"}, nil)
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
	if !strings.Contains(v, "▶ alpha") {
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
	if !strings.Contains(viewOf(m), "▶ beta") {
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
	m := newApp(f, []string{"ctx"}, nil)
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
	resolve := func(name string) (Backend, error) {
		if name == "other" {
			switched = true
			return Backend{Store: other, Cluster: "o", User: "u"}, nil
		}
		return Backend{Store: f}, nil
	}

	m := withBuckets(f, []string{"ctx", "other"}, resolve)

	m = press(m, "c")
	if m.mode != modeContextSwitch {
		t.Fatalf("'c' should open context switcher, mode = %v", m.mode)
	}
	if !strings.Contains(viewOf(m), "contexts") || !strings.Contains(viewOf(m), "other") {
		t.Errorf("context switcher view missing:\n%s", viewOf(m))
	}

	m = press(m, "down") // select "other"
	if m.ctxSel != 1 {
		t.Fatalf("ctxSel = %d, want 1", m.ctxSel)
	}
	m = press(m, "enter") // apply

	if !switched {
		t.Error("resolve was not called for 'other'")
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
	if !strings.Contains(viewOf(m), "S3 browser") {
		t.Errorf("help overlay content missing:\n%s", viewOf(m))
	}
	m = press(m, "x") // any key closes
	if m.mode != modeBuckets {
		t.Errorf("any key should close help, mode = %v", m.mode)
	}
}

func TestQuit(t *testing.T) {
	f := storage.NewFake()
	m := newApp(f, []string{"ctx"}, nil)
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

// --- US4: status feedback (S1–S5) ---

func TestStatusNamedLoading(t *testing.T) { // S1 / FR-015, FR-029
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	cases := map[mode]string{modeBuckets: "buckets", modeTree: "contents", modeObject: "object"}
	for md, want := range cases {
		m := treeApp(f, true)
		m.mode = md
		m.loading = true
		s := m.statusLine(120)
		if !strings.Contains(s, want) {
			t.Errorf("mode %v: loading status %q should name %q", md, s, want)
		}
		if !strings.Contains(s, "Esc to cancel") {
			t.Errorf("mode %v: loading status should say 'Esc to cancel'; got %q", md, s)
		}
	}
}

func TestStatusSearchPending(t *testing.T) { // S2 / FR-016
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.searching = true
	m.searchInput = "ar"
	if s := m.statusLine(120); !strings.Contains(s, "searching") {
		t.Errorf("tree search input should show a pending indicator; got %q", s)
	}
}

func TestStatusNoticeVsErrorHue(t *testing.T) { // S4 / FR-018
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.notice = "recursive delete: 3 removed"
	notice := m.statusLine(120)
	if !strings.Contains(notice, "108") { // colOK green
		t.Errorf("notice should use the green success hue (108); got %q", notice)
	}

	m2 := treeApp(f, true)
	m2.err = storage.ErrNotFound
	errLine := m2.statusLine(120)
	if !strings.Contains(errLine, "174") { // colErr red
		t.Errorf("error should use the red hue (174); got %q", errLine)
	}
	if notice == errLine {
		t.Error("notice and error must be visually distinct")
	}
}

func TestStatusPrecedence(t *testing.T) { // S5 / FR-018a
	f := storage.NewFake()
	f.Seed("b", "a.txt")

	// loading + notice → loading wins (notice suppressed).
	m := treeApp(f, true)
	m.loading = true
	m.notice = "SENTINEL_NOTICE"
	if s := m.statusLine(120); !strings.Contains(s, "loading") || strings.Contains(s, "SENTINEL_NOTICE") {
		t.Errorf("loading must outrank a notice; got %q", s)
	}

	// op prompt + load → op prompt wins (no loading line).
	m2 := treeApp(f, true)
	selectObject(&m2, "a.txt")
	m2.op = &operation{kind: "delete_object", phase: phaseConfirm, tier: confirmTyped, expect: "a.txt", target: "a.txt"}
	m2.loading = true
	if s := m2.statusLine(120); strings.Contains(s, "loading buckets") || strings.Contains(s, "loading contents") {
		t.Errorf("an op prompt must outrank loading; got %q", s)
	}
}

func TestTypedConfirmShowsTarget(t *testing.T) { // S3 / FR-017
	f := storage.NewFake()
	f.Seed("b", "secret.txt")
	m := treeApp(f, true)
	selectObject(&m, "secret.txt")
	m = viaMenu(t, m, "delete")
	if !strings.Contains(m.opPromptLine(120), "secret.txt") {
		t.Errorf("typed-confirm prompt must keep the required target visible; got %q", m.opPromptLine(120))
	}
}
