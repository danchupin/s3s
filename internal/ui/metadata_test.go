package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

func TestMetadataPaneRenders(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "file.txt", storage.FakeObject{Data: []byte("hello")})
	m := enterTree(t, f, "b")

	// Selected entry is the single object; open metadata.
	m = press(m, "i")
	if !m.loading {
		t.Fatal("'i' should arm a metadata load")
	}
	md := storage.ObjectMetadata{
		Key:          "file.txt",
		Size:         5,
		LastModified: time.Unix(1700000000, 0),
		ContentType:  "text/plain",
		StorageClass: "STANDARD",
		ETag:         "\"abc\"",
		UserMetadata: map[string]string{"owner": "alice"},
	}
	m = deliver(m, metadataMsg{gen: m.gen, md: md})

	if m.mode != modeObject {
		t.Fatalf("mode = %v, want metadata", m.mode)
	}
	v := viewOf(m)
	for _, want := range []string{"file.txt", "text/plain", "STANDARD", "owner", "alice"} {
		if !strings.Contains(v, want) {
			t.Errorf("metadata view missing %q:\n%s", want, v)
		}
	}

	// Back returns to the tree.
	m = press(m, "esc")
	if m.mode != modeTree {
		t.Errorf("esc should return to tree, mode = %v", m.mode)
	}
}

func TestMetadataAccessDenied(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "secret", storage.FakeObject{AccessDenied: true})
	m := enterTree(t, f, "b")

	m = press(m, "i")
	m = deliver(m, errMsg{gen: m.gen, err: storage.ErrAccessDenied})
	if !strings.Contains(m.errorText(), "Access denied") {
		t.Errorf("access-denied metadata should surface denied copy, got %q", m.errorText())
	}
}
