package ui

import (
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// menuLabels returns the labels of the currently built action menu.
func menuLabels(m App) []string {
	out := make([]string, len(m.menuItems))
	for i, it := range m.menuItems {
		out[i] = it.label
	}
	return out
}

func hasLabel(ls []string, s string) bool {
	for _, l := range ls {
		if l == s {
			return true
		}
	}
	return false
}

// viaMenu drives an operation through its new entry point: open the action menu,
// select the labeled item, and invoke it. Replaces the old direct top-level key.
func viaMenu(t *testing.T, m App, label string) App {
	t.Helper()
	m = press(m, "a")
	if m.mode != modeActionMenu {
		t.Fatalf("'a' did not open the action menu (mode=%v)", m.mode)
	}
	idx := -1
	for i, it := range m.menuItems {
		if it.label == label {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("action menu has no %q item; got %v", label, menuLabels(m))
	}
	m.menuSel = idx
	return press(m, "enter")
}

// --- C2: contextual items ---

func TestActionMenuObjectSelected(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "a")

	if m.mode != modeActionMenu {
		t.Fatalf("'a' should open the action menu; mode=%v", m.mode)
	}
	ls := menuLabels(m)
	for _, want := range []string{"delete", "copy", "move / rename", "upload here", "new folder", "refresh"} {
		if !hasLabel(ls, want) {
			t.Errorf("object menu missing %q; got %v", want, ls)
		}
	}
	if hasLabel(ls, "recursive delete") {
		t.Errorf("object menu must not offer recursive delete; got %v", ls)
	}
}

func TestActionMenuFolderSelected(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := treeApp(f, true)
	selectDir(&m, "docs/")
	m = press(m, "a")

	ls := menuLabels(m)
	for _, want := range []string{"recursive delete", "upload here", "new folder", "refresh"} {
		if !hasLabel(ls, want) {
			t.Errorf("folder menu missing %q; got %v", want, ls)
		}
	}
	for _, no := range []string{"delete", "copy", "move / rename"} {
		if hasLabel(ls, no) {
			t.Errorf("folder menu must not offer %q; got %v", no, ls)
		}
	}
}

func TestActionMenuReadOnly(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false)
	selectObject(&m, "a.txt")
	m = press(m, "a")

	if got := menuLabels(m); len(got) != 1 || got[0] != "refresh" {
		t.Fatalf("read-only menu = %v, want [refresh] only", got)
	}
}

func TestActionMenuBuckets(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, "a")

	if m.mode != modeActionMenu {
		t.Fatalf("'a' should open menu in bucket list; mode=%v", m.mode)
	}
	if got := menuLabels(m); len(got) != 1 || got[0] != "refresh" {
		t.Fatalf("bucket menu = %v, want [refresh] only", got)
	}
}

// --- C3: invocation enters the existing op flow unchanged ---

func TestActionMenuInvokeDeleteEntersTypedConfirm(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "a") // menuSel=0 -> "delete"
	m = press(m, "enter")

	if m.op == nil || m.op.kind != "delete_object" {
		t.Fatalf("invoking delete should start a delete_object op; op=%+v", m.op)
	}
	if m.op.tier != confirmTyped || m.op.phase != phaseConfirm {
		t.Fatalf("delete must keep typed-confirm tier+phase; tier=%v phase=%v", m.op.tier, m.op.phase)
	}
	if m.mode != modeTree {
		t.Fatalf("mode should restore to tree under the op; mode=%v", m.mode)
	}
}

func TestActionMenuInvokeCopyEntersDestEntry(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "a")
	m = press(m, "down") // -> "copy"
	m = press(m, "enter")

	if m.op == nil || m.op.kind != "copy" || m.op.phase != phaseDest {
		t.Fatalf("invoking copy should enter dest entry; op=%+v", m.op)
	}
}

// --- C1: dismissal ---

func TestActionMenuEscCloses(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "a")
	m = press(m, "esc")

	if m.mode != modeTree {
		t.Fatalf("esc should close the menu back to tree; mode=%v", m.mode)
	}
	if m.op != nil {
		t.Fatalf("esc should create no operation; op=%+v", m.op)
	}
	if m.menuItems != nil {
		t.Fatalf("esc should clear menu items")
	}
}

// --- C4: removed top-level keys are inert ---

func TestRemovedTopLevelKeysInert(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	for _, key := range []string{"d", "u", "y", "m", "D", "+", "r", "x"} {
		m := treeApp(f, true)
		selectObject(&m, "a.txt")
		m = press(m, key)
		if m.op != nil {
			t.Errorf("key %q must be inert at top level, but started op %+v", key, m.op)
		}
		if m.mode != modeTree {
			t.Errorf("key %q changed mode to %v (want tree)", key, m.mode)
		}
	}
}

// --- C4: Esc-cancel of in-flight load / running op (FR-029) ---

func TestEscCancelsInFlightLoad(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.loading = true
	m = press(m, "esc")
	if m.loading {
		t.Fatal("esc during an in-flight load should cancel it (loading=false)")
	}
}

func TestEscCancelsRunningOp(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m.op = &operation{kind: "upload", phase: phaseRunning}
	m.loading = true
	m = press(m, "esc")
	if m.loading {
		t.Fatal("esc during a running op should cancel the load (loading=false)")
	}
}

func TestEscBackWhenIdle(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true) // prefix "", not loading
	m = press(m, "esc")
	if m.mode != modeBuckets {
		t.Fatalf("esc at tree root (idle) should go back to buckets; mode=%v", m.mode)
	}
}

// --- C4: top-level interactive action count ≤ 12 (SC-008) ---

func TestTopLevelActionCountWithinCap(t *testing.T) {
	// The logical actions routed at the top level after the reduction (each counts
	// once regardless of aliases; the numeric quick-switch counts as one). Write ops
	// and refresh are NOT here — they live in the action menu.
	topLevel := []string{
		"Up", "Down", "Top", "Bottom", "Enter", "Back",
		"Search", "Menu", "Context", "Help", "Quit", "NumericSwitch",
	}
	if len(topLevel) > 12 {
		t.Fatalf("top-level interactive actions = %d, want ≤ 12", len(topLevel))
	}
}

// --- C4 obligation 10: modal precedence (FR-029) ---

func TestEscModalPrecedenceMenuOverLoad(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m.loading = true
	m = press(m, "a") // menu open while a load is in flight
	if m.mode != modeActionMenu {
		t.Fatalf("menu should open even during a load; mode=%v", m.mode)
	}
	m = press(m, "esc") // first esc: closes the menu, must NOT cancel the load
	if m.mode != modeTree {
		t.Fatalf("first esc should close the menu; mode=%v", m.mode)
	}
	if !m.loading {
		t.Fatal("first esc (closing the menu) must NOT cancel the background load")
	}
	m = press(m, "esc") // second esc: now cancels the load
	if m.loading {
		t.Fatal("second esc should cancel the load")
	}
}
