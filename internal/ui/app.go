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
)

// levelState is the accumulated, cached view of one tree node across pages.
type levelState struct {
	dirs      []string
	objects   []storage.ObjectRef
	nextToken *string
	complete  bool
}

func (l *levelState) count() int { return len(l.dirs) + len(l.objects) }

// Backend describes the storage client plus the display metadata for the active
// context (shown in the header). The UI never builds clients itself.
type Backend struct {
	Store    storage.Storage
	Cluster  string
	User     string
	Endpoint string
	Region   string
}

// Resolver rebuilds a Backend for a named context (context switch). May be nil
// to disable switching.
type Resolver func(context string) (Backend, error)

// App is the root Bubble Tea model.
type App struct {
	store    storage.Storage
	info     Backend
	ctxName  string
	contexts []string
	resolve  Resolver
	imgProto preview.Protocol

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
	treeOff int

	// metadata / preview panes
	meta    *storage.ObjectMetadata
	prev    *preview.Payload
	prevOff int

	// context switcher
	ctxSel int

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
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// New builds the root model for an initial backend. resolve rebuilds the backend
// on context change (nil disables switching). imgProto is the detected terminal
// image protocol for previews.
func New(initial Backend, ctxName string, contexts []string, resolve Resolver, imgProto preview.Protocol) App {
	m := App{
		store:    initial.Store,
		info:     initial,
		ctxName:  ctxName,
		contexts: contexts,
		resolve:  resolve,
		imgProto: imgProto,
		keys:     defaultKeys(),
		cache:    cache.New[*levelState](),
		mode:     modeBuckets,
		width:    80,
		height:   24,
	}
	// Arm the initial bucket load here, not in Init: in Bubble Tea v2 Init returns
	// only a Cmd and cannot mutate the model, so the generation/context must be set
	// on the model the program actually keeps.
	(&m).beginLoad()
	return m
}

// Init kicks off the initial bucket load armed by New.
func (m App) Init() tea.Cmd {
	return tea.Batch(loadBuckets(m.loadCtx, m.store, m.gen), spinnerTick())
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

	// While typing a search query, the input owns most keys.
	if m.searching {
		return m.onSearchKey(msg)
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

	// Cancel in-flight load.
	if matches(key, m.keys.Cancel) && m.loading {
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
	m.store = be.Store
	m.info = be
	m.ctxName = target
	m.cache.Clear()
	m.buckets = nil
	m.bucketSel = 0
	m.bucketFilter = ""
	m.bucket, m.prefix, m.search = "", "", ""
	m.level = nil
	m.mode = modeBuckets
	ctx := (&m).beginLoad()
	return m, tea.Batch(loadBuckets(ctx, m.store, m.gen), spinnerTick())
}

// clampSelection keeps selection indices within bounds after resize/data change
// and preserves the visible window (Edge Case: resize reflow).
func (m *App) clampSelection() {
	if m.bucketSel >= len(m.buckets) {
		m.bucketSel = max(0, len(m.buckets)-1)
	}
	if m.level != nil && m.treeSel >= m.level.count() {
		m.treeSel = max(0, m.level.count()-1)
	}
	m.adjustTreeWindow()
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
	default:
		return "Something went wrong. Press a key to continue."
	}
}

// View renders the whole UI: a bordered body on top and a multi-line, Claude
// Code-style status footer at the bottom.
func (m App) View() tea.View {
	w := clampW(m.width)

	if m.mode == modeHelp {
		v := tea.NewView(m.helpView())
		v.AltScreen = true
		return v
	}

	footer := m.footerBlock(w)
	footerH := strings.Count(footer, "\n") + 1
	rows := m.height - footerH - 2 - 1 // box borders (2) + gap (1)
	if rows < 1 {
		rows = 1
	}

	var body string
	switch m.mode {
	case modeBuckets:
		body = boxView(m.resourceTitle(), m.selectionName(), m.bucketsView(w-2, rows), w, rows)
	case modeTree:
		body = boxView(m.resourceTitle(), m.selectionName(), m.treeView(w-2, rows), w, rows)
	case modeContextSwitch:
		body = boxView(m.resourceTitle(), m.selectionName(), m.contextView(w-2, rows), w, rows)
	case modeObject:
		body = boxView(m.resourceTitle(), m.objectKind(), m.objectView(w-2, rows), w, rows)
	}

	v := tea.NewView(body + "\n" + footer)
	v.AltScreen = true
	return v
}

// footerBlock is the Claude Code-style status footer: a thin separator, an info
// line (context · cluster · user · …), a keybinding line, and a transient status
// line. Items are joined with dim "·" separators.
func (m App) footerBlock(w int) string {
	dot := dimCellStyle.Render(" · ")

	item := func(label, val string, lim int) string {
		return dimCellStyle.Render(label+" ") + objCellStyle.Render(truncate(val, lim))
	}

	info := []string{roStyle.Render("●") + " " + accentStyle.Render(m.ctxName) + dimCellStyle.Render(" [RO]")}
	if m.info.Cluster != "" {
		info = append(info, item("cluster", m.info.Cluster, 24))
	}
	if m.info.User != "" {
		info = append(info, item("user", m.info.User, 24))
	}
	if m.info.Endpoint != "" {
		info = append(info, item("endpoint", m.info.Endpoint, 44))
	}
	if m.info.Region != "" {
		info = append(info, item("region", m.info.Region, 16))
	}
	info = append(info, item("rev", Version, 16))

	hint := func(k, a string) string {
		return accentStyle.Render(k) + " " + dimCellStyle.Render(a)
	}
	hints := []string{
		hint("enter", "open"), hint("/", "filter"), hint("r", "refresh"),
		hint("c", "context"), hint("1-9", "switch"), hint("?", "help"), hint("q", "quit"),
	}

	lines := []string{
		ruleStyle.Render(strings.Repeat("─", w)),
		strings.Join(info, dot),
		strings.Join(hints, dot),
	}
	if s := m.statusLine(); s != "" {
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

// statusLine is the transient bottom line: filter/search input, loading, or error.
func (m App) statusLine() string {
	if m.searching {
		label := "search"
		if m.mode == modeBuckets {
			label = "filter"
		}
		return accentStyle.Render(label+": ") + objCellStyle.Render(m.searchInput) +
			dimCellStyle.Render("▏  (Enter apply · Esc clear)")
	}
	if m.loading {
		return accentStyle.Render(m.spinnerView()) + dimCellStyle.Render(" loading…  (x to cancel)")
	}
	if txt := m.errorText(); txt != "" {
		return errStyle.Render("error: " + txt)
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
