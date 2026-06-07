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

	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(colErr)
	warnStyle   = lipgloss.NewStyle().Foreground(colWarn)
	noticeStyle = lipgloss.NewStyle().Foreground(colOK) // success notices (green) — distinct from errStyle red
	emptyStyle  = lipgloss.NewStyle().Faint(true).Foreground(colDim)

	// 006 visual backbone — new surfaces REUSE the tokens above (FR-031/FR-032): no new
	// hue is introduced. The hint bar advertises action keys (accent) + labels (dim);
	// the pane reuses the metadata key/value styles; the command bar/form reuse accent
	// for the active cue and dim for the rest, keeping the screen calm (FR-037/FR-038).
	hintLabelStyle  = dimCellStyle                              // contextual hint label (pane cues)
	formActiveStyle = lipgloss.NewStyle().Foreground(colAccent) // focused form field label
	formErrStyle    = errStyle                                  // form/test error line
)

// NOTE on NO_COLOR (FR-034/FR-041/FR-042): every color-carried meaning also has a
// redundant non-color cue — the `▶` selection gutter, the `✓` multi-select mark, the
// `[RW]`/`[RO]` badge TEXT, and the `error:`/`loading…` prefixes — so the UI stays legible
// when lipgloss strips color under NO_COLOR.

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

// truncateTail keeps the TAIL of s within display width w (rune/width-aware), prefixing
// an ellipsis when cut — the mirror of truncate (which keeps the head). Used to keep the
// end of a long typed identifier visible as the operator types (007 FR-023a). Never cuts
// mid-rune.
func truncateTail(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// Walk right-to-left to find the first kept index (reserving one cell for the leading
	// ellipsis), then slice forward — no builder, no reversal.
	runes := []rune(s)
	width := 0
	start := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		cw := lipgloss.Width(string(runes[i]))
		if width+cw > w-1 {
			break
		}
		width += cw
		start = i
	}
	return "…" + string(runes[start:])
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
	if len(lines) > minRows { // never exceed the budgeted height (footer must stay visible)
		lines = lines[:minRows]
	}

	// Top border: "╭─ left ─── «center» ───╮". Cap the left title so a long key or
	// prefix can't overflow the border line and break the layout.
	leftCap := inner - 4
	if center != "" {
		leftCap = inner * 2 / 3
	}
	left = truncate(left, max(1, leftCap))
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

// --- footer: compact identity + contextual, single-row hints ---
//
// The list modes (buckets/tree) render their hints via the 006 hint bar (hintBarView);
// footerHints serves the remaining overlay/list modes (context switch, usage, connection
// manager/form) with generic navigation cues.

// hint is one advertised footer action. prio governs survival under the width degrade
// (higher = kept longer); help/quit use the top prio. Catalog order is the display order.
type hint struct {
	key   string
	label string
	prio  int
	show  func(hintCtx) bool
}

// hintCtx is the snapshot the footer hint catalog reads (no I/O).
type hintCtx struct {
	mode         mode
	searchActive bool
	multiContext bool
	width        int
}

// hintCatalog returns the hints in DISPLAY order.
func hintCatalog() []hint {
	always := func(hintCtx) bool { return true }
	return []hint{
		{"esc", "clear", 85, func(c hintCtx) bool { return c.searchActive }},
		{"esc", "back", 80, func(c hintCtx) bool { return !c.searchActive }},
		{"c", "context", 40, func(c hintCtx) bool { return c.multiContext }},
		{"1-9", "switch", 40, func(c hintCtx) bool { return c.multiContext }},
		{"?", "help", 100, always},
		{"q", "quit", 100, always},
	}
}

// footerHints renders the single-row, width-fit hint line. When a hint is dropped to fit
// the width, a trailing "? more" cue is appended; help/quit (top prio) survive every drop.
func footerHints(c hintCtx) string {
	var app []hint
	for _, h := range hintCatalog() {
		if h.show(c) {
			app = append(app, h)
		}
	}
	dropped := false
	for {
		s, w := renderHintRow(app, dropped)
		if w <= c.width || len(app) <= 1 {
			return s
		}
		app = dropLowestPrio(app)
		dropped = true
	}
}

func renderHintRow(hs []hint, more bool) (string, int) {
	parts := make([]string, 0, len(hs))
	for _, h := range hs {
		parts = append(parts, accentStyle.Render(h.key)+" "+dimCellStyle.Render(h.label))
	}
	s := strings.Join(parts, dimCellStyle.Render(" · "))
	if more {
		if s != "" {
			s += dimCellStyle.Render(" · ")
		}
		s += dimCellStyle.Render("? more")
	}
	return s, lipgloss.Width(s)
}

// dropLowestPrio removes the single lowest-prio hint (preserving order of the rest).
func dropLowestPrio(hs []hint) []hint {
	if len(hs) == 0 {
		return hs
	}
	lo, loIdx := hs[0].prio, 0
	for i, h := range hs {
		if h.prio < lo {
			lo, loIdx = h.prio, i
		}
	}
	return append(append([]hint{}, hs[:loIdx]...), hs[loIdx+1:]...)
}

// writeBadgeStyle is the loud, high-contrast WRITE indicator: bold white on bright
// red, impossible to miss or confuse with read-only (005 FR-027). Used wherever the
// arm state is shown — the footer identity row AND every alt-screen overlay.
var writeBadgeStyle = lipgloss.NewStyle().Bold(true).
	Foreground(lipgloss.Color("231")).Background(lipgloss.Color("196"))

// writeBadge renders the arm-state badge: a loud "[RW]" while writable, a calm "[RO]"
// otherwise. Kept short so it is never the first thing dropped on a narrow width
// (005 FR-027, write-toggle-contract C3).
func writeBadge(writable bool) string {
	if writable {
		return writeBadgeStyle.Render("[RW]")
	}
	return roStyle.Render("[RO]")
}

// footerIdentityCompact renders the single identity row: ● ctx [RW|RO], plus · cluster
// if it fits (FR-007/FR-008). Endpoint/region/user/version are NOT shown here — they
// move to the help surface. The [RW] tag is rendered loud (writeBadgeStyle) so an armed
// session is unmistakable (005 FR-027).
func footerIdentityCompact(width int, ctx, cluster string, writable bool) string {
	tag, dotStyle, tagStyle := "[RO]", roStyle, dimCellStyle
	if writable {
		tag, dotStyle, tagStyle = "[RW]", accentStyle, writeBadgeStyle
	}
	name := truncate(ctx, max(1, width-len(tag)-4))
	head := dotStyle.Render("●") + " " + segCtxStyle.Render(name) + tagStyle.Render(" "+tag)
	used := lipgloss.Width("● " + name + " " + tag)
	if cluster != "" && used+lipgloss.Width(" · "+cluster) <= width {
		head += dimCellStyle.Render(" · ") + segClusterStyle.Render(cluster)
	}
	return head
}

// formatDate renders a date compactly, or an em dash when zero.
func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}
