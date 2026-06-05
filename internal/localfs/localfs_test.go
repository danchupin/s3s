package localfs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadDirOrdering: directories come before files, each alphabetical
// (case-insensitive) — US2 file browser.
func TestReadDirOrdering(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "Zeta"))
	mustMkdir(t, filepath.Join(dir, "alpha"))
	mustWrite(t, filepath.Join(dir, "b.txt"), "bb")
	mustWrite(t, filepath.Join(dir, "A.txt"), "a")

	entries, err := ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	gotNames := make([]string, len(entries))
	for i, e := range entries {
		gotNames[i] = e.Name
	}
	want := []string{"alpha", "Zeta", "A.txt", "b.txt"}
	if len(gotNames) != len(want) {
		t.Fatalf("entries = %v, want %v", gotNames, want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Errorf("entry[%d] = %q, want %q (full=%v)", i, gotNames[i], want[i], gotNames)
		}
	}
	// File size is reported; dirs report dir flag.
	for _, e := range entries {
		if e.Name == "b.txt" {
			if e.IsDir || e.Size != 2 {
				t.Errorf("b.txt entry = %+v, want file size 2", e)
			}
		}
		if e.Name == "alpha" && !e.IsDir {
			t.Errorf("alpha must be a directory: %+v", e)
		}
	}
}

// TestReadDirError: an unreadable/nonexistent directory surfaces an error.
func TestReadDirError(t *testing.T) {
	if _, err := ReadDir(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Error("ReadDir of a missing directory must error")
	}
}

// TestIsReadableFile: passes a regular file; rejects a directory and a missing path
// (upload source validation, FR-015).
func TestIsReadableFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ok.txt")
	mustWrite(t, file, "data")

	if err := IsReadableFile(file); err != nil {
		t.Errorf("IsReadableFile(regular file) = %v, want nil", err)
	}
	if err := IsReadableFile(dir); err == nil {
		t.Error("IsReadableFile(directory) must error")
	}
	if err := IsReadableFile(filepath.Join(dir, "missing")); err == nil {
		t.Error("IsReadableFile(missing) must error")
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", p, err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", p, err)
	}
}
