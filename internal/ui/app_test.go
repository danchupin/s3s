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

// newApp builds a test App with a fake store, the production-default usage budget, and
// no image protocol.
func newApp(f *storage.Fake, contexts []string, resolve Resolver) App {
	return New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x", UsageScanBudget: 20000},
		"ctx", contexts, resolve, nil, preview.ProtoNone)
}

func deliver(m App, msg tea.Msg) App {
	mm, _ := m.Update(msg)
	return mm.(App)
}

// finishSwitch completes an async context switch: it resolves the target (as the event
// loop would, off-thread) and delivers the contextResolvedMsg under the current gen.
func finishSwitch(m App, resolve Resolver, target string) App {
	be, err := resolve(target)
	return deliver(m, contextResolvedMsg{gen: m.gen, target: target, be: be, err: err})
}

func keyMsgFor(s string) tea.KeyPressMsg {
	switch s {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
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
	case "ctrl+x":
		return tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}
	case "ctrl+o":
		return tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}
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
	m = press(m, "enter")                 // initiate (resolve runs off the event loop)
	m = finishSwitch(m, resolve, "other") // deliver the async result

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

func TestStatusSearchPending(t *testing.T) { // S2 / FR-016 — 015: the input now lives in the form
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.searching = true
	m.searchInput = "ar"
	// 015 FR-005: the prominent filter input renders via the always-visible bordered form (no
	// longer statusLine); the form is labeled per scope and shows the live input.
	if s := stripANSI(m.objectFilterField(120)); !strings.Contains(s, "filter objects") || !strings.Contains(s, "ar") {
		t.Errorf("object filter input should render in the labeled form; got %q", s)
	}
}

// T006 / 015 FR-005: statusLine NEVER renders the filter input — it moved to the form. A status
// message (notice/error) and an active filter coexist: the input is in the form, the status in
// the footer's status line, neither clobbering the other.
func TestStatusLineNeverHasFilterInput(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.searching = true
	m.searchInput = "needle"
	m.notice = "SENTINEL_NOTICE"
	s := m.statusLine(120)
	if strings.Contains(s, "needle") {
		t.Errorf("statusLine must not render the filter input; got %q", s)
	}
	if !strings.Contains(s, "SENTINEL_NOTICE") {
		t.Errorf("statusLine should keep showing the notice while a filter is active; got %q", s)
	}
	// And the input IS in the form, side by side with the status.
	if !strings.Contains(m.objectFilterField(120), "needle") {
		t.Errorf("the live input must render in the form; got %q", m.objectFilterField(120))
	}
}

// 015 US1+US2: the filter forms are ALWAYS present in the filterable browse modes — even with no
// committed filter and not editing — as bordered, labeled boxes with a "/ to filter" placeholder.
// In the two-pane browse BOTH forms (buckets + objects) render at once.
func TestFilterFormsAlwaysVisible(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")

	mb := dualApp(f) // modeBuckets, two-pane
	if mb.searching || mb.bucketFilter != "" {
		t.Fatal("setup: the bucket scope should be idle with no committed filter")
	}
	v := stripANSI(viewOf(mb))
	if !strings.Contains(v, "filter buckets") {
		t.Errorf("the bucket filter form must be rendered:\n%s", v)
	}
	if !strings.Contains(v, "filter objects") {
		t.Errorf("the object filter form must ALSO be rendered (two panels always visible):\n%s", v)
	}
	if !strings.Contains(v, "/ to filter") {
		t.Errorf("an idle form must show the placeholder:\n%s", v)
	}

	mt := treeApp(f, false) // modeTree → object scope
	mt.width, mt.height = 120, 30
	if v := stripANSI(viewOf(mt)); !strings.Contains(v, "filter objects") || !strings.Contains(v, "/ to filter") {
		t.Errorf("the object filter form must be rendered in the tree view:\n%s", v)
	}
}

// 015 US2: non-filterable modes reserve NO form band — the cost is paid only where filtering
// applies (the bucket list and the object tree).
func TestNoFilterFormInNonFilterableModes(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")

	mo := treeApp(f, false)
	mo.width, mo.height = 100, 30
	mo.mode = modeObject
	md := storage.ObjectMetadata{Key: "a.txt", ContentType: "text/plain"}
	mo.meta = &md
	if v := stripANSI(viewOf(mo)); strings.Contains(v, "/ to filter") || strings.Contains(v, "filter objects") {
		t.Errorf("modeObject must not render a filter form:\n%s", v)
	}

	mc := dualApp(f)
	mc.mode = modeContextSwitch
	if v := stripANSI(viewOf(mc)); strings.Contains(v, "/ to filter") || strings.Contains(v, "filter buckets") {
		t.Errorf("modeContextSwitch must not render a filter form:\n%s", v)
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
	m = viaMenu(t, m, "delete") // 007 US4: binary tier → centered popup
	if !strings.Contains(m.confirmPopupView(120, 12), "secret.txt") {
		t.Errorf("binary confirm popup must keep the target visible; got %q", m.confirmPopupView(120, 12))
	}
}
