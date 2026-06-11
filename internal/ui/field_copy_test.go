package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// TestFieldCopyFullValue: the field-select rows carry the FULL untruncated value (a long
// KMS ARN), so the copy payload is usable even when the render truncates (017 US2/FR-012).
func TestFieldCopyFullValue(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")
	md := enrichedMeta()
	m.paneMeta = &md
	m.paneSelKey = "report.pdf"

	mm, _ := m.startFieldCopy()
	m = mm.(App)
	if m.fieldCopy == nil {
		t.Fatal("startFieldCopy must open the field-select state")
	}
	var arn string
	for _, r := range m.fieldCopy.rows {
		if r.label == "KMS key" {
			arn = r.value
		}
	}
	if arn != md.SSEKMSKeyID {
		t.Errorf("field row value = %q, want the FULL ARN %q", arn, md.SSEKMSKeyID)
	}
}

// TestFieldCopyEnterCopiesAndConfirms: Enter on a row emits the clipboard cmd and the
// footer names what was copied; the state closes (017 US2/FR-012).
func TestFieldCopyEnterCopiesAndConfirms(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")
	md := enrichedMeta()
	m.paneMeta = &md
	m.paneSelKey = "report.pdf"

	mm, _ := m.startFieldCopy()
	m = mm.(App)
	mm, cmd := m.Update(keyMsgFor("enter"))
	m = mm.(App)
	if cmd == nil {
		t.Error("Enter must emit a clipboard command (best-effort OSC52)")
	}
	if m.fieldCopy != nil {
		t.Error("field-select must close after copying")
	}
	if !strings.Contains(m.notice, "copied") || !strings.Contains(m.notice, "Key") {
		t.Errorf("footer confirmation = %q, want 'copied <label> …'", m.notice)
	}
}

// TestFieldCopyEscCancels: Esc closes without copying (017 US2).
func TestFieldCopyEscCancels(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "report.pdf", storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	selectObject(&m, "report.pdf")
	md := enrichedMeta()
	m.paneMeta = &md
	m.paneSelKey = "report.pdf"

	mm, _ := m.startFieldCopy()
	m = mm.(App)
	mm, cmd := m.Update(keyMsgFor("esc"))
	m = mm.(App)
	if m.fieldCopy != nil {
		t.Error("Esc must close the field-select state")
	}
	if cmd != nil {
		t.Error("Esc must not emit a clipboard command")
	}
	if m.notice != "" {
		t.Errorf("Esc must not confirm a copy, notice = %q", m.notice)
	}
}
