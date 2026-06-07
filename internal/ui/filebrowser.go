package ui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/localfs"
)

// openBrowser reads dir into the file-browser state and resets the selection. Local
// directory reads are fast, so this is done inline rather than via a tea.Cmd. On a
// read error the browser stays at the current directory and surfaces the error.
func (m *App) openBrowser(dir string) (tea.Model, tea.Cmd) {
	entries, err := localfs.ReadDir(dir)
	if err != nil {
		m.err = err
		return *m, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	m.fbDir = abs
	m.fbEntries = entries
	m.fbSel = 0
	return *m, nil
}

// onBrowseKey handles the local file browser (phaseBrowse). Navigation descends into
// directories, ascends to the parent, and selecting a file advances to confirmation.
func (m App) onBrowseKey(key string) (tea.Model, tea.Cmd) {
	n := len(m.fbEntries)
	switch {
	case matches(key, m.keys.Up):
		if m.fbSel > 0 {
			m.fbSel--
		}
	case matches(key, m.keys.Down):
		if m.fbSel < n-1 {
			m.fbSel++
		}
	case matches(key, m.keys.Top):
		m.fbSel = 0
	case matches(key, m.keys.Bottom):
		m.fbSel = max(0, n-1)
	case key == "esc":
		m.op = nil // cancel the whole upload intent
		return m, nil
	case matches(key, m.keys.Back), key == "backspace":
		return (&m).openBrowser(localfs.Parent(m.fbDir))
	case matches(key, m.keys.Enter):
		if m.fbSel < 0 || m.fbSel >= n {
			return m, nil
		}
		e := m.fbEntries[m.fbSel]
		if e.IsDir {
			return (&m).openBrowser(e.Path)
		}
		return m.chooseUploadFile(e)
	}
	return m, nil
}

// chooseUploadFile validates the picked file, resolves the target key, applies the
// advisory overwrite check, and advances to the confirmation phase.
func (m App) chooseUploadFile(e localfs.Entry) (tea.Model, tea.Cmd) {
	if err := localfs.IsReadableFile(e.Path); err != nil {
		m.err = err
		return m, nil
	}
	target := m.op.parent + e.Name
	m.op.localPath = e.Path
	m.op.localSize = e.Size
	m.op.target = target
	m.op.overwrite = m.levelHasKey(target)
	// 007 US4: an overwrite is the BINARY tier (a centered y/N popup, not a typed
	// identifier) — only container-removal types. The overwrite flag still drives the
	// loud confirmation text.
	m.op.tier = confirmSimple
	m.op.phase = phaseConfirm
	return m, nil
}

// fileBrowserView renders the local file browser table body (used as the box body
// while an upload is choosing its source).
func (m App) fileBrowserView(w, rows int) string {
	if len(m.fbEntries) == 0 {
		return emptyStyle.Render("Empty directory — Esc to cancel, ←/h to go up.")
	}
	off, end := windowBounds(len(m.fbEntries), m.fbSel, rows)
	cols := []column{{"name", 0}, {"type", 5}, {"size", 11}}
	data := make([][]string, 0, end-off)
	dirs := make([]bool, 0, end-off)
	for i := off; i < end; i++ {
		e := m.fbEntries[i]
		if e.IsDir {
			data = append(data, []string{sanitizeLabel(e.Name) + "/", "dir", "—"})
		} else {
			data = append(data, []string{sanitizeLabel(e.Name), "file", humanSize(e.Size)})
		}
		dirs = append(dirs, e.IsDir)
	}
	return renderTable(w, cols, data, dirs, m.fbSel-off)
}

// browserTitle is the box title while browsing for an upload source.
func (m App) browserTitle() string {
	return fmt.Sprintf("upload from %s [%d]", sanitizeLabel(m.fbDir), len(m.fbEntries))
}
