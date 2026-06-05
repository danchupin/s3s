package ui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/cache"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// spinnerInterval is the spinner animation tick.
const spinnerInterval = 120 * time.Millisecond

// searchDebounce coalesces keystrokes into at most one in-flight request (FR-017a).
const searchDebounce = 300 * time.Millisecond

// logOpErr records a backend operation error to the file log (Constitution V).
// Cancellations are routine (superseded loads) and logged at debug, not error.
// Only the classified error and non-secret identifiers are logged (FR-021).
func logOpErr(op string, err error, attrs ...any) {
	if errors.Is(err, context.Canceled) {
		slog.Debug(op+" cancelled", attrs...)
		return
	}
	slog.Error(op+" failed", append(attrs, "err", err)...)
}

// loadBuckets fetches the bucket list off the event loop.
func loadBuckets(ctx context.Context, st storage.Storage, gen int) tea.Cmd {
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
// dispatch; this records mutation.done with the classified outcome (FR-008).
func createFolderCmd(ctx context.Context, mut storage.Mutator, bucket, prefix string, gen int) tea.Cmd {
	return func() tea.Msg {
		err := mut.CreateFolder(ctx, bucket, prefix)
		logMutationDone("create_folder", err, "bucket", bucket, "key", prefix)
		return operationDoneMsg{gen: gen, err: err}
	}
}

// logMutationStart records the intent of a mutation BEFORE execution (Constitution
// V). Only non-secret identifiers are logged (FR-008, SC-005).
func logMutationStart(action string, attrs ...any) {
	slog.Info("mutation.start", append([]any{"action", action}, attrs...)...)
}

// logMutationDone records the terminal outcome of a mutation. A context
// cancellation is an indeterminate outcome, never "ok" (FR-007).
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

// debounceSearch fires searchFireMsg after the debounce window (FR-017a).
func debounceSearch(searchGen int, term string) tea.Cmd {
	return tea.Tick(searchDebounce, func(time.Time) tea.Msg {
		return searchFireMsg{searchGen: searchGen, term: term}
	})
}
