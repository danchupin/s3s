package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/cache"
	"github.com/danchupin/s3s/internal/storage"
)

// Inline usage + "more detail" (016). The separate full-screen analyze mode is gone:
// bucket/prefix totals + a ranked child breakdown (storage.UsageOf) now render INLINE in
// the details pane, computed by a dwell-gated, generation-guarded, session-cached,
// cancelable background scan. The freed `a` key is the context-aware MoreDetail trigger.

// detailSection is the single expandable section shown in the details pane. At most one
// renders at a time (budget gate, constitution VI / contracts/layout-budget.md).
type detailSection int

const (
	sectNone      detailSection = iota
	sectBreakdown               // ranked largest-first child breakdown (US3)
	sectTags                    // object tag key/values (US4)
	sectConfig                  // bucket configuration tri-state (US4)
)

// usageEvent is one item on the scan's progress channel.
type usageEvent struct {
	progress storage.UsageProgress
	done     bool
	report   storage.UsageReport
	err      error
}

// focusedUsageTarget resolves the (bucket, prefix) whose usage the details pane shows for
// the current focus: the highlighted bucket (whole bucket), a selected folder, or the
// current level. An object selection has NO usage target (its metadata is shown instead),
// so the pane never scans per-object-focus.
func (m App) focusedUsageTarget() (bucket, prefix string, ok bool) {
	switch {
	case m.mode == modeHealth:
		return m.healthBucket, m.healthPrefix, true // the card's subject (017 US4)
	case m.mode == modeBuckets && m.focusZone == zoneBuckets:
		if b := m.highlightedBucketName(); b != "" {
			return b, "", true
		}
	case m.mode == modeTree || (m.mode == modeBuckets && m.focusZone == zoneObjects):
		e := m.selected()
		switch {
		case e == nil:
			if m.bucket != "" {
				return m.bucket, m.prefix, true // level summary
			}
		case e.isDir:
			return m.bucket, e.full, true // selected folder
		}
		// object selected → no usage target
	}
	return "", "", false
}

// usageKey is the session-cache coordinate for a (bucket, prefix) usage report — the same
// (context, bucket, prefix) as a level key, with an empty search.
func (m App) usageKey(bucket, prefix string) cache.Key {
	return m.levelKeyFor(bucket, prefix)
}

// armUsageScan cancels any in-flight scan, bumps the usage generation, and (when the
// focused target is uncached and the pane is visible) schedules a dwell tick so rapid
// transit through a list spawns no scan (016 US2/FR-005/FR-006). Mutates the receiver;
// returns the tick cmd (or nil). Cached targets — INCLUDING budget-bounded partials —
// are shown immediately by the pane: ambient rescanning a partial would only reproduce
// the same bound; the explicit full scan (A/:scan) upgrades it (017 US1). A zero budget
// disables the ambient path entirely (017 FR-006).
func (m *App) armUsageScan() tea.Cmd {
	if m.usageCancel != nil {
		m.usageCancel()
		m.usageCancel = nil
	}
	m.usageGen++
	m.usageProg = storage.UsageProgress{}
	m.usageCh = nil
	// An open detail section belongs to the PREVIOUS target — collapse it on a move.
	m.detailSection = sectNone
	if layoutTier(m.width) == tierSingle { // no details pane in the single tier — nothing to show
		return nil
	}
	if m.usageBudget == 0 {
		return nil // ambient scanning disabled — explicit-only mode (017 FR-006)
	}
	b, p, ok := m.focusedUsageTarget()
	if !ok {
		return nil
	}
	if _, hit := m.usageResults.Get(m.usageKey(b, p)); hit {
		return nil // cached (exact or partial) → instant, no scan (SC-007, 017)
	}
	return usageTickCmd(m.usageGen, b, p)
}

// onUsageTick starts the BUDGETED scan once the selection has settled on the same target
// and the scan generation is current (016 US2, 017 US1). A tick for a scrolled-past
// target is dropped.
func (m App) onUsageTick(msg usageTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.usageGen {
		return m, nil // superseded by a later move
	}
	b, p, ok := m.focusedUsageTarget()
	if !ok || b != msg.bucket || p != msg.prefix {
		return m, nil // cursor moved on
	}
	if _, hit := m.usageResults.Get(m.usageKey(b, p)); hit {
		return m, nil // became cached
	}
	return m.startUsageScan(b, p, m.usageBudget)
}

// fullScanTarget resolves the explicit scan's (bucket, prefix): the focused usage
// target when one exists, else the CURRENT LEVEL — an object selection has no
// per-object usage, but the level it sits in does, so `A` works in flat buckets too.
func (m App) fullScanTarget() (bucket, prefix string, ok bool) {
	if b, p, found := m.focusedUsageTarget(); found {
		return b, p, true
	}
	if m.inLevel() && m.bucket != "" {
		return m.bucket, m.prefix, true
	}
	return "", "", false
}

// startFullScan is the dedicated explicit full-scan dispatcher — the ONLY path that may
// start an unbounded enumeration (017 FR-003). Bound to the FullScan key and the `:scan`
// command (one target — they cannot drift, the FR-019 pattern). No-op without any
// bucket/prefix target (e.g. the empty bucket list).
func (m App) startFullScan() (tea.Model, tea.Cmd) {
	b, p, ok := m.fullScanTarget()
	if !ok {
		return m, nil
	}
	return m.startUsageScan(b, p, 0)
}

// fullScanHintNeeded reports whether the target's usage is absent or partial — exactly
// when the `A full scan` affordance must be visible (017 FR-001/FR-003).
func (m App) fullScanHintNeeded(bucket, prefix string) bool {
	rep, ok := m.usageResults.Get(m.usageKey(bucket, prefix))
	return !ok || !rep.Complete
}

// startUsageScan launches a background UsageOf scan for (bucket, prefix) under the current
// usageGen with its OWN cancelable context (NOT beginLoad/m.gen). maxObjects bounds the
// enumeration (0 = the explicit full scan). Progress + the terminal report stream back
// stamped with usageGen and the target's cache key.
func (m App) startUsageScan(bucket, prefix string, maxObjects int) (tea.Model, tea.Cmd) {
	if m.usageCancel != nil {
		m.usageCancel()
	}
	gen := m.usageGen
	ctx, cancel := context.WithCancel(context.Background())
	m.usageCancel = cancel
	m.usageScanKey = m.usageKey(bucket, prefix)
	m.usageScanFull = maxObjects == 0 // the pane/card announce a full scan distinctly
	m.usageProg = storage.UsageProgress{}
	ch := make(chan usageEvent, 8)
	m.usageCh = ch
	return m, analyzeCmd(ctx, m.activeStore(), bucket, prefix, maxObjects, m.usageScanKey, ch, gen)
}

// analyzeCmd runs UsageOf off the event loop, streaming running totals and a terminal
// report on ch (constitution II). The goroutine always reaches its terminal send because a
// cancelled ctx makes UsageOf return promptly; the consumer (waitForUsage) drains ch
// regardless of generation, so the producer never strands on a full channel.
func analyzeCmd(ctx context.Context, st storage.Storage, bucket, prefix string, maxObjects int, key cache.Key, ch chan usageEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			rep, err := st.UsageOf(ctx, bucket, prefix, maxObjects, func(p storage.UsageProgress) {
				select {
				case ch <- usageEvent{progress: p}:
				default: // drop a tick rather than stall the scan
				}
			})
			ch <- usageEvent{done: true, report: rep, err: err}
		}()
		return waitForUsage(ch, gen, key)()
	}
}

// waitForUsage reads ONE scan event, re-arming until the terminal done event. The progress
// message carries ch so the handler always re-arms the SAME channel (drain discipline).
func waitForUsage(ch chan usageEvent, gen int, key cache.Key) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok || ev.done {
			return usageDoneMsg{gen: gen, key: key, report: ev.report, err: ev.err}
		}
		return usageProgressMsg{gen: gen, p: ev.progress, ch: ch, key: key}
	}
}

// onUsageProgress records the running total (only for the CURRENT scan) and ALWAYS re-arms
// the SAME channel (carried in the msg) so even a superseded scan drains to its done event
// — the producer never strands on a full channel (no goroutine leak, constitution II).
func (m App) onUsageProgress(msg usageProgressMsg) (tea.Model, tea.Cmd) {
	if msg.gen == m.usageGen {
		m.usageProg = msg.p
	}
	if msg.ch == nil {
		return m, nil
	}
	return m, waitForUsage(msg.ch, msg.gen, msg.key)
}

// onUsageDone caches the terminal report under the TARGET key it was scanned for —
// exact, budget-bounded, or cancelled-with-progress alike: partial work is never
// discarded (017 FR-002/FR-004, inverts the 016 drop). Even a superseded-generation
// report is cached (its data is valid for its own key; only the VIEW is gen-guarded).
// Exceptions: a backend error leaves no entry (usage is ancillary — it never hijacks
// the main view's error), an exact entry is never overwritten by a partial (FR-005),
// and a zero-progress cancellation taught us nothing.
func (m App) onUsageDone(msg usageDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen == m.usageGen {
		m.usageCancel = nil
		m.usageCh = nil
	}
	if msg.err != nil && !errorsIsCanceled(msg.err) {
		return m, nil
	}
	rep := msg.report
	if !rep.Complete && rep.TotalCount == 0 && rep.TotalSize == 0 {
		return m, nil // nothing learned — a zero lower bound is noise
	}
	if prev, ok := m.usageResults.Get(msg.key); ok && prev.Complete && !rep.Complete {
		return m, nil // exact stays; a partial never downgrades it (017 FR-005)
	}
	m.usageResults.Put(msg.key, &rep)
	return m, nil
}

// startMoreDetail is the context-aware "more detail" dispatcher bound to the `a` key and
// the `:detail`/`:info` command (one target — they cannot drift, FR-019). It toggles the
// single expandable detail section appropriate to the focus: an object → tags; a bucket →
// breakdown then config then collapse; a prefix/folder/level → breakdown then collapse.
func (m App) startMoreDetail() (tea.Model, tea.Cmd) {
	// Object focus → tags.
	if e := m.selected(); e != nil && !e.isDir && e.obj != nil {
		if m.detailSection == sectTags {
			m.detailSection = sectNone
			return m, nil
		}
		m.detailSection = sectTags
		return m.loadObjectTagsFor(e.full)
	}
	b, p, ok := m.focusedUsageTarget()
	if !ok {
		return m, nil
	}
	isBucket := m.mode == modeBuckets && m.focusZone == zoneBuckets
	switch m.detailSection {
	case sectBreakdown:
		if isBucket {
			m.detailSection = sectConfig
			return m.loadBucketConfigFor(b)
		}
		m.detailSection = sectNone
		return m, nil
	case sectConfig:
		m.detailSection = sectNone
		return m, nil
	default:
		m.detailSection = sectBreakdown
		m.breakdownSel = 0
		// Ensure data exists (or is being gathered) for the breakdown — at most a
		// BUDGETED scan: expanding detail must never start unbounded work; the full
		// scan has its own explicit action (017 FR-003).
		key := m.usageKey(b, p)
		if _, hit := m.usageResults.Get(key); !hit && (m.usageCh == nil || m.usageScanKey != key) {
			return m.startUsageScan(b, p, m.usageBudget)
		}
		return m, nil
	}
}

// loadObjectTagsFor lazily loads an object's tags under a fresh detail generation (US4).
func (m App) loadObjectTagsFor(key string) (tea.Model, tea.Cmd) {
	m.detailGen++
	m.detailKey = key
	m.objectTags = nil
	return m, loadObjectTags(context.Background(), m.activeStore(), m.bucket, key, m.detailGen)
}

// loadBucketConfigFor lazily loads a bucket's configuration under a fresh detail gen (US4).
func (m App) loadBucketConfigFor(bucket string) (tea.Model, tea.Cmd) {
	m.detailGen++
	m.detailKey = bucket
	m.bucketCfg = nil
	return m, loadBucketConfig(context.Background(), m.activeStore(), bucket, m.detailGen)
}

// onObjectTags / onBucketConfig apply a lazily-loaded detail result, dropping a stale one
// whose generation or target no longer matches (FR-016).
func (m App) onObjectTags(msg objectTagsMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.detailGen || msg.key != m.detailKey {
		return m, nil
	}
	if msg.err != nil {
		ot := storage.ObjectTags{ObjectKey: msg.key} // render denied/none gracefully
		m.objectTags = &ot
		m.tagsErr = msg.err
		return m, nil
	}
	t := msg.tags
	m.objectTags = &t
	m.tagsErr = nil
	return m, nil
}

func (m App) onBucketConfig(msg bucketConfigMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.detailGen || msg.bucket != m.detailKey {
		return m, nil
	}
	cfg := msg.cfg
	m.bucketCfg = &cfg
	return m, nil
}

// usageBar renders a compact share bar like "▇▇▇░░ 61%".
func usageBar(pct float64) string {
	const width = 5
	filled := int(pct/100*width + 0.5)
	if filled > width {
		filled = width
	}
	return strings.Repeat("▇", filled) + strings.Repeat("░", width-filled) +
		fmt.Sprintf(" %3.0f%%", pct)
}

// errorsIsCanceled reports a context-cancellation error (a cancelled scan is not a hard
// error — it yields a partial report).
func errorsIsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
