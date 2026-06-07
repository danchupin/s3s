package ui

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Sortable lists. The sort is applied at RENDER time to a copy of the level,
// keeping selection indices the only mutable list state (the architecture's stateless
// window model). It persists across navigation for the session. Folders have
// no size/date, so when sorting by those columns they group consistently above objects
type sortCol int

const (
	sortName sortCol = iota
	sortSize
	sortModified
)

func (c sortCol) String() string {
	switch c {
	case sortSize:
		return "size"
	case sortModified:
		return "modified"
	default:
		return "name"
	}
}

// cycleSort advances the sort column (name → size → modified → name).
func (m App) cycleSort() (tea.Model, tea.Cmd) {
	m.sortBy = (m.sortBy + 1) % 3
	return m, nil
}

// toggleSortDir flips ascending/descending.
func (m App) toggleSortDir() (tea.Model, tea.Cmd) {
	m.sortAsc = !m.sortAsc
	return m, nil
}

// sortIndicator describes the active sort for the box header (e.g. "size↓").
func (m App) sortIndicator() string {
	arrow := "↓"
	if m.sortAsc {
		arrow = "↑"
	}
	return m.sortBy.String() + arrow
}

// sortedObjects returns a copy of objs ordered by the active sort column/direction.
func (m App) sortedObjects(objs []storage.ObjectRef) []storage.ObjectRef {
	out := make([]storage.ObjectRef, len(objs))
	copy(out, objs)
	less := func(i, j int) bool {
		switch m.sortBy {
		case sortSize:
			if out[i].Size != out[j].Size {
				return out[i].Size < out[j].Size
			}
			return out[i].Key < out[j].Key
		case sortModified:
			if !out[i].LastModified.Equal(out[j].LastModified) {
				return out[i].LastModified.Before(out[j].LastModified)
			}
			return out[i].Key < out[j].Key
		default:
			return out[i].Key < out[j].Key
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if m.sortAsc {
			return less(i, j)
		}
		return less(j, i)
	})
	return out
}

// sortedDirs returns dirs ordered by name (folders have no size/date, FR-021). Direction
// follows the active toggle for name; for size/modified dirs keep a stable name order.
func (m App) sortedDirs(dirs []string) []string {
	out := make([]string, len(dirs))
	copy(out, dirs)
	sort.SliceStable(out, func(i, j int) bool {
		if m.sortBy == sortName && !m.sortAsc {
			return out[i] > out[j]
		}
		return out[i] < out[j]
	})
	return out
}
