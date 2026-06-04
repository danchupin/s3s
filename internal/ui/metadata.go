package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

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
			return "Loading metadata…"
		}
		if txt := m.errorText(); txt != "" {
			return txt
		}
		return ""
	}
	md := m.meta
	var b strings.Builder
	fmt.Fprintf(&b, "Key:            %s\n", sanitizeLabel(md.Key))
	fmt.Fprintf(&b, "Size:           %s (%d bytes)\n", humanSize(md.Size), md.Size)
	fmt.Fprintf(&b, "Last modified:  %s\n", formatTime(md.LastModified))
	fmt.Fprintf(&b, "Content type:   %s\n", orDash(md.ContentType))
	fmt.Fprintf(&b, "Storage class:  %s\n", orDash(md.StorageClass))
	fmt.Fprintf(&b, "ETag:           %s\n", orDash(md.ETag))

	if len(md.UserMetadata) > 0 {
		b.WriteString("\nUser metadata:\n")
		keys := make([]string, 0, len(md.UserMetadata))
		for k := range md.UserMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s: %s\n", sanitizeLabel(k), sanitizeLabel(md.UserMetadata[k]))
		}
	}
	b.WriteString("\n(Esc/← back)")
	return b.String()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format(time.RFC3339)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
