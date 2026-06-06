package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// TestDownloadCmdWritesFile: downloadCmd streams the full object to a byte-identical
// local file, emits progress, and leaves no ".partial" behind (005 FR-001/FR-003).
func TestDownloadCmdWritesFile(t *testing.T) {
	f := storage.NewFake()
	want := strings.Repeat("download-payload-", 4096) // multi-read
	f.SeedObject("b", "dir/data.bin", storage.FakeObject{Data: []byte(want)})

	dest := filepath.Join(t.TempDir(), "data.bin")
	ch := make(chan progressEvent, 8)
	msg := downloadCmd(context.Background(), f, "b", "dir/data.bin", dest, int64(len(want)), ch, 5)()

	sawProgress := false
	var done operationDoneMsg
	for {
		switch ev := msg.(type) {
		case operationProgressMsg:
			sawProgress = true
			msg = waitForProgress(ch, 5)()
		case operationDoneMsg:
			done = ev
			goto finished
		default:
			t.Fatalf("unexpected msg %#v", msg)
		}
	}
finished:
	if done.err != nil {
		t.Fatalf("download err: %v", done.err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != want {
		t.Errorf("downloaded bytes mismatch (len got=%d want=%d)", len(got), len(want))
	}
	if _, err := os.Stat(dest + ".partial"); !os.IsNotExist(err) {
		t.Error(".partial file should not remain after a successful download")
	}
	if !sawProgress {
		t.Error("expected at least one progress event")
	}
}

// TestDownloadCmdFailureNoFile: a backend error leaves no complete-looking file and no
// ".partial" (005 FR-006).
func TestDownloadCmdFailureNoFile(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "secret", storage.FakeObject{Data: []byte("x"), AccessDenied: true})

	dest := filepath.Join(t.TempDir(), "out.bin")
	ch := make(chan progressEvent, 8)
	msg := downloadCmd(context.Background(), f, "b", "secret", dest, 1, ch, 5)()
	done, ok := msg.(operationDoneMsg)
	if !ok || done.err == nil {
		t.Fatalf("expected a failed download, got %#v", msg)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("failed download must not leave the destination file")
	}
	if _, err := os.Stat(dest + ".partial"); !os.IsNotExist(err) {
		t.Error("failed download must not leave a .partial file")
	}
}

// TestDownloadCmdCancelledNoPartial: a cancelled context yields no destination/partial
// file (005 FR-004).
func TestDownloadCmdCancelledNoPartial(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "a", storage.FakeObject{Data: []byte("data")})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := filepath.Join(t.TempDir(), "a")
	ch := make(chan progressEvent, 8)
	msg := downloadCmd(ctx, f, "b", "a", dest, 4, ch, 5)()
	done, ok := msg.(operationDoneMsg)
	if !ok || done.err == nil {
		t.Fatalf("cancelled download should report an error, got %#v", msg)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("cancelled download must not leave a file")
	}
}

// TestDownloadReadOnly: download is offered and dispatches in a read-only context —
// no --write (005 FR-002).
func TestDownloadReadOnly(t *testing.T) {
	t.Setenv("S3S_DOWNLOAD_DIR", t.TempDir())
	f := storage.NewFake()
	f.SeedObject("b", "a.txt", storage.FakeObject{Data: []byte("hi")})
	m := treeApp(f, false) // read-only
	selectObject(&m, "a.txt")

	mm, cmd := m.startDownload()
	got := mm.(App)
	if got.op == nil || got.op.kind != "download" {
		t.Fatalf("download should start in a read-only context; op=%+v", got.op)
	}
	if cmd == nil {
		t.Error("download should dispatch a command")
	}
}

// TestDownloadOverwriteConfirm: an existing local file triggers a simple overwrite
// confirmation before writing (005 FR-005).
func TestDownloadOverwriteConfirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("S3S_DOWNLOAD_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := storage.NewFake()
	f.SeedObject("b", "a.txt", storage.FakeObject{Data: []byte("new")})
	m := treeApp(f, false)
	selectObject(&m, "a.txt")

	mm, _ := m.startDownload()
	got := mm.(App)
	if got.op == nil || got.op.phase != phaseConfirm || got.op.tier != confirmSimple || !got.op.overwrite {
		t.Fatalf("existing local file should require an overwrite confirm; op=%+v", got.op)
	}
}
