package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// treeEntry is one row in the tree view (a directory or an object).
type treeEntry struct {
	label string // display basename (sanitized)
	isDir bool
	full  string // full prefix (dir) or key (object)
	obj   *storage.ObjectRef
}

// treeEntries returns the combined dirs-then-objects list for the current level.
func (m App) treeEntries() []treeEntry {
	if m.level == nil {
		return nil
	}
	out := make([]treeEntry, 0, m.level.count())
	for _, d := range m.level.dirs {
		out = append(out, treeEntry{label: sanitizeLabel(strings.TrimPrefix(d, m.prefix)), isDir: true, full: d})
	}
	for i := range m.level.objects {
		o := &m.level.objects[i]
		out = append(out, treeEntry{label: sanitizeLabel(strings.TrimPrefix(o.Key, m.prefix)), isDir: false, full: o.Key, obj: o})
	}
	return out
}

// onTreeKey handles the tree-navigation view.
func (m App) onTreeKey(key string, _ tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	entries := m.treeEntries()
	n := len(entries)

	switch {
	case matches(key, m.keys.Up):
		if m.treeSel > 0 {
			m.treeSel--
		}
	case matches(key, m.keys.Down):
		if m.treeSel < n-1 {
			m.treeSel++
		} else if m.level != nil && !m.level.complete && !m.loading {
			return m.fetchNextPage() // paging-on-scroll: exactly one load
		}
	case matches(key, m.keys.Top):
		m.treeSel = 0
	case matches(key, m.keys.Bottom):
		m.treeSel = max(0, n-1)
	case matches(key, m.keys.Search):
		return m.startSearch()
	case matches(key, m.keys.Refresh):
		return m.refresh()
	case matches(key, m.keys.NewFolder):
		return m.startCreateFolder()
	case matches(key, m.keys.Delete):
		return m.startRemoveObject()
	case matches(key, m.keys.Upload):
		return m.startUpload()
	case matches(key, m.keys.Copy):
		return m.startCopy()
	case matches(key, m.keys.Move):
		return m.startMove()
	case matches(key, m.keys.DeleteAll):
		return m.startRecursiveDelete()
	case matches(key, m.keys.Context):
		return m.openContextSwitch()
	case matches(key, m.keys.Enter):
		e := m.selected()
		if e == nil {
			return m, nil
		}
		if e.isDir {
			m.prefix = e.full
			m.search = ""
			return m.enterLevel()
		}
		return m.openObject(e.obj)
	case matches(key, m.keys.Back):
		return m.goBack()
	}
	return m, nil
}

// selected returns the currently highlighted entry, or nil.
func (m App) selected() *treeEntry {
	entries := m.treeEntries()
	if m.treeSel < 0 || m.treeSel >= len(entries) {
		return nil
	}
	return &entries[m.treeSel]
}

// enterLevel switches to the tree view for the current (bucket, prefix, search),
// serving from cache on a hit (FR-011) or fetching the first page on a miss.
func (m App) enterLevel() (tea.Model, tea.Cmd) {
	m.mode = modeTree
	m.treeSel = 0
	key := m.levelKey()
	if cached, ok := m.cache.Get(key); ok {
		m.level = cached
		m.loading = false
		return m, nil
	}
	m.level = nil
	ctx := (&m).beginLoad()
	q := storage.LevelQuery{Bucket: m.bucket, Prefix: m.prefix, Search: m.search}
	return m, tea.Batch(loadLevel(ctx, m.store, key, q, m.gen), spinnerTick())
}

// fetchNextPage requests the next page of the current level (FR-010).
func (m App) fetchNextPage() (tea.Model, tea.Cmd) {
	key := m.levelKey()
	token := m.level.nextToken
	ctx := (&m).beginLoad()
	q := storage.LevelQuery{Bucket: m.bucket, Prefix: m.prefix, Search: m.search, Token: token}
	return m, tea.Batch(loadLevel(ctx, m.store, key, q, m.gen), spinnerTick())
}

// refresh discards the cached level and re-fetches from the first page (FR-011a).
func (m App) refresh() (tea.Model, tea.Cmd) {
	key := m.levelKey()
	m.cache.Invalidate(key)
	m.level = nil
	m.treeSel = 0
	ctx := (&m).beginLoad()
	q := storage.LevelQuery{Bucket: m.bucket, Prefix: m.prefix, Search: m.search}
	return m, tea.Batch(loadLevel(ctx, m.store, key, q, m.gen), spinnerTick())
}

// goBack clears an active search, ascends one prefix, or returns to the bucket list.
func (m App) goBack() (tea.Model, tea.Cmd) {
	if m.search != "" {
		m.search = ""
		return m.enterLevel()
	}
	if m.prefix == "" {
		m.mode = modeBuckets
		m.level = nil
		return m, nil
	}
	m.prefix = parentPrefix(m.prefix)
	return m.enterLevel()
}

// openObject opens the combined object view: it fetches metadata (HeadObject)
// and bounded content (ranged GET) concurrently under one generation, so the
// metadata pane and the content/image pane fill in together (no separate steps).
func (m App) openObject(o *storage.ObjectRef) (tea.Model, tea.Cmd) {
	if o == nil {
		return m, nil
	}
	m.meta = nil
	m.prev = nil
	m.prevOff = 0
	m.mode = modeObject
	ctx := (&m).beginLoad()
	return m, tea.Batch(
		loadMetadata(ctx, m.store, m.bucket, o.Key, m.gen),
		loadPreview(ctx, m.store, m.bucket, o.Key, "", o.Size, m.gen),
		spinnerTick(),
	)
}

// onLevel merges a freshly-loaded page into the current level and caches it.
func (m App) onLevel(msg levelMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil // stale generation — drop
	}
	m.loading = false
	if m.level == nil {
		m.level = &levelState{}
	}
	m.level.dirs = append(m.level.dirs, msg.page.Dirs...)
	m.level.objects = append(m.level.objects, msg.page.Objects...)
	m.level.nextToken = msg.page.NextToken
	m.level.complete = msg.page.NextToken == nil
	m.cache.Put(msg.key, m.level)
	return m, nil
}

// treeView renders the windowed tree level table body at the given width.
func (m App) treeView(w, rows int) string {
	if m.level == nil {
		if m.loading {
			return dimCellStyle.Render("Loading…")
		}
		return ""
	}
	entries := m.treeEntries()
	if len(entries) == 0 {
		if m.search != "" {
			return emptyStyle.Render(fmt.Sprintf("No matches for %q.", m.search))
		}
		return emptyStyle.Render("Empty — no folders or objects here.")
	}

	off, end := windowBounds(len(entries), m.treeSel, rows)
	cols := []column{{"name", 0}, {"type", 5}, {"size", 11}, {"modified", 17}}
	data := make([][]string, 0, end-off)
	dirs := make([]bool, 0, end-off)
	for i := off; i < end; i++ {
		e := entries[i]
		if e.isDir {
			data = append(data, []string{e.label, "dir", "—", "—"})
		} else {
			data = append(data, []string{e.label, "obj", humanSize(e.obj.Size), formatDate(e.obj.LastModified)})
		}
		dirs = append(dirs, e.isDir)
	}
	return renderTable(w, cols, data, dirs, m.treeSel-off)
}

// parentPrefix returns the prefix one level up ("a/b/" -> "a/", "a/" -> "").
func parentPrefix(prefix string) string {
	p := strings.TrimSuffix(prefix, "/")
	idx := strings.LastIndexByte(p, '/')
	if idx < 0 {
		return ""
	}
	return p[:idx+1]
}

// sanitizeLabel makes arbitrary key bytes safe to print (Edge Case: non-UTF-8 /
// control characters in key names).
func sanitizeLabel(s string) string {
	s = strings.ToValidUTF8(s, "�")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return '�'
		default:
			return r
		}
	}, s)
}

// humanSize renders a byte count compactly.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
