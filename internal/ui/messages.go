package ui

import (
	"github.com/danchupin/s3s/internal/cache"
	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// tea.Msg types carrying results of async storage calls back to the event loop.
// Every result carries the generation id it was dispatched under so the model can
// drop stale messages from superseded loads (Constitution II).

// bucketsMsg delivers a ListBuckets result.
type bucketsMsg struct {
	gen     int
	buckets []storage.Bucket
}

// levelMsg delivers one ListLevel page for a specific level key.
type levelMsg struct {
	gen  int
	key  cache.Key
	page storage.Page
}

// metadataMsg delivers a HeadObject result.
type metadataMsg struct {
	gen int
	md  storage.ObjectMetadata
}

// previewMsg delivers a classified, bounded preview payload.
type previewMsg struct {
	gen     int
	payload preview.Payload
}

// errMsg delivers a classified error for the in-flight operation.
type errMsg struct {
	gen int
	err error
}

// spinnerTickMsg advances the loading spinner frame.
type spinnerTickMsg struct{}

// operationProgressMsg delivers one tick of live progress for a streaming
// operation (upload bytes, or recursive-delete counts). Carries the generation so a
// superseded result is dropped (Constitution II / FR-010).
type operationProgressMsg struct {
	gen      int
	progress opProgress
}

// operationDoneMsg delivers the terminal outcome of a mutating operation. It
// carries the generation it was dispatched under so a superseded/cancelled result
// is dropped (FR-007). A non-nil err (including context.Canceled) means the
// operation is NOT a success. For recursive delete, summary holds the deleted/failed
// counts; partial==true marks a non-clean outcome (recursive Failed>0 or a partial
// move) that must never be reported as a clean success (FR-011).
type operationDoneMsg struct {
	gen     int
	err     error
	summary *storage.DeleteSummary
	partial bool
}

// searchFireMsg fires after the search debounce window elapses; it carries the
// search generation so only the latest keystroke triggers a request (FR-017a).
type searchFireMsg struct {
	searchGen int
	term      string
}
