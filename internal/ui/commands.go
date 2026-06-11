package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/cache"
	"github.com/danchupin/s3s/internal/plugin"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// discoverCmd invokes one bucket-discovery plugin off the event loop. It runs
// under its own background lifetime (the plugin timeout bounds it) and reports
// under the discovery generation it was dispatched with, so a level load never
// cancels or strands a valid discovery result.
func discoverCmd(r plugin.Runner, d plugin.Decl, conn plugin.Connection, ctxName string, gen int) tea.Cmd {
	return func() tea.Msg {
		res := r.Invoke(context.Background(), d, plugin.Request{
			ContractVersion: plugin.ContractVersion,
			Capability:      plugin.BucketDiscovery,
			Connection:      conn,
		})
		return discoveryDoneMsg{gen: gen, ctx: ctxName, plugin: d.Name, res: res}
	}
}

// enrichCmd invokes one object-metadata plugin off the event loop for a single
// object target, reporting under the enrichment generation.
func enrichCmd(r plugin.Runner, d plugin.Decl, conn plugin.Connection, ctxName, bucket, key string, gen int) tea.Cmd {
	return func() tea.Msg {
		res := r.Invoke(context.Background(), d, plugin.Request{
			ContractVersion: plugin.ContractVersion,
			Capability:      plugin.ObjectMetadata,
			Connection:      conn,
			Target:          &plugin.Target{Bucket: bucket, Key: key},
		})
		return enrichDoneMsg{gen: gen, ctx: ctxName, plugin: d.Name, bucket: bucket, key: key, res: res}
	}
}

// spinnerInterval is the spinner animation tick.
const spinnerInterval = 120 * time.Millisecond

// searchDebounce coalesces keystrokes into at most one in-flight request.
const searchDebounce = 300 * time.Millisecond

// logOpErr records a backend operation error to the file log (Constitution V).
// Cancellations are routine (superseded loads) and logged at debug, not error.
// Only the classified error and non-secret identifiers are logged.
func logOpErr(op string, err error, attrs ...any) {
	if errors.Is(err, context.Canceled) {
		slog.Debug(op+" cancelled", attrs...)
		return
	}
	slog.Error(op+" failed", append(attrs, "err", err)...)
}

// loadBuckets fetches the bucket list off the event loop. When pinned is non-empty the
// connection is "scoped" (bucket-scoped credentials that cannot list all buckets, 010): the
// list is synthesized from the pinned names with NO ListBuckets call. Otherwise it lists as
// before. The synthesized buckets carry only a name (zero CreationDate → rendered as "—").
func loadBuckets(ctx context.Context, st storage.Storage, gen int, pinned []string) tea.Cmd {
	if len(pinned) > 0 {
		buckets := make([]storage.Bucket, 0, len(pinned))
		for _, name := range pinned {
			buckets = append(buckets, storage.Bucket{Name: name})
		}
		return func() tea.Msg {
			slog.Debug("pinned buckets", "count", len(buckets))
			return bucketsMsg{gen: gen, buckets: buckets}
		}
	}
	return func() tea.Msg {
		bs, err := st.ListBuckets(ctx)
		if err != nil {
			logOpErr("list buckets", err)
			return errMsg{gen: gen, err: err}
		}
		slog.Debug("list buckets", "count", len(bs))
		return bucketsMsg{gen: gen, buckets: bs}
	}
}

// loadLevel fetches one page of a tree level off the event loop.
func loadLevel(ctx context.Context, st storage.Storage, key cache.Key, q storage.LevelQuery, gen int) tea.Cmd {
	return func() tea.Msg {
		page, err := st.ListLevel(ctx, q)
		if err != nil {
			logOpErr("list level", err, "bucket", q.Bucket, "prefix", q.Prefix, "search", q.Search)
			return errMsg{gen: gen, err: err}
		}
		slog.Debug("list level", "bucket", q.Bucket, "prefix", q.Prefix,
			"dirs", len(page.Dirs), "objects", len(page.Objects))
		return levelMsg{gen: gen, key: key, page: page}
	}
}

// loadMetadata fetches object metadata off the event loop.
func loadMetadata(ctx context.Context, st storage.Storage, bucket, key string, gen int) tea.Cmd {
	return func() tea.Msg {
		md, err := st.HeadObject(ctx, bucket, key)
		if err != nil {
			logOpErr("head object", err, "bucket", bucket, "key", key)
			return errMsg{gen: gen, err: err}
		}
		slog.Debug("head object", "bucket", bucket, "key", key, "size", md.Size)
		return metadataMsg{gen: gen, md: md}
	}
}

// loadPreview fetches at most preview.Limit bytes and classifies the content.
func loadPreview(ctx context.Context, st storage.Storage, bucket, key string, contentType string, size int64, gen int) tea.Cmd {
	return func() tea.Msg {
		rc, err := st.GetObjectRange(ctx, bucket, key, 0, preview.Limit-1)
		if err != nil {
			logOpErr("get object range", err, "bucket", bucket, "key", key)
			return errMsg{gen: gen, err: err}
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(io.LimitReader(rc, preview.Limit))
		if err != nil {
			logOpErr("read preview", err, "bucket", bucket, "key", key)
			return errMsg{gen: gen, err: err}
		}
		truncated := size > preview.Limit
		slog.Debug("preview", "bucket", bucket, "key", key, "bytes", len(data), "truncated", truncated)
		return previewMsg{gen: gen, payload: preview.Build(key, contentType, data, truncated)}
	}
}

// createFolderCmd creates an empty folder off the event loop, logging the outcome
// (Constitution V). The mutation.start record is emitted by the caller before
// dispatch; this records mutation.done with the classified outcome.
func createFolderCmd(ctx context.Context, mut storage.Mutator, bucket, prefix string, gen int) tea.Cmd {
	return func() tea.Msg {
		err := mut.CreateFolder(ctx, bucket, prefix)
		logMutationDone("create_folder", err, "bucket", bucket, "key", prefix)
		return operationDoneMsg{gen: gen, err: err}
	}
}

// removeObjectCmd deletes a single object. A not-found result is benign (the object
// was already gone) and reported as a clean done so the refresh shows it absent
func removeObjectCmd(ctx context.Context, mut storage.Mutator, bucket, key string, gen int) tea.Cmd {
	return func() tea.Msg {
		err := mut.RemoveObject(ctx, bucket, key)
		logMutationDone("delete_object", err, "bucket", bucket, "key", key)
		if errors.Is(err, storage.ErrNotFound) {
			return operationDoneMsg{gen: gen}
		}
		return operationDoneMsg{gen: gen, err: err}
	}
}

// removeBucketCmd deletes a whole (empty) bucket. A non-empty bucket
// surfaces ErrBucketNotEmpty as the outcome so the UI shows the "purge first" notice.
func removeBucketCmd(ctx context.Context, mut storage.Mutator, bucket string, gen int) tea.Cmd {
	return func() tea.Msg {
		err := mut.RemoveBucket(ctx, bucket)
		logMutationDone("delete_bucket", err, "bucket", bucket)
		return operationDoneMsg{gen: gen, err: err}
	}
}

// copyKeyCmd server-side copies an object to a new key.
func copyKeyCmd(ctx context.Context, mut storage.Mutator, bucket, srcKey, dstKey string, gen int) tea.Cmd {
	return func() tea.Msg {
		err := mut.CopyKey(ctx, bucket, srcKey, dstKey)
		logMutationDone("copy", err, "bucket", bucket, "src", srcKey, "dst", dstKey)
		return operationDoneMsg{gen: gen, err: err}
	}
}

// moveObjectCmd moves/renames an object (copy then delete source). ErrMovePartial is
// surfaced as a partial outcome — never a clean success.
func moveObjectCmd(ctx context.Context, mut storage.Mutator, bucket, srcKey, dstKey string, gen int) tea.Cmd {
	return func() tea.Msg {
		err := mut.MoveObject(ctx, bucket, srcKey, dstKey)
		logMutationDone("move", err, "bucket", bucket, "src", srcKey, "dst", dstKey)
		return operationDoneMsg{gen: gen, err: err}
	}
}

// progressEvent is one item on a streaming operation's progress channel. done marks
// the terminal event carrying the final error/summary/partial flag.
type progressEvent struct {
	progress opProgress
	done     bool
	err      error
	summary  *storage.DeleteSummary
	partial  bool
}

// waitForProgress reads ONE event off the channel and turns it into a message,
// re-issuing itself (via the Update handler) until the terminal done event arrives.
// This keeps the event loop non-blocking during long operations (Constitution II).
func waitForProgress(ch chan progressEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok || ev.done {
			return operationDoneMsg{gen: gen, err: ev.err, summary: ev.summary, partial: ev.partial}
		}
		return operationProgressMsg{gen: gen, progress: ev.progress}
	}
}

// countingReader wraps the upload source and reports bytes read onto ch (best-effort,
// non-blocking so a fast read never stalls on the UI). It feeds the upload's live
// byte progress.
//
// It MUST stay seekable: SigV4 signs the upload by reading the body to compute the
// x-amz-content-sha256 hash, then Seek(0)s and reads again to send. Hiding the
// underlying file's Seek makes the SDK send bytes that don't match the declared hash
// (XAmzContentSHA256Mismatch on Ceph RGW). Seek delegates to the file and resets the
// counter on a rewind-to-start so progress tracks the actual send pass.
type countingReader struct {
	rs    io.ReadSeeker
	read  int64
	total int64
	ch    chan progressEvent
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.rs.Read(p)
	if n > 0 {
		c.read += int64(n)
		select {
		case c.ch <- progressEvent{progress: opProgress{uploaded: c.read, total: c.total}}:
		default: // drop a tick rather than block the upload
		}
	}
	return n, err
}

func (c *countingReader) Seek(offset int64, whence int) (int64, error) {
	pos, err := c.rs.Seek(offset, whence)
	if err == nil && pos == 0 {
		c.read = 0 // SDK rewound before the send pass — count the send, not the hash pass
	}
	return pos, err
}

// uploadCmd opens the local file and streams it to the backend off the event loop,
// emitting byte progress and a terminal outcome on ch. A bad source
// (missing/unreadable) fails before any backend call.
func uploadCmd(ctx context.Context, mut storage.Mutator, bucket, key, localPath string, size int64, ch chan progressEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			f, err := os.Open(localPath)
			if err != nil {
				logMutationDone("upload", err, "bucket", bucket, "key", key)
				ch <- progressEvent{done: true, err: err}
				return
			}
			defer func() { _ = f.Close() }()
			cr := &countingReader{rs: f, total: size, ch: ch}
			err = mut.UploadFile(ctx, bucket, key, cr, size)
			logMutationDone("upload", err, "bucket", bucket, "key", key)
			ch <- progressEvent{done: true, err: err}
		}()
		return waitForProgress(ch, gen)()
	}
}

// recursiveDeleteCmd deletes a prefix subtree best-effort off the event loop,
// streaming deleted/failed progress and a terminal partial-aware outcome on ch
func recursiveDeleteCmd(ctx context.Context, mut storage.Mutator, bucket, prefix string, ch chan progressEvent, gen int) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			onProg := func(s storage.DeleteSummary) {
				select {
				case ch <- progressEvent{progress: opProgress{deleted: s.Deleted, failed: s.Failed}}:
				default:
				}
			}
			sum, err := mut.DeleteRecursive(ctx, bucket, prefix, onProg)
			summary := sum
			logMutationDone("delete_recursive", err, "bucket", bucket, "prefix", prefix,
				"deleted", sum.Deleted, "failed", sum.Failed)
			ch <- progressEvent{done: true, err: err, summary: &summary, partial: err == nil && sum.Failed > 0}
		}()
		return waitForProgress(ch, gen)()
	}
}

// logMutationStart records the intent of a mutation BEFORE execution (Constitution
// V). Only non-secret identifiers are logged.
func logMutationStart(action string, attrs ...any) {
	slog.Info("mutation.start", append([]any{"action", action}, attrs...)...)
}

// logMutationDone records the terminal outcome of a mutation. A context
// cancellation is an indeterminate outcome, never "ok".
func logMutationDone(action string, err error, attrs ...any) {
	base := append([]any{"action", action}, attrs...)
	switch {
	case err == nil:
		slog.Info("mutation.done", append(base, "outcome", "ok")...)
	case errors.Is(err, context.Canceled):
		slog.Info("mutation.done", append(base, "outcome", "cancelled")...)
	default:
		slog.Error("mutation.done", append(base, "outcome", "failed", "err", err)...)
	}
}

// spinnerTick schedules the next spinner frame.
func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return spinnerTickMsg{} })
}

// debounceSearch fires searchFireMsg after the debounce window.
func debounceSearch(searchGen int, term string) tea.Cmd {
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg {
		return searchFireMsg{searchGen: searchGen, term: term}
	})
}

// paneDebounce coalesces fast selection movement: the details-pane fetch fires only
// after the selection settles, so scrolling never triggers a per-row backend call
const paneDebounce = 180 * time.Millisecond

// paneTickMsg fires after the pane debounce window; it carries the pane generation +
// key it was scheduled for, so a tick for a row the user has scrolled past is ignored.
type paneTickMsg struct {
	gen int
	key string
}

// paneTickCmd schedules a debounced pane load for (gen, key).
func paneTickCmd(gen int, key string) tea.Cmd {
	return tea.Tick(paneDebounce, func(time.Time) tea.Msg {
		return paneTickMsg{gen: gen, key: key}
	})
}

// bucketTickMsg fires after the bucket-scroll debounce window. It carries
// the bucket-load generation + the bucket name it was scheduled for, so a tick for a bucket
// the cursor has scrolled past is ignored (mirrors paneTickMsg).
type bucketTickMsg struct {
	gen    int
	bucket string
}

// bucketTickCmd schedules a debounced objects-zone load for (gen, bucket) — the highlighted
// bucket's first level is fetched only once the bucket selection settles.
func bucketTickCmd(gen int, bucket string) tea.Cmd {
	return tea.Tick(paneDebounce, func(time.Time) tea.Msg {
		return bucketTickMsg{gen: gen, bucket: bucket}
	})
}

// usageTickCmd schedules a debounced usage scan for (gen, bucket, prefix): the focused
// target's UsageOf scan starts only once the selection settles, so rapid transit through a
// list spawns no scan (016 US2 FR-005). Reuses the pane debounce window.
func usageTickCmd(gen int, bucket, prefix string) tea.Cmd {
	return tea.Tick(paneDebounce, func(time.Time) tea.Msg {
		return usageTickMsg{gen: gen, bucket: bucket, prefix: prefix}
	})
}

// loadObjectTags fetches an object's tag values off the event loop (016 US4/FR-011).
func loadObjectTags(ctx context.Context, st storage.Storage, bucket, key string, gen int) tea.Cmd {
	return func() tea.Msg {
		ot, err := st.GetObjectTagging(ctx, bucket, key)
		if err != nil {
			logOpErr("get object tagging", err, "bucket", bucket, "key", key)
		}
		return objectTagsMsg{gen: gen, key: key, tags: ot, err: err}
	}
}

// loadBucketConfig fetches a bucket's governance sub-resources off the event loop
// (016 US4/FR-012). Each sub-resource is classified independently inside the storage layer.
func loadBucketConfig(ctx context.Context, st storage.Storage, bucket string, gen int) tea.Cmd {
	return func() tea.Msg {
		cfg, err := st.GetBucketConfiguration(ctx, bucket)
		if err != nil {
			logOpErr("get bucket configuration", err, "bucket", bucket)
		}
		return bucketConfigMsg{gen: gen, bucket: bucket, cfg: cfg, err: err}
	}
}

// loadPaneMeta fetches object metadata for the details pane off the event loop. Unlike
// loadMetadata it emits paneMetaMsg (which does NOT flip modeObject) under the pane gen.
func loadPaneMeta(ctx context.Context, st storage.Storage, bucket, key string, gen int) tea.Cmd {
	return func() tea.Msg {
		md, err := st.HeadObject(ctx, bucket, key)
		if err != nil {
			logOpErr("pane head object", err, "bucket", bucket, "key", key)
			return nil // pane errors are non-fatal — leave the instant fields shown
		}
		return paneMetaMsg{gen: gen, key: key, md: md}
	}
}

// loadPanePreview fetches a bounded preview for the details pane off the event loop,
// emitting panePreviewMsg under the pane gen.
func loadPanePreview(ctx context.Context, st storage.Storage, bucket, key string, size int64, gen int) tea.Cmd {
	return func() tea.Msg {
		rc, err := st.GetObjectRange(ctx, bucket, key, 0, preview.Limit-1)
		if err != nil {
			logOpErr("pane preview", err, "bucket", bucket, "key", key)
			return nil
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(io.LimitReader(rc, preview.Limit))
		if err != nil {
			return nil
		}
		return panePreviewMsg{gen: gen, key: key, payload: preview.Build(key, "", data, size > preview.Limit)}
	}
}
