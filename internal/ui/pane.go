package ui

import (
	"fmt"
	"strings"

	"github.com/danchupin/s3s/internal/preview"
)

// Persistent details/preview pane. Rendered beside the list on wide
// terminals so the highlighted item's metadata + a bounded preview are visible
// without entering the full-screen object view. Instantly-known list fields render
// immediately; the metadata + preview fill in after a debounced load.

// paneView renders the details pane for the current selection within w×rows. It NEVER
// changes m.mode (distinct from the Enter object view). For a folder/level selection it
// shows a summary; for an object it shows list-known fields immediately plus the
// debounced HeadObject metadata and a short preview; loading/empty/error states
// are explicit.
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
// The object metadata is filled in by the existing debounced pane load (afterSelectionMove →
// onPaneTick), armed as the objects cursor moves.
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
	sb.WriteString("\n" + hintLabelStyle.Render(keyHint(m.keys.Analyze, "analyze")+" · "+keyHint(m.keys.Enter, "open")))
	return sb.String()
}

func (m App) paneTree(w, rows int) string {
	e := m.selected()
	if e == nil {
		// Level summary.
		n := 0
		if m.level != nil {
			n = m.level.count()
		}
		return metaRow("Level", fmt.Sprintf("%d items", n), w) +
			"\n" + hintLabelStyle.Render(keyHint(m.keys.Analyze, "analyze this level"))
	}
	if e.isDir {
		return metaRow("Folder", sanitizeLabel(e.label), w) +
			"\n" + hintLabelStyle.Render(keyHint(m.keys.Analyze, "analyze")+" · "+keyHint(m.keys.DeleteChord, "delete")+" · "+keyHint(m.keys.Enter, "open"))
	}

	// Object: once the debounced metadata arrives for THIS key, render the full shared
	// field block (reusing metaFieldRows — same as the Enter view); until then show the
	// instantly-known list fields so the pane is never blank during the fetch.
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
	// Bounded preview.
	if m.panePrev != nil && m.paneSelKey == e.full {
		sb.WriteString("\n" + colHeadStyle.Render("Preview") + "\n")
		sb.WriteString(m.panePreviewLines(w, max(1, rows-8)))
	}
	return strings.TrimRight(sb.String(), "\n")
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
