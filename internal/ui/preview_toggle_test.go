package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// objectApp opens modeObject with the given payload loaded.
func objectApp(payload preview.Payload) App {
	f := storage.NewFake()
	f.SeedObject("b", payload.Key, storage.FakeObject{Data: []byte("x")})
	m := treeApp(f, false)
	m.mode = modeObject
	md := storage.ObjectMetadata{Key: payload.Key}
	m.meta = &md
	m.prev = &payload
	return m
}

// TestPreviewPrettyDefaultAndToggle: JSON renders pretty by default; `p` flips to the
// byte-identical raw form and back (017 US5/FR-025).
func TestPreviewPrettyDefaultAndToggle(t *testing.T) {
	p := preview.Build("d.json", "application/json", []byte(`{"alpha":1,"beta":[2,3]}`), false)
	m := objectApp(p)

	v := stripANSI(viewOf(m))
	if !strings.Contains(v, `"alpha": 1`) {
		t.Errorf("JSON must render pretty by default:\n%s", v)
	}
	m = press(m, "p")
	v = stripANSI(viewOf(m))
	if !strings.Contains(v, `{"alpha":1,"beta":[2,3]}`) {
		t.Errorf("raw toggle must show the original bytes:\n%s", v)
	}
	m = press(m, "p")
	if v := stripANSI(viewOf(m)); !strings.Contains(v, `"alpha": 1`) {
		t.Errorf("second toggle must return to pretty:\n%s", v)
	}
}

// TestPreviewToggleResetsPerObject: rawPreview resets when a new payload arrives.
func TestPreviewToggleResetsPerObject(t *testing.T) {
	p := preview.Build("d.json", "application/json", []byte(`{"a":1}`), false)
	m := objectApp(p)
	m = press(m, "p")
	if !m.rawPreview {
		t.Fatal("p must set rawPreview")
	}
	next := preview.Build("e.json", "application/json", []byte(`{"b":2}`), false)
	m = deliver(m, previewMsg{gen: m.gen, payload: next})
	if m.rawPreview {
		t.Error("a new payload must reset the raw toggle")
	}
}

// TestPreviewToggleNoopForPlainText: `p` is inert for non-JSON kinds.
func TestPreviewToggleNoopForPlainText(t *testing.T) {
	p := preview.Build("a.txt", "text/plain", []byte("plain words"), false)
	m := objectApp(p)
	before := viewOf(m)
	m = press(m, "p")
	if viewOf(m) != before {
		t.Error("p must be a no-op for plain text")
	}
}

// TestPreviewBinaryHexDump: binary payloads render the offset+hex+ASCII dump with the
// summary as the header line (017 US5/FR-027).
func TestPreviewBinaryHexDump(t *testing.T) {
	p := preview.Build("blob.bin", "application/octet-stream", []byte{0x00, 0x01, 'A', 'B'}, false)
	m := objectApp(p)
	v := stripANSI(viewOf(m))
	if !strings.Contains(v, "00000000") || !strings.Contains(v, "00 01 41 42") {
		t.Errorf("binary preview must hex-dump:\n%s", v)
	}
	if !strings.Contains(v, "binary content") {
		t.Errorf("the summary header must survive:\n%s", v)
	}
}

// TestPreviewGzipIndicator: a decompressed payload names both sizes (017 FR-026).
func TestPreviewGzipIndicator(t *testing.T) {
	p := preview.Payload{
		Key: "log.gz", Data: []byte("decompressed text"), Kind: preview.KindText,
		Compressed: &preview.Compressed{From: 64},
	}
	m := objectApp(p)
	v := stripANSI(viewOf(m))
	if !strings.Contains(v, "gzip") || !strings.Contains(v, "64") {
		t.Errorf("gzip indicator must carry the compressed size:\n%s", v)
	}
}
