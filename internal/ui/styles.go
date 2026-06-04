package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Version is shown in the header bar. Overridable at build time via
// -ldflags "-X github.com/danchupin/s3s/internal/ui.Version=vX.Y.Z".
var Version = "dev"

// Palette (256-color), k9s-ish.
var (
	colAccent = lipgloss.Color("39")  // cyan/blue
	colPink   = lipgloss.Color("213") // context value
	colDir    = lipgloss.Color("75")  // directories
	colText   = lipgloss.Color("252")
	colDim    = lipgloss.Color("245")
	colWarn   = lipgloss.Color("214")
	colErr    = lipgloss.Color("203")
	colSelBg  = lipgloss.Color("24")
	colSelFg  = lipgloss.Color("231")
)

var (
	hdrKeyStyle = lipgloss.NewStyle().Foreground(colWarn)
	hdrValStyle = lipgloss.NewStyle().Bold(true).Foreground(colText)
	hdrNumStyle = lipgloss.NewStyle().Bold(true).Foreground(colPink)
	roStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("84"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	ruleStyle   = lipgloss.NewStyle().Foreground(colAccent)

	colHeadStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	selRowStyle  = lipgloss.NewStyle().Background(colSelBg).Foreground(colSelFg)
	dirCellStyle = lipgloss.NewStyle().Bold(true).Foreground(colDir)
	objCellStyle = lipgloss.NewStyle().Foreground(colText)
	dimCellStyle = lipgloss.NewStyle().Foreground(colDim)

	metaKeyStyle = lipgloss.NewStyle().Foreground(colAccent)
	metaValStyle = lipgloss.NewStyle().Foreground(colText)

	footerStyle = lipgloss.NewStyle().Foreground(colDim)
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(colErr)
	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	emptyStyle  = lipgloss.NewStyle().Faint(true).Foreground(colDim)
)

// column is a table column; width 0 means flex (absorbs remaining width).
type column struct {
	title string
	width int
}

// clampW returns at least 1.
func clampW(w int) int {
	if w < 1 {
		return 1
	}
	return w
}

// layoutWidths assigns concrete widths to columns within the total width,
// accounting for a 2-char gutter and one space between columns.
func layoutWidths(width int, cols []column) []int {
	w := make([]int, len(cols))
	fixed, flex := 0, -1
	for i, c := range cols {
		if c.width > 0 {
			w[i] = c.width
			fixed += c.width
		} else {
			flex = i
		}
	}
	if flex >= 0 {
		rem := width - 2 - fixed - (len(cols) - 1)
		if rem < 6 {
			rem = 6
		}
		w[flex] = rem
	}
	return w
}

// truncate shortens s to display width w, adding an ellipsis when cut.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	width := 0
	for _, ch := range s {
		cw := lipgloss.Width(string(ch))
		if width+cw > w-1 {
			break
		}
		b.WriteRune(ch)
		width += cw
	}
	b.WriteRune('…')
	return b.String()
}

// pad truncates then right-pads s to width w.
func pad(s string, w int) string {
	s = truncate(s, w)
	if gap := w - lipgloss.Width(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

// renderTable renders a k9s-style table: bold column header, a rule, then rows.
// The selected row is reverse-highlighted with a "▶" gutter; dirFlags (optional,
// indexed like rows) colors the NAME column (col 0) as a directory.
func renderTable(width int, cols []column, rows [][]string, dirFlags []bool, sel int) string {
	widths := layoutWidths(width, cols)

	var b strings.Builder
	hdr := "  "
	for i, c := range cols {
		hdr += pad(strings.ToUpper(c.title), widths[i])
		if i < len(cols)-1 {
			hdr += " "
		}
	}
	b.WriteString(colHeadStyle.Render(hdr))
	b.WriteByte('\n')
	b.WriteString(ruleStyle.Render(strings.Repeat("─", clampW(width))))
	b.WriteByte('\n')

	for ri, row := range rows {
		cells := make([]string, len(cols))
		for ci := range cols {
			v := ""
			if ci < len(row) {
				v = sanitizeLabel(row[ci])
			}
			cells[ci] = pad(v, widths[ci])
		}
		if ri == sel {
			line := "▶ " + strings.Join(cells, " ")
			b.WriteString(selRowStyle.Width(clampW(width)).Render(line))
		} else {
			nameStyle := objCellStyle
			if dirFlags != nil && ri < len(dirFlags) && dirFlags[ri] {
				nameStyle = dirCellStyle
			}
			cells[0] = nameStyle.Render(cells[0])
			for ci := 1; ci < len(cells); ci++ {
				cells[ci] = dimCellStyle.Render(cells[ci])
			}
			b.WriteString("  " + strings.Join(cells, " "))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// windowBounds returns the [off,end) slice of n items that keeps sel visible
// within rows lines (bottom-anchored scrolling).
func windowBounds(n, sel, rows int) (int, int) {
	if rows < 1 {
		rows = 1
	}
	if n <= rows {
		return 0, n
	}
	off := sel - rows + 1
	if off < 0 {
		off = 0
	}
	if off > n-rows {
		off = n - rows
	}
	return off, off + rows
}

// padLine right-pads a (possibly ANSI-styled) line to width w (no truncation).
func padLine(s string, w int) string {
	if gap := w - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// boxView wraps body in a rounded border (k9s-style). The top border carries a
// left resource label and a centered, highlighted selection label. Body lines are
// padded to the inner width and to at least minRows rows.
func boxView(left, center, body string, width, minRows int) string {
	inner := width - 2
	if inner < 1 {
		inner = 1
	}

	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = padLine(lines[i], inner)
	}
	for len(lines) < minRows {
		lines = append(lines, strings.Repeat(" ", inner))
	}

	// Top border: "╭─ left ─── «center» ───╮"
	leftPlain := "─ " + left + " "
	centerPlain := ""
	if center != "" {
		centerPlain = " " + truncate(center, max(0, inner-lipgloss.Width(leftPlain)-4)) + " "
	}
	wl := lipgloss.Width(leftPlain)
	wc := lipgloss.Width(centerPlain)
	leftDashes := (inner-wc)/2 - wl
	if leftDashes < 1 {
		leftDashes = 1
	}
	rightDashes := inner - wl - leftDashes - wc
	if rightDashes < 0 {
		rightDashes = 0
		leftDashes = inner - wl - wc
		if leftDashes < 0 {
			leftDashes = 0
		}
	}
	topInner := ruleStyle.Render("─ ") + titleStyle.Render(left) + ruleStyle.Render(" "+strings.Repeat("─", leftDashes))
	if centerPlain != "" {
		topInner += selRowStyle.Render(centerPlain)
	}
	topInner += ruleStyle.Render(strings.Repeat("─", rightDashes))

	var b strings.Builder
	b.WriteString(ruleStyle.Render("╭") + topInner + ruleStyle.Render("╮") + "\n")
	for _, l := range lines {
		b.WriteString(ruleStyle.Render("│") + l + ruleStyle.Render("│") + "\n")
	}
	b.WriteString(ruleStyle.Render("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}

// formatDate renders a date compactly, or an em dash when zero.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}
