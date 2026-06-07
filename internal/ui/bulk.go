package ui

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// Bulk operations over a multi-select. Each item reuses the proven
// single-object backend call; the run continues past per-item failures and reports a
// truthful succeeded/failed summary. Bulk download is a READ (works RO); bulk
// delete/copy require the armed write state. Recursive folder deletion is NOT here.

// startBulkDownload pulls every marked object, recreating its key hierarchy as local
// subdirectories under the download dir. A read — no write needed.
func (m App) startBulkDownload() (tea.Model, tea.Cmd) {
	keys := m.selectedKeys()
	if len(keys) == 0 {
		return m, nil
	}
	m.err = nil
	m.op = &operation{kind: "bulk_download", bucket: m.bucket, parent: m.prefix, bulkKeys: keys}
	return m.dispatchBulk(m.op)
}

// startBulkDelete deletes every marked object behind a typed confirmation on the count
// ; requires write.
func (m App) startBulkDelete() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	keys := m.selectedKeys()
	if len(keys) == 0 {
		return m, nil
	}
	m.err = nil
	// Selected-group (bulk) delete is the BINARY tier: a centered y/N popup,
	// not a typed count. Only container-removal (recursive/bucket/connection) types.
	m.op = &operation{
		kind: "bulk_delete", bucket: m.bucket, parent: m.prefix, bulkKeys: keys,
		tier:   confirmSimple,
		target: fmt.Sprintf("%d objects", len(keys)), phase: phaseConfirm,
	}
	return m, nil
}

// startBulkCopy copies every marked object under a destination prefix; requires write.
func (m App) startBulkCopy() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	keys := m.selectedKeys()
	if len(keys) == 0 {
		return m, nil
	}
	m.err = nil
	// Leave the destination prefix empty (NOT prefilled with the current prefix) — a
	// prefill equal to the source level would copy every object onto itself
	// (CopyKey dst==src → ErrInvalidName). The engineer types the target prefix.
	m.op = &operation{
		kind: "bulk_copy", bucket: m.bucket, parent: m.prefix, bulkKeys: keys,
		phase: phaseDest,
	}
	return m, nil
}

// dispatchBulk runs the bulk action off the event loop, streaming per-item progress and
// a truthful summary (Constitution II, FR-018).
func (m App) dispatchBulk(op *operation) (tea.Model, tea.Cmd) {
	op.phase = phaseRunning
	op.progress = opProgress{total: int64(len(op.bulkKeys))}
	logMutationStart(op.kind, "bucket", op.bucket, "count", len(op.bulkKeys), "context", m.ctxName)
	ctx := (&m).beginLoad()
	ch := make(chan progressEvent, 8)
	m.opCh = ch
	return m, tea.Batch(
		bulkCmd(ctx, m.activeStore(), op.kind, op.bucket, op.parent, op.dstKey, op.bulkKeys, m.downloadDir(), ch, m.gen),
		spinnerTick(),
	)
}

// bulkCmd applies the action to each key, continuing past failures, and reports counts.
func bulkCmd(ctx context.Context, st storage.Storage, kind, bucket, parent, dstPrefix string, keys []string, dlDir string, ch chan progressEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			mut, _ := st.(storage.Mutator)
			var done, failed int
			for _, key := range keys {
				if err := ctx.Err(); err != nil {
					ch <- progressEvent{done: true, err: err, summary: &storage.DeleteSummary{Deleted: done, Failed: failed}}
					return
				}
				var e error
				switch kind {
				case "bulk_download":
					e = downloadOne(ctx, st, bucket, key, parent, dlDir)
				case "bulk_delete":
					e = mut.RemoveObject(ctx, bucket, key)
				case "bulk_copy":
					e = mut.CopyKey(ctx, bucket, key, dstPrefix+strings.TrimPrefix(key, parent))
				}
				if e != nil {
					failed++
					slog.Warn("bulk.item", "kind", kind, "key", key, "outcome", "failed", "err", e)
				} else {
					done++
				}
				select {
				case ch <- progressEvent{progress: opProgress{deleted: done, failed: failed, total: int64(len(keys))}}:
				default:
				}
			}
			summary := storage.DeleteSummary{Deleted: done, Failed: failed}
			logMutationDone(kind, nil, "bucket", bucket, "done", done, "failed", failed)
			ch <- progressEvent{done: true, summary: &summary, partial: failed > 0}
		}()
		return waitForProgress(ch, gen)()
	}
}

// downloadOne streams one object to a local file mirroring its key path under parent
// , via a temp file + atomic rename.
func downloadOne(ctx context.Context, st storage.Storage, bucket, key, parent, dlDir string) error {
	rel := filepath.FromSlash(strings.TrimPrefix(key, parent))
	// An S3 key is arbitrary bytes and may contain "..": refuse anything that would
	// escape the download dir so a malicious key cannot overwrite a file outside it
	// (path traversal). filepath.IsLocal rejects "..", absolute, and (on Windows)
	// reserved paths.
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("refusing unsafe object key %q (would escape the download dir)", key)
	}
	dest := filepath.Join(dlDir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //nolint:gosec // user-chosen dir
		return err
	}
	rc, err := st.GetObject(ctx, bucket, key)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	partial := dest + ".partial"
	f, err := os.Create(partial) //nolint:gosec // user-chosen destination
	if err != nil {
		return err
	}
	_, cerr := io.Copy(f, rc)
	closeErr := f.Close()
	if cerr != nil || closeErr != nil {
		_ = os.Remove(partial)
		if cerr != nil {
			return cerr
		}
		return closeErr
	}
	return os.Rename(partial, dest)
}
