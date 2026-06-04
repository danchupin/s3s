package ui

import (
	"fmt"
	"sort"
	"strings"
)

// metaKeyWidth is the fixed label column width in the metadata pane.
const metaKeyWidth = 14

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
	valW := w - metaKeyWidth
	if valW < 1 {
		valW = 1
	}
	var b strings.Builder
	row := func(k, v string) {
		b.WriteString(metaKeyStyle.Render(pad(k, metaKeyWidth)) +
			metaValStyle.Render(truncate(v, valW)) + "\n")
	}
	row("Key", sanitizeLabel(md.Key))
	row("Size", fmt.Sprintf("%s (%d B)", humanSize(md.Size), md.Size))
	row("Modified", formatDate(md.LastModified))
	row("Type", orDash(md.ContentType))
	row("Class", orDash(md.StorageClass))
	row("ETag", orDash(md.ETag))

	if len(md.UserMetadata) > 0 {
		b.WriteString("\n" + colHeadStyle.Render("User metadata") + "\n")
		keys := make([]string, 0, len(md.UserMetadata))
		for k := range md.UserMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			row(sanitizeLabel(k), sanitizeLabel(md.UserMetadata[k]))
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
