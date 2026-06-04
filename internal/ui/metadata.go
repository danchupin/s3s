package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// onMetadataKey handles the metadata pane (Esc/← returns to the tree).
func (m App) onMetadataKey(key string) (tea.Model, tea.Cmd) {
	if matches(key, m.keys.Back) {
		m.mode = modeTree
		m.meta = nil
	}
	return m, nil
}

// metadataView renders the object metadata pane (FR-013). Access-denied and other
// errors surface via the footer error line (errorText), keeping copy consistent.
func (m App) metadataView() string {
	if m.meta == nil {
		if m.loading {
			return dimCellStyle.Render("Loading metadata…")
		}
		if txt := m.errorText(); txt != "" {
			return errStyle.Render(txt)
		}
		return ""
	}
	md := m.meta
	var b strings.Builder
	row := func(k, v string) {
		b.WriteString(metaKeyStyle.Render(k) + metaValStyle.Render(v) + "\n")
	}
	row("Key", sanitizeLabel(md.Key))
	row("Size", fmt.Sprintf("%s (%d bytes)", humanSize(md.Size), md.Size))
	row("Last modified", formatDate(md.LastModified))
	row("Content type", orDash(md.ContentType))
	row("Storage class", orDash(md.StorageClass))
	row("ETag", orDash(md.ETag))

	if len(md.UserMetadata) > 0 {
		b.WriteString("\n" + colHeadStyle.Render("User metadata") + "\n")
		keys := make([]string, 0, len(md.UserMetadata))
		for k := range md.UserMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			row(k, sanitizeLabel(md.UserMetadata[k]))
		}
	}
	b.WriteString("\n" + dimCellStyle.Render("(Esc/← back)"))
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
