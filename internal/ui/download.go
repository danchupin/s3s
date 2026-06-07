package ui

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Download streams a full object to a local file. It is a READ — usable in
// read-only contexts and against production: it never mutates the remote, only
// writes locally. The transfer goes to "<dest>.partial" and is atomically renamed on
// success; a cancel/failure removes the partial so a half-file never looks complete
// Reuses the operation/progress/cancel machinery.

// startDownload begins a download of the selected object. If a local file already
// exists at the destination it confirms an overwrite first; otherwise it
// dispatches immediately.
func (m App) startDownload() (tea.Model, tea.Cmd) {
	e := m.selected()
	if e == nil || e.isDir {
		return m, nil // download targets an object
	}
	m.err = nil
	dest := filepath.Join(m.downloadDir(), filepath.Base(e.full))
	var total int64
	if e.obj != nil {
		total = e.obj.Size
	}
	m.op = &operation{
		kind:      "download",
		bucket:    m.bucket,
		parent:    m.prefix,
		srcKey:    e.full,
		localPath: dest,
		localSize: total,
		target:    dest,
	}
	if _, err := os.Stat(dest); err == nil {
		m.op.tier = confirmSimple
		m.op.overwrite = true
		m.op.phase = phaseConfirm
		return m, nil
	}
	return m.dispatchDownload(m.op)
}

// downloadDir resolves the default local destination directory: the S3S_DOWNLOAD_DIR
// env var, else the config downloadDir, else the current working directory.
func (m App) downloadDir() string {
	if d := os.Getenv("S3S_DOWNLOAD_DIR"); d != "" {
		return d
	}
	if m.info.DownloadDir != "" {
		return m.info.DownloadDir
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// dispatchDownload runs the download off the event loop under a fresh generation
// (Constitution II). Download is a read, so it uses activeStore() (the guard passes
// reads through) and never requires write mode.
func (m App) dispatchDownload(op *operation) (tea.Model, tea.Cmd) {
	op.phase = phaseRunning
	op.progress = opProgress{total: op.localSize}
	slog.Info("download.start", "bucket", op.bucket, "key", op.srcKey, "dest", op.localPath, "context", m.ctxName)
	ctx := (&m).beginLoad()
	ch := make(chan progressEvent, 8)
	m.opCh = ch
	return m, tea.Batch(
		downloadCmd(ctx, m.activeStore(), op.bucket, op.srcKey, op.localPath, op.localSize, ch, m.gen),
		spinnerTick(),
	)
}

// countingDownloadWriter forwards bytes to the destination file and reports live byte
// progress on ch (best-effort, non-blocking). It reuses opProgress.uploaded as the
// transferred-bytes counter.
type countingDownloadWriter struct {
	w       io.Writer
	written int64
	total   int64
	ch      chan progressEvent
}

func (c *countingDownloadWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		c.written += int64(n)
		select {
		case c.ch <- progressEvent{progress: opProgress{uploaded: c.written, total: c.total}}:
		default: // drop a tick rather than stall the copy
		}
	}
	return n, err
}

// downloadCmd streams the object body to "<dest>.partial", renames to dest on success,
// and removes the partial on cancel/failure. A read — takes a
// storage.Storage, not a Mutator.
func downloadCmd(ctx context.Context, st storage.Storage, bucket, key, destPath string, total int64, ch chan progressEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			rc, err := st.GetObject(ctx, bucket, key)
			if err != nil {
				slog.Error("download.done", "key", key, "outcome", "failed", "err", err)
				ch <- progressEvent{done: true, err: err}
				return
			}
			defer func() { _ = rc.Close() }()

			partial := destPath + ".partial"
			f, ferr := os.Create(partial) //nolint:gosec // user-chosen local destination
			if ferr != nil {
				slog.Error("download.done", "key", key, "outcome", "failed", "err", ferr)
				ch <- progressEvent{done: true, err: ferr}
				return
			}
			cw := &countingDownloadWriter{w: f, total: total, ch: ch}
			_, cerr := io.Copy(cw, rc)
			closeErr := f.Close()
			if cerr != nil || closeErr != nil {
				_ = os.Remove(partial) // never leave a complete-looking partial file
				e := cerr
				if e == nil {
					e = closeErr
				}
				slog.Error("download.done", "key", key, "outcome", "failed", "err", e)
				ch <- progressEvent{done: true, err: e}
				return
			}
			if rerr := os.Rename(partial, destPath); rerr != nil {
				_ = os.Remove(partial)
				slog.Error("download.done", "key", key, "outcome", "failed", "err", rerr)
				ch <- progressEvent{done: true, err: rerr}
				return
			}
			slog.Info("download.done", "key", key, "dest", destPath, "outcome", "ok")
			ch <- progressEvent{done: true}
		}()
		return waitForProgress(ch, gen)()
	}
}
