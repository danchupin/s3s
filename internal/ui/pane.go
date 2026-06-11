package ui

import (
	"fmt"
	"sort"
	"strings"

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
	if u := m.usageLine(b.Name, ""); u != "" {
		sb.WriteString(u + "\n")
	}
	if sec := m.detailSectionView(b.Name, "", w); sec != "" {
		sb.WriteString("\n" + sec + "\n")
	}
	sb.WriteString(hintLabelStyle.Render(keyHint(m.keys.MoreDetail, "detail") + sepDot + keyHint(m.keys.Enter, "open")))
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
		if u := m.usageLine(m.bucket, m.prefix); u != "" {
			sb.WriteString(u + "\n")
		}
		if sec := m.detailSectionView(m.bucket, m.prefix, w); sec != "" {
			sb.WriteString("\n" + sec + "\n")
		}
		sb.WriteString(hintLabelStyle.Render(keyHint(m.keys.MoreDetail, "detail")))
		return strings.TrimRight(sb.String(), "\n")
	}
	if e.isDir {
		var sb strings.Builder
		sb.WriteString(metaRow("Folder", sanitizeLabel(e.label), w))
		if u := m.usageLine(m.bucket, e.full); u != "" {
			sb.WriteString(u + "\n")
		}
		if sec := m.detailSectionView(m.bucket, e.full, w); sec != "" {
			sb.WriteString("\n" + sec + "\n")
		}
		sb.WriteString(hintLabelStyle.Render(keyHint(m.keys.MoreDetail, "detail") + sepDot + keyHint(m.keys.DeleteChord, "delete") + sepDot + keyHint(m.keys.Enter, "open")))
		return strings.TrimRight(sb.String(), "\n")
	}

	// Object: once the debounced metadata arrives for THIS key, render the full shared
	// field block (reusing metaFieldRows — same as the Enter view, incl. the 016 enriched
	// fields); until then show the instantly-known list fields so the pane is never blank.
	var sb strings.Builder
	if m.paneMeta != nil && m.paneSelKey == e.full {
		sb.WriteString(metaFieldRows(*m.paneMeta, w))
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
	// US4 tags section (object) — shown only when toggled by MoreDetail.
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

// usageLine renders the focused target's inline total — the cached report's
// "total <size> · N objects" (with a partial marker), or a running "scanning…" line while
// the scan is in flight, or "" when there is nothing to show (016 US2).
func (m App) usageLine(bucket, prefix string) string {
	key := m.usageKey(bucket, prefix)
	if rep, ok := m.usageResults.Get(key); ok {
		s := accentStyle.Render(fmt.Sprintf("total %s", humanSize(rep.TotalSize))) +
			dimCellStyle.Render(fmt.Sprintf("  ·  %d objects", rep.TotalCount))
		if !rep.Complete {
			s += warnStyle.Render("  (partial)")
		}
		return s
	}
	if m.usageCh != nil && m.usageScanKey == key {
		return dimCellStyle.Render(fmt.Sprintf("scanning… %d objects, %s so far",
			m.usageProg.ScannedCount, humanSize(m.usageProg.ScannedSize)))
	}
	return ""
}

// detailSectionView renders the single open bucket/prefix detail section (US3 breakdown or
// US4 config). The object tags section is rendered inline in paneTree.
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
// bar (US3). Overflow past the budget is summarised by a "… +N more (i to reveal)" line so
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

// detailConfigView renders the bucket configuration tri-state (US4). Each item is a text
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

// detailTagsView renders an object's tag key/values (US4): "loading…", "unknown (denied)",
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

// configLabel maps a tri-state config item to a NO_COLOR-distinguishable text label,
// keeping the "none" / "unknown (denied)" / "unsupported" distinction explicit (FR-013).
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
