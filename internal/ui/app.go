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
	"charm.land/lipgloss/v2"

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
	modeUsage       // du analytics view — 005 US2
	modeCommand     // `:` command bar — 006 US3
	modeConnections // in-app connection manager list — 006 US4
	modeConnForm    // add-connection form — 006 US4
)

// selKind classifies the current tree selection for the hint bar and direct-action gating.
type selKind int

const (
	selNone selKind = iota
	selObject
	selFolder
)

// selKind reports the kind of the current tree selection (selNone outside the tree
// or when nothing is selected). Used by the hint bar and direct-action dispatch.
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

	// details/preview pane (006 US2) — debounced per-selection load; NEVER flips modeObject
	paneGen     int
	paneSelKey  string                  // key the in-flight/loaded pane data belongs to
	paneMeta    *storage.ObjectMetadata // debounced HeadObject result
	panePrev    *preview.Payload        // debounced ranged-GET preview
	paneVisible bool                    // false collapses the pane on narrow terminals

	// command bar (006 US3 / modeCommand)
	cmdInput string

	// connection manager (006 US4)
	connect Connector // nil disables in-app connection add
	connSel int       // selection in modeConnections list
	form    *connForm // active add-connection form (modeConnForm)

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

	// multi-select (005 US3): marked OBJECT keys in the current level, cleared on nav
	sel map[string]bool

	// sort order (005 US4): session-persistent across navigation
	sortBy  sortCol
	sortAsc bool

	// du analytics (005 US2 / modeUsage)
	usage       *storage.UsageReport // completed report (nil while scanning)
	usageSel    int                  // selected child row (for drill-down)
	usageBucket string
	usagePrefix string
	usageReturn mode                  // mode to restore when leaving the analytics view (005 US2)
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
func New(initial Backend, ctxName string, contexts []string, resolve Resolver, connect Connector, imgProto preview.Protocol) App {
	m := App{
		raw:         initial.Store,
		info:        initial,
		ctxName:     ctxName,
		contexts:    contexts,
		resolve:     resolve,
		connect:     connect,
		imgProto:    imgProto,
		armed:       initial.Writable,
		ctxReadOnly: initial.ReadOnly,
		sortAsc:     true, // default to ascending (A→Z / smallest / oldest first) — 005 FR-020
		paneVisible: true, // details pane on by default; collapses on narrow terminals (006 US2)
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

// afterSelectionMove rearms the debounced details-pane load for the new tree selection
// (006 US2). It bumps the pane generation (superseding any in-flight pane load for the
// previous row), clears the stale pane data, and — for an object selection — schedules a
// paneTick. Folders/levels need no fetch (instant summary), so no tick is scheduled.
func (m App) afterSelectionMove() (tea.Model, tea.Cmd) {
	m.paneGen++
	m.paneMeta = nil
	m.panePrev = nil
	if e := m.selected(); e != nil && !e.isDir {
		m.paneSelKey = e.full
		return m, paneTickCmd(m.paneGen, m.paneSelKey)
	}
	m.paneSelKey = ""
	return m, nil
}

// onPaneTick fires the debounced pane fetch if the selection is unchanged (gen+key match)
// and still on an object; otherwise the tick is for a scrolled-past row and is dropped
// (006 US2, FR-009/FR-012). Pane loads use a background context — they are cheap, bounded,
// and superseded by the generation check, so they never disturb the main load.
func (m App) onPaneTick(msg paneTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.paneGen || msg.key != m.paneSelKey || m.paneSelKey == "" {
		return m, nil
	}
	e := m.selected()
	if e == nil || e.isDir || e.obj == nil || e.full != msg.key {
		return m, nil
	}
	st := m.activeStore()
	return m, tea.Batch(
		loadPaneMeta(context.Background(), st, m.bucket, e.full, m.paneGen),
		loadPanePreview(context.Background(), st, m.bucket, e.full, e.obj.Size, m.paneGen),
	)
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
			// Count ticks during a running op so the progress bar appears only after a
			// brief threshold (007 US6 / FR-035) — fast ops finish first and never flash.
			if m.op != nil && m.op.phase == phaseRunning {
				m.op.ticks++
			}
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

	case paneTickMsg:
		return m.onPaneTick(msg)

	case paneMetaMsg:
		if msg.gen != m.paneGen || msg.key != m.paneSelKey {
			return m, nil // scrolled past — drop
		}
		md := msg.md
		m.paneMeta = &md
		return m, nil

	case panePreviewMsg:
		if msg.gen != m.paneGen || msg.key != m.paneSelKey {
			return m, nil
		}
		p := msg.payload
		m.panePrev = &p
		return m, nil

	case connTestedMsg:
		return m.onConnTested(msg)

	case connSavedMsg:
		return m.onConnSaved(msg)

	case connDeletedMsg:
		return m.onConnDeleted(msg)

	case contextResolvedMsg:
		return m.onContextResolved(msg)

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

	// ctrl+c always quits, even inside a modal text input (search / command bar /
	// connection form) where `q` is a literal character — the universal escape hatch
	// must never be trapped by a modal mode handler.
	if key == "ctrl+c" {
		(&m).cancelLoad()
		return m, tea.Quit
	}

	// While typing a search query, the input owns most keys.
	if m.searching {
		return m.onSearchKey(msg)
	}

	// A running mutation is modal in EVERY mode: only cancel (Esc/Back) and quit work.
	// Checked before the mode-specific handlers so a running op (incl. a connection delete
	// in modeConnections) never traps quit/cancel behind a mode handler (review #7).
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

	// The command bar / connection form own all keys while open (text input) — modal,
	// before the global bindings so typing "q"/":" is not intercepted (006 US3/US4).
	if m.mode == modeCommand {
		return m.onCommandKey(key, msg)
	}
	if m.mode == modeConnForm {
		return m.onConnFormKey(key, msg)
	}
	if m.mode == modeConnections {
		// A connection-delete confirmation (007 US5) is an active op: it owns keys (typed
		// name entry) before the list navigation, so typing the name isn't intercepted.
		if m.op != nil {
			return m.onOpKey(key, msg)
		}
		return m.onConnectionsKey(key)
	}

	// A pending write-arm confirmation is modal: y/N resolves it (005 US5).
	if m.armConfirm {
		return m.onArmConfirmKey(key)
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

	if matches(key, m.keys.Help) {
		m.prevMode = m.mode
		m.mode = modeHelp
		return m, nil
	}

	// `:` opens the command bar (006 US3). Search/op/form already own keys above, so it
	// never interferes with in-progress text entry (FR-019).
	if matches(key, m.keys.Command) && m.canOpenCommand() {
		return m.startCommand()
	}

	// Add-connection affordance (007 US2): a visible, discoverable key in the list modes
	// opens the in-app connection manager (FR-011/FR-012).
	if matches(key, m.keys.AddConn) && m.connect != nil && (m.mode == modeBuckets || m.mode == modeTree) {
		return m.openConnections()
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
	default:
		// Dangerous-action chord (007 US4): ctrl+x → delete the selected bucket.
		if mm, cmd, ok := m.dispatchChord(key); ok {
			return mm, cmd
		}
		// Direct single-key actions (006 US1): analyze / refresh on the bucket list.
		if mm, cmd, ok := m.dispatchActionKey(key); ok {
			return mm, cmd
		}
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

// applyContext begins a switch to the named context. Resolving the backend can be slow
// (a keychain unlock or an external credential command, 005 US6), so it runs OFF the
// event loop — Update applies the result via contextResolvedMsg (Constitution II). A
// no-op (back to buckets) when the context is already active or switching is disabled.
func (m App) applyContext(target string) (tea.Model, tea.Cmd) {
	if target == m.ctxName || m.resolve == nil {
		m.mode = modeBuckets
		return m, nil
	}
	m.mode = modeBuckets
	(&m).beginLoad() // bump the generation + show the spinner; supersedes a prior switch
	return m, tea.Batch(resolveContextCmd(m.resolve, target, m.gen), spinnerTick())
}

// resolveContextCmd resolves a context's backend off the event loop (credential
// resolution may block), reporting the outcome via contextResolvedMsg.
func resolveContextCmd(resolve Resolver, target string, gen int) tea.Cmd {
	return func() tea.Msg {
		be, err := resolve(target)
		return contextResolvedMsg{gen: gen, target: target, be: be, err: err}
	}
}

// onContextResolved applies a resolved context switch (or surfaces its error), then
// resets browsing state and loads the new bucket list. Stale results (a superseded
// switch) are dropped by the generation check.
func (m App) onContextResolved(msg contextResolvedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil // superseded
	}
	if msg.err != nil {
		m.loading = false
		m.err = msg.err
		m.mode = modeBuckets
		return m, nil
	}
	be := msg.be
	m.raw = be.Store
	m.info = be
	// Preserve the arm intent across the switch; re-derive the context lock. Switching
	// to a writable context keeps write armed; switching to a readonly:true context
	// forces read-only via writable() (FR-029).
	m.ctxReadOnly = be.ReadOnly
	m.op = nil
	m.ctxName = msg.target
	m.cache.Clear()
	m.buckets = nil
	m.bucketSel = 0
	m.bucketFilter = ""
	m.bucket, m.prefix, m.search = "", "", ""
	m.level = nil
	m.sel = nil
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

	// The alt-screen help overlay renders without the footer, so the loud WRITE/RO
	// badge is injected here too — it must be present on EVERY screen (005 FR-027,
	// write-toggle-contract C3).
	badge := writeBadge(m.writable()) + "\n"

	if m.mode == modeHelp {
		v := tea.NewView(badge + m.helpView())
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

	// The command bar overlays the footer; its body is the underlying (prev) view (006 US3).
	bodyMode := m.mode
	if m.mode == modeCommand {
		bodyMode = m.prevMode
	}

	var body string
	switch bodyMode {
	case modeBuckets:
		body = m.listWithPane(m.resourceTitle(), m.selectionName(), w, rows, dataRows,
			func(lw, dr int) string { return m.bucketsView(lw, dr) })
	case modeTree:
		body = m.listWithPane(m.resourceTitle(), m.selectionName(), w, rows, dataRows,
			func(lw, dr int) string { return m.treeView(lw, dr) })
	case modeContextSwitch:
		body = boxView(m.resourceTitle(), m.selectionName(), m.contextView(w-2, dataRows), w, rows)
	case modeObject:
		body = boxView(m.resourceTitle(), m.objectKind(), m.objectView(w-2, rows), w, rows)
	case modeUsage:
		body = boxView(m.usageTitle(), "", m.usageView(w-2, dataRows), w, rows)
	case modeConnections:
		body = boxView("connections", "", m.connectionsView(w-2, dataRows), w, rows)
	case modeConnForm:
		body = boxView("add connection", "", m.connFormView(w-2), w, rows)
	}

	// Binary-tier confirmation (007 US4): a centered popup over the body (the typed tier
	// uses the prominent inline form in the footer instead). Replaces the body so the
	// dialog is centered and never clipped (SC-009).
	if m.op != nil && m.op.phase == phaseConfirm && m.op.tier != confirmTyped {
		bodyH := strings.Count(body, "\n") + 1
		body = m.confirmPopupView(w, bodyH)
	}

	v := tea.NewView(body + "\n" + footer)
	v.AltScreen = true
	return v
}

// paneSplitMin is the minimum terminal width at which the details pane is shown beside
// the list; below it the list spans full width and the pane collapses (006 US2, FR-013).
const paneSplitMin = 100

// listWithPane composes the browse body: on a wide terminal it joins the list (bounded
// width) and the persistent details pane side by side; on a narrow terminal it returns
// the full-width list and the pane collapses (FR-008/FR-013). renderList draws the list
// table at the width it is given.
func (m App) listWithPane(title, sel string, w, rows, dataRows int, renderList func(lw, dr int) string) string {
	if w < paneSplitMin || !m.paneVisible {
		return boxView(title, sel, renderList(w-2, dataRows), w, rows)
	}
	paneW := w / 3
	if paneW > 40 {
		paneW = 40
	}
	if paneW < 24 {
		paneW = 24
	}
	listW := w - paneW
	listBody := boxView(title, sel, renderList(listW-2, dataRows), listW, rows)
	paneBody := boxView("details", "", m.paneView(paneW-2, rows-2), paneW, rows)
	return lipgloss.JoinHorizontal(lipgloss.Top, listBody, paneBody)
}

// footerBlock is the Claude Code-style status footer: a thin separator, an info
// line (context · cluster · user · …) with one hue per parameter, a keybinding
// line, and a transient status line. Each line is fit to the width segment-by-
// segment so nothing wraps (which would otherwise hide a line).
func (m App) footerBlock(w int) string {
	// ≤ 3 rows: compact identity, one contextual hint row, optional status (FR-006).
	// In the list modes the always-visible hint bar advertises the valid direct actions
	// for the current selection/capability (006 US1, FR-003); other modes keep the
	// legacy contextual hints.
	// List modes (buckets/tree) render the three-block command bar (007 US1), which
	// carries its own identity + loud arm badge in the info block. Other modes keep the
	// compact identity line + generic contextual hints.
	var lines []string
	if m.mode == modeBuckets || m.mode == modeTree {
		lines = append(lines, m.commandBarView(w))
	} else {
		hintLine := footerHints(hintCtx{
			mode:         m.mode,
			searchActive: m.searchActive(),
			multiContext: len(m.contexts) > 1,
			width:        w,
		})
		lines = append(lines, footerIdentityCompact(w, m.ctxName, m.info.Cluster, m.writable()), hintLine)
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
	// The command bar owns the status line while open (006 US3).
	if m.mode == modeCommand {
		return m.commandLine(w)
	}
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
	// Multi-select summary (005 US3): count + combined size of the marked objects.
	if m.selCount() > 0 {
		return noticeStyle.Render(fmt.Sprintf("%d selected · %s  (d/x/y: bulk download/delete/copy)", m.selCount(), humanSize(m.selSize())))
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
		return fmt.Sprintf("%s[%d%s] %s", loc, n, more, m.sortIndicator())
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

// ctxTable renders a context-style two-column table (name + active status) windowed to
// rows. Shared by the context switcher and the connection manager list (006 US4).
func ctxTable(w, rows, sel int, header string, items []string, activeName string) string {
	off, end := windowBounds(len(items), sel, rows)
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		status := ""
		if items[i] == activeName {
			status = "active"
		}
		data = append(data, []string{items[i], status})
	}
	return renderTable(w, []column{{header, 0}, {"status", 10}}, data, nil, sel-off)
}

// contextView renders the context switcher table body at the given width.
func (m App) contextView(w, rows int) string {
	if len(m.contexts) == 0 {
		return emptyStyle.Render("No contexts configured.")
	}
	return ctxTable(w, rows, m.ctxSel, "context", m.contexts, m.ctxName)
}
