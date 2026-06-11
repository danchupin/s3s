package ui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/config"
	"github.com/danchupin/s3s/internal/plugin"
	"github.com/danchupin/s3s/internal/storage"
)

// Plugin integration: external capability providers declared in config.
// Discovery results merge additively into the bucket list; metadata results
// render as attributed groups in the details pane; the status surface
// (modePlugins) gives per-plugin visibility and the enable/disable toggle.
// Zero declared plugins ⇒ none of this is reachable and nothing changes.

// pluginState is the live operational state of one declared plugin. States are
// shared by pointer so handlers on value-copied models observe one truth.
type pluginState struct {
	decl         config.PluginDecl
	enabled      bool   // live toggle (persisted via Connector.SetPluginEnabled)
	unavailable  string // non-empty reason (unknown connection / missing executable) ⇒ never invoked
	incompatible int    // contract version the plugin answered with; > 0 ⇒ skipped for the session
	discRunning  bool // a discovery invocation is in flight
	discRunGen   int  // discovery generation that invocation was dispatched under
	hasRun       bool
	lastOutcome  plugin.Outcome
	lastErr      string
	lastDur      time.Duration
	lastAtTime   time.Time
	lastBucket   string // last enrichment target (retry)
	lastKey      string
	noticed      bool // the first-failure notice was already posted this session
}

// discKey / enrichKey are the session cache coordinates (D11): discovery per
// (context, plugin); enrichment per (context, plugin, bucket, key).
type discKey struct{ ctx, plugin string }

type discEntry struct {
	names     []string
	discarded int
	truncated bool
}

type enrichKey struct{ ctx, plugin, bucket, key string }

type enrichEntry struct {
	fields    []plugin.Field
	truncated bool
	failed    string // sanitized failure text; "" = success (fields may still be empty)
}

// WithPlugins wires the declared plugins and their runner into the model. Must
// be called right after New: when an initial load is armed there, the initial
// discovery legs are armed here too (Init cannot mutate the model) and batched
// by Init alongside the bucket load.
func (m App) WithPlugins(decls []config.PluginDecl, r plugin.Runner) App {
	if len(decls) == 0 || r == nil {
		return m
	}
	m.pluginRunner = r
	m.discCache = map[discKey]discEntry{}
	m.enrichCache = map[enrichKey]enrichEntry{}
	m.enrichInflight = map[enrichKey]bool{}
	known := map[string]bool{}
	for _, c := range m.contexts {
		known[c] = true
	}
	for _, d := range decls {
		st := &pluginState{decl: d, enabled: d.IsEnabled()}
		resolves := false
		for _, conn := range d.ScopeConnections() {
			if known[conn] {
				resolves = true
				break
			}
		}
		if !resolves {
			st.unavailable = "unknown connection"
		}
		m.plugins = append(m.plugins, st)
	}
	if m.raw != nil {
		m.initPluginCmds = (&m).discoveryLegs()
	}
	return m
}

// runnerDecl converts a config declaration to the runner's execution slice.
func runnerDecl(d config.PluginDecl) plugin.Decl {
	return plugin.Decl{Name: d.Name, Cmd: d.Cmd, Timeout: d.ResolvedTimeout()}
}

// pluginConn is the identity context a plugin receives — public identifiers
// only, never credential material.
func (m App) pluginConn() plugin.Connection {
	return plugin.Connection{
		Name:        m.ctxName,
		Endpoint:    m.info.Endpoint,
		UserLabel:   m.info.User,
		AccessKeyID: m.info.AccessKeyID,
	}
}

// pluginByName returns the state of the named plugin, or nil.
func (m App) pluginByName(name string) *pluginState {
	for _, st := range m.plugins {
		if st.decl.Name == name {
			return st
		}
	}
	return nil
}

// discoveryApplies reports whether a discovery plugin should run for the
// active connection: enabled, available, compatible, and assigned to it.
func (m App) discoveryApplies(st *pluginState) bool {
	return st.enabled && st.unavailable == "" && st.incompatible == 0 &&
		st.decl.Capability == string(plugin.BucketDiscovery) &&
		slices.Contains(st.decl.Connections, m.ctxName)
}

// discoveryLegs returns one command per applicable discovery plugin that has
// no cached result for the active context, marking each as in flight under the
// current discovery generation. Cache hits dispatch nothing — the cached names
// merge when the bucket list lands.
func (m *App) discoveryLegs() []tea.Cmd {
	if m.pluginRunner == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, st := range m.plugins {
		if !m.discoveryApplies(st) {
			continue
		}
		if _, ok := m.discCache[discKey{m.ctxName, st.decl.Name}]; ok {
			continue
		}
		st.discRunning = true
		st.discRunGen = m.discGen
		cmds = append(cmds, discoverCmd(m.pluginRunner, runnerDecl(st.decl), m.pluginConn(), m.ctxName, m.discGen))
	}
	return cmds
}

// discoveryRunning reports any in-flight discovery invocation (drives the
// "discovering…" segment and keeps the spinner ticking).
func (m App) discoveryRunning() bool {
	for _, st := range m.plugins {
		if st.discRunning {
			return true
		}
	}
	return false
}

// invalidateDiscovery drops the active context's discovery cache and bumps the
// discovery generation so in-flight results from before the refresh are
// dropped before they can be cached.
func (m *App) invalidateDiscovery() {
	for _, st := range m.plugins {
		delete(m.discCache, discKey{m.ctxName, st.decl.Name})
	}
	m.discGen++
}

// onDiscoveryDone applies one discovery invocation outcome. A result from a
// superseded discovery generation is dropped before the cache is written; the
// running flag is released only when no newer dispatch owns it.
func (m App) onDiscoveryDone(msg discoveryDoneMsg) (tea.Model, tea.Cmd) {
	st := m.pluginByName(msg.plugin)
	if st == nil {
		return m, nil
	}
	if st.discRunGen == msg.gen {
		st.discRunning = false
	}
	if msg.gen != m.discGen {
		return m, nil // superseded by refresh/context switch — never cached
	}
	st.hasRun = true
	st.lastOutcome = msg.res.Outcome
	st.lastErr = msg.res.ErrDetail
	st.lastDur = msg.res.Duration
	st.lastAtTime = m.now()

	switch msg.res.Outcome {
	case plugin.OutcomeOK:
		names, discarded, truncated := plugin.FilterBuckets(msg.res.Buckets)
		m.discCache[discKey{msg.ctx, msg.plugin}] = discEntry{names: names, discarded: discarded, truncated: truncated}
		(&m).applyDiscovered()
		if discarded > 0 || truncated {
			note := fmt.Sprintf("discovery: %d buckets", len(names))
			if discarded > 0 {
				note += fmt.Sprintf(" (%d invalid discarded)", discarded)
			}
			if truncated {
				note += fmt.Sprintf(" (truncated at %d)", plugin.MaxBuckets)
			}
			m.notice = note
		}
	case plugin.OutcomeIncompatible:
		st.incompatible = msg.res.Incompatible
		(&m).pluginFailNotice(st, pluginFailReason(msg.res))
	default:
		if msg.res.Unavailable {
			st.unavailable = "executable not found"
		}
		(&m).pluginFailNotice(st, pluginFailReason(msg.res))
	}
	return m, nil
}

// applyDiscovered merges every cached, currently-applicable discovery result
// for the active context into the bucket list: pinned ∪ listed ∪ discovered,
// deduplicated, name-sorted. No cached results ⇒ strictly no change.
func (m *App) applyDiscovered() {
	var extra [][]string
	for _, st := range m.plugins {
		if !st.enabled || !slices.Contains(st.decl.Connections, m.ctxName) {
			continue
		}
		if e, ok := m.discCache[discKey{m.ctxName, st.decl.Name}]; ok {
			extra = append(extra, e.names)
		}
	}
	if len(extra) == 0 {
		return
	}
	m.buckets = mergeBuckets(m.buckets, extra...)
	m.clampSelection()
}

// mergeBuckets returns the deduplicated, name-sorted union of the listed (or
// pinned-synthesized) buckets and the discovered name sets. A listed entry
// keeps its creation date; a discovered-only name carries none.
func mergeBuckets(listed []storage.Bucket, discovered ...[]string) []storage.Bucket {
	seen := make(map[string]bool, len(listed))
	out := make([]storage.Bucket, 0, len(listed))
	for _, b := range listed {
		if seen[b.Name] {
			continue
		}
		seen[b.Name] = true
		out = append(out, b)
	}
	for _, set := range discovered {
		for _, n := range set {
			if seen[n] {
				continue
			}
			seen[n] = true
			out = append(out, storage.Bucket{Name: n})
		}
	}
	slices.SortFunc(out, func(a, b storage.Bucket) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// pluginFailNotice posts the transient failure notice for a plugin — first
// failure per plugin per session only (repeat failures stay visible in the
// status surface without nagging).
func (m *App) pluginFailNotice(st *pluginState, reason string) {
	if st.noticed {
		return
	}
	st.noticed = true
	m.notice = fmt.Sprintf("discovery failed: %s (%s) — %s for details",
		st.decl.Name, reason, glyph(firstBind(m.keys.Plugins)))
}

// enrichApplies reports whether a metadata plugin should run for an object:
// enabled, available, compatible, and the object inside its match scope.
func (m App) enrichApplies(st *pluginState, bucket, key string) bool {
	return st.enabled && st.unavailable == "" && st.incompatible == 0 &&
		st.decl.Capability == string(plugin.ObjectMetadata) &&
		st.decl.Match != nil && st.decl.Match.Matches(m.ctxName, bucket, key)
}

// enrichLegs returns one command per matching metadata plugin for the object,
// skipping cache hits and targets already in flight — rapid repeated selection
// of one object coalesces to a single invocation per (plugin, target).
func (m *App) enrichLegs(bucket, key string) []tea.Cmd {
	if m.pluginRunner == nil || bucket == "" || key == "" {
		return nil
	}
	var cmds []tea.Cmd
	for _, st := range m.plugins {
		if !m.enrichApplies(st, bucket, key) {
			continue
		}
		k := enrichKey{m.ctxName, st.decl.Name, bucket, key}
		if _, ok := m.enrichCache[k]; ok {
			continue
		}
		if m.enrichInflight[k] {
			continue
		}
		m.enrichInflight[k] = true
		cmds = append(cmds, enrichCmd(m.pluginRunner, runnerDecl(st.decl), m.pluginConn(), m.ctxName, bucket, key, m.enrichGen))
	}
	return cmds
}

// invalidateEnrichment drops the active context's cached enrichment for one
// bucket and bumps the enrichment generation: in-flight results from before
// the refresh are dropped before the cache is ever written.
func (m *App) invalidateEnrichment(bucket string) {
	for k := range m.enrichCache {
		if k.ctx == m.ctxName && k.bucket == bucket {
			delete(m.enrichCache, k)
		}
	}
	for k := range m.enrichInflight {
		if k.ctx == m.ctxName && k.bucket == bucket {
			delete(m.enrichInflight, k)
		}
	}
	m.enrichGen++
}

// onEnrichDone applies one object-metadata invocation outcome. A result from a
// superseded enrichment generation is dropped entirely; a result for a
// no-longer-selected object is still cached for its own key (the data is valid
// there — render simply doesn't show it), mirroring the usage-scan rule.
func (m App) onEnrichDone(msg enrichDoneMsg) (tea.Model, tea.Cmd) {
	st := m.pluginByName(msg.plugin)
	if st == nil {
		return m, nil
	}
	if msg.gen != m.enrichGen {
		return m, nil // superseded by refresh/context switch — never cached
	}
	k := enrichKey{msg.ctx, msg.plugin, msg.bucket, msg.key}
	delete(m.enrichInflight, k)
	st.hasRun = true
	st.lastOutcome = msg.res.Outcome
	st.lastErr = msg.res.ErrDetail
	st.lastDur = msg.res.Duration
	st.lastAtTime = m.now()
	st.lastBucket, st.lastKey = msg.bucket, msg.key

	switch msg.res.Outcome {
	case plugin.OutcomeOK:
		fields, truncated := plugin.CapFields(msg.res.Fields)
		m.enrichCache[k] = enrichEntry{fields: fields, truncated: truncated}
	case plugin.OutcomeIncompatible:
		st.incompatible = msg.res.Incompatible
		m.enrichCache[k] = enrichEntry{failed: pluginFailReason(msg.res)}
	default:
		if msg.res.Unavailable {
			st.unavailable = "executable not found"
		}
		m.enrichCache[k] = enrichEntry{failed: pluginFailReason(msg.res)}
	}
	return m, nil
}

// enrichGroupsView renders the attributed plugin groups for an object,
// appended after the native metadata groups: `From <plugin>` per matching
// plugin in declaration order, with text-distinct states — pending while in
// flight, fields when populated, an explicit empty state, failed: <reason>.
// Out-of-scope objects render nothing (and were never invoked).
func (m App) enrichGroupsView(bucket, key string, w int) string {
	if m.pluginRunner == nil {
		return ""
	}
	var b strings.Builder
	for _, st := range m.plugins {
		if !m.enrichApplies(st, bucket, key) {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(colHeadStyle.Render("From "+sanitizeLabel(st.decl.Name)) + "\n")
		e, ok := m.enrichCache[enrichKey{m.ctxName, st.decl.Name, bucket, key}]
		switch {
		case !ok:
			b.WriteString(dimCellStyle.Render("pending") + "\n")
		case e.failed != "":
			b.WriteString(errStyle.Render(truncate("failed: "+e.failed, max(1, w))) + "\n")
		case len(e.fields) == 0:
			b.WriteString(emptyStyle.Render("— (empty)") + "\n")
		default:
			for _, fl := range e.fields {
				b.WriteString(metaRow(fl.Name, fl.Value, w))
			}
			if e.truncated {
				b.WriteString(dimCellStyle.Render(fmt.Sprintf("… first %d fields shown", plugin.MaxFields)) + "\n")
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// enrichFieldRows exposes the populated plugin fields to the per-field copy
// overlay — the same stored (sanitized, capped) values the pane renders.
func (m App) enrichFieldRows(bucket, key string) []metaField {
	if m.pluginRunner == nil {
		return nil
	}
	var rows []metaField
	for _, st := range m.plugins {
		if !m.enrichApplies(st, bucket, key) {
			continue
		}
		e, ok := m.enrichCache[enrichKey{m.ctxName, st.decl.Name, bucket, key}]
		if !ok || e.failed != "" {
			continue
		}
		for _, fl := range e.fields {
			rows = append(rows, metaField{label: fl.Name, value: fl.Value})
		}
	}
	return rows
}

// --- status surface (modePlugins) ---

// openPlugins enters the full-screen plugin status surface. A no-op with zero
// declared plugins — zero-config users never see plugin chrome.
func (m App) openPlugins() (tea.Model, tea.Cmd) {
	if len(m.plugins) == 0 || m.mode == modePlugins {
		return m, nil
	}
	m.prevMode = m.mode
	m.mode = modePlugins
	if m.pluginSel >= len(m.plugins) {
		m.pluginSel = 0
	}
	return m, nil
}

// onPluginsKey owns input inside the status surface: row navigation, Enter
// reveals the full error detail, space toggles enable/disable (persisted),
// r retries the selected plugin's last failed target, Esc returns.
func (m App) onPluginsKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case matches(key, m.keys.Up):
		if m.pluginSel > 0 {
			m.pluginSel--
		}
	case matches(key, m.keys.Down):
		if m.pluginSel < len(m.plugins)-1 {
			m.pluginSel++
		}
	case matches(key, m.keys.Top):
		m.pluginSel = 0
	case matches(key, m.keys.Bottom):
		m.pluginSel = max(0, len(m.plugins)-1)
	case matches(key, m.keys.Back):
		m.mode = m.prevMode
	case matches(key, m.keys.Enter):
		st := m.plugins[m.pluginSel]
		if detail := pluginRevealText(st); detail != "" {
			m.reveal = &revealState{kind: revealPlugin, value: detail, detail: "plugin: " + sanitizeLabel(st.decl.Name)}
		}
	case matches(key, m.keys.Mark):
		return m.togglePlugin()
	case matches(key, m.keys.Refresh):
		return m.retryPlugin()
	}
	return m, nil
}

// togglePlugin flips the selected plugin immediately (the very next load skips
// or includes it) and persists via the Connector off the event loop; a
// persistence failure reverts the flip.
func (m App) togglePlugin() (tea.Model, tea.Cmd) {
	if len(m.plugins) == 0 {
		return m, nil
	}
	if m.connect == nil {
		m.notice = "plugin toggle unavailable (no config writer)"
		return m, nil
	}
	st := m.plugins[m.pluginSel]
	target := !st.enabled
	st.enabled = target // optimistic — reverted by onPluginToggled on error
	return m, setPluginEnabledCmd(m.connect, st.decl.Name, target)
}

// setPluginEnabledCmd persists the toggle off the event loop (config mutation).
func setPluginEnabledCmd(c Connector, name string, enabled bool) tea.Cmd {
	return func() tea.Msg {
		err := c.SetPluginEnabled(context.Background(), name, enabled)
		return pluginToggledMsg{name: name, enabled: enabled, err: err}
	}
}

// onPluginToggled applies the toggle persistence outcome: an error reverts the
// optimistic flip and posts a notice; success confirms quietly.
func (m App) onPluginToggled(msg pluginToggledMsg) (tea.Model, tea.Cmd) {
	st := m.pluginByName(msg.name)
	if st == nil {
		return m, nil
	}
	if msg.err != nil {
		st.enabled = !msg.enabled
		m.notice = "plugin toggle failed: " + msg.err.Error()
		return m, nil
	}
	verb := "disabled"
	if msg.enabled {
		verb = "enabled"
	}
	m.notice = "plugin " + verb + ": " + sanitizeLabel(msg.name)
	return m, nil
}

// retryPlugin re-invokes the selected plugin against its last target —
// discovery for the active connection, or the last enriched object. An
// incompatible plugin needs a fix + restart, never a silent retry.
func (m App) retryPlugin() (tea.Model, tea.Cmd) {
	if len(m.plugins) == 0 || m.pluginRunner == nil {
		return m, nil
	}
	st := m.plugins[m.pluginSel]
	if st.incompatible > 0 {
		m.notice = "plugin speaks an incompatible contract — fix the plugin and restart"
		return m, nil
	}
	if !st.enabled {
		m.notice = "plugin is disabled — space to enable"
		return m, nil
	}
	switch plugin.Capability(st.decl.Capability) {
	case plugin.BucketDiscovery:
		if !slices.Contains(st.decl.Connections, m.ctxName) {
			m.notice = "plugin is not assigned to the active connection"
			return m, nil
		}
		st.unavailable = "" // a fixed executable gets to re-probe
		delete(m.discCache, discKey{m.ctxName, st.decl.Name})
		st.discRunning = true
		st.discRunGen = m.discGen
		return m, discoverCmd(m.pluginRunner, runnerDecl(st.decl), m.pluginConn(), m.ctxName, m.discGen)
	case plugin.ObjectMetadata:
		if st.lastBucket == "" {
			m.notice = "nothing to retry yet — select a matching object first"
			return m, nil
		}
		st.unavailable = ""
		k := enrichKey{m.ctxName, st.decl.Name, st.lastBucket, st.lastKey}
		delete(m.enrichCache, k)
		m.enrichInflight[k] = true
		return m, enrichCmd(m.pluginRunner, runnerDecl(st.decl), m.pluginConn(), m.ctxName, st.lastBucket, st.lastKey, m.enrichGen)
	}
	return m, nil
}

// pluginRevealText is the full sanitized detail behind the truncated state
// column — the reveal popup's payload.
func pluginRevealText(st *pluginState) string {
	switch {
	case st.incompatible > 0:
		return fmt.Sprintf("incompatible: plugin answered contract v%d (this build speaks v%d)", st.incompatible, plugin.ContractVersion)
	case st.unavailable != "":
		return "unavailable: " + st.unavailable
	case st.lastErr != "":
		return st.lastErr
	}
	return ""
}

// pluginStateText is the STATE column vocabulary — text-distinct, NO_COLOR-safe.
func (m App) pluginStateText(st *pluginState) string {
	switch {
	case st.incompatible > 0:
		return fmt.Sprintf("incompatible: contract v%d", st.incompatible)
	case !st.enabled:
		return "disabled"
	case st.unavailable != "":
		return "unavailable: " + st.unavailable
	case m.pluginRunning(st):
		return "running"
	case !st.hasRun:
		return "ready"
	case st.lastOutcome == plugin.OutcomeOK:
		return fmt.Sprintf("ok %s · %s", st.lastDur.Round(time.Millisecond), relTime(m.now(), st.lastAtTime))
	default:
		return fmt.Sprintf("failed: %s · %s", pluginFailText(st), relTime(m.now(), st.lastAtTime))
	}
}

// pluginFailText is the short failure reason for the state column.
func pluginFailText(st *pluginState) string {
	if st.lastOutcome == plugin.OutcomeTimeout {
		return "timeout"
	}
	if st.lastErr != "" {
		return st.lastErr
	}
	return string(st.lastOutcome)
}

// pluginRunning reports any in-flight invocation for the plugin (discovery
// flag or an enrichment in-flight entry).
func (m App) pluginRunning(st *pluginState) bool {
	if st.discRunning {
		return true
	}
	for k := range m.enrichInflight {
		if k.plugin == st.decl.Name {
			return true
		}
	}
	return false
}

// pluginScopeText summarizes a declaration's scope for the SCOPE column.
func pluginScopeText(d config.PluginDecl) string {
	if d.Match != nil {
		s := strings.Join(d.Match.Connections, ",")
		if len(d.Match.Buckets) > 0 {
			s += " " + strings.Join(d.Match.Buckets, ",")
		}
		return s
	}
	return strings.Join(d.Connections, ",")
}

// pluginsView renders the status surface body: one row per declared plugin
// (name, capability, scope, state) plus the surface's own key hints. Long
// values truncate in the table; Enter reveals the full sanitized detail.
func (m App) pluginsView(w, rows int) string {
	if len(m.plugins) == 0 {
		return emptyStyle.Render("No plugins declared.")
	}
	body := max(1, rows-2) // reserve the inline hint line + its separator
	off, end := windowBounds(len(m.plugins), m.pluginSel, body)
	cols := []column{{"name", 0}, {"capability", 16}, {"scope", 20}, {"state", 34}}
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		st := m.plugins[i]
		data = append(data, []string{
			sanitizeLabel(st.decl.Name),
			st.decl.Capability,
			sanitizeLabel(pluginScopeText(st.decl)),
			sanitizeLabel(m.pluginStateText(st)),
		})
	}
	hint := keyHint(m.keys.Enter, "detail") + sepDot + "Space enable/disable" + sepDot +
		keyHint(m.keys.Refresh, "retry") + sepDot + keyHint(m.keys.Back, "back")
	return renderTable(w, cols, data, nil, m.pluginSel-off) + "\n\n" +
		dimCellStyle.Render(truncate(hint, max(1, w-1)))
}

// pluginFailReason maps a failed result to its short display reason.
func pluginFailReason(res plugin.Result) string {
	switch res.Outcome {
	case plugin.OutcomeTimeout:
		return "timeout"
	case plugin.OutcomeIncompatible:
		return fmt.Sprintf("incompatible: contract v%d", res.Incompatible)
	case plugin.OutcomeInvalidOutput:
		if res.ErrDetail != "" {
			return res.ErrDetail
		}
		return "invalid output"
	default:
		if res.ErrDetail != "" {
			return res.ErrDetail
		}
		return string(res.Outcome)
	}
}
