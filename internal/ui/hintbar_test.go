package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// labelsOf returns the labels of the currently-available actions.
func availLabels(m App) []string {
	out := make([]string, 0)
	for _, a := range m.availableActions() {
		out = append(out, a.label)
	}
	return out
}

func hasStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// --- US1: hint bar contents by selection / capability (FR-003/FR-004) ---

func TestHintBarObjectWritable(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")

	ls := availLabels(m)
	for _, want := range []string{"download", "delete", "copy", "move", "upload", "mkdir", "refresh"} {
		if !hasStr(ls, want) {
			t.Errorf("writable object hint bar missing %q; got %v", want, ls)
		}
	}
	if hasStr(ls, "rm -r") {
		t.Errorf("object selection must not offer recursive delete; got %v", ls)
	}
	// The rendered bar must include the global cues.
	bar := m.hintBarView(120)
	if !strings.Contains(bar, "help") || !strings.Contains(bar, "quit") {
		t.Errorf("hint bar must always show help/quit; got %q", bar)
	}
}

func TestHintBarFolderWritable(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := treeApp(f, true)
	selectDir(&m, "docs/")

	ls := availLabels(m)
	for _, want := range []string{"analyze", "rm -r", "upload", "mkdir"} {
		if !hasStr(ls, want) {
			t.Errorf("folder hint bar missing %q; got %v", want, ls)
		}
	}
	for _, no := range []string{"delete", "copy", "move", "download"} {
		if hasStr(ls, no) {
			t.Errorf("folder selection must not offer %q; got %v", no, ls)
		}
	}
}

func TestHintBarReadOnlyOmitsWrite(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false) // read-only
	selectObject(&m, "a.txt")

	ls := availLabels(m)
	if !hasStr(ls, "download") { // read — always available
		t.Errorf("read-only must still offer download; got %v", ls)
	}
	for _, no := range []string{"delete", "copy", "move", "upload", "mkdir"} {
		if hasStr(ls, no) {
			t.Errorf("read-only must omit write action %q; got %v", no, ls)
		}
	}
}

// --- US1: direct keys route into the EXISTING flows, no menu (FR-001/FR-002/FR-005) ---

func TestDirectDeleteEntersTypedConfirm(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "x")

	if m.op == nil || m.op.kind != "delete_object" {
		t.Fatalf("'x' should start a delete_object op; op=%+v", m.op)
	}
	if m.op.tier != confirmTyped || m.op.phase != phaseConfirm {
		t.Fatalf("delete must keep typed-confirm tier+phase; tier=%v phase=%v", m.op.tier, m.op.phase)
	}
	if m.mode != modeTree {
		t.Fatalf("mode must stay tree (no menu); got %v", m.mode)
	}
}

func TestDirectDownloadStartsImmediately(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, "d")

	if m.op == nil || m.op.kind != "download" {
		t.Fatalf("'d' should start a download op immediately; op=%+v", m.op)
	}
}

func TestDirectAnalyzeNoMenu(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "docs/x.txt")
	m := treeApp(f, true)
	selectDir(&m, "docs/")
	m = press(m, "a")

	if m.mode != modeUsage {
		t.Fatalf("'a' on a folder should analyze (modeUsage), not open a menu; mode=%v", m.mode)
	}
}

func TestReadOnlyWriteKeyIsSafeNoop(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, false) // read-only
	selectObject(&m, "a.txt")
	m = press(m, "x") // delete attempt

	if m.op != nil {
		t.Fatalf("a write key in read-only must not start an op; op=%+v", m.op)
	}
	if m.mode != modeTree {
		t.Fatalf("mode must stay tree; got %v", m.mode)
	}
	if m.errorText() == "" {
		t.Error("a refused write should surface an explanatory status")
	}
}

// --- US1: multi-select routes d/x/y to the bulk variant (FR-006) ---

func TestMarkedRoutesDeleteToBulk(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "c.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, " ") // mark a.txt
	if m.selCount() != 1 {
		t.Fatalf("space should mark the object; selCount=%d", m.selCount())
	}
	m = press(m, "x") // delete with a mark → bulk
	if m.op == nil || m.op.kind != "bulk_delete" {
		t.Fatalf("marked delete should route to bulk_delete; op=%+v", m.op)
	}
	// The hint bar should advertise the bulk count.
	if bar := m.hintBarView(120); !strings.Contains(bar, "delete 1") {
		_ = bar // op already consumed selection on confirm path; checked on a fresh model below
	}
}

func TestHintBarBulkLabelWithCount(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt", "c.txt")
	m := treeApp(f, true)
	selectObject(&m, "a.txt")
	m = press(m, " ") // mark
	bar := m.hintBarView(120)
	if !strings.Contains(bar, "delete 1") || !strings.Contains(bar, "download 1") {
		t.Errorf("hint bar should show bulk variant + count when marked; got %q", bar)
	}
}

// --- US1: bucket list offers analyze + refresh directly ---

func TestBucketListDirectAnalyze(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, "a")
	if m.mode != modeUsage {
		t.Fatalf("'a' in the bucket list should analyze the bucket; mode=%v", m.mode)
	}
}
