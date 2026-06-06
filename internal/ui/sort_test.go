package ui

import (
	"context"
	"testing"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

// firstObjectKey returns the key of the first OBJECT entry in display order.
func firstObjectKey(m App) string {
	for _, e := range m.treeEntries() {
		if !e.isDir {
			return e.full
		}
	}
	return ""
}

// TestSortBySizeDescAndToggle: cycling to size sorts; with descending direction the
// largest object is first, and toggling direction reverses it (005 FR-020).
func TestSortBySizeDescAndToggle(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "small.txt", storage.FakeObject{Data: make([]byte, 10)})
	f.SeedObject("b", "big.bin", storage.FakeObject{Data: make([]byte, 9000)})
	f.SeedObject("b", "mid.dat", storage.FakeObject{Data: make([]byte, 500)})
	m := treeApp(f, false)

	// name → size; default direction is descending (sortAsc=false).
	mm, _ := m.cycleSort()
	m = mm.(App)
	if m.sortBy != sortSize {
		t.Fatalf("cycle should land on size; got %v", m.sortBy)
	}
	if got := firstObjectKey(m); got != "big.bin" {
		t.Errorf("size-desc first object = %q, want big.bin", got)
	}
	// Toggle to ascending → smallest first.
	mm, _ = m.toggleSortDir()
	m = mm.(App)
	if got := firstObjectKey(m); got != "small.txt" {
		t.Errorf("size-asc first object = %q, want small.txt", got)
	}
}

// TestSortPersistsAcrossNavigation: the chosen sort applies to a newly entered level
// (005 FR-020).
func TestSortPersistsAcrossNavigation(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "dir/s.txt", storage.FakeObject{Data: make([]byte, 5)})
	f.SeedObject("b", "dir/big.bin", storage.FakeObject{Data: make([]byte, 8000)})
	f.SeedObject("b", "top.txt", storage.FakeObject{Data: make([]byte, 1)})
	m := treeApp(f, false)

	mm, _ := m.cycleSort() // size, descending
	m = mm.(App)
	// Enter dir/ — the sort must carry over.
	m.prefix = "dir/"
	mm, _ = m.enterLevel()
	m = mm.(App)
	// Re-seed the loaded level (test harness doesn't run the async load).
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Prefix: "dir/"})
	m.level = &levelState{dirs: page.Dirs, objects: page.Objects, complete: true}
	if m.sortBy != sortSize {
		t.Fatalf("sort did not persist; got %v", m.sortBy)
	}
	if got := firstObjectKey(m); got != "dir/big.bin" {
		t.Errorf("persisted size-desc first = %q, want dir/big.bin", got)
	}
}

// TestSortByModified: sorting by last-modified orders by time; folders (no date) stay
// grouped consistently above objects (005 FR-021).
func TestSortByModified(t *testing.T) {
	f := storage.NewFake()
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f.SeedObject("b", "old.txt", storage.FakeObject{Data: []byte("o"), LastModified: old})
	f.SeedObject("b", "new.txt", storage.FakeObject{Data: []byte("n"), LastModified: recent})
	f.Seed("b", "sub/x")
	m := treeApp(f, false)

	m.sortBy = sortModified
	m.sortAsc = false // newest first
	if got := firstObjectKey(m); got != "new.txt" {
		t.Errorf("modified-desc first object = %q, want new.txt", got)
	}
	// The first entry overall is the folder (dirs group above objects).
	if first := m.treeEntries()[0]; !first.isDir {
		t.Errorf("folders should group above objects when sorting by modified; first=%+v", first)
	}
}
