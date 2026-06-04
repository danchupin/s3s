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
	modeMetadata
	modePreview
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

// ContextSwitcher rebuilds Storage for a newly selected context. Injected by the
// entrypoint so the UI never imports config/SDK construction directly.
type ContextSwitcher func(name string) (storage.Storage, error)

// App is the root Bubble Tea model.
type App struct {
	store    storage.Storage
	ctxName  string
	contexts []string
	switchFn ContextSwitcher

	keys  keyMap
	cache *cache.Cache[*levelState]

	mode          mode
	prevMode      mode // to restore after help
	width, height int

	// buckets
	buckets   []storage.Bucket
	bucketSel int

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

	// search input
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

// New builds the root model. contexts is the list for the switcher; switchFn
// rebuilds Storage on context change (may be nil to disable switching).
func New(store storage.Storage, ctxName string, contexts []string, switchFn ContextSwitcher) App {
	m := App{
		store:    store,
		ctxName:  ctxName,
		contexts: contexts,
		switchFn: switchFn,
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
		m.mode = modeMetadata
		return m, nil

	case previewMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		p := msg.payload
		m.prev = &p
		m.prevOff = 0
		m.mode = modePreview
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
	case modeMetadata:
		return m.onMetadataKey(key)
	case modePreview:
		return m.onPreviewKey(key)
	case modeContextSwitch:
		return m.onContextKey(key)
	}
	return m, nil
}

// onBucketsKey handles the bucket-list view.
func (m App) onBucketsKey(key string) (tea.Model, tea.Cmd) {
	switch {
	case matches(key, m.keys.Up):
		if m.bucketSel > 0 {
			m.bucketSel--
		}
	case matches(key, m.keys.Down):
		if m.bucketSel < len(m.buckets)-1 {
			m.bucketSel++
		}
	case matches(key, m.keys.Top):
		m.bucketSel = 0
	case matches(key, m.keys.Bottom):
		m.bucketSel = max(0, len(m.buckets)-1)
	case matches(key, m.keys.Context):
		return m.openContextSwitch()
	case matches(key, m.keys.Enter):
		if len(m.buckets) == 0 {
			return m, nil
		}
		m.bucket = m.buckets[m.bucketSel].Name
		m.prefix = ""
		m.search = ""
		return m.enterLevel()
	}
	return m, nil
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
		target := m.contexts[m.ctxSel]
		if target == m.ctxName || m.switchFn == nil {
			m.mode = modeBuckets
			return m, nil
		}
		st, err := m.switchFn(target)
		if err != nil {
			m.err = err
			m.mode = modeBuckets
			return m, nil
		}
		// Reset all browsing state for the new backend.
		m.store = st
		m.ctxName = target
		m.cache.Clear()
		m.buckets = nil
		m.bucketSel = 0
		m.bucket, m.prefix, m.search = "", "", ""
		m.level = nil
		m.mode = modeBuckets
		ctx := (&m).beginLoad()
		return m, tea.Batch(loadBuckets(ctx, m.store, m.gen), spinnerTick())
	}
	return m, nil
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

// View renders the whole UI.
func (m App) View() tea.View {
	var b strings.Builder

	b.WriteString(m.headerView())
	b.WriteString("\n\n")

	switch m.mode {
	case modeBuckets:
		b.WriteString(m.bucketsView())
	case modeTree:
		b.WriteString(m.treeView())
	case modeMetadata:
		b.WriteString(m.metadataView())
	case modePreview:
		b.WriteString(m.previewView())
	case modeContextSwitch:
		b.WriteString(m.contextView())
	case modeHelp:
		b.WriteString(strings.Join(helpLines(), "\n"))
	}

	b.WriteString("\n\n")
	b.WriteString(m.footerView())

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// headerView shows the active context and breadcrumb.
func (m App) headerView() string {
	loc := m.breadcrumb()
	return fmt.Sprintf("s3s  [context: %s]  %s", m.ctxName, loc)
}

// breadcrumb describes the current location (FR-009).
func (m App) breadcrumb() string {
	switch m.mode {
	case modeBuckets:
		return "buckets"
	case modeContextSwitch:
		return "contexts"
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

// footerView shows loading/error/hints.
func (m App) footerView() string {
	if m.searching {
		return fmt.Sprintf("search: %s_  (Enter to apply, Esc to clear)", m.searchInput)
	}
	if m.loading {
		return fmt.Sprintf("%s loading…  (x to cancel)", m.spinnerView())
	}
	if txt := m.errorText(); txt != "" {
		return "error: " + txt
	}
	return "↑/↓ move · →/Enter open · ← back · / search · i meta · p preview · r refresh · c context · ? help · q quit"
}

// bucketsView renders the bucket list.
func (m App) bucketsView() string {
	if len(m.buckets) == 0 {
		if m.loading {
			return "Loading buckets…"
		}
		return "No buckets visible for this context."
	}
	var b strings.Builder
	for i, bk := range m.buckets {
		cursor := "  "
		if i == m.bucketSel {
			cursor = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, bk.Name)
	}
	return strings.TrimRight(b.String(), "\n")
}

// contextView renders the context switcher.
func (m App) contextView() string {
	if len(m.contexts) == 0 {
		return "No contexts configured."
	}
	var b strings.Builder
	b.WriteString("Switch context:\n\n")
	for i, n := range m.contexts {
		cursor := "  "
		if i == m.ctxSel {
			cursor = "> "
		}
		marker := ""
		if n == m.ctxName {
			marker = " (active)"
		}
		fmt.Fprintf(&b, "%s%s%s\n", cursor, n, marker)
	}
	return strings.TrimRight(b.String(), "\n")
}
