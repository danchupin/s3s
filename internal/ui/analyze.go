package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// du analytics (005 US2). A read: recursively aggregates a prefix into total
// size/count + a ranked immediate-child breakdown (storage.UsageOf), rendered in
// modeUsage with live progress, cancel, and drill-down. Works read-only.

// usageEvent is one item on the scan's progress channel.
type usageEvent struct {
	progress storage.UsageProgress
	done     bool
	report   storage.UsageReport
	err      error
}

// startAnalyze begins a du scan. From the bucket list it analyzes the highlighted
// bucket (whole bucket); in the tree it analyzes the selected folder, or the current
// level's prefix when no folder is selected.
func (m App) startAnalyze() (tea.Model, tea.Cmd) {
	bucket, prefix := m.usageTarget()
	if bucket == "" {
		return m, nil
	}
	return m.runAnalyze(bucket, prefix)
}

// usageTarget resolves the (bucket, prefix) to analyze from the current selection.
func (m App) usageTarget() (bucket, prefix string) {
	switch m.mode {
	case modeBuckets:
		fb := m.filteredBuckets()
		if m.bucketSel >= 0 && m.bucketSel < len(fb) {
			return fb[m.bucketSel].Name, ""
		}
	case modeTree:
		if e := m.selected(); e != nil && e.isDir {
			return m.bucket, e.full // selected folder
		}
		return m.bucket, m.prefix // the current level itself
	}
	return "", ""
}

// runAnalyze enters modeUsage and dispatches the scan off the event loop.
func (m App) runAnalyze(bucket, prefix string) (tea.Model, tea.Cmd) {
	m.mode = modeUsage
	m.usage = nil
	m.usageSel = 0
	m.usageBucket = bucket
	m.usagePrefix = prefix
	m.usageProg = storage.UsageProgress{}
	ctx := (&m).beginLoad()
	ch := make(chan usageEvent, 8)
	m.usageCh = ch
	return m, tea.Batch(analyzeCmd(ctx, m.activeStore(), bucket, prefix, ch, m.gen), spinnerTick())
}

// analyzeCmd runs UsageOf off the event loop, streaming running totals and a terminal
// report on ch (Constitution II, FR-011).
func analyzeCmd(ctx context.Context, st storage.Storage, bucket, prefix string, ch chan usageEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			rep, err := st.UsageOf(ctx, bucket, prefix, func(p storage.UsageProgress) {
				select {
				case ch <- usageEvent{progress: p}:
				default: // drop a tick rather than stall the scan
				}
			})
			ch <- usageEvent{done: true, report: rep, err: err}
		}()
		return waitForUsage(ch, gen)()
	}
}

// waitForUsage reads ONE scan event, re-arming until the terminal done event arrives.
func waitForUsage(ch chan usageEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok || ev.done {
			return usageDoneMsg{gen: gen, report: ev.report, err: ev.err}
		}
		return usageProgressMsg{gen: gen, p: ev.progress}
	}
}

// onUsageProgress stores a running-totals tick and re-arms the wait command.
func (m App) onUsageProgress(msg usageProgressMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen || m.mode != modeUsage {
		return m, nil // superseded
	}
	m.usageProg = msg.p
	return m, waitForUsage(m.usageCh, m.gen)
}

// onUsageDone applies the terminal report.
func (m App) onUsageDone(msg usageDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil // superseded
	}
	m.loading = false
	m.usageCh = nil
	if msg.err != nil && !errorsIsCanceled(msg.err) {
		m.err = msg.err
		m.mode = modeTree
		return m, nil
	}
	rep := msg.report
	m.usage = &rep
	if m.usageSel >= len(rep.Children) {
		m.usageSel = max(0, len(rep.Children)-1)
	}
	return m, nil
}

// onUsageKey handles navigation, drill-down, and exit while in modeUsage.
func (m App) onUsageKey(key string) (tea.Model, tea.Cmd) {
	n := 0
	if m.usage != nil {
		n = len(m.usage.Children)
	}
	switch {
	case matches(key, m.keys.Up):
		if m.usageSel > 0 {
			m.usageSel--
		}
	case matches(key, m.keys.Down):
		if m.usageSel < n-1 {
			m.usageSel++
		}
	case matches(key, m.keys.Top):
		m.usageSel = 0
	case matches(key, m.keys.Bottom):
		m.usageSel = max(0, n-1)
	case matches(key, m.keys.Enter):
		// Drill into the selected sub-prefix to locate the exact consumer (FR-013).
		if m.usage != nil && m.usageSel >= 0 && m.usageSel < n {
			c := m.usage.Children[m.usageSel]
			if c.IsDir {
				return m.runAnalyze(m.usageBucket, m.usagePrefix+c.Name)
			}
		}
	case matches(key, m.keys.Back):
		// Leave the analytics view back to the tree (cancel any running scan).
		(&m).cancelLoad()
		m.mode = modeTree
		m.usage = nil
		m.usageCh = nil
	}
	return m, nil
}

// usageTitle is the box title for the analytics view.
func (m App) usageTitle() string {
	loc := m.usageBucket
	if m.usagePrefix != "" {
		loc += "/" + strings.TrimSuffix(sanitizeLabel(m.usagePrefix), "/")
	}
	return "du " + loc
}

// usageView renders the totals header plus the ranked immediate-child breakdown with a
// size bar and share-of-parent (005 FR-009/FR-012).
func (m App) usageView(w, rows int) string {
	if m.usage == nil {
		// Still scanning — show running totals so the wait reads as intentional.
		return dimCellStyle.Render(fmt.Sprintf("scanning… %d objects, %s so far  (Esc to cancel)",
			m.usageProg.ScannedCount, humanSize(m.usageProg.ScannedSize)))
	}
	rep := m.usage
	header := accentStyle.Render(fmt.Sprintf("total %s", humanSize(rep.TotalSize))) +
		dimCellStyle.Render(fmt.Sprintf("  ·  %d objects", rep.TotalCount))
	if !rep.Complete {
		header += warnStyle.Render("  (partial — cancelled)")
	}
	if len(rep.Children) == 0 {
		return header + "\n\n" + emptyStyle.Render("empty — nothing beneath this prefix")
	}

	off, end := windowBounds(len(rep.Children), m.usageSel, rows-2)
	cols := []column{{"child", 0}, {"size", 12}, {"share", 14}}
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		c := rep.Children[i]
		share := 0.0
		if rep.TotalSize > 0 {
			share = float64(c.Size) / float64(rep.TotalSize) * 100
		}
		name := sanitizeLabel(c.Name)
		if !c.IsDir {
			name = "  " + name // visually distinguish a direct object from a sub-prefix
		}
		data = append(data, []string{name, humanSize(c.Size), usageBar(share)})
	}
	return header + "\n" + renderTable(w, cols, data, nil, m.usageSel-off)
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

// errorsIsCanceled reports a context-cancellation error (a cancelled scan is not a
// hard error — it yields a partial report).
func errorsIsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
