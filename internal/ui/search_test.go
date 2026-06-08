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

// T013 / 015 US3: the bucket and object filter scopes are independent — committing or clearing
// one never changes the other.
func TestDualScopeIndependent(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha", "log.txt", "data.txt")
	f.Seed("beta")
	m := dualApp(f) // focus buckets

	m = press(m, "/")
	for _, r := range "alph" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if m.bucketFilter != "alph" {
		t.Fatalf("setup: bucket filter = %q", m.bucketFilter)
	}

	m = crossToObjects(m, f, "alpha")
	m = press(m, "/")
	for _, r := range "log" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	page, _ := f.ListLevel(context.Background(), storage.LevelQuery{Bucket: "alpha", Search: "log"})
	m = deliver(m, levelMsg{gen: m.gen, key: m.levelKey(), page: page})
	if m.bucketFilter != "alph" || m.search != "log" {
		t.Fatalf("both scopes should be committed: bucket=%q object=%q", m.bucketFilter, m.search)
	}

	// Clear the OBJECT filter → the bucket filter is untouched.
	m = press(m, "esc") // objectsBack clears the active search
	if m.search != "" {
		t.Fatalf("esc should clear the object search, got %q", m.search)
	}
	if m.bucketFilter != "alph" {
		t.Errorf("clearing the object filter must NOT change the bucket filter, got %q", m.bucketFilter)
	}
}

// T013 / 015 FR-013: a keystroke burst coalesces — a stale (older-gen) searchFireMsg never
// supersedes a newer keystroke (the searchGen guard in onSearchFire).
func TestObjectSearchSupersedes(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "apple", "apricot", "banana")
	m := enterTree(t, f, "b")

	m = press(m, "/")
	m = press(m, "a")
	m = press(m, "p")
	latest := m.searchGen

	genBefore := m.gen
	m = deliver(m, searchFireMsg{searchGen: latest - 1, term: "a"})
	if m.gen != genBefore || m.search != "" {
		t.Errorf("a stale debounce must not supersede a newer keystroke (search=%q)", m.search)
	}

	m = deliver(m, searchFireMsg{searchGen: latest, term: "ap"})
	if m.search != "ap" {
		t.Errorf("the latest debounce should apply, got search=%q", m.search)
	}
}

// 015 US5: the always-visible bucket FORM reflects the full lifecycle — placeholder → live edit →
// committed term → reopen pre-filled → Esc revert → clear.
func TestFilterFormLifecycle(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	m := dualApp(f) // focus buckets

	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "/ to filter") {
		t.Errorf("idle form should show the placeholder; got %q", s)
	}

	m = press(m, "/")
	for _, r := range "alph" {
		m = press(m, string(r))
	}
	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "filter buckets") || !strings.Contains(s, "alph") {
		t.Errorf("active form should show the live input; got %q", s)
	}

	m = press(m, "enter")
	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "alph") {
		t.Errorf("committed form should show the term; got %q", s)
	}

	// Reopen pre-filled, edit, then Esc reverts to the committed term.
	m = press(m, "/")
	if m.searchInput != "alph" {
		t.Fatalf("reopen should pre-fill the committed term, got %q", m.searchInput)
	}
	m = press(m, "backspace")
	m = press(m, "esc")
	if m.bucketFilter != "alph" {
		t.Errorf("Esc should revert to the committed term, got %q", m.bucketFilter)
	}

	// Clear (empty + commit) → form back to the placeholder.
	m = press(m, "/")
	for range "alph" {
		m = press(m, "backspace")
	}
	m = press(m, "enter")
	if m.bucketFilter != "" {
		t.Fatalf("clearing should empty the bucket filter, got %q", m.bucketFilter)
	}
	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "/ to filter") || strings.Contains(s, "alph") {
		t.Errorf("after clear the form should return to the placeholder; got %q", s)
	}
}

// 015 FR-016: switching context clears that level's filter and resets the form.
func TestNavigateAwayClearsFilter(t *testing.T) {
	f := storage.NewFake()
	f.Seed("alpha")
	f.Seed("beta")
	other := storage.NewFake()
	other.Seed("gamma")
	resolve := func(n string) (Backend, error) {
		if n == "two" {
			return Backend{Store: other}, nil
		}
		return Backend{Store: f}, nil
	}
	m := withBuckets(f, []string{"one", "two"}, resolve)
	m.ctxName = "one"

	m = press(m, "/")
	for _, r := range "alph" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if m.bucketFilter != "alph" {
		t.Fatalf("setup: bucket filter = %q", m.bucketFilter)
	}

	m = press(m, "2")
	m = finishSwitch(m, resolve, "two")
	if m.ctxName != "two" {
		t.Fatalf("context switch failed, ctx = %q", m.ctxName)
	}
	if m.bucketFilter != "" {
		t.Errorf("switching context must clear the bucket filter, got %q", m.bucketFilter)
	}
	if s := stripANSI(m.bucketFilterField(40)); !strings.Contains(s, "/ to filter") || strings.Contains(s, "alph") {
		t.Errorf("after a context switch the form resets to the placeholder; got %q", s)
	}
}
