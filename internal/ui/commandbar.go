package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Three-block command bar: info · read · write, laid out as side-by-side
// columns. The write block is ALWAYS shown — dimmed in a read-only context, active
// (caution) when armed — so the operator sees the full capability
// map even read-only. Below blockColMin the columns collapse to a
// compact wrapped row that still lists the write entries (dimmed) and keeps the badge
// Colors reuse the existing palette only — no new hue.

// blockColMin is the minimum width at which the three blocks render as columns; below
// it the bar collapses to a compact single row.
const blockColMin = 100

// colGap is the gap between the three command-bar columns (: widened 2→3 spaces). The
// natural-width guard derives its reserve from len(colGap) so the two never drift.
const colGap = "   "

// styleRole maps a bar entry to a palette role. Roles reuse existing tokens;
// no new hue is introduced. roleWriteDimmed (faint) and roleWriteInapplicable (plain dim)
// are deliberately DISTINCT so an inactive-because-read-only entry reads differently from
// a not-applicable-to-this-selection one. The role is the single source of an
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
var barGlobals = keyStyle.Render(":") + " " + dimCellStyle.Render("cmds") +
	barSep + keyStyle.Render("?") + " " + dimCellStyle.Render("help") +
	barSep + keyStyle.Render("q") + " " + dimCellStyle.Render("quit")

// readEntries builds the read block: open + search/filter, then the read (non-write)
// actions. A read action inapplicable to the current selection is marked (still shown)
// never hidden. kind and cat are passed in (computed ONCE per render by the caller) to
// avoid re-deriving the selection (re-sorts the level) and rebuilding the catalog of
// closures.
func (m App) readEntries(kind selKind, cat []action) []barEntry {
	searchLabel := "search"
	if m.mode == modeBuckets {
		searchLabel = "filter"
	}
	out := []barEntry{
		// Sort affordance + current field/direction, advertised in the bar.
		{key: glyph(firstBind(m.keys.Sort)), label: m.sortIndicator(), role: roleRead},
		{key: glyph(firstBind(m.keys.Enter)), label: "open", role: roleRead},
		{key: glyph(firstBind(m.keys.Search)), label: searchLabel, role: roleRead},
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
	// Plugin status surface — advertised only when at least one plugin is
	// declared (zero-config users never see plugin chrome).
	if len(m.plugins) > 0 {
		out = append(out, barEntry{key: glyph(firstBind(m.keys.Plugins)), label: "plugins", role: roleRead})
	}
	// Reset affordance: shown only when a filter term is applied AND the
	// input is closed (not while typing — that mode shows its own Enter/Esc hints).
	if m.searchActive() && !m.searching {
		out = append(out, barEntry{key: glyph(m.keys.Back[0]), label: "clear", role: roleRead})
	}
	return out
}

// writeEntries builds the write block: every write action, ALWAYS shown. The role is
// dimmed when the context is read-only, inapplicable for the wrong selection
// , else active. Dangerous actions display their chord glyph.
func (m App) writeEntries(kind selKind, cat []action) []barEntry {
	writable := m.writable()
	//: the duplicate-"delete" problem only exists when the catalog has the delete
	// PAIR (object + recursive, tree mode). Suppress an inapplicable delete ONLY then — never
	// when "delete" is the lone write action (bucket mode), so the write group is never emptied
	// (preserves: the write block always shows the capability, dimmed when N/A).
	deletePair := 0
	for _, a := range cat {
		if a.writeOnly && a.label == "delete" {
			deletePair++
		}
	}
	var out []barEntry
	for _, a := range cat {
		if !a.writeOnly {
			continue
		}
		// Show only the selection-applicable delete so the write group never carries two
		// identical "delete" labels — applied only when the delete pair is present.
		if deletePair > 1 && a.label == "delete" && a.avail != nil && !a.avail(m, kind) {
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

// infoFields builds the info block: identity (with the loud arm badge), cluster, user
// region, and the s3s version — plus the add-connection affordance is appended
// by the renderer. ctx carries the [RW]/[RO] badge so the bar always shows write state
// without a separate identity line.
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
	st := roleStyle[e.role]
	// Bold the KEY glyph while the label stays the role style — the key
	// keeps its role color but gains emphasis, distinct from its label.
	return st.Bold(true).Render(e.key) + st.Render("  "+e.label)
}

// commandBarView renders the three-block command bar (or the collapsed compact row on a
// narrow terminal). The selection kind AND the action catalog are derived ONCE here and
// threaded into the builders, and the narrow path returns early WITHOUT
// building the wide columns it would discard.
func (m App) commandBarView(w int) string {
	kind := m.selKind()
	cat := m.actionCatalog()
	if w < blockColMin {
		return m.collapsedBarView(w, kind, cat)
	}
	info := m.infoColumn()
	read := entryColumn(m.readEntries(kind, cat))
	write := m.writeColumn(kind, cat)
	natural := lipgloss.Width(info) + lipgloss.Width(read) + lipgloss.Width(write) + 2*len(colGap)
	if natural > w {
		return m.collapsedBarView(w, kind, cat)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, info, colGap, read, colGap, write)
}

// infoColumn renders the info block as a column. No heading — the column
// + inter-column gap carry the grouping; the identity line leads.
func (m App) infoColumn() string {
	rows := []string{footerIdentityCompact(40, m.ctxName, "")} // ● ctx (mode lives on the border chip)
	for _, f := range m.infoFields() {
		val := f.value
		if val == "" {
			val = "—"
		}
		rows = append(rows, dimCellStyle.Render(pad(f.label, 7))+" "+f.style.Render(val))
	}
	if m.connect != nil {
		// "connections" opens the manager (switch / add / delete) via the context key `c`
		// (: the standalone `n` add-connection key was removed; the "+ add connection"
		// row inside the manager is the sole add affordance).
		rows = append(rows, keyStyle.Render(glyph(m.keys.Context[0]))+" "+dimCellStyle.Render("connections"))
	}
	rows = append(rows, barGlobals)
	return blockColumn(rows)
}

// barColMaxRows caps a block's height so a richer action set widens the bar instead of
// growing the footer past its height budget (: adding health/share/reveal/mark must
// not scroll the list off). Matches the info column's natural height.
const barColMaxRows = 7

// entryColumn renders entries as a column, overflowing into ADDITIONAL side-by-side
// columns past barColMaxRows rows — width absorbs growth, never footer height.
func entryColumn(entries []barEntry) string {
	cols := make([]string, 0, 2)
	for start := 0; start < len(entries); start += barColMaxRows {
		end := min(start+barColMaxRows, len(entries))
		rows := make([]string, 0, end-start)
		for _, e := range entries[start:end] {
			rows = append(rows, entryStyled(e))
		}
		cols = append(cols, blockColumn(rows))
	}
	if len(cols) == 0 {
		return ""
	}
	out := cols[0]
	for _, c := range cols[1:] {
		out = lipgloss.JoinHorizontal(lipgloss.Top, out, "  ", c)
	}
	return out
}

// writeColumn renders the write block as a column. With no WRITE heading, the read-only cue
// is a literal "w to arm" lead row (amber, NO_COLOR-safe) shown only when not writable — the
// defined surface preserving the read-only state cue.
func (m App) writeColumn(kind selKind, cat []action) string {
	var rows []string
	// Symmetric, always-present toggle cue sourced from the keymap: a clear
	// "enable write" when disarmed, a "→ read-only" disarm cue when armed, and an explicit
	// unavailable cue on a read-only context — so the toggle key and current state never vanish.
	wkey := glyph(firstBind(m.keys.WriteToggle))
	switch {
	case m.armed:
		rows = append(rows, warnStyle.Render(wkey+" → read-only"))
	case m.ctxReadOnly:
		rows = append(rows, dimCellStyle.Render("read-only context"))
	default:
		rows = append(rows, warnStyle.Render(wkey+" enable write"))
	}
	for _, e := range m.writeEntries(kind, cat) {
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
// at least one entry + a "…" so the write block is never dropped entirely.
func (m App) collapsedBarView(w int, kind selKind, cat []action) string {
	identity := footerIdentityCompact(w, m.ctxName, m.info.Cluster)
	read := append([]barEntry{}, m.readEntries(kind, cat)...)
	if m.connect != nil {
		// Prepend "connections" so width-trimming (which drops trailing entries) never drops
		// it first — it stays discoverable on a narrow bar. Opened via `c`.
		read = append([]barEntry{{key: glyph(m.keys.Context[0]), label: "connections", role: roleRead}}, read...)
	}
	// Globals (help/quit) always survive; fit the read entries into the remaining width.
	globals := barSep + barGlobals
	readRow := fitEntries(read, max(1, w-lipgloss.Width(globals)), 0) + globals
	writeRow := fitEntries(m.writeEntries(kind, cat), w, 1) // keep ≥1 write entry
	return identity + "\n" + readRow + "\n" + writeRow
}

// fitEntries joins entries with " · " and drops TRAILING entries until the row fits w
// appending a dim "…" when any were dropped. keepMin entries are always retained (the row
// may then exceed w only on an extremely narrow terminal, unavoidable without clipping
// styled text). O(n): each entry is rendered and measured once.
func fitEntries(entries []barEntry, w, keepMin int) string {
	if len(entries) == 0 {
		return ""
	}
	sep := barSep
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
