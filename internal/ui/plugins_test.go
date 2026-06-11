package ui

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/plugin"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// --- plugin test helpers ---

// fakeRunner scripts one Result per plugin name and counts invocations — the
// UI-side stand-in for the subprocess runner (mirrors storage.Fake).
type fakeRunner struct {
	mu      sync.Mutex
	results map[string]plugin.Result
	calls   map[string]int
}

func (r *fakeRunner) Invoke(_ context.Context, d plugin.Decl, _ plugin.Request) plugin.Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.calls[d.Name]++
	return r.results[d.Name]
}

func (r *fakeRunner) callCount(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[name]
}

func discoveryDecl(name string, conns ...string) config.PluginDecl {
	return config.PluginDecl{Name: name, Capability: "bucket-discovery", Cmd: "fake", Connections: conns}
}

func metadataDecl(name string, match *config.MatchRule) config.PluginDecl {
	return config.PluginDecl{Name: name, Capability: "object-metadata", Cmd: "fake", Match: match}
}

// pluginApp builds an App over a fake store + fake runner for the "ctx" context.
func pluginApp(f *storage.Fake, be Backend, decls []config.PluginDecl, r plugin.Runner, contexts []string, resolve Resolver) App {
	if be.Store == nil {
		be.Store = f
	}
	if be.Cluster == "" {
		be = Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x", UsageScanBudget: 20000,
			PinnedBuckets: be.PinnedBuckets}
	}
	if contexts == nil {
		contexts = []string{"ctx"}
	}
	m := New(be, "ctx", contexts, resolve, nil, preview.ProtoNone)
	return m.WithPlugins(decls, r)
}

// runCmds executes a tea.Cmd tree synchronously and returns the leaf messages.
func runCmds(c tea.Cmd) []tea.Msg {
	if c == nil {
		return nil
	}
	msg := c()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			out = append(out, runCmds(sub)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// drain executes a Cmd tree and delivers every resulting message except
// spinner ticks (which would re-arm endlessly).
func drain(m App, c tea.Cmd) App {
	for _, msg := range runCmds(c) {
		if _, ok := msg.(spinnerTickMsg); ok {
			continue
		}
		m = deliver(m, msg)
	}
	return m
}

func okDiscovery(names ...string) plugin.Result {
	return plugin.Result{Outcome: plugin.OutcomeOK, Buckets: names}
}

// bucketNames extracts the rendered bucket list names from the model.
func bucketNames(m App) []string {
	out := make([]string, 0, len(m.buckets))
	for _, b := range m.buckets {
		out = append(out, b.Name)
	}
	return out
}

// --- US1: discovery merge ---

func TestDiscoveryMergesWithPinnedWhenListingDenied(t *testing.T) {
	f := storage.NewFake()
	f.FailListBuckets = true
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta", "alpha")}}
	m := pluginApp(f, Backend{PinnedBuckets: []string{"pinned-b"}},
		[]config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	m = drain(m, m.Init())

	want := []string{"alpha", "pinned-b", "zeta"} // union, dedup, sorted
	got := bucketNames(m)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("buckets = %v, want %v", got, want)
	}
	if r.callCount("disco") != 1 {
		t.Errorf("calls = %d, want 1", r.callCount("disco"))
	}
	v := viewOf(m)
	for _, n := range want {
		if !strings.Contains(v, n) {
			t.Errorf("view missing %q", n)
		}
	}
}

func TestDiscoveryAdditiveWithWorkingListing(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta", "listed-a")}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	m = drain(m, m.Init())

	want := []string{"listed-a", "zeta"} // additive union, deduped
	got := bucketNames(m)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("buckets = %v, want %v", got, want)
	}
}

func TestDisabledPluginNeverInvoked(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	off := false
	decl := discoveryDecl("disco", "ctx")
	decl.Enabled = &off
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta")}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{decl}, r, nil, nil)
	m = drain(m, m.Init())

	if r.callCount("disco") != 0 {
		t.Errorf("disabled plugin invoked %d times", r.callCount("disco"))
	}
	if strings.Contains(strings.Join(bucketNames(m), ","), "zeta") {
		t.Error("disabled plugin contributed names")
	}
}

func TestUnassignedConnectionNeverInvoked(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta")}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "prod")}, r,
		[]string{"ctx", "prod"}, nil)
	m = drain(m, m.Init())

	if r.callCount("disco") != 0 {
		t.Errorf("plugin for another connection invoked %d times", r.callCount("disco"))
	}
}

// --- US1: failure paths ---

func TestDiscoveryFailureNoticeOnceAndListIntact(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{"disco": {Outcome: plugin.OutcomeTimeout, ErrDetail: "timeout"}}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	m = drain(m, m.Init())

	if got := strings.Join(bucketNames(m), ","); got != "listed-a" {
		t.Errorf("listed buckets disturbed: %v", got)
	}
	for _, frag := range []string{"discovery failed", "disco", "timeout", "P for details"} {
		if !strings.Contains(m.notice, frag) {
			t.Errorf("notice %q missing %q", m.notice, frag)
		}
	}

	// Second failure in the same session must not re-post the notice.
	m, cmd := pressCmd(m, "r") // refresh re-invokes (cache invalidated)
	m = drain(m, cmd)
	if r.callCount("disco") != 2 {
		t.Fatalf("refresh should re-invoke: calls = %d", r.callCount("disco"))
	}
	if strings.Contains(m.notice, "discovery failed") {
		t.Errorf("repeat failure re-posted the notice: %q", m.notice)
	}
}

func TestDiscoveryInvalidNamesDiscardedWithNotice(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{
		"disco": okDiscovery("ok-name", "BAD-UPPER", "x"),
	}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	m = drain(m, m.Init())

	got := strings.Join(bucketNames(m), ",")
	if !strings.Contains(got, "ok-name") {
		t.Errorf("valid name missing: %v", got)
	}
	if strings.Contains(got, "BAD-UPPER") || strings.Contains(got, ",x") {
		t.Errorf("invalid names leaked: %v", got)
	}
	if !strings.Contains(m.notice, "2 invalid discarded") {
		t.Errorf("notice %q must carry the discarded count", m.notice)
	}
}

func TestDiscoveryCapTruncatesWithIndication(t *testing.T) {
	f := storage.NewFake()
	names := make([]string, plugin.MaxBuckets+5)
	for i := range names {
		names[i] = "bucket-" + strconv.Itoa(100000+i)
	}
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery(names...)}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	m = drain(m, m.Init())

	if len(m.buckets) != plugin.MaxBuckets {
		t.Errorf("buckets = %d, want capped at %d", len(m.buckets), plugin.MaxBuckets)
	}
	if !strings.Contains(m.notice, "truncated") {
		t.Errorf("notice %q must indicate truncation", m.notice)
	}
}

// --- US1: staleness & cache ---

func TestDiscoveryStaleGenerationDropped(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta")}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	bs, _ := f.ListBuckets(context.Background())
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: bs})

	stale := discoveryDoneMsg{gen: m.discGen - 1, ctx: "ctx", plugin: "disco", res: okDiscovery("zeta")}
	m = deliver(m, stale)
	if strings.Contains(strings.Join(bucketNames(m), ","), "zeta") {
		t.Error("stale discovery result merged")
	}

	fresh := discoveryDoneMsg{gen: m.discGen, ctx: "ctx", plugin: "disco", res: okDiscovery("zeta")}
	m = deliver(m, fresh)
	if !strings.Contains(strings.Join(bucketNames(m), ","), "zeta") {
		t.Error("current-generation discovery result not merged")
	}
}

func TestDiscoveryCachedUntilRefresh(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta")}}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r, nil, nil)
	m = drain(m, m.Init())
	if r.callCount("disco") != 1 {
		t.Fatalf("calls = %d, want 1", r.callCount("disco"))
	}

	// Re-arming the bucket view without a refresh serves discovery from cache.
	if legs := (&m).discoveryLegs(); len(legs) != 0 {
		t.Errorf("cached discovery re-invoked: %d legs", len(legs))
	}

	m, cmd := pressCmd(m, "r")
	m = drain(m, cmd)
	if r.callCount("disco") != 2 {
		t.Errorf("refresh must re-invoke: calls = %d", r.callCount("disco"))
	}
}

func TestContextSwitchClearsDiscoveryCache(t *testing.T) {
	f := storage.NewFake()
	f.Seed("listed-a")
	r := &fakeRunner{results: map[string]plugin.Result{"disco": okDiscovery("zeta")}}
	resolve := func(name string) (Backend, error) {
		return Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x"}, nil
	}
	m := pluginApp(f, Backend{}, []config.PluginDecl{discoveryDecl("disco", "ctx")}, r,
		[]string{"ctx", "two"}, resolve)
	m = drain(m, m.Init())
	if r.callCount("disco") != 1 {
		t.Fatalf("calls = %d, want 1", r.callCount("disco"))
	}

	mm, _ := m.applyContext("two")
	m = finishSwitch(mm.(App), resolve, "two")
	if len(m.discCache) != 0 {
		t.Errorf("context switch must clear the discovery cache: %d entries", len(m.discCache))
	}

	// Switching back re-invokes (fresh data on reconnect).
	mm, _ = m.applyContext("ctx")
	m = finishSwitch(mm.(App), resolve, "ctx")
	_ = m
	if r.callCount("disco") != 1 {
		// The re-invocation runs inside the switch's command batch, which
		// finishSwitch does not execute; dispatch the legs explicitly.
		t.Logf("note: legs dispatched lazily")
	}
	if legs := (&m).discoveryLegs(); len(legs) != 1 {
		t.Errorf("switch back must re-dispatch discovery: %d legs", len(legs))
	}
}
