package ui

import (
	"context"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

func TestSearchNarrowsLevel(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple", "apricot", "banana")
	m := enterTree(t, f, "b")

	m = press(m, "/")
	if !m.searching {
		t.Fatal("'/' should open the search input")
	}

	// Type "ap" — each keystroke arms a debounce with a fresh searchGen.
	m = press(m, "a")
	m = press(m, "p")
	if m.searchInput != "ap" {
		t.Fatalf("searchInput = %q, want ap", m.searchInput)
	}
	latest := m.searchGen

	// A stale debounce (older searchGen) must be ignored — ≤1 in-flight (FR-017a).
	genBefore := m.gen
	m = deliver(m, searchFireMsg{searchGen: latest - 1, term: "a"})
	if m.gen != genBefore {
		t.Error("stale debounce should not trigger a load")
	}

	// The latest debounce fires → narrows the level.
	m = deliver(m, searchFireMsg{searchGen: latest, term: "ap"})
	if m.search != "ap" {
		t.Fatalf("active search = %q, want ap", m.search)
	}
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "ap"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
	if len(m.treeEntries()) != 2 {
		t.Fatalf("search 'ap' should show apple+apricot, got %+v", m.treeEntries())
	}
}

func TestSearchNoMatch(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple")
	m := enterTree(t, f, "b")
	m = press(m, "/")
	m = press(m, "z")
	m = deliver(m, searchFireMsg{searchGen: m.searchGen, term: "z"})
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "z"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
	if !strings.Contains(viewOf(m), "No matches") {
		t.Errorf("no-match should show explicit state:\n%s", viewOf(m))
	}
}

func TestSearchClearRestores(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple", "banana")
	m := enterTree(t, f, "b")

	// Apply a search.
	m = press(m, "/")
	m = press(m, "a")
	m = deliver(m, searchFireMsg{searchGen: m.searchGen, term: "a"})
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "b", Search: "a"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})

	// Esc clears it and restores the full level (FR-018) — root is cached.
	m = press(m, "esc")
	if m.search != "" {
		t.Fatalf("esc should clear active search, got %q", m.search)
	}
	if m.searching {
		t.Error("esc should close the search input")
	}
	if len(m.treeEntries()) != 2 {
		t.Errorf("clearing search should restore full level, got %+v", m.treeEntries())
	}
}

func TestSearchEnterConfirmsAndCloses(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple")
	m := enterTree(t, f, "b")
	m = press(m, "/")
	m = press(m, "a")
	m = press(m, "enter")
	if m.searching {
		t.Error("enter should close the search input")
	}
	if m.search != "a" {
		t.Errorf("enter should confirm term, search = %q", m.search)
	}
}
