package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/danchupin/s3s/internal/storage"
)

// metaKeyWidth is the fixed label column width in the metadata pane.
const metaKeyWidth = 14

// metaRow renders one fixed-width label + truncated value line. Shared by the full-screen
// object view (metaPane) and the persistent details pane (006 US2), so both render the
// same field column.
func metaRow(k, v string, w int) string {
	valW := w - metaKeyWidth
	if valW < 1 {
		valW = 1
	}
	return metaKeyStyle.Render(pad(k, metaKeyWidth)) + metaValStyle.Render(truncate(v, valW)) + "\n"
}

// metaFieldRows renders the standard object field block (Key/Size/Modified/Type/Class/
// ETag) for the given metadata at width w. The single source of truth for both the Enter
// object view and the details pane.
func metaFieldRows(md storage.ObjectMetadata, w int) string {
	var b strings.Builder
	b.WriteString(metaRow("Key", sanitizeLabel(md.Key), w))
	b.WriteString(metaRow("Size", fmt.Sprintf("%s (%d B)", humanSize(md.Size), md.Size), w))
	b.WriteString(metaRow("Modified", formatDate(md.LastModified), w))
	b.WriteString(metaRow("Type", orDash(md.ContentType), w))
	b.WriteString(metaRow("Class", orDash(md.StorageClass), w))
	b.WriteString(metaRow("ETag", orDash(md.ETag), w))
	return b.String()
}

// metaPane renders the object metadata block within width w (FR-013). It is the
// left column of the combined object view; access-denied and other errors surface
// in the footer (errorText).
func (m App) metaPane(w int) string {
	if m.meta == nil {
		if m.loading {
			return dimCellStyle.Render("metadata…")
		}
		if txt := m.errorText(); txt != "" {
			return errStyle.Render(truncate(txt, w))
		}
		return dimCellStyle.Render("(no metadata)")
	}
	md := m.meta
	var b strings.Builder
	b.WriteString(metaFieldRows(*md, w))

	if len(md.UserMetadata) > 0 {
		b.WriteString("\n" + colHeadStyle.Render("User metadata") + "\n")
		keys := make([]string, 0, len(md.UserMetadata))
		for k := range md.UserMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(metaRow(sanitizeLabel(k), sanitizeLabel(md.UserMetadata[k]), w))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
