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
	// keyStyle renders advertised hotkey glyphs bold so the key stands out from its label
	// Bold is an SGR attribute, not a color, so it is the key cue; the
	// leading-token position of every key ("d download") is the redundant non-color cue that
	// survives NO_COLOR.
	keyStyle = accentStyle.Bold(true)

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

	// 006 visual backbone — new surfaces REUSE the tokens above: no new
	// hue is introduced. The hint bar advertises action keys (accent) + labels (dim);
	// the pane reuses the metadata key/value styles; the command bar/form reuse accent
	// for the active cue and dim for the rest, keeping the screen calm.
	hintLabelStyle  = dimCellStyle                              // contextual hint label (pane cues)
	formActiveStyle = lipgloss.NewStyle().Foreground(colAccent) // focused form field label
	formErrStyle    = errStyle                                  // form/test error line
)

// NOTE on NO_COLOR: every color-carried meaning also has a
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

// elideMiddle fits a "/"-separated path into display width w by dropping the MIDDLE segments
// (keeping the first segment and the deepest), joined by an ellipsis; it falls back to
// end-truncation when even first+deepest will not fit.
func elideMiddle(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	parts := strings.Split(s, "/")
	if len(parts) <= 2 {
		return truncate(s, w)
	}
	if cand := parts[0] + "/…/" + parts[len(parts)-1]; lipgloss.Width(cand) <= w {
		return cand
	}
	return truncate(s, w)
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

// renderTableActive is renderTable with automatic active-row wrap: when the
// selected row's NAME is truncated and spareRows blank rows are available below the window,
// the full name is wrapped onto up to spareRows dim continuation lines so the highlighted
// item is always readable without exceeding the box height. spareRows<=0 disables the wrap
// (the reveal popup is the fallback for a full screen).
func renderTableActive(width int, cols []column, rows [][]string, dirFlags []bool, sel, spareRows int) string {
	base := renderTable(width, cols, rows, dirFlags, sel)
	if spareRows <= 0 || sel < 0 || sel >= len(rows) || len(rows[sel]) == 0 {
		return base
	}
	widths := layoutWidths(width, cols)
	name := sanitizeLabel(rows[sel][0])
	if lipgloss.Width(name) <= widths[0] {
		return base // not truncated — nothing to reveal
	}
	cont := wrapValue(name, max(1, clampW(width)-2), spareRows)
	if len(cont) == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(base)
	for _, line := range cont {
		b.WriteString("\n  " + dimCellStyle.Render(line))
	}
	return b.String()
}

// wrapValue splits s into chunks of display width w, at most maxLines chunks.
func wrapValue(s string, w, maxLines int) []string {
	if w < 1 {
		w = 1
	}
	var lines []string
	var cur strings.Builder
	curW := 0
	for _, ch := range s {
		cw := lipgloss.Width(string(ch))
		if curW+cw > w {
			lines = append(lines, cur.String())
			if len(lines) >= maxLines {
				return lines
			}
			cur.Reset()
			curW = 0
		}
		cur.WriteRune(ch)
		curW += cw
	}
	if cur.Len() > 0 && len(lines) < maxLines {
		lines = append(lines, cur.String())
	}
	return lines
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
	return boxViewWith(left, center, "", "", body, width, minRows, titleStyle)
}

// boxViewFocus is boxView with an active/inactive title style for the multi-pane zones
// : the focused zone's title is the accent (active) style, an unfocused zone's
// title is dim — the deterministic active-zone indicator.
func boxViewFocus(left, center, body string, width, minRows int, active bool) string {
	return boxViewWith(left, center, "", "", body, width, minRows, focusTitleStyle(active))
}

// boxViewChip is boxView with up to two right-aligned chips inset into the top border:
// the applied-filter chip (inboard) and the read/write mode chip (outboard, right-most).
// Each is an already-styled string; "" omits that slot.
func boxViewChip(left, center, filterChip, modeChip, body string, width, minRows int) string {
	return boxViewWith(left, center, filterChip, modeChip, body, width, minRows, titleStyle)
}

// boxViewFocusChip combines the focus title style with the border chips — used by the
// browse list boxes (mode chip) and the per-pane applied-filter chip.
func boxViewFocusChip(left, center, filterChip, modeChip, body string, width, minRows int, active bool) string {
	return boxViewWith(left, center, filterChip, modeChip, body, width, minRows, focusTitleStyle(active))
}

func focusTitleStyle(active bool) lipgloss.Style {
	if active {
		return titleStyle
	}
	return dimCellStyle
}

// boxViewWith renders the bordered box with the given title style (shared by the boxView
// family). Up to two already-styled chips are inset into the top border before the closing
// corner — the applied-filter chip (inboard) and the mode chip (outboard, right-most). When
// the border is too narrow it drops the centered label first, then the filter chip, and the
// mode chip only as a last resort (the mode chip is safety-critical, FR-005/contract C3).
func boxViewWith(left, center, filterChip, modeChip, body string, width, minRows int, titleSt lipgloss.Style) string {
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

	// buildRight composes the right-border chips (filter inboard, mode right-most before the
	// corner) and returns the rendered segment + its display width. Each present chip renders
	// as " ‹chip›"; a trailing " ─" hugs the corner.
	buildRight := func(includeFilter, includeMode bool) (string, int) {
		var parts []string
		if includeFilter && filterChip != "" {
			parts = append(parts, filterChip)
		}
		if includeMode && modeChip != "" {
			parts = append(parts, modeChip)
		}
		if len(parts) == 0 {
			return "", 0
		}
		seg, w := "", 0
		for _, c := range parts {
			seg += ruleStyle.Render(" ") + c
			w += 1 + lipgloss.Width(c)
		}
		seg += ruleStyle.Render(" ─")
		return seg, w + 2
	}

	// Top border: "╭─ left ─── «center» ─── ‹filter› ‹mode› ─╮". Cap the left title so a long
	// key or prefix can't overflow the border line and break the layout.
	leftCap := inner - 4
	if center != "" {
		leftCap = inner * 2 / 3
	}
	left = truncate(left, max(1, leftCap))
	leftPlain := "─ " + left + " "
	wl := lipgloss.Width(leftPlain)

	// Budget the center against the worst case (both chips present).
	_, wBoth := buildRight(true, true)
	centerPlain := ""
	if center != "" {
		centerPlain = " " + truncate(center, max(0, inner-wl-wBoth-4)) + " "
	}
	wc := lipgloss.Width(centerPlain)

	// Degrade order: center → filter chip → mode chip (mode survives last).
	includeFilter, includeMode := true, true
	rightSeg, wchip := buildRight(includeFilter, includeMode)
	avail := inner - wl - wc - wchip
	if avail < 0 && centerPlain != "" {
		centerPlain, wc = "", 0
		avail = inner - wl - wchip
	}
	if avail < 0 && includeFilter && filterChip != "" {
		includeFilter = false
		rightSeg, wchip = buildRight(includeFilter, includeMode)
		avail = inner - wl - wc - wchip
	}
	if avail < 0 && includeMode && modeChip != "" {
		includeMode = false
		rightSeg, wchip = buildRight(includeFilter, includeMode)
		avail = inner - wl - wc - wchip
	}
	if avail < 0 {
		avail = 0
	}
	leftDashes := avail / 2
	if centerPlain == "" && rightSeg == "" {
		leftDashes = avail // single gap: fill after the left label
	}
	rightDashes := avail - leftDashes

	topInner := ruleStyle.Render("─ ") + titleSt.Render(left) + ruleStyle.Render(" ") +
		ruleStyle.Render(strings.Repeat("─", leftDashes))
	if centerPlain != "" {
		topInner += selRowStyle.Render(centerPlain)
	}
	topInner += ruleStyle.Render(strings.Repeat("─", rightDashes)) + rightSeg

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
		parts = append(parts, keyStyle.Render(h.key)+" "+dimCellStyle.Render(h.label))
	}
	s := strings.Join(parts, barSep)
	if more {
		if s != "" {
			s += barSep
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
// red, impossible to miss or confuse with read-only. Used wherever the
// arm state is shown — the footer identity row AND every alt-screen overlay.
var writeBadgeStyle = lipgloss.NewStyle().Bold(true).
	Foreground(lipgloss.Color("231")).Background(lipgloss.Color("196"))

// writeBadge renders the arm-state badge: a loud "[RW]" while writable, a calm "[RO]"
// otherwise. Kept short so it is never the first thing dropped on a narrow width
func writeBadge(writable bool) string {
	if writable {
		return writeBadgeStyle.Render("[RW]")
	}
	return roStyle.Render("[RO]")
}

// barSep is the single, single-sourced footer / command-bar element separator (013 US3). The
// dot already carries surrounding spaces, so the breathing room US3 adds lands on the actual
// cram points — the key↔label gap (entryStyled) and the inter-column gap (colGap) — while this
// stays compact so narrow widths keep advertising the affordances (e.g. "filter"). sepDot is its
// raw form, used where a width is measured against it or it sits inside an already-styled span.
const sepDot = " · "

var barSep = dimCellStyle.Render(sepDot)

// footerIdentityCompact renders the single identity row: ● ctx, plus · cluster if it fits.
// The read/write mode is NOT shown here — 013 US1 removes the old [RW]/[RO] tag; the mode lives
// solely on the border mode chip. Endpoint/region/user/version move to the help surface.
func footerIdentityCompact(width int, ctx, cluster string) string {
	name := truncate(ctx, max(1, width-4))
	head := accentStyle.Render("●") + " " + segCtxStyle.Render(name)
	used := lipgloss.Width("● " + name)
	if cluster != "" && used+lipgloss.Width(sepDot+cluster) <= width {
		head += barSep + segClusterStyle.Render(cluster)
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
