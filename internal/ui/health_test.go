package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// distReport builds an exact report with recognizable distributions.
func distReport(complete bool) *storage.UsageReport {
	rep := &storage.UsageReport{
		Bucket: "b", TotalCount: 10, TotalSize: 1000,
		Complete: complete, Bounded: !complete,
		ClassDist: map[string]storage.DistBucket{
			"STANDARD": {Count: 9, Size: 900},
			"COLD":     {Count: 1, Size: 100},
		},
	}
	rep.AgeDist[0] = storage.DistBucket{Count: 4, Size: 400}
	rep.AgeDist[5] = storage.DistBucket{Count: 6, Size: 600}
	rep.SizeDist[0] = storage.DistBucket{Count: 4, Size: 40}
	rep.SizeDist[2] = storage.DistBucket{Count: 6, Size: 960}
	return rep
}

// healthApp: bucket list with "b" highlighted, exact report cached, MPUs seeded.
func healthApp(t *testing.T, f *storage.Fake) App {
	t.Helper()
	m := withBuckets(f, []string{"ctx"}, nil)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m.usageResults.Put(m.usageKey("b", ""), distReport(true))
	return m
}

// TestHealthCardOpensAndRenders: `H` opens the full-screen card with histograms from
// the cached report and the MPU block from the probe (017 US4/FR-020/FR-021).
func TestHealthCardOpensAndRenders(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	f.SeedIncompleteUpload("b", storage.FakeIncompleteUpload{Key: "k1", Initiated: mpuTime(), PartSizes: []int64{100}})
	f.SeedIncompleteUpload("b", storage.FakeIncompleteUpload{Key: "k2", Initiated: mpuTime().Add(time.Hour), PartSizes: []int64{200}})
	m := healthApp(t, f)

	mm, cmd := pressCmd(m, "H")
	m = mm
	if m.mode != modeHealth {
		t.Fatalf("mode = %v, want modeHealth", m.mode)
	}
	if cmd != nil {
		m = drainBatch(t, m, cmd) // the MPU probe
	}
	v := stripANSI(viewOf(m))
	for _, want := range []string{"Age", "<1d", ">1y", "Size", "<128KiB", "STANDARD", "COLD", "Incomplete multipart", "2 in progress"} {
		if !strings.Contains(v, want) {
			t.Errorf("health card missing %q:\n%s", want, v)
		}
	}
}

// TestHealthPartialLabels: a budget-bounded report renders every figure as a lower
// bound with the full-scan affordance (017 US4/FR-024).
func TestHealthPartialLabels(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := healthApp(t, f)
	m.usageResults.Put(m.usageKey("b", ""), distReport(false))

	mm, _ := pressCmd(m, "H")
	v := stripANSI(viewOf(mm))
	if !strings.Contains(v, "≥") || !strings.Contains(v, "partial") {
		t.Errorf("partial card must carry ≥ + partial:\n%s", v)
	}
	if !strings.Contains(v, "full scan") {
		t.Errorf("partial card must advertise the full-scan affordance:\n%s", v)
	}
}

// TestHealthMPUStates: denied/unsupported are explicit states — never zero-as-clean
// (017 FR-022, SC-007).
func TestHealthMPUStates(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	f.FailListUploads = true
	m := healthApp(t, f)

	mm, cmd := pressCmd(m, "H")
	m = drainBatch(t, mm, cmd)
	v := stripANSI(viewOf(m))
	if !strings.Contains(v, "denied") {
		t.Errorf("denied probe must render 'denied':\n%s", v)
	}
	if strings.Contains(v, "uploads: none") {
		t.Errorf("denied must NEVER read as clean/none:\n%s", v)
	}

	f.FailListUploads = false
	f.UnsupportedListUploads = true
	m2 := healthApp(t, f)
	mm, cmd = pressCmd(m2, "H")
	m2 = drainBatch(t, mm, cmd)
	if v := stripANSI(viewOf(m2)); !strings.Contains(v, "unsupported") {
		t.Errorf("unsupported probe must render 'unsupported':\n%s", v)
	}
}

// TestHealthSmallObjectWarning: fires above the share threshold with both numbers in
// text; silent at/below it (017 FR-023).
func TestHealthSmallObjectWarning(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := healthApp(t, f)
	rep := distReport(true)
	rep.SizeDist[0] = storage.DistBucket{Count: 6, Size: 6} // 60% < 128 KiB
	rep.SizeDist[2] = storage.DistBucket{Count: 4, Size: 994}
	m.usageResults.Put(m.usageKey("b", ""), rep)

	mm, _ := pressCmd(m, "H")
	v := stripANSI(viewOf(mm))
	if !strings.Contains(v, "60%") || !strings.Contains(v, "128") || !strings.Contains(v, "small-object") {
		t.Errorf("warning must name share + threshold:\n%s", v)
	}

	rep2 := distReport(true)
	rep2.SizeDist[0] = storage.DistBucket{Count: 5, Size: 5} // exactly 50% — not above
	rep2.SizeDist[2] = storage.DistBucket{Count: 5, Size: 995}
	m.usageResults.Put(m.usageKey("b", ""), rep2)
	mm, _ = pressCmd(m, "H")
	if v := stripANSI(viewOf(mm)); strings.Contains(v, "small-object") {
		t.Errorf("warning must stay silent at ≤ the share threshold:\n%s", v)
	}
}

// TestHealthEscRestores: Esc returns to the exact prior browsing position (017 US4
// acceptance 7).
func TestHealthEscRestores(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := healthApp(t, f)
	m.bucketSel = 0

	mm, _ := pressCmd(m, "H")
	m = mm
	m = press(m, "esc")
	if m.mode != modeBuckets || m.bucketSel != 0 {
		t.Errorf("Esc must restore mode/selection, got mode=%v sel=%d", m.mode, m.bucketSel)
	}
}

// TestHealthObjectFocusNoop: an object selection has no card target — no-op + footer
// note (contract health-card-view.md).
func TestHealthObjectFocusNoop(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	m = press(m, "H")
	if m.mode == modeHealth {
		t.Error("object focus must not open the health card")
	}
	if m.notice == "" {
		t.Error("the no-op must explain itself in the footer")
	}
}

// TestHealthUncachedStartsBudgetedOnly: opening the card never starts unbounded work —
// at most the budgeted scan (017 FR-003).
func TestHealthUncachedStartsBudgetedOnly(t *testing.T) {
	f := storage.NewFake()
	seedKeys(f, "b", 1500)
	m := withBuckets(f, []string{"ctx"}, nil)
	m = deliver(m, tea.WindowSizeMsg{Width: 140, Height: 40})
	m.usageBudget = 1000

	mm, cmd := pressCmd(m, "H")
	m = drainBatch(t, mm, cmd)
	rep, ok := m.usageResults.Get(m.usageKey("b", ""))
	if !ok || !rep.Bounded {
		t.Fatalf("card-started scan = ok=%v %+v, want budget-bounded", ok, rep)
	}
	if f.UsagePages != 1 {
		t.Errorf("UsagePages = %d, want 1 — H must never run unbounded work", f.UsagePages)
	}
}

// TestHealthCommandOpens: `:health` reaches the same dispatcher as `H`.
func TestHealthCommandOpens(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: []byte("y")})
	m := healthApp(t, f)

	m = press(m, ":")
	for _, r := range "health" {
		m = press(m, string(r))
	}
	mm, _ := pressCmd(m, "enter")
	if mm.mode != modeHealth {
		t.Errorf(":health must open the card, mode = %v", mm.mode)
	}
}

func mpuTime() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

// drainBatch executes a cmd (possibly a tea.Batch) and applies every produced message
// that the model recognises, returning the settled model.
func drainBatch(t *testing.T, m App, cmd tea.Cmd) App {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	switch batch := msg.(type) {
	case tea.BatchMsg:
		for _, c := range batch {
			m = drainBatch(t, m, c)
		}
		return m
	default:
		if msg == nil {
			return m
		}
		switch ev := msg.(type) {
		case usageProgressMsg:
			mm, _ := m.Update(ev)
			return drainBatch(t, mm.(App), waitForUsage(ev.ch, ev.gen, ev.key))
		default:
			mm, next := m.Update(msg)
			return drainBatch(t, mm.(App), next)
		}
	}
}
