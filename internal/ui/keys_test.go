package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// helpApp builds an App with rich connection metadata for help-surface assertions.
func helpApp(writable bool) App {
	f := storage.NewFake()
	f.Seed("b")
	return New(Backend{
		Store: f, Cluster: "cl", User: "alice@example.com",
		Endpoint: "http://ep:9000", Region: "us-east-1", Writable: writable,
	}, "myctx", []string{"myctx"}, nil, nil, preview.ProtoNone)
}

func TestHelpCategoriesAndAliases(t *testing.T) { // H2/H3, obligation 1 (incl vim, FR-014c)
	h := helpApp(true).helpView()
	for _, sec := range []string{"Navigation", "Search & View", "Actions", "Context", "Global", "Connection"} {
		if !strings.Contains(h, sec) {
			t.Errorf("help missing section %q", sec)
		}
	}
	for _, alias := range []string{"↑/k", "↓/j", "Home", "End"} {
		if !strings.Contains(h, alias) {
			t.Errorf("help should list the vim/secondary alias %q alongside arrows", alias)
		}
	}
}

func TestHelpActionsSection(t *testing.T) { // obligation 1a (FR-014b)
	h := helpApp(true).helpView()
	if !strings.Contains(h, "single key on the selection") {
		t.Error("Actions section should document the menu-less direct keys")
	}
	for _, item := range []string{"download", "analyze", "create a folder", "delete the selected object", "upload a local file", "move/rename", "recursively delete"} {
		if !strings.Contains(h, item) {
			t.Errorf("Actions section missing item %q", item)
		}
	}
}

func TestHelpWriteCapabilityReflection(t *testing.T) { // H4, obligation 3 (FR-013)
	if w := helpApp(true).helpView(); !strings.Contains(w, "(write)") {
		t.Error("writable help should mark write items as (write)")
	}
	if ro := helpApp(false).helpView(); !strings.Contains(ro, "needs --write") {
		t.Error("read-only help should mark write items as needing --write")
	}
}

func TestHelpConnectionSection(t *testing.T) { // H5, obligation 2 (FR-014a)
	h := helpApp(true).helpView()
	for _, v := range []string{"http://ep:9000", "us-east-1", "alice@example.com", "cl", "myctx"} {
		if !strings.Contains(h, v) {
			t.Errorf("Connection section missing %q", v)
		}
	}
}

func TestHelpRedactionGuard(t *testing.T) { // obligation 2a (FR-021)
	// The help renderer sources only Backend display fields; credentials never reach
	// Backend, so no credential material can appear in the rendered help.
	h := helpApp(true).helpView()
	for _, secret := range []string{"SecretKey", "AKIA", "aws_secret"} {
		if strings.Contains(h, secret) {
			t.Errorf("help must not surface credential material; found %q", secret)
		}
	}
}

func TestHelpCloseHint(t *testing.T) { // H1, obligation 4 (FR-012)
	if !strings.Contains(helpApp(true).helpView(), "press any key to close") {
		t.Error("help must state how to close it")
	}
}
