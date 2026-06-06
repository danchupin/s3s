package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// --- US3: command bar (FR-016..FR-019) ---

func TestCommandBarOpensWithColon(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, ":")
	if m.mode != modeCommand {
		t.Fatalf("':' should open the command bar; mode=%v", m.mode)
	}
	if !strings.Contains(viewOf(m), ":") {
		t.Errorf("command bar should render in the footer:\n%s", viewOf(m))
	}
}

func TestCommandDispatchKnown(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := withBuckets(f, []string{"ctx", "other"}, func(string) (Backend, error) { return Backend{Store: f}, nil })
	m = press(m, ":")
	m = typeStr(m, "ctx") // alias for contexts
	m = press(m, "enter")
	if m.mode != modeContextSwitch {
		t.Fatalf("':ctx' should open the context switcher; mode=%v", m.mode)
	}
}

func TestCommandPrefixMatch(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, ":")
	m = typeStr(m, "hel") // unique prefix of "help"
	m = press(m, "enter")
	if m.mode != modeHelp {
		t.Fatalf("':hel' should resolve to help; mode=%v", m.mode)
	}
}

func TestCommandUnknownNotice(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, ":")
	m = typeStr(m, "zzz")
	m = press(m, "enter")
	if m.mode != modeBuckets {
		t.Fatalf("unknown command should return to the prior mode; mode=%v", m.mode)
	}
	if !strings.Contains(m.notice, "unknown command") {
		t.Errorf("unknown command should set a notice; got %q", m.notice)
	}
}

func TestCommandEscNoEffect(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b")
	m := withBuckets(f, []string{"ctx"}, nil)
	m = press(m, ":")
	m = typeStr(m, "quit")
	m = press(m, "esc")
	if m.mode != modeBuckets {
		t.Fatalf("esc should close the bar back to buckets; mode=%v", m.mode)
	}
	if m.cmdInput != "" {
		t.Errorf("esc should clear the command input; got %q", m.cmdInput)
	}
}

func TestCommandColonInertWhileSearching(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := treeApp(f, true)
	m = press(m, "/") // start a search
	if !m.searching {
		t.Fatalf("'/' should start a search")
	}
	m = press(m, ":") // ':' is text in the search, not a command-bar open
	if m.mode == modeCommand {
		t.Error("':' must not open the command bar while searching")
	}
}
