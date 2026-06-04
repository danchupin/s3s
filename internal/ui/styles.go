package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Version is shown in the header bar. Overridable at build time via
// -ldflags "-X github.com/danchupin/s3s/internal/ui.Version=vX.Y.Z".
var Version = "dev"

// Palette (256-color) — Claude Code-ish: warm coral/orange accent on muted grays.
var (
	colAccent = lipgloss.Color("173") // coral/orange (Claude signature ~#d7875f)
	colDir    = lipgloss.Color("180") // soft tan — directories
	colText   = lipgloss.Color("252")
	colDim    = lipgloss.Color("244") // labels, separators
	colBorder = lipgloss.Color("240") // subtle box borders
	colWarn   = lipgloss.Color("179")
	colErr    = lipgloss.Color("174")
	colOK     = lipgloss.Color("108") // muted green (e.g. [RO])
	colSelBg  = lipgloss.Color("238")
	colSelFg  = lipgloss.Color("223") // warm white

	// distinct footer-segment hues (Claude Code-style colored params)
	colCyan   = lipgloss.Color("109")
	colBlue   = lipgloss.Color("74")
	colPurple = lipgloss.Color("139")
)

var (
	roStyle    = lipgloss.NewStyle().Bold(true).Foreground(colOK)
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	ruleStyle  = lipgloss.NewStyle().Foreground(colBorder)

	colHeadStyle = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	selRowStyle  = lipgloss.NewStyle().Background(colSelBg).Foreground(colSelFg)
	dirCellStyle = lipgloss.NewStyle().Bold(true).Foreground(colDir)
	objCellStyle = lipgloss.NewStyle().Foreground(colText)
	dimCellStyle = lipgloss.NewStyle().Foreground(colDim)

	metaKeyStyle = lipgloss.NewStyle().Foreground(colAccent)
	metaValStyle = lipgloss.NewStyle().Foreground(colText)

	accentStyle = lipgloss.NewStyle().Foreground(colAccent)

	// footer segment styles (one hue per parameter type)
	segCtxStyle      = lipgloss.NewStyle().Bold(true).Foreground(colOK)
	segClusterStyle  = lipgloss.NewStyle().Foreground(colAccent)
	segUserStyle     = lipgloss.NewStyle().Foreground(colCyan)
	segEndpointStyle = lipgloss.NewStyle().Foreground(colBlue)
	segRegionStyle   = lipgloss.NewStyle().Foreground(colPurple)

	errStyle   = lipgloss.NewStyle().Bold(true).Foreground(colErr)
	warnStyle  = lipgloss.NewStyle().Foreground(colWarn)
	emptyStyle = lipgloss.NewStyle().Faint(true).Foreground(colDim)
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

// fseg is a pre-styled footer segment paired with its display width.
type fseg struct {
	s string
	w int
}

// joinFit joins segments with dim "·" separators, dropping trailing ones that
// would overflow width (so the line never wraps and hide following footer lines).
func joinFit(width int, segs []fseg) string {
	sep := dimCellStyle.Render(" · ")
	const sepW = 3
	var b strings.Builder
	used := 0
	for i, sg := range segs {
		add := sg.w
		if i > 0 {
			add += sepW
		}
		if used+add > width {
			break
		}
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(sg.s)
		used += add
	}
	return b.String()
}

// footerInfoLine builds the colored info line: each parameter gets its own hue.
func footerInfoLine(width int, ctx, cluster, user, endpoint, region, rev string) string {
	segs := []fseg{{
		s: roStyle.Render("●") + " " + segCtxStyle.Render(ctx) + dimCellStyle.Render(" [RO]"),
		w: lipgloss.Width("● " + ctx + " [RO]"),
	}}
	add := func(label, val string, st lipgloss.Style, lim int) {
		if val == "" {
			return
		}
		v := truncate(val, lim)
		segs = append(segs, fseg{
			s: dimCellStyle.Render(label+" ") + st.Render(v),
			w: lipgloss.Width(label + " " + v),
		})
	}
	add("cluster", cluster, segClusterStyle, 24)
	add("user", user, segUserStyle, 24)
	add("endpoint", endpoint, segEndpointStyle, 46)
	add("region", region, segRegionStyle, 16)
	add("s3s ver", rev, dimCellStyle, 16)
	return joinFit(width, segs)
}

// footerHintsLine builds the colored keybinding line.
func footerHintsLine(width int) string {
	h := func(k, a string) fseg {
		return fseg{s: accentStyle.Render(k) + " " + dimCellStyle.Render(a), w: lipgloss.Width(k + " " + a)}
	}
	segs := []fseg{
		h("enter", "open"), h("/", "filter"), h("r", "refresh"),
		h("c", "context"), h("1-9", "switch"), h("?", "help"), h("q", "quit"),
	}
	return joinFit(width, segs)
}

// formatDate renders a date compactly, or an em dash when zero.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}
