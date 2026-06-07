package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/preview"
)

// onObjectKey handles the combined object view: scroll content, Esc/← back.
func (m App) onObjectKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case matches(key, m.keys.Back):
		m.mode = m.objReturn // restore the browse mode we opened from
		m.meta = nil
		m.prev = nil
		m.prevOff = 0
	case matches(key, m.keys.Down):
		m.prevOff++
	case matches(key, m.keys.Up):
		if m.prevOff > 0 {
			m.prevOff--
		}
	case matches(key, m.keys.Top):
		m.prevOff = 0
	}
	return m, nil
}

// objectView lays out the combined object view. Images get the full width
// (metadata as a compact strip above) so the half-block render is as sharp as the
// cell grid allows; text/binary keep a metadata column beside the content.
func (m App) objectView(w, rows int) string {
	if m.prev != nil && m.prev.Kind == preview.KindImage {
		meta := m.metaPane(w)
		metaH := strings.Count(meta, "\n") + 1
		imgRows := rows - metaH - 1
		if imgRows < 1 {
			imgRows = 1
		}
		content := m.contentPane(w, imgRows)
		rule := dimCellStyle.Render(strings.Repeat("─", w))
		return meta + "\n" + rule + "\n" + content
	}

	metaW := w / 3
	if metaW > 40 {
		metaW = 40
	}
	if metaW < 18 {
		metaW = 18
	}
	if metaW > w-12 {
		metaW = max(1, w-12)
	}
	contentW := w - metaW - 3
	if contentW < 1 {
		contentW = 1
	}

	meta := lipgloss.NewStyle().Width(metaW).Render(m.metaPane(metaW))
	sep := vrule(rows)
	content := m.contentPane(contentW, rows)
	return lipgloss.JoinHorizontal(lipgloss.Top, meta, " ", sep, " ", content)
}

// vrule returns a vertical divider of n rows.
func vrule(n int) string {
	if n < 1 {
		n = 1
	}
	line := dimCellStyle.Render("│")
	lines := make([]string, n)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// contentPane renders the object content within width w / rows.
func (m App) contentPane(w, rows int) string {
	if m.prev == nil {
		if m.loading {
			return dimCellStyle.Render("content…")
		}
		if txt := m.errorText(); txt != "" {
			return errStyle.Render(truncate(txt, w))
		}
		return dimCellStyle.Render("(no content)")
	}
	p := m.prev
	head := ""
	if p.Truncated {
		head = warnStyle.Render("[truncated at 5 MiB]") + "\n"
	}

	switch p.Kind {
	case preview.KindText:
		return head + m.renderTextPreview(p, w, rows)
	case preview.KindImage:
		if w < 1 {
			w = 1
		}
		img, err := preview.RenderImage(p.Data, m.imgProto, w, rows)
		if err != nil {
			return head + warnStyle.Render("image preview unavailable") + "\n" + wrapText(preview.Summary(*p), w)
		}
		return head + img
	default:
		return head + wrapText(preview.Summary(*p), w)
	}
}

// renderTextPreview windows the text content by the scroll offset, truncated to w.
func (m App) renderTextPreview(p *preview.Payload, w, rows int) string {
	return textPreviewLines(p.Data, m.prevOff, w, rows)
}

// textPreviewLines windows raw text content from offset off, truncating each of up to rows
// lines to width w. Shared by the full-screen object view and the details pane,
// so text windowing/sanitization stays consistent.
func textPreviewLines(data []byte, off, w, rows int) string {
	if rows < 1 {
		rows = 1
	}
	lines := strings.Split(string(data), "\n")
	if off > len(lines)-1 {
		off = max(0, len(lines)-1)
	}
	if off < 0 {
		off = 0
	}
	end := min(off+rows, len(lines))

	var b strings.Builder
	for i := off; i < end; i++ {
		b.WriteString(truncate(sanitizeLabel(lines[i]), w))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// wrapText truncates each line of s to width w (summaries are short one-liners).
func wrapText(s string, w int) string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		out = append(out, truncate(line, w))
	}
	return strings.Join(out, "\n")
}
