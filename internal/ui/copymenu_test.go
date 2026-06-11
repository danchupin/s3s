package ui

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

func menuLabels(m App) []string {
	if m.copyMenu == nil {
		return nil
	}
	out := make([]string, 0, len(m.copyMenu.items))
	for _, it := range m.copyMenu.items {
		out = append(out, it.label)
	}
	return out
}

func hasLabel(labels []string, sub string) bool {
	for _, l := range labels {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

// TestCopyMenuItemsObject: an object focus offers URI / URL / command / presigned /
// field — and no export without a visible report (017 US3/FR-014, T031 matrix).
func TestCopyMenuItemsObject(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	mm, _ := m.openCopyMenu()
	labels := menuLabels(mm.(App))
	for _, want := range []string{"S3 URI", "HTTPS URL", "download command", "presigned link", "presigned curl", "copy a field"} {
		if !hasLabel(labels, want) {
			t.Errorf("object menu missing %q: %v", want, labels)
		}
	}
	if hasLabel(labels, "export") {
		t.Errorf("no report cached — export must be absent: %v", labels)
	}
}

// TestCopyMenuItemsBucketAndPrefix: bucket/prefix focus offers the URI (and export only
// when a report is visible).
func TestCopyMenuItemsBucketAndPrefix(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "docs/a.txt", storage.FakeObject{Data: []byte("x")})
	m := withBuckets(f, []string{"ctx"}, nil) // bucket focus

	mm, _ := m.openCopyMenu()
	labels := menuLabels(mm.(App))
	if !hasLabel(labels, "S3 URI") || hasLabel(labels, "HTTPS URL") {
		t.Errorf("bucket menu = %v, want URI only", labels)
	}

	m.usageResults.Put(m.usageKey("b", ""), &storage.UsageReport{Bucket: "b", TotalCount: 1, TotalSize: 1, Complete: true})
	mm, _ = m.openCopyMenu()
	labels = menuLabels(mm.(App))
	if !hasLabel(labels, "export CSV") || !hasLabel(labels, "export JSON") {
		t.Errorf("report visible — export items required: %v", labels)
	}
}

// TestCopyMenuURICopy: Enter on S3 URI emits the clipboard cmd, closes the menu, and
// the footer names the artifact (017 US3, FR-014/menu confirmation).
func TestCopyMenuURICopy(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	mm, _ := m.openCopyMenu()
	m = mm.(App)
	mm2, cmd := m.Update(keyMsgFor("enter")) // first item = S3 URI
	m = mm2.(App)
	if cmd == nil {
		t.Error("Enter must emit the clipboard cmd")
	}
	if m.copyMenu != nil {
		t.Error("menu must close after copying")
	}
	if !strings.Contains(m.notice, "S3 URI") {
		t.Errorf("footer must name the artifact, notice = %q", m.notice)
	}
}

// TestCopyMenuKeyAndCommand: `Y` and `:copy` both open the same menu (one dispatcher).
func TestCopyMenuKeyAndCommand(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	if m2 := press(m, "Y"); m2.copyMenu == nil {
		t.Error("Y must open the copy menu")
	}
	m = press(m, ":")
	for _, r := range "copy" {
		m = press(m, string(r))
	}
	if m2 := press(m, "enter"); m2.copyMenu == nil {
		t.Error(":copy must open the copy menu")
	}
}

// TestCopyMenuTTLPicker: the presigned item opens a 4-preset picker defaulting to 1h;
// Enter dispatches the presign and the done msg copies + confirms (017 FR-015).
func TestCopyMenuTTLPicker(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	mm, _ := m.openCopyMenu()
	m = mm.(App)
	// Move to the "presigned link…" item.
	for m.copyMenu.items[m.copyMenu.sel].kind != copyKindPresign {
		m = deliver(m, keyMsgFor("down"))
	}
	m = deliver(m, keyMsgFor("enter"))
	if m.copyMenu == nil || !m.copyMenu.ttlPick {
		t.Fatal("presigned item must open the TTL picker")
	}
	if got := storage.PresignTTLs[m.copyMenu.ttlSel]; got != time.Hour {
		t.Errorf("default TTL = %v, want 1h", got)
	}
	mm2, cmd := m.Update(keyMsgFor("enter"))
	m = mm2.(App)
	if cmd == nil {
		t.Fatal("TTL Enter must dispatch the presign cmd")
	}
	msg := cmd()
	mm2, clip := m.Update(msg)
	m = mm2.(App)
	if clip == nil {
		t.Error("presign done must emit the clipboard cmd")
	}
	if !strings.Contains(m.notice, "presigned") {
		t.Errorf("notice = %q, want presigned confirmation", m.notice)
	}
}

// TestPresignCredWarnSurfaces: a TTL outliving the credentials surfaces the warning in
// the footer notice (017 FR-017).
func TestPresignCredWarnSurfaces(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	f.PresignCredsExpireIn = 30 * time.Minute
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	cmd := m.presignCmd("b", "report.pdf", 24*time.Hour, false)
	mm, _ := m.Update(cmd())
	if n := mm.(App).notice; !strings.Contains(n, "expire") {
		t.Errorf("notice = %q, want the cred-expiry warning", n)
	}
}

// TestPresignLogRedaction: the whole presign flow leaves NO URL/signature in the log —
// the link is a bearer secret (017 FR-016, constitution V).
func TestPresignLogRedaction(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)

	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")

	cmd := m.presignCmd("b", "report.pdf", time.Hour, false)
	_, _ = m.Update(cmd())

	logs := buf.String()
	if strings.Contains(logs, "fake.presign") || strings.Contains(logs, "X-Amz-") {
		t.Errorf("presigned URL leaked into the log:\n%s", logs)
	}
}

// TestExportWritesFile: export lands a correctly-named CSV in DownloadDir and the
// footer carries the path (017 US3/FR-018).
func TestExportWritesFile(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "a.txt", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	dir := t.TempDir()
	m.info.DownloadDir = dir
	m.now = func() time.Time { return time.Date(2026, 6, 11, 15, 4, 5, 0, time.UTC) }
	rep := storage.UsageReport{Bucket: "b", TotalCount: 1, TotalSize: 1, Complete: true}
	m.usageResults.Put(m.usageKey("b", ""), &rep)

	cmd := m.exportReportCmd(rep, "csv")
	mm, _ := m.Update(cmd())
	m = mm.(App)

	want := filepath.Join(dir, "s3s-report-b-20260611-150405.csv")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("export file missing: %v", err)
	}
	if !strings.HasPrefix(string(data), "section,label,count,bytes,bounded") {
		t.Errorf("export content header wrong:\n%s", data)
	}
	if !strings.Contains(m.notice, want) {
		t.Errorf("footer must carry the export path, notice = %q", m.notice)
	}
}

// TestExportFailureLeavesNoFile: an unwritable destination reports the error and leaves
// no partial file behind (017 FR-018, spec edge case).
func TestExportFailureLeavesNoFile(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "a.txt", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.info.DownloadDir = blocker // a FILE, not a directory → writes fail
	rep := storage.UsageReport{Bucket: "b", TotalCount: 1, TotalSize: 1, Complete: true}

	cmd := m.exportReportCmd(rep, "csv")
	mm, _ := m.Update(cmd())
	m = mm.(App)
	if m.err == nil {
		t.Error("export failure must surface an error")
	}
	entries, _ := os.ReadDir(filepath.Dir(blocker))
	for _, e := range entries {
		if strings.Contains(e.Name(), "s3s-report") {
			t.Errorf("partial export file left behind: %s", e.Name())
		}
	}
}
