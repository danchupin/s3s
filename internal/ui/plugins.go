package ui

import (
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

// onEnrichDone applies one object-metadata invocation outcome (US2).
func (m App) onEnrichDone(msg enrichDoneMsg) (tea.Model, tea.Cmd) {
	return m, nil
}

// onPluginToggled applies the toggle persistence outcome (US3).
func (m App) onPluginToggled(msg pluginToggledMsg) (tea.Model, tea.Cmd) {
	return m, nil
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
