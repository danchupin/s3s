package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/cache"
	"github.com/danchupin/s3s/internal/storage"
)

// Operator health card (017 US4): a dedicated full-screen view (modeHealth) answering,
// for one bucket/prefix, how the data ages, how it is sized, how it spreads across
// storage classes, and what is wasted in incomplete multipart uploads. Distributions
// come from the SAME enumeration the usage scan performed (zero extra requests,
// FR-020); the MPU probe is its own lazy, cancellable read (FR-021). Opening the card
// NEVER starts unbounded work — at most the budgeted scan (FR-003).

// openHealth enters the card for the focused bucket/prefix target. An object
// selection has no card target — no-op with a footer note (contract).
func (m App) openHealth() (tea.Model, tea.Cmd) {
	b, p, ok := m.focusedUsageTarget()
	if !ok || m.mode == modeHealth {
		m.notice = "health: select a bucket or folder"
		return m, nil
	}
	m.prevMode = m.mode
	m.mode = modeHealth
	m.healthBucket, m.healthPrefix = b, p

	var cmds []tea.Cmd
	// Usage data: a budgeted scan when nothing is cached (never unbounded — FR-003).
	if _, hit := m.usageResults.Get(m.usageKey(b, p)); !hit && m.usageCh == nil {
		mm, cmd := m.startUsageScan(b, p, m.usageBudget)
		m = mm.(App)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// The MPU probe: lazy, generation-guarded, cancellable (FR-021).
	if _, hit := m.mpuResults.Get(m.usageKey(b, p)); !hit {
		m.healthGen++
		if m.healthCancel != nil {
			m.healthCancel()
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.healthCancel = cancel
		cmds = append(cmds, loadIncompleteUploads(ctx, m.activeStore(), b, p, m.usageKey(b, p), m.healthGen))
	}
	return m, tea.Batch(cmds...)
}

// loadIncompleteUploads runs the probe off the event loop (constitution II).
func loadIncompleteUploads(ctx context.Context, st storage.Storage, bucket, prefix string, key cache.Key, gen int) tea.Cmd {
	return func() tea.Msg {
		res, err := st.ListIncompleteUploads(ctx, bucket, prefix)
		return incompleteUploadsMsg{gen: gen, key: key, res: res, err: err}
	}
}

// onIncompleteUploads caches the probe result; a stale generation is dropped, an error
// surfaces in the footer and never masquerades as a zero (FR-022).
func (m App) onIncompleteUploads(msg incompleteUploadsMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.healthGen {
		return m, nil
	}
	m.healthCancel = nil
	if msg.err != nil && !errorsIsCanceled(msg.err) {
		m.err = msg.err
		return m, nil
	}
	res := msg.res
	m.mpuResults.Put(msg.key, &res)
	return m, nil
}

// onHealthKey owns input inside the card: Esc returns to the prior browse position;
// the explicit full scan re-targets the card's subject. Copy/share (`Y`) and the
// command bar are handled by the global routing.
func (m App) onHealthKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case matches(key, m.keys.Back):
		if m.healthCancel != nil {
			m.healthCancel()
			m.healthCancel = nil
		}
		m.healthGen++
		m.mode = m.prevMode
		return m, nil
	case matches(key, m.keys.FullScan):
		return m.startUsageScan(m.healthBucket, m.healthPrefix, 0)
	}
	return m, nil
}

// healthSection is one card block; collapsible sections carry a priority (classes
// collapse first, then size, then age — contract collapse order).
type healthSection struct {
	lines    []string
	collapse int // 0 = never collapses; higher collapses earlier
	summary  string
}

// healthView composes the card under the rows budget: sections collapse to one
// explicit line instead of clipping values (constitution VI).
func (m App) healthView(w, rows int) string {
	b, p := m.healthBucket, m.healthPrefix
	key := m.usageKey(b, p)
	rep, hasRep := m.usageResults.Get(key)

	head := []string{metaRow("Target", sanitizeLabel("s3://"+b+"/"+p), w)}
	if totals := m.usageLine(b, p); totals != "" {
		head = append(head, totals+"\n")
	}
	if hasRep && !rep.Complete {
		head = append(head, warnStyle.Render("partial data — every figure is a lower bound")+
			dimCellStyle.Render(sepDot+keyHint(m.keys.FullScan, "full scan"))+"\n")
	}

	sections := []healthSection{{lines: head}}
	if hasRep {
		lb := ""
		if !rep.Complete {
			lb = "≥"
		}
		sections = append(sections,
			healthSection{lines: distLines("Age", storage.AgeBucketLabels, rep.AgeDist[:], rep.TotalCount, lb), collapse: 1, summary: "Age"},
			healthSection{lines: distLines("Size", storage.SizeBucketLabels, rep.SizeDist[:], rep.TotalCount, lb), collapse: 2, summary: "Size"},
			healthSection{lines: classLines(rep, lb, w), collapse: 3, summary: "Classes"},
		)
		if warnLine := m.smallObjectWarning(rep); warnLine != "" {
			sections = append(sections, healthSection{lines: []string{warnLine + "\n"}})
		}
	}
	sections = append(sections, healthSection{lines: m.mpuLines(key)})

	total := 0
	for _, s := range sections {
		total += len(s.lines) + 1 // +1: blank separator
	}
	// Collapse lower-priority sections (highest collapse value first) until it fits.
	for pass := 3; pass >= 1 && total > rows; pass-- {
		for i := range sections {
			if sections[i].collapse == pass && len(sections[i].lines) > 1 {
				total -= len(sections[i].lines) - 1
				sections[i].lines = []string{
					colHeadStyle.Render(sections[i].summary) + dimCellStyle.Render("  (collapsed — "+keyHint(m.keys.CopyMenu, "export")+" for full data)") + "\n",
				}
			}
		}
	}

	var sb strings.Builder
	for i, s := range sections {
		if i > 0 {
			sb.WriteString("\n")
		}
		for _, l := range s.lines {
			sb.WriteString(l)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// distLines renders one 6-row histogram: label, share bar by count, count + bytes.
func distLines(title string, labels [6]string, dist []storage.DistBucket, totalCount int, lb string) []string {
	out := []string{colHeadStyle.Render(title) + "\n"}
	for i, bkt := range dist {
		share := 0.0
		if totalCount > 0 {
			share = float64(bkt.Count) / float64(totalCount) * 100
		}
		out = append(out, objCellStyle.Render(pad(labels[i], 12))+" "+
			dimCellStyle.Render(usageBar(share))+
			dimCellStyle.Render(fmt.Sprintf("  %s%d obj · %s%s", lb, bkt.Count, lb, humanSize(bkt.Size)))+"\n")
	}
	return out
}

// classLines renders the storage-class spread on one line, largest share first.
func classLines(rep *storage.UsageReport, lb string, w int) []string {
	type cs struct {
		name string
		b    storage.DistBucket
	}
	classes := make([]cs, 0, len(rep.ClassDist))
	for name, b := range rep.ClassDist {
		classes = append(classes, cs{name, b})
	}
	sort.Slice(classes, func(i, j int) bool {
		if classes[i].b.Size != classes[j].b.Size {
			return classes[i].b.Size > classes[j].b.Size
		}
		return classes[i].name < classes[j].name
	})
	parts := make([]string, 0, len(classes))
	for _, c := range classes {
		share := 0.0
		if rep.TotalSize > 0 {
			share = float64(c.b.Size) / float64(rep.TotalSize) * 100
		}
		parts = append(parts, fmt.Sprintf("%s %s%.0f%%", c.name, lb, share))
	}
	return []string{
		colHeadStyle.Render("Classes") + "\n",
		objCellStyle.Render(truncate(strings.Join(parts, sepDot), max(1, w))) + "\n",
	}
}

// smallObjectWarning fires when more than the configured share of objects falls below
// the configured threshold — computed from the SizeDist buckets fully below it
// (017 FR-023). Text names both numbers; ⚠ is a text marker (NO_COLOR-safe).
func (m App) smallObjectWarning(rep *storage.UsageReport) string {
	if rep.TotalCount == 0 {
		return ""
	}
	threshold := int64(m.healthKiB) * 1024
	small := 0
	edges := []int64{128 << 10, 1 << 20, 16 << 20, 128 << 20, 1 << 30}
	for i, bkt := range rep.SizeDist {
		if i < len(edges) && edges[i] <= threshold {
			small += bkt.Count
		}
	}
	share := float64(small) / float64(rep.TotalCount)
	if share <= m.healthShare {
		return ""
	}
	return warnStyle.Render(fmt.Sprintf("⚠ %.0f%% of objects < %d KiB — small-object index pressure", share*100, m.healthKiB))
}

// mpuLines renders the incomplete-multipart block per its tri-state — denied and
// unsupported are explicit, never zero-as-clean (FR-022).
func (m App) mpuLines(key cache.Key) []string {
	head := colHeadStyle.Render("Incomplete multipart uploads") + "\n"
	res, ok := m.mpuResults.Get(key)
	if !ok {
		return []string{head, dimCellStyle.Render("loading…") + "\n"}
	}
	switch res.State {
	case storage.ConfigDenied:
		return []string{head, warnStyle.Render("denied — cannot read in-progress uploads") + "\n"}
	case storage.ConfigUnsupported:
		return []string{head, dimCellStyle.Render("unsupported by this backend") + "\n"}
	case storage.ConfigNone:
		return []string{head, dimCellStyle.Render("uploads: none") + "\n"}
	}
	size := humanSize(res.TotalSize)
	if res.SizedCount < res.Count {
		size = fmt.Sprintf("≥%s (%d of %d sized)", size, res.SizedCount, res.Count)
	}
	oldest := ""
	if !res.OldestInitiated.IsZero() {
		oldest = sepDot + "oldest " + relTime(m.now(), res.OldestInitiated)
	}
	return []string{
		head,
		objCellStyle.Render(fmt.Sprintf("%d in progress", res.Count)) +
			dimCellStyle.Render(sepDot+size+oldest) + "\n",
	}
}
