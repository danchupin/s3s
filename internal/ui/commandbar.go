package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Three-block command bar (007 US1): info · read · write, laid out as side-by-side
// columns. The write block is ALWAYS shown — dimmed in a read-only context (reversing
// 006 FR-004), active (caution) when armed — so the operator sees the full capability
// map even read-only (FR-006/FR-007/FR-008). Below blockColMin the columns collapse to a
// compact wrapped row that still lists the write entries (dimmed) and keeps the badge
// (FR-016). Colors reuse the existing palette only — no new hue (FR-013).

// blockColMin is the minimum width at which the three blocks render as columns; below
// it the bar collapses to a compact single row (FR-016).
const blockColMin = 100

// styleRole maps a bar entry to a palette role. Roles reuse existing tokens (FR-013);
// no new hue is introduced. write-dimmed (faint) and write-inapplicable (plain dim) are
// deliberately DISTINCT so an inactive-because-read-only entry reads differently from a
// not-applicable-to-this-selection one (FR-018).
type styleRole int

const (
	roleInfo styleRole = iota
	roleRead
	roleWriteActive
	roleWriteDimmed
	roleWriteInapplicable
)

// roleStyle is the role→style map (single source so tests can assert distinctness).
var roleStyle = map[styleRole]lipgloss.Style{
	roleInfo:              dimCellStyle, // labels; values carry their own per-field hue
	roleRead:              accentStyle,  // read keys — coral
	roleWriteActive:       warnStyle,    // armed write keys — amber (distinct from read coral)
	roleWriteDimmed:       emptyStyle,   // read-only write — faint
	roleWriteInapplicable: ruleStyle,    // wrong-selection write — border-grey (distinct from info dim & faint)
}

// entryState is a write entry's applicability/arm state.
type entryState int

const (
	entryActive entryState = iota
	entryDimmed
	entryInapplicable
)

// barEntry is one row in a read/write block.
type barEntry struct {
	key   string // displayed key glyph (bare key, or the "^x" chord for dangerous actions)
	label string
	role  styleRole
	state entryState
}

// infoField is one labelled value in the info block.
type infoField struct {
	label string
	value string
	style lipgloss.Style
}

// readEntries builds the read block: open + search/filter, then the read (non-write)
// actions for the current mode. A read action inapplicable to the current selection is
// marked entryInapplicable (still shown), never hidden.
func (m App) readEntries() []barEntry {
	kind := m.selKind()
	searchLabel := "search"
	if m.mode == modeBuckets {
		searchLabel = "filter"
	}
	out := []barEntry{
		{key: "↵", label: "open", role: roleRead, state: entryActive},
		{key: "/", label: searchLabel, role: roleRead, state: entryActive},
	}
	for _, a := range m.actionCatalog() {
		if a.writeOnly {
			continue
		}
		st := entryActive
		if a.avail != nil && !a.avail(m, kind) {
			st = entryInapplicable
		}
		out = append(out, barEntry{key: glyph(a.binds[0]), label: m.actionLabel(a), role: roleRead, state: st})
	}
	return out
}

// writeEntries builds the write block: every write action, ALWAYS shown. State is dimmed
// when the context is read-only (FR-007), inapplicable for the wrong selection (FR-018),
// else active (FR-008). Dangerous actions display their chord glyph (FR-026).
func (m App) writeEntries() []barEntry {
	kind := m.selKind()
	writable := m.writable()
	var out []barEntry
	for _, a := range m.actionCatalog() {
		if !a.writeOnly {
			continue
		}
		key := glyph(a.binds[0])
		role := roleWriteActive
		state := entryActive
		if a.dangerous {
			key = glyph(a.chord)
		}
		switch {
		case !writable:
			role, state = roleWriteDimmed, entryDimmed
		case a.avail != nil && !a.avail(m, kind):
			role, state = roleWriteInapplicable, entryInapplicable
		}
		out = append(out, barEntry{key: key, label: m.actionLabel(a), role: role, state: state})
	}
	return out
}

// infoFields builds the info block: identity (with the loud arm badge), cluster, user,
// region, and the s3s version (FR-003) — plus the add-connection affordance is appended
// by the renderer. ctx carries the [RW]/[RO] badge so the bar always shows write state
// (FR-020) without a separate identity line.
func (m App) infoFields() []infoField {
	return []infoField{
		{label: "cluster", value: m.info.Cluster, style: segClusterStyle},
		{label: "user", value: m.info.User, style: segUserStyle},
		{label: "region", value: m.info.Region, style: segRegionStyle},
		{label: "s3s", value: Version, style: dimCellStyle},
	}
}

// entryStyled renders one read/write entry as "key label" in its role style.
func entryStyled(e barEntry) string {
	return roleStyle[e.role].Render(e.key) + " " + roleStyle[e.role].Render(e.label)
}

// commandBarView renders the three-block command bar (or the collapsed compact row on a
// narrow terminal). It always includes the arm badge and the write block.
func (m App) commandBarView(w int) string {
	info, read, write := m.infoColumn(), m.readColumn(), m.writeColumn()
	natural := lipgloss.Width(info) // info is the widest fixed column; add read+write+gaps
	natural += lipgloss.Width(read) + lipgloss.Width(write) + 4
	if w < blockColMin || natural > w {
		return m.collapsedBarView(w)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, info, "  ", read, "  ", write)
}

// blockTitle renders a calm (dim, bold) block heading.
var blockTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colDim)

// infoColumn renders the info block as a titled column.
func (m App) infoColumn() string {
	rows := []string{blockTitleStyle.Render("INFO")}
	rows = append(rows, footerIdentityCompact(40, m.ctxName, "", m.writable())) // ● ctx [badge]
	for _, f := range m.infoFields() {
		val := f.value
		if val == "" {
			val = "—"
		}
		rows = append(rows, dimCellStyle.Render(pad(f.label, 7))+" "+f.style.Render(val))
	}
	if m.connect != nil {
		rows = append(rows, accentStyle.Render(glyph(m.keys.AddConn[0]))+" "+dimCellStyle.Render("new conn"))
	}
	rows = append(rows, accentStyle.Render("?")+" "+dimCellStyle.Render("help")+"  "+
		accentStyle.Render("q")+" "+dimCellStyle.Render("quit"))
	return blockColumn(rows)
}

// readColumn renders the read block as a titled column.
func (m App) readColumn() string {
	rows := []string{blockTitleStyle.Render("READ")}
	for _, e := range m.readEntries() {
		rows = append(rows, entryStyled(e))
	}
	return blockColumn(rows)
}

// writeColumn renders the write block as a titled column. The title gains a "(w to arm)"
// cue when read-only so the inactive state survives NO_COLOR (FR-015).
func (m App) writeColumn() string {
	title := blockTitleStyle.Render("WRITE")
	if !m.writable() {
		title += warnStyle.Render("  (w to arm)")
	}
	rows := []string{title}
	for _, e := range m.writeEntries() {
		rows = append(rows, entryStyled(e))
	}
	return blockColumn(rows)
}

// blockColumn pads each row of a block to the block's max width and joins them.
func blockColumn(rows []string) string {
	w := 0
	for _, r := range rows {
		if rw := lipgloss.Width(r); rw > w {
			w = rw
		}
	}
	for i := range rows {
		rows[i] = padLine(rows[i], w)
	}
	return strings.Join(rows, "\n")
}

// collapsedBarView is the compact fallback (narrow terminal): three rows — identity +
// badge, a read row, and a write row — each width-fit by DROPPING trailing entries (never
// by clipping styled text mid-escape, which would corrupt the line). The write row keeps
// at least one entry + a "…" so the write block is never dropped entirely (FR-016).
func (m App) collapsedBarView(w int) string {
	identity := footerIdentityCompact(w, m.ctxName, m.info.Cluster, m.writable())
	read := append([]barEntry{}, m.readEntries()...)
	if m.connect != nil {
		read = append(read, barEntry{key: glyph(m.keys.AddConn[0]), label: "new conn", role: roleRead, state: entryActive})
	}
	// Globals (help/quit) always survive; fit the read entries into the remaining width.
	globals := dimCellStyle.Render(" · ") + accentStyle.Render("?") + " " + dimCellStyle.Render("help") +
		dimCellStyle.Render(" · ") + accentStyle.Render("q") + " " + dimCellStyle.Render("quit")
	readRow := fitEntries(read, max(1, w-lipgloss.Width(globals)), 0) + globals
	writeRow := fitEntries(m.writeEntries(), w, 1) // keep ≥1 write entry (FR-016)
	return identity + "\n" + readRow + "\n" + writeRow
}

// fitEntries joins entries with " · " and drops TRAILING entries until the row fits w,
// appending a dim "…" when any were dropped. keepMin entries are always retained (the
// row may then exceed w only on an extremely narrow terminal, which is unavoidable
// without clipping styled text).
func fitEntries(entries []barEntry, w, keepMin int) string {
	sep := dimCellStyle.Render(" · ")
	ell := dimCellStyle.Render(" …")
	render := func(n int, dropped bool) string {
		parts := make([]string, 0, n)
		for i := 0; i < n; i++ {
			parts = append(parts, entryStyled(entries[i]))
		}
		s := strings.Join(parts, sep)
		if dropped {
			s += ell
		}
		return s
	}
	n := len(entries)
	for n > keepMin && n > 1 && lipgloss.Width(render(n, n < len(entries))) > w {
		n--
	}
	return render(n, n < len(entries))
}
