package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// Persistent details/preview pane. Rendered beside the list on wide
// terminals so the highlighted item's metadata + a bounded preview are visible
// without entering the full-screen object view. Instantly-known list fields render
// immediately; the metadata + preview fill in after a debounced load. 016 folds the old
// full-screen usage view in here: bucket/prefix totals render inline, with ONE expandable
// detail section (breakdown XOR tags XOR config) toggled by the MoreDetail key.

// paneView renders the details pane for the current selection within w×rows. It NEVER
// changes m.mode (distinct from the Enter object view).
func (m App) paneView(w, rows int) string {
	if w < 1 {
		w = 1
	}
	switch m.mode {
	case modeBuckets:
		return m.paneBucket(w)
	case modeTree:
		return m.paneTree(w, rows)
	}
	return ""
}

// browseDetailsView renders the adaptive third zone for the bucket browse: the
// highlighted bucket's metadata when focus is on the bucket list, or the selected object's
// metadata + bounded preview when focus is in the objects zone (reusing the 006 pane render).
func (m App) browseDetailsView(w, rows int) string {
	if m.focusZone == zoneObjects {
		return m.paneTree(w, rows)
	}
	return m.paneBucket(w)
}

func (m App) paneBucket(w int) string {
	fb := m.filteredBuckets()
	if m.bucketSel < 0 || m.bucketSel >= len(fb) {
		return dimCellStyle.Render("(no bucket selected)")
	}
	b := fb[m.bucketSel]
	var sb strings.Builder
	sb.WriteString(metaRow("Bucket", sanitizeLabel(b.Name), w))
	sb.WriteString(metaRow("Created", formatDate(b.CreationDate), w))
	if u := m.usageLine(b.Name, "", w); u != "" {
		sb.WriteString(u + "\n")
	}
	if sec := m.detailSectionView(b.Name, "", w); sec != "" {
		sb.WriteString("\n" + sec + "\n")
	}
	sb.WriteString(hintLabelStyle.Render(truncate(m.usageHints(b.Name, "")+sepDot+keyHint(m.keys.Enter, "open"), w)))
	return sb.String()
}

func (m App) paneTree(w, rows int) string {
	e := m.selected()
	if e == nil {
		// Level summary + the level's usage total.
		n := 0
		if m.level != nil {
			n = m.level.count()
		}
		var sb strings.Builder
		sb.WriteString(metaRow("Level", fmt.Sprintf("%d items", n), w))
		if u := m.usageLine(m.bucket, m.prefix, w); u != "" {
			sb.WriteString(u + "\n")
		}
		if sec := m.detailSectionView(m.bucket, m.prefix, w); sec != "" {
			sb.WriteString("\n" + sec + "\n")
		}
		sb.WriteString(hintLabelStyle.Render(truncate(m.usageHints(m.bucket, m.prefix), w)))
		return strings.TrimRight(sb.String(), "\n")
	}
	if e.isDir {
		var sb strings.Builder
		sb.WriteString(metaRow("Folder", sanitizeLabel(e.label), w))
		if u := m.usageLine(m.bucket, e.full, w); u != "" {
			sb.WriteString(u + "\n")
		}
		if sec := m.detailSectionView(m.bucket, e.full, w); sec != "" {
			sb.WriteString("\n" + sec + "\n")
		}
		sb.WriteString(hintLabelStyle.Render(truncate(m.usageHints(m.bucket, e.full)+sepDot+keyHint(m.keys.DeleteChord, "delete")+sepDot+keyHint(m.keys.Enter, "open"), w)))
		return strings.TrimRight(sb.String(), "\n")
	}

	// Object: once the debounced metadata arrives for THIS key, render the full shared
	// field block (reusing metaFieldRows — same as the Enter view, incl. the 016 enriched
	// fields); until then show the instantly-known list fields so the pane is never blank.
	var sb strings.Builder
	if m.paneMeta != nil && m.paneSelKey == e.full {
		sb.WriteString(metaFieldRows(*m.paneMeta, w, m.now()))
	} else {
		sb.WriteString(metaRow("Key", sanitizeLabel(e.label), w))
		if e.obj != nil {
			sb.WriteString(metaRow("Size", fmt.Sprintf("%s (%d B)", humanSize(e.obj.Size), e.obj.Size), w))
			sb.WriteString(metaRow("Modified", formatDate(e.obj.LastModified), w))
		}
		if m.paneSelKey == e.full {
			sb.WriteString(dimCellStyle.Render(pad("Type", metaKeyWidth)) + dimCellStyle.Render("loading…") + "\n")
		}
	}
	// Attributed plugin groups follow the native metadata block.
	if pg := m.enrichGroupsView(m.bucket, e.full, w); pg != "" {
		sb.WriteString("\n" + pg + "\n")
	}
	// tags section (object) — shown only when toggled by MoreDetail.
	if m.detailSection == sectTags {
		sb.WriteString("\n" + m.detailTagsView(e.full, w) + "\n")
	}
	// Bounded preview.
	if m.panePrev != nil && m.paneSelKey == e.full {
		sb.WriteString("\n" + colHeadStyle.Render("Preview") + "\n")
		sb.WriteString(m.panePreviewLines(w, max(1, rows-8)))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// usageLine renders the focused target's inline usage within width w (NOTHING may
// exceed it — overflow used to clip silently at the box border, finding 1).
// Precedence: an IN-FLIGHT scan for this target outranks the cached entry — a full scan
// running over a cached partial must be visibly alive, announcing itself and its running
// totals ( finding 2); then the cached report (a partial as an explicit "≥" lower
// bound + "partial" text marker — NO_COLOR-safe); else "".
func (m App) usageLine(bucket, prefix string, w int) string {
	key := m.usageKey(bucket, prefix)
	if m.usageCh != nil && m.usageScanKey == key {
		verb := "scanning…"
		if m.usageScanFull {
			verb = "full scan…"
		}
		return warnStyle.Render(truncate(fmt.Sprintf("%s %d obj · %s",
			verb, m.usageProg.ScannedCount, humanSize(m.usageProg.ScannedSize)), w))
	}
	rep, ok := m.usageResults.Get(key)
	if !ok {
		return ""
	}
	lb := ""
	if !rep.Complete {
		lb = "≥"
	}
	size := lb + humanSize(rep.TotalSize)
	objs := fmt.Sprintf("%s%d obj", lb, rep.TotalCount)
	marker := ""
	if !rep.Complete {
		marker = " (partial)"
	}
	// Width guard: when the styled segments would overflow, degrade to one
	// single-style truncated line — the "≥" survives at any width.
	if lipgloss.Width(size+" · "+objs+marker) > w {
		st := accentStyle
		if !rep.Complete {
			st = warnStyle
		}
		return st.Render(truncate(size+" · "+objs+marker, w))
	}
	s := accentStyle.Render(size) + dimCellStyle.Render(" · "+objs)
	if marker != "" {
		s += warnStyle.Render(marker)
	}
	return s
}

// usageHints assembles the pane's hint tail for a bucket/prefix target: the MoreDetail
// hint plus — while the target's usage is absent or partial — the explicit full-scan
// affordance.
func (m App) usageHints(bucket, prefix string) string {
	h := keyHint(m.keys.MoreDetail, "detail")
	if m.fullScanHintNeeded(bucket, prefix) {
		h += sepDot + keyHint(m.keys.FullScan, "full scan")
	}
	return h
}

// detailSectionView renders the single open bucket/prefix detail section ( breakdown or
// config). The object tags section is rendered inline in paneTree.
func (m App) detailSectionView(bucket, prefix string, w int) string {
	switch m.detailSection {
	case sectBreakdown:
		return m.detailBreakdownView(bucket, prefix, w)
	case sectConfig:
		return m.detailConfigView(w)
	}
	return ""
}

// detailBreakdownView renders the ranked, largest-first child breakdown with a size + share
// bar. Overflow past the budget is summarised by a "… +N more (i to reveal)" line so
// nothing is silently clipped (constitution VI).
func (m App) detailBreakdownView(bucket, prefix string, w int) string {
	rep, ok := m.usageResults.Get(m.usageKey(bucket, prefix))
	if !ok {
		return colHeadStyle.Render("breakdown") + "\n" + dimCellStyle.Render("scanning…")
	}
	if len(rep.Children) == 0 {
		return colHeadStyle.Render("breakdown") + "\n" + emptyStyle.Render("(nothing beneath this prefix)")
	}
	var sb strings.Builder
	sb.WriteString(colHeadStyle.Render("breakdown") + "\n")
	const maxRows = 8
	shown, more := rep.Children, 0
	if len(shown) > maxRows {
		more = len(shown) - maxRows
		shown = shown[:maxRows]
	}
	nameW := max(1, w-22)
	for _, c := range shown {
		share := 0.0
		if rep.TotalSize > 0 {
			share = float64(c.Size) / float64(rep.TotalSize) * 100
		}
		sb.WriteString(objCellStyle.Render(pad(truncate(sanitizeLabel(c.Name), nameW), nameW)) + " " +
			dimCellStyle.Render(usageBar(share)) + "\n")
	}
	if more > 0 {
		sb.WriteString(dimCellStyle.Render(fmt.Sprintf("… +%d more (%s to reveal)", more, glyph(firstBind(m.keys.Reveal)))))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// detailConfigView renders the bucket configuration tri-state. Each item is a text
// label (none / unknown-denied / unsupported / a summary), distinguishable under NO_COLOR.
func (m App) detailConfigView(w int) string {
	if m.bucketCfg == nil {
		return colHeadStyle.Render("config") + "\n" + dimCellStyle.Render("loading…")
	}
	c := m.bucketCfg
	var sb strings.Builder
	sb.WriteString(colHeadStyle.Render("config") + "\n")
	sb.WriteString(metaRow("Versioning", configLabel(c.Versioning), w))
	sb.WriteString(metaRow("Encryption", configLabel(c.Encryption), w))
	sb.WriteString(metaRow("Lifecycle", configLabel(c.Lifecycle), w))
	sb.WriteString(metaRow("Replication", configLabel(c.Replication), w))
	sb.WriteString(metaRow("Public access", configLabel(c.PublicAccessBlock), w))
	sb.WriteString(metaRow("Location", configLabel(c.Location), w))
	return strings.TrimRight(sb.String(), "\n")
}

// detailTagsView renders an object's tag key/values: "loading…", "unknown (denied)"
// "none", or the sorted pairs.
func (m App) detailTagsView(key string, w int) string {
	head := colHeadStyle.Render("tags") + "\n"
	if m.objectTags == nil || m.detailKey != key {
		return head + dimCellStyle.Render("loading…")
	}
	if m.tagsErr != nil {
		return head + dimCellStyle.Render("unknown (denied)")
	}
	if len(m.objectTags.Tags) == 0 {
		return head + dimCellStyle.Render("none")
	}
	keys := make([]string, 0, len(m.objectTags.Tags))
	for k := range m.objectTags.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(head)
	for _, k := range keys {
		sb.WriteString(metaRow(sanitizeLabel(k), sanitizeLabel(m.objectTags.Tags[k]), w))
	}
	return strings.TrimRight(sb.String(), "\n")
}

// configLabel maps a tri-state config item to a NO_COLOR-distinguishable text label
// keeping the "none" / "unknown (denied)" / "unsupported" distinction explicit.
func configLabel(it storage.ConfigItem) string {
	switch it.State {
	case storage.ConfigConfigured:
		if it.Detail != "" {
			return it.Detail
		}
		return "configured"
	case storage.ConfigDenied:
		return "unknown (denied)"
	case storage.ConfigUnsupported:
		return "unsupported"
	default:
		return "none"
	}
}

// panePreviewLines renders a short, bounded preview of the debounced payload, reusing the
// shared text-windowing and summary helpers (no forked preview logic).
func (m App) panePreviewLines(w, rows int) string {
	p := m.panePrev
	if p == nil {
		return ""
	}
	if p.Kind == preview.KindText {
		return textPreviewLines(p.Data, 0, w, rows)
	}
	return dimCellStyle.Render(wrapText(preview.Summary(*p), w))
}
