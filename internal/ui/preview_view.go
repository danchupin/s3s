package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/preview"
)

// onPreviewKey handles the preview pane: scroll text, Esc/← back.
func (m App) onPreviewKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case matches(key, m.keys.Back):
		m.mode = modeTree
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

// previewView renders the preview pane (FR-014/015/016).
func (m App) previewView() string {
	if m.prev == nil {
		if m.loading {
			return "Loading preview…"
		}
		if txt := m.errorText(); txt != "" {
			return txt
		}
		return ""
	}
	p := m.prev
	notice := ""
	if p.Truncated {
		notice = "[preview truncated at 5 MiB]\n\n"
	}

	switch p.Kind {
	case preview.KindText:
		return notice + m.renderTextPreview(p)
	case preview.KindImage:
		cols := m.width
		if cols < 1 {
			cols = 80
		}
		img, err := preview.RenderHalfBlock(p.Data, cols, m.bodyRows())
		if err != nil {
			// Non-decodable or non-capable: safe summary instead of a degraded
			// render (FR-015).
			return notice + "Image preview unavailable.\n" + preview.Summary(*p) + "\n\n(Esc back)"
		}
		return notice + img + "\n\n(Esc back)"
	default:
		return notice + preview.Summary(*p) + "\n\n(Esc back)"
	}
}

// renderTextPreview windows the text by the scroll offset.
func (m App) renderTextPreview(p *preview.Payload) string {
	lines := strings.Split(string(p.Data), "\n")
	rows := m.bodyRows() - 1
	if rows < 1 {
		rows = 1
	}
	off := m.prevOff
	if off > len(lines)-1 {
		off = max(0, len(lines)-1)
	}
	if off < 0 {
		off = 0
	}
	end := min(off+rows, len(lines))

	var b strings.Builder
	for i := off; i < end; i++ {
		b.WriteString(sanitizeLabel(lines[i]))
		b.WriteByte('\n')
	}
	b.WriteString("\n(↑/↓ scroll · Esc back)")
	return b.String()
}
