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

// --- US2: metadata enrichment ---

// imagesMatch scopes a metadata plugin to ctx + images* buckets + hex-prefixed keys.
func imagesMatch() *config.MatchRule {
	return &config.MatchRule{Connections: []string{"ctx"}, Buckets: []string{"images*"}, KeyPattern: "^[0-9a-f]{4}"}
}

func okFields(fields ...plugin.Field) plugin.Result {
	return plugin.Result{Outcome: plugin.OutcomeOK, Fields: fields}
}

// treeApp builds an App positioned in modeTree on the named bucket with its
// root level delivered.
func pluginTreeApp(t *testing.T, f *storage.Fake, decls []config.PluginDecl, r plugin.Runner, bucket string) App {
	t.Helper()
	m := pluginApp(f, Backend{}, decls, r, nil, nil)
	bs, _ := f.ListBuckets(context.Background())
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
	m.bucket = bucket
	m.mode = modeTree
	page, err := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	return deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
}

// openSelected opens the full-screen object view for the current tree selection
// and delivers its HeadObject metadata, leaving plugin legs unexecuted.
func openSelected(t *testing.T, m App, f *storage.Fake) (App, tea.Cmd) {
	t.Helper()
	e := m.selected()
	if e == nil || e.isDir {
		t.Fatal("no object selected")
	}
	m, cmd := pressCmd(m, "enter")
	md, err := f.HeadObject(context.Background(), m.bucket, e.full)
	if err != nil {
		t.Fatal(err)
	}
	return deliver(m, metadataMsg{gen: m.gen, md: md}), cmd
}

func TestEnrichGroupPendingThenPopulated(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-photo.jpg")
	r := &fakeRunner{results: map[string]plugin.Result{
		"meta": okFields(plugin.Field{Name: "Moderation", Value: "approved"}, plugin.Field{Name: "Owner", Value: "svc-img"}),
	}}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "images")
	m, _ = openSelected(t, m, f)

	v := viewOf(m)
	if !strings.Contains(v, "From meta") {
		t.Fatalf("attributed group header missing:\n%s", v)
	}
	if !strings.Contains(v, "pending") {
		t.Errorf("in-flight group must show pending:\n%s", v)
	}

	m = deliver(m, enrichDoneMsg{gen: m.enrichGen, ctx: "ctx", plugin: "meta",
		bucket: "images", key: "0af3-photo.jpg",
		res: r.results["meta"]})
	v = viewOf(m)
	if strings.Contains(v, "pending") {
		t.Errorf("pending must clear once fields land:\n%s", v)
	}
	// Fields render in plugin order under the attributed header.
	mi := strings.Index(v, "Moderation")
	oi := strings.Index(v, "Owner")
	if mi < 0 || oi < 0 || mi > oi {
		t.Errorf("fields missing or out of order (Moderation@%d, Owner@%d):\n%s", mi, oi, v)
	}
	if !strings.Contains(v, "approved") {
		t.Errorf("field value missing:\n%s", v)
	}
}

func TestEnrichFailedAndEmptyAreDistinct(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-a.jpg")
	r := &fakeRunner{}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "images")
	m, _ = openSelected(t, m, f)

	failed := deliver(m, enrichDoneMsg{gen: m.enrichGen, ctx: "ctx", plugin: "meta",
		bucket: "images", key: "0af3-a.jpg",
		res: plugin.Result{Outcome: plugin.OutcomeContractError, ErrDetail: "api 503"}})
	fv := viewOf(failed)
	if !strings.Contains(fv, "failed: api 503") {
		t.Errorf("failed state missing:\n%s", fv)
	}

	empty := deliver(m, enrichDoneMsg{gen: m.enrichGen, ctx: "ctx", plugin: "meta",
		bucket: "images", key: "0af3-a.jpg", res: okFields()})
	ev := viewOf(empty)
	if !strings.Contains(ev, "(empty)") {
		t.Errorf("empty state missing:\n%s", ev)
	}
	if strings.Contains(ev, "failed") {
		t.Errorf("empty result must not read as failure:\n%s", ev)
	}
}

func TestEnrichNonMatchingObjectNoGroupNoInvocation(t *testing.T) {
	f := storage.NewFake()
	f.Seed("logs", "0af3-entry.log") // bucket outside the images* glob
	r := &fakeRunner{results: map[string]plugin.Result{"meta": okFields(plugin.Field{Name: "X", Value: "y"})}}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "logs")
	m, cmd := openSelected(t, m, f)
	m = drain(m, cmd)

	if r.callCount("meta") != 0 {
		t.Errorf("out-of-scope object invoked the plugin %d times", r.callCount("meta"))
	}
	if strings.Contains(viewOf(m), "From meta") {
		t.Errorf("out-of-scope object must render no group:\n%s", viewOf(m))
	}
}

func TestEnrichFieldsFlowThroughFieldCopy(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-b.jpg")
	r := &fakeRunner{}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "images")
	m, _ = openSelected(t, m, f)
	m = deliver(m, enrichDoneMsg{gen: m.enrichGen, ctx: "ctx", plugin: "meta",
		bucket: "images", key: "0af3-b.jpg",
		res: okFields(plugin.Field{Name: "Image ID", Value: "0af3deadbeef"})})

	mm, _ := m.startFieldCopy()
	m = mm.(App)
	if m.fieldCopy == nil {
		t.Fatal("field copy overlay did not open")
	}
	found := false
	for _, row := range m.fieldCopy.rows {
		if row.label == "Image ID" && row.value == "0af3deadbeef" {
			found = true
		}
	}
	if !found {
		t.Errorf("plugin field missing from the copyable rows: %+v", m.fieldCopy.rows)
	}
}

func TestEnrichValueAndCountCaps(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-c.jpg")
	r := &fakeRunner{}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "images")
	m, _ = openSelected(t, m, f)

	long := strings.Repeat("v", plugin.MaxFieldValue+50)
	many := make([]plugin.Field, plugin.MaxFields+6)
	for i := range many {
		many[i] = plugin.Field{Name: "F" + strconv.Itoa(i), Value: "x"}
	}
	many[0] = plugin.Field{Name: "Big", Value: long}
	m = deliver(m, enrichDoneMsg{gen: m.enrichGen, ctx: "ctx", plugin: "meta",
		bucket: "images", key: "0af3-c.jpg", res: okFields(many...)})

	key := enrichKey{"ctx", "meta", "images", "0af3-c.jpg"}
	e, ok := m.enrichCache[key]
	if !ok {
		t.Fatal("result not cached")
	}
	if len(e.fields) != plugin.MaxFields || !e.truncated {
		t.Errorf("field count cap: %d fields, truncated=%v", len(e.fields), e.truncated)
	}
	if !strings.HasSuffix(e.fields[0].Value, plugin.TruncationMarker) {
		t.Error("over-cap value must carry the truncation marker")
	}
	if pg := m.enrichGroupsView("images", "0af3-c.jpg", 80); !strings.Contains(pg, "first 64 fields") {
		t.Errorf("count truncation must be indicated in the rendered group:\n%s", pg)
	}
	// The capped, marked value stays fully copyable.
	mm, _ := m.startFieldCopy()
	m = mm.(App)
	got := ""
	for _, row := range m.fieldCopy.rows {
		if row.label == "Big" {
			got = row.value
		}
	}
	if got != e.fields[0].Value {
		t.Error("copy path must carry the full stored value")
	}
}

func TestEnrichStaleDropsAndReselectHitsCache(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-one.jpg", "0bf3-two.jpg")
	r := &fakeRunner{results: map[string]plugin.Result{"meta": okFields(plugin.Field{Name: "K", Value: "v1"})}}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "images")

	// Selection settles on the first object → debounced pane tick fires the legs.
	first := m.selected().full
	m.paneSelKey = first
	mm, cmd := m.onPaneTick(paneTickMsg{gen: m.paneGen, key: first})
	m = mm.(App)
	msgs := runCmds(cmd)
	if r.callCount("meta") != 1 {
		t.Fatalf("calls = %d, want 1", r.callCount("meta"))
	}

	// Selection moves on before the result lands — the late result must not
	// disturb the new selection's view, but stays valid for its own key.
	m = press(m, "down")
	for _, msg := range msgs {
		if em, ok := msg.(enrichDoneMsg); ok {
			m = deliver(m, em)
		}
	}
	if strings.Contains(viewOf(m), "v1") {
		t.Errorf("late result rendered for the wrong selection:\n%s", viewOf(m))
	}

	// Reselecting the first object serves from cache: no second invocation.
	m = press(m, "up")
	m.paneSelKey = first
	mm, cmd = m.onPaneTick(paneTickMsg{gen: m.paneGen, key: first})
	m = mm.(App)
	runCmds(cmd)
	if r.callCount("meta") != 1 {
		t.Errorf("reselect must hit the cache: calls = %d", r.callCount("meta"))
	}

	// Refresh invalidates: an in-flight result from before the refresh is
	// dropped before the cache is written, and the next tick re-invokes.
	mm, _ = m.refresh()
	m = mm.(App)
	stale := enrichDoneMsg{gen: m.enrichGen - 1, ctx: "ctx", plugin: "meta",
		bucket: "images", key: first, res: okFields(plugin.Field{Name: "K", Value: "v2"})}
	m = deliver(m, stale)
	if _, ok := m.enrichCache[enrichKey{"ctx", "meta", "images", first}]; ok {
		t.Error("stale-generation result must never be cached")
	}
}

func TestEnrichTwoPluginsTwoGroupsDeclarationOrder(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-d.jpg")
	r := &fakeRunner{}
	decls := []config.PluginDecl{
		metadataDecl("alpha-meta", imagesMatch()),
		metadataDecl("beta-meta", imagesMatch()),
	}
	m := pluginTreeApp(t, f, decls, r, "images")
	m, _ = openSelected(t, m, f)
	for _, name := range []string{"beta-meta", "alpha-meta"} { // arrival order ≠ declaration order
		m = deliver(m, enrichDoneMsg{gen: m.enrichGen, ctx: "ctx", plugin: name,
			bucket: "images", key: "0af3-d.jpg",
			res: okFields(plugin.Field{Name: "Src", Value: name})})
	}
	v := viewOf(m)
	ai := strings.Index(v, "From alpha-meta")
	bi := strings.Index(v, "From beta-meta")
	if ai < 0 || bi < 0 || ai > bi {
		t.Errorf("groups missing or out of declaration order (alpha@%d, beta@%d):\n%s", ai, bi, v)
	}
}

func TestEnrichSingleInflightPerTarget(t *testing.T) {
	f := storage.NewFake()
	f.Seed("images", "0af3-e.jpg")
	r := &fakeRunner{results: map[string]plugin.Result{"meta": okFields(plugin.Field{Name: "K", Value: "v"})}}
	m := pluginTreeApp(t, f, []config.PluginDecl{metadataDecl("meta", imagesMatch())}, r, "images")

	key := m.selected().full
	m.paneSelKey = key
	mm, cmd := m.onPaneTick(paneTickMsg{gen: m.paneGen, key: key})
	m = mm.(App)
	msgs := runCmds(cmd) // executes the first leg — result NOT delivered yet
	if r.callCount("meta") != 1 {
		t.Fatalf("calls = %d, want 1", r.callCount("meta"))
	}

	// A second settled tick for the same object must coalesce, not re-invoke.
	mm, cmd = m.onPaneTick(paneTickMsg{gen: m.paneGen, key: key})
	m = mm.(App)
	runCmds(cmd)
	if r.callCount("meta") != 1 {
		t.Errorf("repeat selection re-invoked: calls = %d", r.callCount("meta"))
	}

	for _, msg := range msgs {
		if em, ok := msg.(enrichDoneMsg); ok {
			m = deliver(m, em)
		}
	}
	if _, ok := m.enrichCache[enrichKey{"ctx", "meta", "images", key}]; !ok {
		t.Error("landed result must cache")
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
