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
	"time"

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
	modeCommand     // `:` command bar
	modeConnections // in-app connection manager list
	modeConnForm    // add-connection form
	modeAddBucket   // "+ add bucket" name input
	modeHealth      // full-screen operator health card
)

// zone names which interactive column owns keyboard input in the multi-pane browse
// : the bucket list or the objects zone. It is a focus selector layered on the
// existing mode machine — meaningful only while m.mode == modeBuckets at the Dual/Full
// tiers; in the Single tier the mode stack (modeBuckets/modeTree) disambiguates and the
// focus is implicitly the single visible column.
type zone int

const (
	zoneBuckets zone = iota // default — the bucket list owns input
	zoneObjects             // the objects zone (highlighted bucket's level) owns input
)

// selKind classifies the current tree selection for the hint bar and direct-action gating.
type selKind int

const (
	selNone selKind = iota
	selObject
	selFolder
)

// inLevel reports whether the current view operates on a loaded object level — the
// full-screen tree (modeTree) OR the objects zone of the two-pane browse (modeBuckets with
// focus crossed in). Both reuse m.level/treeSel, so selection + per-item actions apply to
// both.
func (m App) inLevel() bool {
	return m.mode == modeTree || (m.mode == modeBuckets && m.focusZone == zoneObjects)
}

// selKind reports the kind of the current level selection (selNone outside a level or when
// nothing is selected). Used by the hint bar and direct-action dispatch.
func (m App) selKind() selKind {
	if !m.inLevel() {
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
// state. Writable is the *initial* arm intent (the --write flag); ReadOnly
// is the context's absolute readonly:true lock.
type Backend struct {
	Store       storage.Storage
	Cluster     string
	User        string
	Endpoint    string
	Region      string
	Writable    bool   // initial write-arm intent (--write)
	ReadOnly    bool   // context is readonly:true — never armable
	DownloadDir string // default local download dir from config
	// PathStyle mirrors the cluster's addressing style so copied HTTPS URLs match
	// what the user's own tooling expects.
	PathStyle bool
	// UsageScanBudget caps the ambient usage scan in enumerated objects
	// the RESOLVED config value (main applies the default); 0 = ambient scanning off.
	UsageScanBudget int
	// Health-card small-object warning knobs; zero ⇒ 128 KiB / 0.5.
	HealthSmallObjectKiB   int
	HealthSmallObjectShare float64
	// PinnedBuckets are the connection's declared bucket names. Non-empty ⇒ the
	// bucket list is rendered from these and ListBuckets is never called (scoped creds).
	PinnedBuckets []string
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
	armed       bool // runtime write-arm intent (toggle / --write initial)
	ctxReadOnly bool // active context is readonly:true (absolute lock)

	keys  keyMap
	cache *cache.Cache[*levelState]

	mode          mode
	prevMode      mode // to restore after help
	width, height int

	// buckets
	buckets      []storage.Bucket
	bucketSel    int
	bucketFilter string // client-side name filter at the bucket list
	// bucketLoadGen debounces the objects-zone reload as the bucket cursor moves
	// each move bumps it + schedules a bucketTickMsg; only the settled tick fires the load
	// so fast scrolling never issues a ListLevel per intermediate bucket.
	bucketLoadGen int

	// tree navigation
	bucket  string
	prefix  string
	search  string
	level   *levelState
	treeSel int

	// multi-pane focus: which zone owns input while modeBuckets at Dual/Full.
	// The objects zone reuses m.level (content) + m.treeSel (cursor); focusZone only
	// selects which cursor the nav keys move and which box renders active.
	focusZone zone
	// objReturn is the browse mode to restore when leaving the full-screen object view
	//: modeBuckets when opened from the objects zone, modeTree from the Single
	// tier. Defaults to modeBuckets (the zero value) — set in openObject.
	objReturn mode

	// metadata / preview panes
	meta       *storage.ObjectMetadata
	prev       *preview.Payload
	prevOff    int
	rawPreview bool //: pretty↔raw toggle; reset per payload

	// context switcher
	ctxSel int

	// details/preview pane — debounced per-selection load; NEVER flips modeObject
	paneGen     int
	paneSelKey  string                  // key the in-flight/loaded pane data belongs to
	paneMeta    *storage.ObjectMetadata // debounced HeadObject result
	panePrev    *preview.Payload        // debounced ranged-GET preview
	paneVisible bool                    // false collapses the pane on narrow terminals

	// command bar
	cmdInput string

	// connection manager
	connect Connector      // nil disables in-app connection add
	connSel int            // selection in modeConnections list
	form    *connForm      // active add-connection form (modeConnForm)
	addForm *bucketAddForm // active "+ add bucket" input (modeAddBucket)

	// bucketsScoped marks the current bucket list as scoped (pins present, or list-all
	// failed/denied/empty): the "+ add bucket" row is shown only then.
	bucketsScoped bool

	// search/filter input
	searching    bool
	searchInput  string
	searchGen    int
	filterBefore string // committed filter term captured when the input opened

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

	// write-mode arm confirmation pending: true while awaiting y/N to arm write
	armConfirm bool

	// reveal/inspect popup: non-nil while showing the full identifier of a selection
	reveal *revealState

	// field-select copy overlay: non-nil while choosing a field
	fieldCopy *fieldCopyState

	// copy/share menu overlay: non-nil while open
	copyMenu *copyMenuState

	// health card: target, probe lifecycle, MPU session cache, warning knobs
	healthBucket string
	healthPrefix string
	healthGen    int                                      // MPU-probe generation
	healthCancel context.CancelFunc                       // cancels the in-flight probe
	mpuResults   *cache.Cache[*storage.IncompleteUploads] // keyed like usageResults
	healthKiB    int                                      // small-object threshold (KiB)
	healthShare  float64                                  // warning share threshold

	// now is the injectable clock for relative dates (017 D14); tests pin it.
	now func() time.Time

	// multi-select: marked OBJECT keys in the current level, cleared on nav
	sel map[string]bool

	// sort order: session-persistent across navigation
	sortBy  sortCol
	sortAsc bool

	// inline usage: a dwell-gated, generation-guarded, session-cached
	// background UsageOf scan whose totals + breakdown render in the details pane. The
	// scan owns its OWN generation + cancel, ISOLATED from m.gen/loadCancel (the main
	// load), so a level load never cancels a usage scan and vice-versa.
	usageResults  *cache.Cache[*storage.UsageReport] // session cache keyed by (ctx,bucket,prefix)
	usageProg     storage.UsageProgress              // running partial during the active scan
	usageScanKey  cache.Key                          // target of the in-flight scan
	usageGen      int                                // scan generation (NOT m.gen)
	usageCancel   context.CancelFunc                 // cancels the in-flight scan's own ctx
	usageCh       chan usageEvent                    // in-flight scan progress channel
	usageBudget   int                                // ambient-scan cap; 0 = ambient off
	usageScanFull bool                               // the in-flight scan is the explicit FULL scan

	// inline "more detail" sections: at most ONE renders at a time (budget
	// gate). detailSection selects which; the loads below back the tags/config sections.
	detailSection detailSection
	breakdownSel  int                   // selected child row in the usage breakdown
	objectTags    *storage.ObjectTags   // lazily-loaded tags for the focused object
	tagsErr       error                 // non-nil when the tags load was denied/failed
	bucketCfg     *storage.BucketConfig // lazily-loaded config for the focused bucket
	detailKey     string                // object key / bucket the tags/config belong to
	detailGen     int                   // generation for tag/config loads

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
		raw:          initial.Store,
		info:         initial,
		ctxName:      ctxName,
		contexts:     contexts,
		resolve:      resolve,
		connect:      connect,
		imgProto:     imgProto,
		armed:        initial.Writable,
		ctxReadOnly:  initial.ReadOnly,
		sortAsc:      true, // default to ascending (A→Z / smallest / oldest first)
		paneVisible:  true, // details pane on by default; collapses on narrow terminals
		keys:         defaultKeys(),
		cache:        cache.New[*levelState](),
		usageResults: cache.New[*storage.UsageReport](),
		usageBudget:  initial.UsageScanBudget,
		mpuResults:   cache.New[*storage.IncompleteUploads](),
		healthKiB:    healthKiBOrDefault(initial.HealthSmallObjectKiB),
		healthShare:  healthShareOrDefault(initial.HealthSmallObjectShare),
		now:          time.Now,
		mode:         modeBuckets,
		width:        80,
		height:       24,
	}
	// No active backend (nil store) — don't try to load buckets from a backend that does not
	// exist. If connections already exist but none is active (no current-context/flag/env)
	// open the connection manager so the user PICKS one in the UI. Otherwise (true first run
	// no connections yet) open the add-connection form. beginLoad is skipped — there is
	// nothing to load until a context is chosen.
	if m.raw == nil && connect != nil {
		if len(contexts) > 0 {
			m.mode = modeConnections
			return m
		}
		m.mode = modeConnForm
		m.form = &connForm{pathStyle: true} // path-style default (Ceph RGW / MinIO)
		return m
	}
	// Arm the initial bucket load here, not in Init: in Bubble Tea v2 Init returns
	// only a Cmd and cannot mutate the model, so the generation/context must be set
	// on the model the program actually keeps.
	(&m).beginLoad()
	return m
}

// healthKiBOrDefault / healthShareOrDefault apply the 017 D8 defaults for zero-valued
// Backend knobs (test helpers and older callers construct Backend without them).
func healthKiBOrDefault(v int) int {
	if v == 0 {
		return 128
	}
	return v
}

func healthShareOrDefault(v float64) float64 {
	if v == 0 {
		return 0.5
	}
	return v
}

// writable reports whether mutations are currently permitted: the session is armed
// AND the active context is not hard-locked readonly. This
// derived value replaces the old static field, recomputed on every toggle/switch.
func (m App) writable() bool { return m.armed && !m.ctxReadOnly }

// activeStore returns the backend guarded for the current arm state: the raw client
// when writable, else a read-only wrapper that refuses mutations without a network
// call. This is the single runtime enforcement point — every operation uses it, so a
// mutating call is impossible while disarmed.
func (m App) activeStore() storage.Storage { return storage.Guard(m.raw, m.writable()) }

// Init kicks off the initial bucket load armed by New. On first run (no backend yet
// the add-connection form is open) there is nothing to load.
func (m App) Init() tea.Cmd {
	if m.raw == nil {
		return nil
	}
	return tea.Batch(loadBuckets(m.loadCtx, m.activeStore(), m.gen, m.info.PinnedBuckets), spinnerTick())
}

// beginLoad cancels any in-flight load, bumps the generation, and starts a fresh
// cancellable context. Returns the context the new load must use; it is also
// stored on the model so Init/refresh can reference it.
func (m *App) beginLoad() context.Context {
	if m.loadCancel != nil {
		m.loadCancel()
	}
	// A navigation/main load also supersedes any in-flight inline usage scan: cancel its
	// own context and bump the usage generation so its late messages are dropped (016
	//, isolated from m.gen).
	if m.usageCancel != nil {
		m.usageCancel()
		m.usageCancel = nil
	}
	m.usageGen++
	m.usageCh = nil
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
// It bumps the pane generation (superseding any in-flight pane load for the
// previous row), clears the stale pane data, and — for an object selection — schedules a
// paneTick. Folders/levels need no fetch (instant summary), so no tick is scheduled.
func (m App) afterSelectionMove() (tea.Model, tea.Cmd) {
	usageCmd := (&m).armUsageScan() // dwell-gated inline usage for the new target
	m.paneGen++
	m.paneMeta = nil
	m.panePrev = nil
	if e := m.selected(); e != nil && !e.isDir {
		m.paneSelKey = e.full
		return m, tea.Batch(paneTickCmd(m.paneGen, m.paneSelKey), usageCmd)
	}
	m.paneSelKey = ""
	return m, usageCmd
}

// onPaneTick fires the debounced pane fetch if the selection is unchanged (gen+key match)
// and still on an object; otherwise the tick is for a scrolled-past row and is dropped
// Pane loads use a background context — they are cheap, bounded
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

// highlightedBucketName returns the name of the bucket under the cursor, or "" when the
// cursor is on the synthetic "+ add bucket" row or the list is empty.
func (m App) highlightedBucketName() string {
	fb := m.filteredBuckets()
	if m.bucketSel < 0 || m.bucketSel >= len(fb) {
		return ""
	}
	return fb[m.bucketSel].Name
}

// afterBucketMove re-arms the debounced objects-zone load for the newly-highlighted bucket
// It bumps bucketLoadGen (superseding any pending tick) and schedules a
// fresh bucketTick. No load is issued in the Single tier (no objects zone there → the narrow
// flow is unchanged and issues zero object listings) or when the cursor is on
// the "+ add bucket" row.
func (m App) afterBucketMove() (tea.Model, tea.Cmd) {
	usageCmd := (&m).armUsageScan() // dwell-gated inline usage for the highlighted bucket
	if layoutTier(m.width) == tierSingle {
		return m, usageCmd
	}
	b := m.highlightedBucketName()
	if b == "" {
		return m, usageCmd
	}
	m.bucketLoadGen++
	return m, tea.Batch(bucketTickCmd(m.bucketLoadGen, b), usageCmd)
}

// onBucketTick fires the debounced objects-zone load once the bucket selection settles
// A tick for a scrolled-past bucket (gen/name mismatch) is dropped.
// On a cache hit the level is served with no backend call; on a miss the existing
// loadLevel/onLevel path fills m.level. An in-flight load for the same bucket is not duplicated
// The level is loaded into m.level (the shared level state, Design B) so crossing
// into it and the full-screen tree view share the same cache entry.
func (m App) onBucketTick(msg bucketTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.bucketLoadGen || m.highlightedBucketName() != msg.bucket {
		return m, nil // scrolled past — drop
	}
	if m.bucket == msg.bucket && m.prefix == "" && m.level != nil {
		return m, nil // already showing this bucket's root level
	}
	return m.loadObjectsLevel(msg.bucket, "", "")
}

// loadObjectsLevel points the objects zone at (bucket, prefix, search): a cache hit serves
// instantly with no backend call, a miss kicks off the shared loadLevel under a
// fresh gen. treeSel resets (a new level). It stays in modeBuckets (multi-pane). The result
// lands in m.level via the existing onLevel, shared with the full-screen tree view (Design B).
func (m App) loadObjectsLevel(bucket, prefix, search string) (tea.Model, tea.Cmd) {
	m.bucket = bucket
	m.prefix = prefix
	m.search = search
	m.treeSel = 0
	m.sel = nil // marks are per-level — cleared on bucket/level change
	key := m.levelKey()
	if cached, ok := m.cache.Get(key); ok {
		m.level = cached
		return m, nil
	}
	m.level = nil
	ctx := (&m).beginLoad()
	q := storage.LevelQuery{Bucket: bucket, Prefix: prefix, Search: search}
	return m, tea.Batch(loadLevel(ctx, m.activeStore(), key, q, m.gen), spinnerTick())
}

// crossIntoObjects moves focus from the bucket list into the objects zone.
// The objects level is already lazily previewed; crossing only sets the focus — it loads the
// level only if it does not yet match the highlighted bucket's root (e.g. the cross preceded
// the settle). No-op on the "+ add bucket" row.
func (m App) crossIntoObjects() (tea.Model, tea.Cmd) {
	b := m.highlightedBucketName()
	if b == "" {
		return m, nil
	}
	m.focusZone = zoneObjects
	// Already showing this bucket's level — preserve the objects-zone position (incl. a drilled
	// prefix) so Tab/→ back into objects resumes where the cursor was.
	if m.bucket == b && m.level != nil {
		return m, nil
	}
	return m.loadObjectsLevel(b, "", "")
}

// toggleFocus flips focus between the bucket list and the objects zone
// the symmetric Tab toggle. No objects zone exists in the Single tier, so it is inert there.
func (m App) toggleFocus() (tea.Model, tea.Cmd) {
	if layoutTier(m.width) == tierSingle {
		return m, nil
	}
	if m.focusZone == zoneObjects {
		m.focusZone = zoneBuckets
		return m, nil
	}
	return m.crossIntoObjects()
}

// onObjectsKey handles keys while focus is in the objects zone (modeBuckets,
// multi-pane). It reuses the level cursor/paging/drill primitives on m.level but stays in
// modeBuckets: a folder descends the objects zone, an object opens the full-screen view, and
// back ascends or returns focus to the bucket list per (search → ascend → return).
func (m App) onObjectsKey(key string) (tea.Model, tea.Cmd) {
	entries := m.treeEntries()
	n := len(entries)
	switch {
	case matches(key, m.keys.Up):
		if m.treeSel > 0 {
			m.treeSel--
			return m.afterSelectionMove()
		}
	case matches(key, m.keys.Down):
		if m.treeSel < n-1 {
			m.treeSel++
			return m.afterSelectionMove()
		} else if m.level != nil && !m.level.complete && !m.loading {
			return m.fetchNextPage()
		}
	case matches(key, m.keys.Top):
		if m.treeSel != 0 {
			m.treeSel = 0
			return m.afterSelectionMove()
		}
	case matches(key, m.keys.Bottom):
		if nb := max(0, n-1); m.treeSel != nb {
			m.treeSel = nb
			return m.afterSelectionMove()
		}
	case matches(key, m.keys.Search):
		return m.startSearch()
	case matches(key, m.keys.Mark):
		return m.toggleMark()
	case matches(key, m.keys.Sort):
		return m.cycleSort()
	case matches(key, m.keys.SortDir):
		return m.toggleSortDir()
	case matches(key, m.keys.Context):
		// A context switch is a global mode change — return focus to the bucket list so the
		// restore lands there, not in a now-stale objects zone.
		m.focusZone = zoneBuckets
		return m.openContexts()
	case matches(key, m.keys.Enter):
		e := m.selected()
		if e == nil {
			return m, nil
		}
		if e.isDir {
			return m.loadObjectsLevel(m.bucket, e.full, "") // descend, stay multi-pane
		}
		return m.openObject(e.obj) // → modeObject (full screen)
	case matches(key, m.keys.Back):
		return m.objectsBack()
	default:
		// Full per-item parity with the tree view: dangerous chords first
		// (ctrl+x delete / ctrl+o move), then the direct single-key actions — routed through the
		// SAME dispatch + object catalog the full-screen level uses.
		if mm, cmd, ok := m.dispatchChord(key); ok {
			return mm, cmd
		}
		if mm, cmd, ok := m.dispatchActionKey(key); ok {
			return mm, cmd
		}
	}
	return m, nil
}

// objectsBack implements the precedence for the objects zone: clear an active search
// else ascend one prefix, else (at the root) return focus to the bucket list.
func (m App) objectsBack() (tea.Model, tea.Cmd) {
	if m.search != "" {
		return m.loadObjectsLevel(m.bucket, m.prefix, "")
	}
	if m.prefix == "" {
		m.focusZone = zoneBuckets
		return m, nil
	}
	return m.loadObjectsLevel(m.bucket, parentPrefix(m.prefix), "")
}

// objectsView renders the objects zone: the highlighted bucket's first level
// reusing the tree table render of m.level, with explicit loading/empty/error states.
func (m App) objectsView(w, rows int) string {
	if m.highlightedBucketName() == "" {
		return dimCellStyle.Render("(no bucket selected)")
	}
	// A denied/failed objects listing surfaces an in-pane error without disturbing the
	// bucket list. The error is not cached, so a revisit re-attempts.
	if m.err != nil && m.level == nil {
		return errStyle.Render("error: " + truncate(m.errorText(), max(1, w-7)))
	}
	if m.level == nil {
		return dimCellStyle.Render("loading…")
	}
	if m.level.count() == 0 {
		return emptyStyle.Render("(empty)")
	}
	return m.treeView(w, rows)
}

// levelKey is the cache coordinate for the current tree node.
func (m *App) levelKey() cache.Key {
	return cache.Key{Context: m.ctxName, Bucket: m.bucket, Prefix: m.prefix, Search: m.search}
}

// levelKeyFor is the cache coordinate for an arbitrary (bucket, prefix) in the active
// context, with an empty search — used to invalidate a level other than the current view
// after a cross-prefix mutation.
func (m App) levelKeyFor(bucket, prefix string) cache.Key {
	return cache.Key{Context: m.ctxName, Bucket: bucket, Prefix: prefix, Search: ""}
}

// invalidateLevel drops a single cached level so the next navigation to it re-fetches
// Precise — never a whole-cache clear.
func (m App) invalidateLevel(key cache.Key) { m.cache.Invalidate(key) }

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
			// brief threshold — fast ops finish first and never flash.
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
		// Scoped: the "+ add bucket" row shows when this connection has pinned
		// buckets OR list-all returned nothing — never on a working list-all with results.
		m.bucketsScoped = len(m.info.PinnedBuckets) > 0 || len(msg.buckets) == 0
		// Arm the objects-zone preview for the first bucket; no-op on a narrow tier.
		return m.afterBucketMove()

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
		m.rawPreview = false //: the pretty/raw toggle is per-payload
		m.mode = modeObject
		return m, nil

	case errMsg:
		if msg.gen != m.gen {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		// A failed/denied list-all on the bucket list is the scoped-credentials case: offer
		// the "+ add bucket" row so the user can reach a known bucket directly.
		// Gate on m.bucket=="" so an objects-zone listing error (which has a bucket
		// highlighted) is NOT mistaken for a list-all failure and does not flip bucketsScoped.
		if m.mode == modeBuckets && m.bucket == "" {
			m.bucketsScoped = true
		}
		return m, nil

	case operationProgressMsg:
		return m.onOperationProgress(msg)

	case operationDoneMsg:
		return m.onOperationDone(msg)

	case usageProgressMsg:
		return m.onUsageProgress(msg)

	case usageDoneMsg:
		return m.onUsageDone(msg)

	case usageTickMsg:
		return m.onUsageTick(msg)

	case presignDoneMsg:
		return m.onPresignDone(msg)

	case incompleteUploadsMsg:
		return m.onIncompleteUploads(msg)

	case exportDoneMsg:
		return m.onExportDone(msg)

	case objectTagsMsg:
		return m.onObjectTags(msg)

	case bucketConfigMsg:
		return m.onBucketConfig(msg)

	case paneTickMsg:
		return m.onPaneTick(msg)

	case bucketTickMsg:
		return m.onBucketTick(msg)

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

	case addBucketMsg:
		return m.onAddBucket(msg)

	case contextResolvedMsg:
		return m.onContextResolved(msg)

	case searchFireMsg:
		return m.onSearchFire(msg)

	case tea.PasteMsg:
		return m.onPaste(msg.Content)

	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// sanitizePaste collapses a clipboard payload to a single line: trailing
// newlines are dropped (clipboards often append one) and any interior newline becomes a
// space, so a paste never breaks a single-line field or submits a form prematurely.
func sanitizePaste(s string) string {
	s = strings.TrimRight(s, "\r\n")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// onPaste routes a bracketed-paste payload to the active text surface
// the search/command input, the add-connection form's focused field, the typed-confirm input
// or an operation's destination-key / new-folder name entry. No-op when no text surface is
// focused. The caret-aware surfaces (connForm, typed-confirm) insert at the caret; the plain
// append-style surfaces (search/command/dest-key/name) append, matching their keystroke path.
func (m App) onPaste(content string) (tea.Model, tea.Cmd) {
	text := sanitizePaste(content)
	if text == "" {
		return m, nil
	}
	switch {
	case m.searching:
		m.searchInput += text
		return m.afterFilterEdit()
	case m.mode == modeCommand:
		m.cmdInput += text
		return m, nil
	case m.mode == modeConnForm && m.form != nil:
		m.formAppend(text)
		return m, nil
	case m.mode == modeAddBucket && m.addForm != nil:
		m.addForm.name.Insert(text)
		return m, nil
	case m.op != nil && m.op.phase == phaseConfirm && m.op.tier == confirmTyped:
		m.op.input.Insert(text)
		return m, nil
	case m.op != nil && m.op.phase == phaseDest:
		m.op.dstKey += text
		return m, nil
	case m.op != nil && m.op.phase == phaseName:
		m.op.name += text
		return m, nil
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
	// in modeConnections) never traps quit/cancel behind a mode handler.
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

	// The command bar / connection form own all keys while open (text input) — modal
	// before the global bindings so typing "q"/":" is not intercepted.
	if m.mode == modeCommand {
		return m.onCommandKey(key, msg)
	}
	if m.mode == modeConnForm {
		return m.onConnFormKey(key, msg)
	}
	if m.mode == modeAddBucket {
		return m.onAddBucketKey(key, msg)
	}
	if m.mode == modeConnections {
		// A connection-delete confirmation is an active op: it owns keys (typed
		// name entry) before the list navigation, so typing the name isn't intercepted.
		if m.op != nil {
			return m.onOpKey(key, msg)
		}
		return m.onConnectionsKey(key)
	}

	// A pending write-arm confirmation is modal: y/N resolves it.
	if m.armConfirm {
		return m.onArmConfirmKey(key)
	}

	// An interactive operation (name/dest entry, confirmation, file browser) owns
	// keys before the global bindings, so text input (incl. "q", "h", "+") is not
	// intercepted.
	if m.op != nil {
		return m.onOpKey(key, msg)
	}

	// The reveal popup is a transient read-only overlay: any key closes it.
	if m.reveal != nil {
		m.reveal = nil
		return m, nil
	}

	// The field-select copy overlay owns navigation/Enter/Esc while open.
	if m.fieldCopy != nil {
		return m.onFieldCopyKey(key)
	}

	// The copy/share menu owns input while open.
	if m.copyMenu != nil {
		return m.onCopyMenuKey(key)
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

	// `:` opens the command bar. Search/op/form already own keys above, so it
	// never interferes with in-progress text entry.
	if matches(key, m.keys.Command) && m.canOpenCommand() {
		return m.startCommand()
	}

	// Write-arm toggle: a global safety primitive. Arming prompts to
	// confirm; disarming is instant. Refused on a readonly:true context.
	if matches(key, m.keys.WriteToggle) {
		return m.toggleWrite()
	}

	// Reveal/inspect the full identifier of the current selection — read-only
	// globally available in the browse modes.
	if matches(key, m.keys.Reveal) {
		return m.openReveal()
	}

	// The copy/share menu — globally available in the browse modes + the card.
	if matches(key, m.keys.CopyMenu) && (m.mode == modeBuckets || m.mode == modeTree || m.mode == modeObject || m.mode == modeHealth) {
		return m.openCopyMenu()
	}

	// The operator health card — from the browse modes only.
	if matches(key, m.keys.Health) && (m.mode == modeBuckets || m.mode == modeTree) {
		return m.openHealth()
	}

	// Cancel an in-flight load via the back/escape key (no modal overlay open here).
	if matches(key, m.keys.Back) && m.loading {
		(&m).cancelLoad()
		return m, nil
	}

	switch m.mode {
	case modeBuckets:
		// Multi-pane focus: Tab toggles focus; otherwise the focused zone owns input.
		if matches(key, m.keys.Tab) {
			return m.toggleFocus()
		}
		if m.focusZone == zoneObjects {
			return m.onObjectsKey(key)
		}
		return m.onBucketsKey(key)
	case modeTree:
		return m.onTreeKey(key, msg)
	case modeObject:
		return m.onObjectKey(key)
	case modeContextSwitch:
		return m.onContextKey(key)
	case modeHealth:
		return m.onHealthKey(key)
	}
	return m, nil
}

// onBucketsKey handles the bucket-list view.
func (m App) onBucketsKey(key string) (tea.Model, tea.Cmd) {
	n := len(m.filteredBuckets())
	if m.bucketAddRowVisible() {
		n++ // the "+ add bucket" row is selectable past the last real bucket
	}
	switch {
	case matches(key, m.keys.Up):
		if m.bucketSel > 0 {
			m.bucketSel--
			return m.afterBucketMove() // re-arm the debounced objects-zone preview
		}
	case matches(key, m.keys.Down):
		if m.bucketSel < n-1 {
			m.bucketSel++
			return m.afterBucketMove()
		}
	case matches(key, m.keys.Top):
		if m.bucketSel != 0 {
			m.bucketSel = 0
			return m.afterBucketMove()
		}
	case matches(key, m.keys.Bottom):
		if nb := max(0, n-1); m.bucketSel != nb {
			m.bucketSel = nb
			return m.afterBucketMove()
		}
	case matches(key, m.keys.Search):
		return m.startSearch() // "/" filters bucket names (FR: bucket filter)
	case matches(key, m.keys.Context):
		return m.openContexts()
	case len(key) == 1 && key[0] >= '1' && key[0] <= '9':
		// Quick context switch by number (k9s-style).
		if idx := int(key[0] - '1'); idx < len(m.contexts) {
			return m.applyContext(m.contexts[idx])
		}
	case matches(key, m.keys.Enter):
		fb := m.filteredBuckets()
		// The "+ add bucket" row (one past the last bucket) opens the name input.
		if m.bucketAddRowVisible() && m.bucketSel == len(fb) {
			return m.openAddBucket()
		}
		if m.bucketSel < 0 || m.bucketSel >= len(fb) {
			return m, nil
		}
		// Multi-pane (Dual/Full): Enter/→/l crosses focus into the objects zone — the level is
		// already previewed. Single tier: drill full-screen as today.
		if layoutTier(m.width) != tierSingle {
			return m.crossIntoObjects()
		}
		m.bucket = fb[m.bucketSel].Name
		m.prefix = ""
		m.search = ""
		return m.enterLevel()
	default:
		// Dangerous-action chord: ctrl+x → delete the selected bucket.
		if mm, cmd, ok := m.dispatchChord(key); ok {
			return mm, cmd
		}
		// Direct single-key actions: analyze / refresh on the bucket list.
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

// openContexts is the `c` entry point: it opens the in-app connection manager
// (switch / add via the "+ add connection" row / delete) when a config writer is wired, else
// the read-only context switcher. This replaces the removed standalone `n` add-connection key.
func (m App) openContexts() (tea.Model, tea.Cmd) {
	if m.connect != nil {
		return m.openConnections()
	}
	return m.openContextSwitch()
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
// (a keychain unlock or an external credential command), so it runs OFF the
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
	// forces read-only via writable.
	m.ctxReadOnly = be.ReadOnly
	m.op = nil
	m.ctxName = msg.target
	m.usageBudget = be.UsageScanBudget //: the budget rides the resolved backend
	m.cache.Clear()
	m.usageResults.Clear() //: usage totals are per-context — never bleed across a switch
	m.mpuResults.Clear()   //: the MPU probe cache is per-context too
	m.healthKiB = healthKiBOrDefault(be.HealthSmallObjectKiB)
	m.healthShare = healthShareOrDefault(be.HealthSmallObjectShare)
	m.detailSection = sectNone
	m.objectTags, m.bucketCfg, m.tagsErr = nil, nil, nil
	m.buckets = nil
	m.bucketSel = 0
	m.bucketFilter = ""
	m.bucket, m.prefix, m.search = "", "", ""
	m.level = nil
	m.sel = nil
	m.mode = modeBuckets
	ctx := (&m).beginLoad()
	return m, tea.Batch(loadBuckets(ctx, m.activeStore(), m.gen, m.info.PinnedBuckets), spinnerTick())
}

// clampSelection keeps selection indices within bounds after resize/data change.
// The visible window is recomputed statelessly at render time (windowBounds), so
// only the selection indices need clamping here (Edge Case: resize reflow).
func (m *App) clampSelection() {
	limit := len(m.buckets) - 1
	if m.bucketAddRowVisible() {
		limit = len(m.buckets) // the "+ add bucket" row sits one past the last real bucket
	}
	if m.bucketSel > limit {
		m.bucketSel = max(0, limit)
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

// errorText returns a human, secret-free message for the current error.
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
	// badge is injected here too — it must be present on EVERY screen
	// write-toggle-contract C3).
	badge := writeBadge(m.writable()) + "\n"

	if m.mode == modeHelp {
		v := tea.NewView(badge + m.helpView())
		v.AltScreen = true
		return v
	}

	footer := m.footerBlock(w)
	footerH := strings.Count(footer, "\n") + 1

	// The command bar overlays the footer; its body is the underlying (prev) view.
	bodyMode := m.mode
	if m.mode == modeCommand {
		bodyMode = m.prevMode
	}

	// The always-visible filter forms reserve a 3-line band (a bordered input box) in the
	// filterable browse modes (modeBuckets/modeTree); listWithPane stacks the form(s) under the
	// list box(es), so the band is subtracted from the LIST, never the footer.
	// -contract). The upload file browser takes over the body — no form there.
	filterFieldH := 0
	if (m.op == nil || m.op.phase != phaseBrowse) && (bodyMode == modeBuckets || bodyMode == modeTree) {
		filterFieldH = 3
	}

	// Inner box height = total minus the footer, the reserved filter-form band, and the two
	// border lines. The box body MUST NOT exceed this, or the footer (incl. the hints line)
	// scrolls off.
	rows := m.height - footerH - filterFieldH - 2
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
		body = boxViewChip(m.resourceTitle(), m.objectKind(), "", m.modeChip(), m.objectView(w-2, rows), w, rows)
	case modeConnections:
		body = boxView("connections", "", m.connectionsView(w-2, dataRows), w, rows)
	case modeConnForm:
		body = boxView("add connection", "", m.connFormView(w-2), w, rows)
	case modeAddBucket:
		body = boxView("add bucket", "", m.addBucketView(w-2), w, rows)
	case modeHealth:
		body = boxView("health", "", m.healthView(w-2, dataRows), w, rows)
	}

	// Binary-tier confirmation: a centered popup over the body (the typed tier
	// uses the prominent inline form in the footer instead). Replaces the body so the
	// dialog is centered and never clipped.
	if m.op != nil && m.op.phase == phaseConfirm && m.op.tier != confirmTyped {
		bodyH := strings.Count(body, "\n") + 1
		body = m.confirmPopupView(w, bodyH)
	}

	// The write-arm confirmation overlays the body as a prominent centered popup.
	if m.armConfirm {
		bodyH := strings.Count(body, "\n") + 1
		body = m.armConfirmPopupView(w, bodyH)
	}

	// The reveal/inspect popup overlays the body, centered.
	if m.reveal != nil {
		bodyH := strings.Count(body, "\n") + 1
		body = m.revealView(w, bodyH)
	}

	// The field-select copy overlay overlays the body, centered.
	if m.fieldCopy != nil {
		bodyH := strings.Count(body, "\n") + 1
		body = m.fieldCopyView(w, bodyH)
	}

	// The copy/share menu overlays the body, centered.
	if m.copyMenu != nil {
		bodyH := strings.Count(body, "\n") + 1
		body = m.copyMenuView(w, bodyH)
	}

	// The filter forms live inside body (listWithPane stacks them under the list boxes), so the
	// composed view is simply body over the footer; the reserved band keeps the footer in place.
	v := tea.NewView(body + "\n" + footer)
	v.AltScreen = true
	return v
}

// paneSplitMin is the minimum terminal width at which the details pane is shown beside
// the list; below it the list spans full width and the pane collapses.
const paneSplitMin = 100

// fullTierMin is the width at which the third (details) zone joins the split — the Full
// tier of the three-zone master-detail browse.
const fullTierMin = 130

// tier classifies the browse layout by terminal width into the three normative tiers
// : Full (≥130) renders buckets│objects│details; Dual (100–129) renders
// buckets│objects with details collapsed; Single (<100) is the current single-column
// mode-stack, unchanged.
type tier int

const (
	tierSingle tier = iota // w < paneSplitMin (≤ 99)
	tierDual               // [paneSplitMin, fullTierMin) = [100, 129]
	tierFull               // w ≥ fullTierMin (≥ 130)
)

// layoutTier classifies terminal width w into the browse layout tier. The
// boundaries are inclusive as documented: 130 ⇒ Full, 129/100 ⇒ Dual, 99 ⇒ Single.
func layoutTier(w int) tier {
	switch {
	case w >= fullTierMin:
		return tierFull
	case w >= paneSplitMin:
		return tierDual
	default:
		return tierSingle
	}
}

// listWithPane composes the browse body: on a wide terminal it joins the list (bounded
// width) and the persistent details pane side by side; on a narrow terminal it returns
// the full-width list and the pane collapses. renderList draws the list
// table at the width it is given.
func (m App) listWithPane(title, sel string, w, rows, dataRows int, renderList func(lw, dr int) string) string {
	// Single tier (or a user-collapsed pane) renders the full-width list; the Dual/Full
	// tiers add the side pane. differentiate the Dual objects zone
	// from the Full details zone; today both render the existing details pane.
	// One-column layout (Single tier or a collapsed pane): the list box with the focused scope's
	// filter form stacked beneath it (015 — the form replaces the old border filter chip).
	if layoutTier(w) == tierSingle || !m.paneVisible {
		box := boxViewChip(title, sel, "", m.modeChip(), renderList(w-2, dataRows), w, rows)
		return lipgloss.JoinVertical(lipgloss.Left, box, m.scopeFilterField(w))
	}
	paneW := w / 3
	if paneW > 40 {
		paneW = 40
	}
	if paneW < 24 {
		paneW = 24
	}
	// Miller-columns proportion for the bucket list: the buckets column is the
	// NARROW one and the OBJECTS zone (the highlighted bucket's first level) takes the rest
	// so object names have room. The tree view keeps the wide list + thin details pane.
	// Each list box carries its own always-visible filter form below it: the bucket and
	// object forms sit side by side under their panes — both visible at once.
	if m.mode == modeBuckets {
		bucketW := m.bucketsColWidth(w, paneW)
		bucketsBox := boxViewFocusChip(title, sel, "", m.modeChip(), renderList(bucketW-2, dataRows), bucketW, rows, m.focusZone == zoneBuckets)
		bucketCol := lipgloss.JoinVertical(lipgloss.Left, bucketsBox, m.bucketFilterField(bucketW))
		// Full tier (≥130): a third, non-focusable details zone (006 pane, preserved per )
		// joins buckets│objects; it adapts to focus (bucket meta vs object meta+preview). Dual
		// the details zone collapses and the objects zone takes the rest.
		if layoutTier(w) == tierFull && m.paneVisible {
			detW := paneW
			objW := w - bucketW - detW
			objBox := boxViewFocusChip(m.objectsZoneTitle(objW), "", "", "", m.objectsView(objW-2, dataRows), objW, rows, m.focusZone == zoneObjects)
			objCol := lipgloss.JoinVertical(lipgloss.Left, objBox, m.objectFilterField(objW))
			detBox := boxView("details", "", m.browseDetailsView(detW-2, rows-2), detW, rows)
			// Top-aligned: the details box (no form) pads with blanks beside the forms.
			return lipgloss.JoinHorizontal(lipgloss.Top, bucketCol, objCol, detBox)
		}
		objW := w - bucketW
		objBox := boxViewFocusChip(m.objectsZoneTitle(objW), "", "", "", m.objectsView(objW-2, dataRows), objW, rows, m.focusZone == zoneObjects)
		objCol := lipgloss.JoinVertical(lipgloss.Left, objBox, m.objectFilterField(objW))
		return lipgloss.JoinHorizontal(lipgloss.Top, bucketCol, objCol)
	}
	listW := w - paneW
	listBox := boxViewChip(title, sel, "", m.modeChip(), renderList(listW-2, dataRows), listW, rows)
	listCol := lipgloss.JoinVertical(lipgloss.Left, listBox, m.objectFilterField(listW))
	paneBody := boxView("details", "", m.paneView(paneW-2, rows-2), paneW, rows)
	return lipgloss.JoinHorizontal(lipgloss.Top, listCol, paneBody)
}

// modeChip is the border-mounted read/write mode label: a loud "WRITE" accent
// when write is armed, a calm neutral "RO" otherwise. Inset into the primary box's top border
// by the boxViewChip family. NO_COLOR-safe (the text itself carries the state).
func (m App) modeChip() string {
	if m.writable() {
		return writeBadgeStyle.Render("WRITE")
	}
	return roStyle.Render("RO")
}

// bucketFilterField builds the always-visible, prominent filter FORM for the bucket list
// a bordered, labeled input under the bucket box. Active (accent) while the bucket pane holds
// focus; editing (live input + caret) while its scope is being typed. The committed term is the
// local name filter; the match count rides the box title above ("buckets[M/T]").
func (m App) bucketFilterField(w int) string {
	return filterFieldView("buckets", m.bucketFilter, m.searchInput, w,
		m.focusZone == zoneBuckets, m.searching && m.filterIsBucketList())
}

// objectFilterField builds the always-visible, prominent filter FORM for an object level
// a bordered, labeled input under the objects box. Active in the full-screen tree or while the
// objects pane holds focus; editing while its scope is typed. The match count rides the box
// title above ("objects[N]"); the level total is not fetched (paginated).
func (m App) objectFilterField(w int) string {
	return filterFieldView("objects", m.search, m.searchInput, w,
		m.mode == modeTree || m.focusZone == zoneObjects, m.searching && !m.filterIsBucketList())
}

// scopeFilterField is the single filter form for the one-column layouts (Single tier or a
// collapsed pane): the focused scope's field.
func (m App) scopeFilterField(w int) string {
	if m.filterIsBucketList() {
		return m.bucketFilterField(w)
	}
	return m.objectFilterField(w)
}

// bucketsColWidth grows the bucket column to fit the longest bucket name when the objects
// zone has slack, bounded by a per-tier max so the objects zone stays legible.
// Never shrinks below the base width; never starves the objects zone below objMinW.
func (m App) bucketsColWidth(w, base int) int {
	const objMinW = 30
	maxW := w - objMinW
	if layoutTier(w) == tierFull && m.paneVisible {
		maxW -= base // reserve the details zone (detW == base)
	}
	bw := base
	if want := m.bucketsZoneWantWidth(); want > bw {
		bw = want
	}
	if bw > maxW {
		bw = maxW
	}
	if bw < base {
		bw = base
	}
	return bw
}

// bucketsZoneWantWidth is the box width that fits the longest visible bucket name plus the
// created column and table chrome — "▶ " gutter (2) + name + sep (1) + created (19) + border (2).
func (m App) bucketsZoneWantWidth() int {
	maxName := len("name")
	for _, b := range m.filteredBuckets() {
		if n := lipgloss.Width(sanitizeLabel(b.Name)); n > maxName {
			maxName = n
		}
	}
	return maxName + 2 + 1 + 19 + 2
}

// objectsZoneTitle is the left label on the objects-zone box border: the
// highlighted bucket's name plus its loaded entry count.
// objectsZoneTitle is the objects-zone box label: the full location breadcrumb
// (context → bucket → prefix chain) middle-elided to fit width w, with the loaded entry count
// and an active-search marker appended after the elision. The full path
// is revealable via the reveal popup ('i').
func (m App) objectsZoneTitle(w int) string {
	b := m.highlightedBucketName()
	if b == "" {
		return "objects"
	}
	path := sanitizeLabel(m.ctxName) + " → " + sanitizeLabel(b)
	if m.bucket == b && m.prefix != "" {
		path += "/" + strings.TrimSuffix(sanitizeLabel(m.prefix), "/")
	}
	suffix := ""
	if m.level != nil && m.bucket == b {
		suffix = fmt.Sprintf("[%d]", m.level.count())
	}
	// The active filter term is shown on the objects box's border chip, not appended
	// to the breadcrumb title.
	return elideMiddle(path, max(8, w-6-lipgloss.Width(suffix))) + suffix
}

// footerBlock is the Claude Code-style status footer: a thin separator, an info
// line (context · cluster · user · …) with one hue per parameter, a keybinding
// line, and a transient status line. Each line is fit to the width segment-by-
// segment so nothing wraps (which would otherwise hide a line).
func (m App) footerBlock(w int) string {
	// ≤ 3 rows: compact identity, one contextual hint row, optional status.
	// In the list modes the always-visible hint bar advertises the valid direct actions
	// for the current selection/capability; other modes keep the
	// legacy contextual hints.
	// List modes (buckets/tree) render the three-block command bar, which
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
		lines = append(lines, footerIdentityCompact(w, m.ctxName, m.info.Cluster), hintLine)
	}
	if s := m.statusLine(w); s != "" {
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}

// searchActive reports whether a search/filter is applied or being typed (drives the
// footer's esc-clear vs esc-back disambiguation).
func (m App) searchActive() bool {
	if m.filterIsBucketList() {
		return m.bucketFilter != "" || m.searching
	}
	return m.search != "" || m.searching
}

// statusLine is the transient bottom line: filter/search input, loading, or
// error. Plain text is truncated to width so it never wraps (which would push the
// box up and clip a footer line).
func (m App) statusLine(w int) string {
	// The command bar owns the status line while open.
	if m.mode == modeCommand {
		return m.commandLine(w)
	}
	// A pending write-arm confirmation is shown as a prominent centered popup over the body
	//, not in the status line — so it falls through here.
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
	// The filter input moved to the always-visible strip; statusLine now carries
	// only loading / notice / error / op-prompt, which coexist with an active filter.
	if m.loading {
		// Name what is loading; cancel is the back/escape key now.
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
		// Success notices are green (noticeStyle), visually distinct from red errors.
		return noticeStyle.Render(truncate(m.notice, max(1, w-1)))
	}
	if txt := m.errorText(); txt != "" {
		return errStyle.Render("error: " + truncate(txt, max(1, w-7)))
	}
	// Multi-select summary: count + combined size of the marked objects.
	if m.selCount() > 0 {
		return noticeStyle.Render(fmt.Sprintf("%d selected · %s  (%s/%s/%s: bulk download/delete/copy)",
			m.selCount(), humanSize(m.selSize()),
			glyph(firstBind(m.keys.Download)), glyph(firstBind(m.keys.Delete)), glyph(firstBind(m.keys.Copy))))
	}
	return ""
}

// resourceTitle is the left label on the box top border, e.g. "buckets[12]".
func (m App) resourceTitle() string {
	switch m.mode {
	case modeTree:
		// Full-screen tier breadcrumb: context → bucket → prefix chain.
		loc := sanitizeLabel(m.ctxName) + " → " + sanitizeLabel(m.bucket)
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
		// The active filter term is shown on the level box's border chip, not appended
		// to the breadcrumb title.
		return fmt.Sprintf("%s[%d%s]", loc, n, more) // sort now advertised in the command bar
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

// selectionName is the centered, highlighted label on the box top border
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
		loc := sanitizeLabel(m.ctxName) + " → " + sanitizeLabel(m.bucket)
		if m.prefix != "" {
			loc += "/" + sanitizeLabel(m.prefix)
		}
		if m.search != "" {
			loc += fmt.Sprintf("  (search: %s)", sanitizeLabel(m.search))
		}
		return loc
	}
}

// bucketAddRowVisible reports whether the synthetic "+ add bucket" row is shown: only for a
// scoped bucket list (pins present, or list-all failed/empty) and only when a Connector is
// wired to persist the addition.
func (m App) bucketAddRowVisible() bool { return m.bucketsScoped && m.connect != nil }

// bucketsView renders the bucket list table body (filtered) at the given width. For a scoped
// list it appends a "+ add bucket" row (mirrors the "+ add connection" row) so the user can
// reach a bucket by name.
func (m App) bucketsView(w, rows int) string {
	fb := m.filteredBuckets()
	addRow := m.bucketAddRowVisible()
	if m.loading && len(fb) == 0 {
		return dimCellStyle.Render("Loading buckets…")
	}
	if len(fb) == 0 && !addRow {
		if m.bucketFilter != "" {
			return emptyStyle.Render(fmt.Sprintf("No buckets match %q.", m.bucketFilter))
		}
		return emptyStyle.Render("No buckets visible for this context.")
	}
	n := len(fb)
	if addRow {
		n++
	}
	off, end := windowBounds(n, m.bucketSel, rows)
	cols := []column{{"name", 0}, {"created", 19}}
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		if addRow && i == len(fb) {
			data = append(data, []string{"+ add bucket", ""})
		} else {
			data = append(data, []string{fb[i].Name, formatDate(fb[i].CreationDate)})
		}
	}
	return renderTableActive(w, cols, data, nil, m.bucketSel-off, rows-(end-off))
}

// ctxTable renders a context-style two-column table (name + active status) windowed to
// rows. Shared by the context switcher and the connection manager list.
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
