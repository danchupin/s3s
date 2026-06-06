// Package ui is the Bubble Tea (v2) TUI layer. It depends only on the storage
// interface, never the SDK (Constitution I). Every backend call runs in a tea.Cmd
// so the event loop never blocks (Constitution II); superseded loads are cancelled
// and their stale results dropped via a generation id.
package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/cache"
	"github.com/danchupin/s3s/internal/localfs"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

type mode int

const (
	modeBuckets mode = iota
	modeTree
	modeObject // combined metadata + content view (opened with Enter)
	modeContextSwitch
	modeHelp
	modeActionMenu // contextual action menu (opened with 'a')
	modeUsage      // du analytics view (opened via the action menu) — 005 US2
)

// selKind classifies the current tree selection for contextual hints/menu items.
type selKind int

const (
	selNone selKind = iota
	selObject
	selFolder
)

// selKind reports the kind of the current tree selection (selNone outside the tree
// or when nothing is selected). Used by the action menu and the footer hint catalog.
func (m App) selKind() selKind {
	if m.mode != modeTree {
		return selNone
	}
	e := m.selected()
	switch {
	case e == nil:
		return selNone
	case e.isDir:
		return selFolder
	default:
		return selObject
	}
}

// levelState is the accumulated, cached view of one tree node across pages.
type levelState struct {
	dirs      []string
	objects   []storage.ObjectRef
	nextToken *string
	complete  bool
}

func (l *levelState) count() int { return len(l.dirs) + len(l.objects) }

// Backend describes the storage client plus the display metadata for the active
// context (shown in the header). The UI never builds clients itself. Store is the
// RAW (unguarded) client — the UI guards it dynamically per the runtime write-arm
// state (005 US5). Writable is the *initial* arm intent (the --write flag); ReadOnly
// is the context's absolute readonly:true lock.
type Backend struct {
	Store       storage.Storage
	Cluster     string
	User        string
	Endpoint    string
	Region      string
	Writable    bool   // initial write-arm intent (--write)
	ReadOnly    bool   // context is readonly:true — never armable (FR-028)
	DownloadDir string // default local download dir from config (005 FR-007)
}

// Resolver rebuilds a Backend for a named context (context switch). May be nil
// to disable switching.
type Resolver func(context string) (Backend, error)

// App is the root Bubble Tea model.
type App struct {
	raw         storage.Storage // RAW (unguarded) client; guarded dynamically per arm state
	info        Backend
	ctxName     string
	contexts    []string
	resolve     Resolver
	imgProto    preview.Protocol
	armed       bool // runtime write-arm intent (toggle / --write initial) — 005 US5
	ctxReadOnly bool // active context is readonly:true (absolute lock, FR-028)

	keys  keyMap
	cache *cache.Cache[*levelState]

	mode          mode
	prevMode      mode // to restore after help
	width, height int

	// buckets
	buckets      []storage.Bucket
	bucketSel    int
	bucketFilter string // client-side name filter at the bucket list

	// tree navigation
	bucket  string
	prefix  string
	search  string
	level   *levelState
	treeSel int

	// metadata / preview panes
	meta    *storage.ObjectMetadata
	prev    *preview.Payload
	prevOff int

	// context switcher
	ctxSel int

	// action menu (modeActionMenu)
	menuItems []menuItem
	menuSel   int

	// search/filter input
	searching   bool
	searchInput string
	searchGen   int

	// async state
	gen        int
	loading    bool
	loadCtx    context.Context
	loadCancel context.CancelFunc
	spin       int
	err        error

	// in-flight mutating operation (nil when none); see operation.go
	op     *operation
	opCh   chan progressEvent // streaming progress channel (upload / recursive delete)
	notice string             // transient non-error outcome line (e.g. partial counts)

	// write-mode arm confirmation pending (005 US5): true while awaiting y/N to arm write
	armConfirm bool

	// du analytics (005 US2 / modeUsage)
	usage       *storage.UsageReport // completed report (nil while scanning)
	usageSel    int                  // selected child row (for drill-down)
	usageBucket string
	usagePrefix string
	usageProg   storage.UsageProgress // running totals during a scan
	usageCh     chan usageEvent       // scan progress channel

	// local file browser state (active during an upload's phaseBrowse)
	fbDir     string
	fbEntries []localfs.Entry
	fbSel     int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// New builds the root model for an initial backend. resolve rebuilds the backend
// on context change (nil disables switching). imgProto is the detected terminal
// image protocol for previews.
func New(initial Backend, ctxName string, contexts []string, resolve Resolver, imgProto preview.Protocol) App {
	m := App{
		raw:         initial.Store,
		info:        initial,
		ctxName:     ctxName,
		contexts:    contexts,
		resolve:     resolve,
		imgProto:    imgProto,
		armed:       initial.Writable,
		ctxReadOnly: initial.ReadOnly,
		keys:        defaultKeys(),
		cache:       cache.New[*levelState](),
		mode:        modeBuckets,
		width:       80,
		height:      24,
	}
	// Arm the initial bucket load here, not in Init: in Bubble Tea v2 Init returns
	// only a Cmd and cannot mutate the model, so the generation/context must be set
	// on the model the program actually keeps.
	(&m).beginLoad()
	return m
}

// writable reports whether mutations are currently permitted: the session is armed
// AND the active context is not hard-locked readonly (005 FR-024/FR-028). This
// derived value replaces the old static field, recomputed on every toggle/switch.
func (m App) writable() bool { return m.armed && !m.ctxReadOnly }

// activeStore returns the backend guarded for the current arm state: the raw client
// when writable, else a read-only wrapper that refuses mutations without a network
// call. This is the single runtime enforcement point — every operation uses it, so a
// mutating call is impossible while disarmed (005 US5, write-toggle-contract C1).
func (m App) activeStore() storage.Storage { return storage.Guard(m.raw, m.writable()) }

// Init kicks off the initial bucket load armed by New.
func (m App) Init() tea.Cmd {
	return tea.Batch(loadBuckets(m.loadCtx, m.activeStore(), m.gen), spinnerTick())
}

// beginLoad cancels any in-flight load, bumps the generation, and starts a fresh
// cancellable context. Returns the context the new load must use; it is also
// stored on the model so Init/refresh can reference it.
func (m *App) beginLoad() context.Context {
	if m.loadCancel != nil {
		m.loadCancel()
	}
	m.gen++
	ctx, cancel := context.WithCancel(context.Background())
	m.loadCtx = ctx
	m.loadCancel = cancel
	m.loading = true
	m.err = nil
	return ctx
}

// cancelLoad cancels the in-flight load without starting a new one.
func (m *App) cancelLoad() {
	if m.loadCancel != nil {
		m.loadCancel()
		m.loadCancel = nil
	}
	m.loading = false
}

// levelKey is the cache coordinate for the current tree node.
func (m *App) levelKey() cache.Key {
	return cache.Key{Context: m.ctxName, Bucket: m.bucket, Prefix: m.prefix, Search: m.search}
}

// Update is the single message dispatcher.
func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampSelection()
		return m, nil

	case spinnerTickMsg:
		if m.loading {
			m.spin = (m.spin + 1) % len(spinnerFrames)
			return m, spinnerTick()
		}
		return m, nil

	case bucketsMsg:
		if msg.gen != m.gen {
			return m, nil // stale
		}
		m.loading = false
		m.buckets = msg.buckets
		m.bucketSel = 0
		return m, nil

	case levelMsg:
		return m.onLevel(msg)

	case metadataMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		md := msg.md
		m.meta = &md
		m.mode = modeObject
		return m, nil

	case previewMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		p := msg.payload
		m.prev = &p
		m.mode = modeObject
		return m, nil

	case errMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		return m, nil

	case operationProgressMsg:
		return m.onOperationProgress(msg)

	case operationDoneMsg:
		return m.onOperationDone(msg)

	case usageProgressMsg:
		return m.onUsageProgress(msg)

	case usageDoneMsg:
		return m.onUsageDone(msg)

	case searchFireMsg:
		return m.onSearchFire(msg)

	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// onKey routes a keypress by mode.
func (m App) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// A transient outcome notice (e.g. partial counts) clears on the next keypress.
	m.notice = ""

	// While typing a search query, the input owns most keys.
	if m.searching {
		return m.onSearchKey(msg)
	}

	// A pending write-arm confirmation is modal: y/N resolves it (005 US5).
	if m.armConfirm {
		return m.onArmConfirmKey(key)
	}

	// A running mutation is modal: only cancel (Esc/Back) and quit work. Navigation and
	// starting another mutation are blocked so a streaming op is never superseded
	// out from under its progress reader (which would orphan the worker goroutine)
	// and so two mutations can't run at once. Cancel is the back/escape key (FR-029).
	if m.op != nil && m.op.phase == phaseRunning {
		switch {
		case matches(key, m.keys.Quit):
			(&m).cancelLoad()
			return m, tea.Quit
		case matches(key, m.keys.Back):
			(&m).cancelLoad()
			return m, nil
		}
		return m, nil
	}

	// An interactive operation (name/dest entry, confirmation, file browser) owns
	// keys before the global bindings, so text input (incl. "q", "h", "+") is not
	// intercepted.
	if m.op != nil {
		return m.onOpKey(key, msg)
	}

	// Global quit.
	if matches(key, m.keys.Quit) {
		(&m).cancelLoad()
		return m, tea.Quit
	}

	// Help overlay: any key closes it.
	if m.mode == modeHelp {
		m.mode = m.prevMode
		return m, nil
	}

	// The action menu is modal: it owns keys (incl. Esc, which closes the menu) BEFORE
	// the load-cancel binding below, so an open menu's Esc closes the menu and does NOT
	// cancel a background load (FR-029 modal precedence).
	if m.mode == modeActionMenu {
		return m.onMenuKey(key)
	}

	if matches(key, m.keys.Help) {
		m.prevMode = m.mode
		m.mode = modeHelp
		return m, nil
	}

	// Write-arm toggle: a global safety primitive (005 US5). Arming prompts to
	// confirm; disarming is instant. Refused on a readonly:true context.
	if matches(key, m.keys.WriteToggle) {
		return m.toggleWrite()
	}

	// Cancel an in-flight load via the back/escape key (no modal overlay open here).
	if matches(key, m.keys.Back) && m.loading {
		(&m).cancelLoad()
		return m, nil
	}

	switch m.mode {
	case modeBuckets:
		return m.onBucketsKey(key)
	case modeTree:
		return m.onTreeKey(key, msg)
	case modeObject:
		return m.onObjectKey(key)
	case modeContextSwitch:
		return m.onContextKey(key)
	case modeUsage:
		return m.onUsageKey(key)
	}
	return m, nil
}

// onBucketsKey handles the bucket-list view.
func (m App) onBucketsKey(key string) (tea.Model, tea.Cmd) {
	n := len(m.filteredBuckets())
	switch {
	case matches(key, m.keys.Up):
		if m.bucketSel > 0 {
			m.bucketSel--
		}
	case matches(key, m.keys.Down):
		if m.bucketSel < n-1 {
			m.bucketSel++
		}
	case matches(key, m.keys.Top):
		m.bucketSel = 0
	case matches(key, m.keys.Bottom):
		m.bucketSel = max(0, n-1)
	case matches(key, m.keys.Search):
		return m.startSearch() // "/" filters bucket names (FR: bucket filter)
	case matches(key, m.keys.Menu):
		return m.openActionMenu()
	case matches(key, m.keys.Context):
		return m.openContextSwitch()
	case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
		// Quick context switch by number (k9s-style).
		if idx := int(key[0] - '1'); idx < len(m.contexts) {
			return m.applyContext(m.contexts[idx])
		}
	case matches(key, m.keys.Enter):
		fb := m.filteredBuckets()
		if m.bucketSel < 0 || m.bucketSel >= len(fb) {
			return m, nil
		}
		m.bucket = fb[m.bucketSel].Name
		m.prefix = ""
		m.search = ""
		return m.enterLevel()
	}
	return m, nil
}

// filteredBuckets returns the buckets whose name contains the (case-insensitive)
// bucket filter; the full list when no filter is set.
func (m App) filteredBuckets() []storage.Bucket {
	if m.bucketFilter == "" {
		return m.buckets
	}
	needle := strings.ToLower(m.bucketFilter)
	out := make([]storage.Bucket, 0, len(m.buckets))
	for _, b := range m.buckets {
		if strings.Contains(strings.ToLower(b.Name), needle) {
			out = append(out, b)
		}
	}
	return out
}

// openContextSwitch enters the context switcher positioned on the active context.
func (m App) openContextSwitch() (tea.Model, tea.Cmd) {
	m.mode = modeContextSwitch
	m.ctxSel = 0
	for i, n := range m.contexts {
		if n == m.ctxName {
			m.ctxSel = i
		}
	}
	return m, nil
}

// onContextKey handles selecting/applying a context.
func (m App) onContextKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case matches(key, m.keys.Up):
		if m.ctxSel > 0 {
			m.ctxSel--
		}
	case matches(key, m.keys.Down):
		if m.ctxSel < len(m.contexts)-1 {
			m.ctxSel++
		}
	case matches(key, m.keys.Back):
		m.mode = modeBuckets
	case matches(key, m.keys.Enter):
		if len(m.contexts) == 0 {
			m.mode = modeBuckets
			return m, nil
		}
		return m.applyContext(m.contexts[m.ctxSel])
	}
	return m, nil
}

// applyContext switches to the named context, rebuilds the backend, resets all
// browsing state, and reloads buckets. A no-op (back to buckets) when the context
// is already active or switching is disabled.
func (m App) applyContext(target string) (tea.Model, tea.Cmd) {
	if target == m.ctxName || m.resolve == nil {
		m.mode = modeBuckets
		return m, nil
	}
	be, err := m.resolve(target)
	if err != nil {
		m.err = err
		m.mode = modeBuckets
		return m, nil
	}
	m.raw = be.Store
	m.info = be
	// Preserve the arm intent across the switch; re-derive the context lock. Switching
	// to a writable context keeps write armed; switching to a readonly:true context
	// forces read-only via writable() (FR-029).
	m.ctxReadOnly = be.ReadOnly
	m.op = nil
	m.ctxName = target
	m.cache.Clear()
	m.buckets = nil
	m.bucketSel = 0
	m.bucketFilter = ""
	m.bucket, m.prefix, m.search = "", "", ""
	m.level = nil
	m.mode = modeBuckets
	ctx := (&m).beginLoad()
	return m, tea.Batch(loadBuckets(ctx, m.activeStore(), m.gen), spinnerTick())
}

// clampSelection keeps selection indices within bounds after resize/data change.
// The visible window is recomputed statelessly at render time (windowBounds), so
// only the selection indices need clamping here (Edge Case: resize reflow).
func (m *App) clampSelection() {
	if m.bucketSel >= len(m.buckets) {
		m.bucketSel = max(0, len(m.buckets)-1)
	}
	if m.level != nil && m.treeSel >= m.level.count() {
		m.treeSel = max(0, m.level.count()-1)
	}
}

// spinnerView renders the current spinner frame.
func (m App) spinnerView() string {
	if !m.loading {
		return ""
	}
	return spinnerFrames[m.spin%len(spinnerFrames)]
}

// errorText returns a human, secret-free message for the current error (FR-020).
func (m App) errorText() string {
	switch {
	case m.err == nil:
		return ""
	case errors.Is(m.err, storage.ErrAccessDenied):
		return "Access denied — check the credentials for this context."
	case errors.Is(m.err, storage.ErrNotFound):
		return "Not found — the bucket or object does not exist."
	case errors.Is(m.err, storage.ErrUnreachable):
		return "Backend unreachable — check the endpoint and your network."
	case errors.Is(m.err, storage.ErrInvalidConfig):
		return "Invalid configuration for this context."
	case errors.Is(m.err, storage.ErrReadOnly):
		return "This context is read-only — start s3s with --write to enable changes."
	case errors.Is(m.err, storage.ErrInvalidName):
		return "Invalid folder name — use a non-empty name without control characters."
	case errors.Is(m.err, errFolderExists):
		return "A folder with that name already exists here."
	case errors.Is(m.err, errConfirmMismatch):
		return "Confirmation did not match — nothing was changed."
	default:
		return "Something went wrong. Press a key to continue."
	}
}

// View renders the whole UI: a bordered body on top and a multi-line, Claude
// Code-style status footer at the bottom.
func (m App) View() tea.View {
	w := clampW(m.width)

	// The alt-screen overlays (help, action menu) render without the footer, so the
	// loud WRITE/RO badge is injected here too — it must be present on EVERY screen
	// (005 FR-027, write-toggle-contract C3).
	badge := writeBadge(m.writable()) + "\n"

	if m.mode == modeHelp {
		v := tea.NewView(badge + m.helpView())
		v.AltScreen = true
		return v
	}

	if m.mode == modeActionMenu {
		v := tea.NewView(badge + m.actionMenuView(w))
		v.AltScreen = true
		return v
	}

	footer := m.footerBlock(w)
	footerH := strings.Count(footer, "\n") + 1
	// Inner box height = total minus the footer and the two border lines. The box
	// body MUST NOT exceed this, or the footer (incl. the hints line) scrolls off.
	rows := m.height - footerH - 2
	if rows < 3 {
		rows = 3
	}
	// Table views render a 2-line header (column titles + rule) inside the box, so
	// the data-row budget is two fewer than the box's inner height.
	dataRows := rows - 2
	if dataRows < 1 {
		dataRows = 1
	}

	// An upload's file browser takes over the body while choosing a source.
	if m.op != nil && m.op.phase == phaseBrowse {
		body := boxView(m.browserTitle(), "", m.fileBrowserView(w-2, dataRows), w, rows)
		v := tea.NewView(body + "\n" + footer)
		v.AltScreen = true
		return v
	}

	var body string
	switch m.mode {
	case modeBuckets:
		body = boxView(m.resourceTitle(), m.selectionName(), m.bucketsView(w-2, dataRows), w, rows)
	case modeTree:
		body = boxView(m.resourceTitle(), m.selectionName(), m.treeView(w-2, dataRows), w, rows)
	case modeContextSwitch:
		body = boxView(m.resourceTitle(), m.selectionName(), m.contextView(w-2, dataRows), w, rows)
	case modeObject:
		body = boxView(m.resourceTitle(), m.objectKind(), m.objectView(w-2, rows), w, rows)
	case modeUsage:
		body = boxView(m.usageTitle(), "", m.usageView(w-2, dataRows), w, rows)
	}

	v := tea.NewView(body + "\n" + footer)
	v.AltScreen = true
	return v
}

// footerBlock is the Claude Code-style status footer: a thin separator, an info
// line (context · cluster · user · …) with one hue per parameter, a keybinding
// line, and a transient status line. Each line is fit to the width segment-by-
// segment so nothing wraps (which would otherwise hide a line).
func (m App) footerBlock(w int) string {
	// ≤ 3 rows: compact identity, one contextual hint row, optional status (FR-006).
	lines := []string{
		footerIdentityCompact(w, m.ctxName, m.info.Cluster, m.writable()),
		footerHints(hintCtx{
			mode:         m.mode,
			searchActive: m.searchActive(),
			multiContext: len(m.contexts) > 1,
			width:        w,
		}),
	}
	if s := m.statusLine(w); s != "" {
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

// searchActive reports whether a search/filter is applied or being typed (drives the
// footer's esc-clear vs esc-back disambiguation, FR-009).
func (m App) searchActive() bool {
	if m.mode == modeBuckets {
		return m.bucketFilter != "" || m.searching
	}
	return m.search != "" || m.searching
}

// statusLine is the transient bottom line: filter/search input, loading, or
// error. Plain text is truncated to width so it never wraps (which would push the
// box up and clip a footer line).
func (m App) statusLine(w int) string {
	// A pending write-arm confirmation owns the status line (005 US5).
	if m.armConfirm {
		return m.armConfirmLine()
	}
	// An interactive operation prompt (name/dest entry, confirmation) takes priority.
	// A running streaming op shows live progress; other phases fall through.
	if m.op != nil {
		switch m.op.phase {
		case phaseRunning:
			return m.opProgressLine()
		case phaseBrowse:
			return dimCellStyle.Render("↑/↓ select · →/Enter open · ←/h up · Esc cancel")
		default:
			return m.opPromptLine(w)
		}
	}
	if m.searching {
		// Bucket filter is instant; a tree search is debounced — show it is pending
		// (FR-016) so the delay reads as intentional.
		label, suffix := "search", "▏  searching…  (Enter apply · Esc clear)"
		if m.mode == modeBuckets {
			label, suffix = "filter", "▏  (Enter apply · Esc clear)"
		}
		input := truncate(m.searchInput, max(1, w-len(label)-2-len(suffix)))
		return accentStyle.Render(label+": ") + objCellStyle.Render(input) + dimCellStyle.Render(suffix)
	}
	if m.loading {
		// Name what is loading (FR-015); cancel is the back/escape key now (FR-029).
		what := "loading…"
		switch m.mode {
		case modeBuckets:
			what = "loading buckets…"
		case modeTree:
			what = "loading contents…"
		case modeObject:
			what = "loading object…"
		}
		return accentStyle.Render(m.spinnerView()) + dimCellStyle.Render(" "+what+"  (Esc to cancel)")
	}
	if m.notice != "" {
		// Success notices are green (noticeStyle), visually distinct from red errors (FR-018).
		return noticeStyle.Render(truncate(m.notice, max(1, w-1)))
	}
	if txt := m.errorText(); txt != "" {
		return errStyle.Render("error: " + truncate(txt, max(1, w-7)))
	}
	return ""
}

// resourceTitle is the left label on the box top border, e.g. "buckets[12]".
func (m App) resourceTitle() string {
	switch m.mode {
	case modeTree:
		loc := m.bucket
		if m.prefix != "" {
			loc += "/" + strings.TrimSuffix(sanitizeLabel(m.prefix), "/")
		}
		n := 0
		if m.level != nil {
			n = m.level.count()
		}
		more := ""
		if m.level != nil && !m.level.complete {
			more = "+"
		}
		if m.search != "" {
			loc += fmt.Sprintf("/%s*", sanitizeLabel(m.search))
		}
		return fmt.Sprintf("%s[%d%s]", loc, n, more)
	case modeContextSwitch:
		return fmt.Sprintf("contexts[%d]", len(m.contexts))
	case modeObject:
		if m.meta != nil {
			return sanitizeLabel(m.meta.Key)
		}
		return "object"
	default:
		fb := m.filteredBuckets()
		if m.bucketFilter != "" {
			return fmt.Sprintf("buckets[%d/%d]", len(fb), len(m.buckets))
		}
		return fmt.Sprintf("buckets[%d]", len(m.buckets))
	}
}

// objectKind is the highlighted center label for the object box (content kind).
func (m App) objectKind() string {
	if m.prev != nil {
		return m.prev.Kind.String()
	}
	return ""
}

// selectionName is the centered, highlighted label on the box top border —
// the currently selected item (k9s-style).
func (m App) selectionName() string {
	switch m.mode {
	case modeTree:
		if e := m.selected(); e != nil {
			return e.label
		}
	case modeContextSwitch:
		if m.ctxSel >= 0 && m.ctxSel < len(m.contexts) {
			return m.contexts[m.ctxSel]
		}
	default:
		fb := m.filteredBuckets()
		if m.bucketSel >= 0 && m.bucketSel < len(fb) {
			return fb[m.bucketSel].Name
		}
	}
	return ""
}

// breadcrumb describes the current location (used by the box title / tests).
func (m App) breadcrumb() string {
	switch m.mode {
	case modeBuckets:
		if m.bucketFilter != "" {
			return "filter: " + m.bucketFilter
		}
		return "/"
	case modeContextSwitch:
		return "select a context"
	default:
		loc := m.bucket
		if m.prefix != "" {
			loc += "/" + sanitizeLabel(m.prefix)
		}
		if m.search != "" {
			loc += fmt.Sprintf("  (search: %s)", sanitizeLabel(m.search))
		}
		return loc
	}
}

// bucketsView renders the bucket list table body (filtered) at the given width.
func (m App) bucketsView(w, rows int) string {
	fb := m.filteredBuckets()
	if len(fb) == 0 {
		if m.loading {
			return dimCellStyle.Render("Loading buckets…")
		}
		if m.bucketFilter != "" {
			return emptyStyle.Render(fmt.Sprintf("No buckets match %q.", m.bucketFilter))
		}
		return emptyStyle.Render("No buckets visible for this context.")
	}
	off, end := windowBounds(len(fb), m.bucketSel, rows)
	cols := []column{{"name", 0}, {"created", 19}}
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		data = append(data, []string{fb[i].Name, formatDate(fb[i].CreationDate)})
	}
	return renderTable(w, cols, data, nil, m.bucketSel-off)
}

// contextView renders the context switcher table body at the given width.
func (m App) contextView(w, rows int) string {
	if len(m.contexts) == 0 {
		return emptyStyle.Render("No contexts configured.")
	}
	off, end := windowBounds(len(m.contexts), m.ctxSel, rows)
	cols := []column{{"context", 0}, {"status", 10}}
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		status := ""
		if m.contexts[i] == m.ctxName {
			status = "active"
		}
		data = append(data, []string{m.contexts[i], status})
	}
	return renderTable(w, cols, data, nil, m.ctxSel-off)
}
