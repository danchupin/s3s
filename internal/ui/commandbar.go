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
// no new hue is introduced. roleWriteDimmed (faint) and roleWriteInapplicable (plain dim)
// are deliberately DISTINCT so an inactive-because-read-only entry reads differently from
// a not-applicable-to-this-selection one (FR-018). The role is the single source of an
// entry's visible state — there is no separate state enum to keep in sync.
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

// barEntry is one read/write entry; role carries both its block identity and its
// active/dimmed/inapplicable state (see styleRole).
type barEntry struct {
	key   string // displayed key glyph (bare key, or the "^x" chord for dangerous actions)
	label string
	role  styleRole
}

// infoField is one labelled value in the info block.
type infoField struct {
	label string
	value string
	style lipgloss.Style
}

// barGlobals is the always-present help/quit cue, rendered once at init (it never
// changes per frame). Shared by the info column and the collapsed bar so the two never
// drift.
var barGlobals = accentStyle.Render("?") + " " + dimCellStyle.Render("help") +
	dimCellStyle.Render(" · ") + accentStyle.Render("q") + " " + dimCellStyle.Render("quit")

// readEntries builds the read block: open + search/filter, then the read (non-write)
// actions. A read action inapplicable to the current selection is marked (still shown),
// never hidden. kind and cat are passed in (computed ONCE per render by the caller) to
// avoid re-deriving the selection (re-sorts the level) and rebuilding the catalog of
// closures (review #9).
func (m App) readEntries(kind selKind, cat []action) []barEntry {
	searchLabel := "search"
	if m.mode == modeBuckets {
		searchLabel = "filter"
	}
	out := []barEntry{
		{key: "↵", label: "open", role: roleRead},
		{key: "/", label: searchLabel, role: roleRead},
	}
	for _, a := range cat {
		if a.writeOnly {
			continue
		}
		role := roleRead
		if a.avail != nil && !a.avail(m, kind) {
			role = roleWriteInapplicable // greyed: not applicable to this selection
		}
		out = append(out, barEntry{key: glyph(a.binds[0]), label: m.actionLabel(a), role: role})
	}
	return out
}

// writeEntries builds the write block: every write action, ALWAYS shown. The role is
// dimmed when the context is read-only (FR-007), inapplicable for the wrong selection
// (FR-018), else active (FR-008). Dangerous actions display their chord glyph (FR-026).
func (m App) writeEntries(kind selKind, cat []action) []barEntry {
	writable := m.writable()
	var out []barEntry
	for _, a := range cat {
		if !a.writeOnly {
			continue
		}
		key := glyph(a.binds[0])
		if a.dangerous {
			key = glyph(firstBind(a.chordKeys))
		}
		role := roleWriteActive
		switch {
		case !writable:
			role = roleWriteDimmed
		case a.avail != nil && !a.avail(m, kind):
			role = roleWriteInapplicable
		}
		out = append(out, barEntry{key: key, label: m.actionLabel(a), role: role})
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

// entryStyled renders one read/write entry as "key label" in its role style (one Render
// of the whole string).
func entryStyled(e barEntry) string {
	return roleStyle[e.role].Render(e.key + " " + e.label)
}

// commandBarView renders the three-block command bar (or the collapsed compact row on a
// narrow terminal). The selection kind AND the action catalog are derived ONCE here and
// threaded into the builders (review #9), and the narrow path returns early WITHOUT
// building the wide columns it would discard.
func (m App) commandBarView(w int) string {
	kind := m.selKind()
	cat := m.actionCatalog()
	if w < blockColMin {
		return m.collapsedBarView(w, kind, cat)
	}
	info := m.infoColumn()
	read := entryColumn(blockTitleStyle.Render("READ"), m.readEntries(kind, cat))
	write := m.writeColumn(kind, cat)
	natural := lipgloss.Width(info) + lipgloss.Width(read) + lipgloss.Width(write) + 4
	if natural > w {
		return m.collapsedBarView(w, kind, cat)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, info, "  ", read, "  ", write)
}

// blockTitleStyle renders a calm (dim, bold) block heading.
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
	rows = append(rows, barGlobals)
	return blockColumn(rows)
}

// entryColumn renders a column from a pre-rendered title and its entries. Shared by the
// read and write blocks (writeColumn supplies a title that carries the read-only cue).
func entryColumn(title string, entries []barEntry) string {
	rows := []string{title}
	for _, e := range entries {
		rows = append(rows, entryStyled(e))
	}
	return blockColumn(rows)
}

// writeColumn renders the write block as a titled column. The title gains a "(w to arm)"
// cue when read-only so the inactive state survives NO_COLOR (FR-015).
func (m App) writeColumn(kind selKind, cat []action) string {
	title := blockTitleStyle.Render("WRITE")
	if !m.writable() {
		title += warnStyle.Render("  (w to arm)")
	}
	return entryColumn(title, m.writeEntries(kind, cat))
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
func (m App) collapsedBarView(w int, kind selKind, cat []action) string {
	identity := footerIdentityCompact(w, m.ctxName, m.info.Cluster, m.writable())
	read := append([]barEntry{}, m.readEntries(kind, cat)...)
	if m.connect != nil {
		read = append(read, barEntry{key: glyph(m.keys.AddConn[0]), label: "new conn", role: roleRead})
	}
	// Globals (help/quit) always survive; fit the read entries into the remaining width.
	globals := dimCellStyle.Render(" · ") + barGlobals
	readRow := fitEntries(read, max(1, w-lipgloss.Width(globals)), 0) + globals
	writeRow := fitEntries(m.writeEntries(kind, cat), w, 1) // keep ≥1 write entry (FR-016)
	return identity + "\n" + readRow + "\n" + writeRow
}

// fitEntries joins entries with " · " and drops TRAILING entries until the row fits w,
// appending a dim "…" when any were dropped. keepMin entries are always retained (the row
// may then exceed w only on an extremely narrow terminal, unavoidable without clipping
// styled text). O(n): each entry is rendered and measured once.
func fitEntries(entries []barEntry, w, keepMin int) string {
	if len(entries) == 0 {
		return ""
	}
	sep := dimCellStyle.Render(" · ")
	ell := dimCellStyle.Render(" …")
	sepW, ellW := lipgloss.Width(sep), lipgloss.Width(ell)

	styled := make([]string, len(entries))
	for i, e := range entries {
		styled[i] = entryStyled(e)
	}
	n, total := 0, 0
	for i := range styled {
		add := lipgloss.Width(styled[i])
		if i > 0 {
			add += sepW
		}
		reserve := 0 // reserve the ellipsis cells only when entries would remain after i
		if i < len(styled)-1 {
			reserve = ellW
		}
		if i+1 > keepMin && total+add+reserve > w {
			break
		}
		total += add
		n = i + 1
	}
	if n < 1 {
		n = 1
	}
	s := strings.Join(styled[:n], sep)
	if n < len(styled) {
		s += ell
	}
	return s
}
