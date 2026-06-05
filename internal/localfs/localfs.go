// Package localfs is a small, UI-agnostic reader of the local filesystem used by
// the upload file browser. It is deliberately kept out of internal/ui so its logic
// is unit-testable without Bubble Tea (Constitution I), mirroring how internal/storage
// keeps S3 out of the UI.
package localfs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Entry is one row in the file browser: a local file or directory.
type Entry struct {
	Name  string // base name
	Path  string // absolute path
	IsDir bool
	Size  int64 // bytes (files only; 0 for directories)
}

// ReadDir lists dir's entries with directories first, then files, each group sorted
// alphabetically (case-insensitive). It returns a classifiable error when dir cannot
// be read. Entries whose stat fails are skipped rather than aborting the listing.
func ReadDir(dir string) ([]Entry, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", dir, err)
	}
	des, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", abs, err)
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		info, err := de.Info()
		if err != nil {
			continue // a racing unlink/permission issue on one entry — skip it
		}
		out = append(out, Entry{
			Name:  de.Name(),
			Path:  filepath.Join(abs, de.Name()),
			IsDir: de.IsDir(),
			Size:  info.Size(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir // directories first
		}
		return lowerLess(out[i].Name, out[j].Name)
	})
	return out, nil
}

// IsReadableFile returns nil if path is an existing, readable regular file, and a
// classifiable error otherwise (missing, a directory, or unreadable). Called before
// dispatching an upload so a bad source never reaches the backend.
func IsReadableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %q: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory, not a file", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %q: %w", path, err)
	}
	_ = f.Close()
	return nil
}

// Parent returns the parent directory of dir (its own value at the filesystem root).
func Parent(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return filepath.Dir(abs)
}

func lowerLess(a, b string) bool {
	la, lb := toLowerASCII(a), toLowerASCII(b)
	if la == lb {
		return a < b
	}
	return la < lb
}

func toLowerASCII(s string) string {
	bs := []byte(s)
	for i, c := range bs {
		if c >= 'A' && c <= 'Z' {
			bs[i] = c + ('a' - 'A')
		}
	}
	return string(bs)
}
