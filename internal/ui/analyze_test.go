package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// driveUsageScan runs a usage scan started on m to its terminal report, applying every
// event via Update (so the cache ends populated). It re-arms the SAME channel carried in
// each progress msg (the drain discipline the production handler uses).
func driveUsageScan(t *testing.T, m App, bucket, prefix string) App {
	t.Helper()
	mm, cmd := m.startUsageScan(bucket, prefix, 0)
	m = mm.(App)
	if cmd == nil {
		t.Fatal("startUsageScan returned no cmd")
	}
	msg := cmd()
	for {
		switch ev := msg.(type) {
		case usageProgressMsg:
			mm, _ := m.Update(ev)
			m = mm.(App)
			if ev.ch == nil {
				t.Fatal("usageProgressMsg missing its channel (drain would leak)")
			}
			msg = waitForUsage(ev.ch, ev.gen, ev.key)()
		case usageDoneMsg:
			mm, _ := m.Update(ev)
			return mm.(App)
		default:
			t.Fatalf("unexpected msg %#v", ev)
		}
	}
}

// scanToDone runs a scan cmd to its raw terminal usageDoneMsg WITHOUT applying it (used to
// test generation isolation).
func scanToDone(t *testing.T, cmd tea.Cmd) usageDoneMsg {
	t.Helper()
	msg := cmd()
	for {
		switch ev := msg.(type) {
		case usageProgressMsg:
			msg = waitForUsage(ev.ch, ev.gen, ev.key)()
		case usageDoneMsg:
			return ev
		default:
			t.Fatalf("unexpected msg %#v", ev)
		}
	}
}

// TestInlineUsageTotalsCached: a completed scan caches the report and the inline usage line
// shows the total + count, ranked children largest-first (016 US2, FR-004/FR-007).
func TestInlineUsageTotalsCached(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "logs/a.log", storage.FakeObject{Data: make([]byte, 300)})
	f.SeedObject("b", "media/x.png", storage.FakeObject{Data: make([]byte, 100)})
	f.SeedObject("b", "big.bin", storage.FakeObject{Data: make([]byte, 250)})
	m := treeApp(f, false)

	m = driveUsageScan(t, m, "b", "")
	rep, ok := m.usageResults.Get(m.usageKey("b", ""))
	if !ok || rep.TotalCount != 3 || rep.TotalSize != 650 {
		t.Fatalf("usage not cached or totals wrong: ok=%v rep=%+v", ok, rep)
	}
	if rep.Children[0].Name != "logs/" || rep.Children[1].Name != "big.bin" {
		t.Errorf("children ranking: %+v", rep.Children)
	}
	if line := stripANSI(m.usageLine("b", "", 60)); !strings.Contains(line, "650 B") || !strings.Contains(line, "3 obj") {
		t.Errorf("usageLine = %q, want size + 3 obj", line)
	}
}

// TestSupersededScanReportCached (017 INVERTS the 016 discard): a scan's late terminal
// report carries data valid for ITS OWN target key — it is cached there even after the
// user navigated on (gen mismatch). The VIEW stays generation-guarded; only the work is
// no longer thrown away (017 US1/FR-004).
func TestSupersededScanReportCached(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "x", storage.FakeObject{Data: make([]byte, 10)})
	m := treeApp(f, false)

	mm, cmd := m.startUsageScan("b", "", 0)
	m = mm.(App)
	key := m.usageScanKey
	done := scanToDone(t, cmd) // raw terminal msg, stamped the scan's gen

	(&m).beginLoad() // navigation supersedes the scan: usageGen++ + cancel
	mm, _ = m.Update(done)
	m = mm.(App)
	rep, ok := m.usageResults.Get(key)
	if !ok || rep.TotalCount != 1 {
		t.Errorf("superseded scan's report must be cached under its own key: ok=%v rep=%+v", ok, rep)
	}
}

// TestInlineBreakdownAndConfigCycle: on a bucket, MoreDetail cycles None → breakdown →
// config → None (one section at a time). The breakdown lists ranked children; the config
// section renders the tri-state labels (016 US3 + US4).
func TestInlineBreakdownAndConfigCycle(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "logs/a.log", storage.FakeObject{Data: make([]byte, 300)})
	f.SeedObject("b", "big.bin", storage.FakeObject{Data: make([]byte, 250)})
	f.Buckets["b"].BucketConfig = storage.BucketConfig{
		Versioning: storage.ConfigItem{State: storage.ConfigConfigured, Detail: "Enabled"},
	}
	f.Buckets["b"].UnsupportedGetConfigs = map[string]bool{"replication": true}
	m := withBuckets(f, []string{"ctx"}, nil) // bucket list, "b" highlighted
	m = driveUsageScan(t, m, "b", "")

	// Press 1 → breakdown.
	mm, _ := m.startMoreDetail()
	m = mm.(App)
	if m.detailSection != sectBreakdown {
		t.Fatalf("press 1: want sectBreakdown, got %v", m.detailSection)
	}
	if v := m.detailBreakdownView("b", "", 60); !strings.Contains(v, "breakdown") || !strings.Contains(v, "logs/") {
		t.Errorf("breakdown view:\n%s", v)
	}

	// Press 2 → config (loads it). Drive the bucketConfigMsg.
	mm, cmd := m.startMoreDetail()
	m = mm.(App)
	if m.detailSection != sectConfig {
		t.Fatalf("press 2: want sectConfig, got %v", m.detailSection)
	}
	if cmd != nil {
		mm, _ = m.Update(cmd())
		m = mm.(App)
	}
	v := stripANSI(m.detailConfigView(60))
	if !strings.Contains(v, "Enabled") || !strings.Contains(v, "unsupported") || !strings.Contains(v, "none") {
		t.Errorf("config view must show configured/unsupported/none labels:\n%s", v)
	}

	// Press 3 → collapse.
	mm, _ = m.startMoreDetail()
	if mm.(App).detailSection != sectNone {
		t.Errorf("press 3: want sectNone, got %v", mm.(App).detailSection)
	}
}

// TestObjectTagsToggle: MoreDetail on an object loads + shows its tags; an object with no
// tags shows "none" (016 US4/FR-011).
func TestObjectTagsToggle(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "doc.txt", storage.FakeObject{Data: []byte("x"), Tags: map[string]string{"env": "prod"}})
	m := treeApp(f, false)
	selectObject(&m, "doc.txt")

	mm, cmd := m.startMoreDetail()
	m = mm.(App)
	if m.detailSection != sectTags {
		t.Fatalf("want sectTags, got %v", m.detailSection)
	}
	if cmd != nil {
		mm, _ = m.Update(cmd())
		m = mm.(App)
	}
	v := stripANSI(m.detailTagsView("doc.txt", 60))
	if !strings.Contains(v, "env") || !strings.Contains(v, "prod") {
		t.Errorf("tags view must show the KV pair:\n%s", v)
	}
}
