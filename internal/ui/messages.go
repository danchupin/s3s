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
// is dropped. A non-nil err (including context.Canceled) means the
// operation is NOT a success. For recursive delete, summary holds the deleted/failed
// counts; partial==true marks a non-clean outcome (recursive Failed>0 or a partial
// move) that must never be reported as a clean success.
type operationDoneMsg struct {
	gen     int
	err     error
	summary *storage.DeleteSummary
	partial bool
}

// searchFireMsg fires after the search debounce window elapses; it carries the
// search generation so only the latest keystroke triggers a request.
type searchFireMsg struct {
	searchGen int
	term      string
}

// usageProgressMsg delivers one running-totals tick during a du scan.
type usageProgressMsg struct {
	gen int
	p   storage.UsageProgress
}

// usageDoneMsg delivers the terminal du report (or error). A cancelled scan carries a
// partial report with Complete=false.
type usageDoneMsg struct {
	gen    int
	report storage.UsageReport
	err    error
}

// paneMetaMsg / panePreviewMsg deliver the debounced details-pane loads.
// They carry the selection key + pane generation and DO NOT set modeObject (distinct
// from the Enter object view). A msg whose gen ≠ paneGen is dropped (scrolled past).
type paneMetaMsg struct {
	gen int
	key string
	md  storage.ObjectMetadata
}

type panePreviewMsg struct {
	gen     int
	key     string
	payload preview.Payload
}

// connTestedMsg / connSavedMsg deliver the off-loop connection test/save results
// connSavedMsg carries the updated context-name list on success.
type connTestedMsg struct{ err error }

type connSavedMsg struct {
	names []string
	err   error
}

// addBucketMsg delivers the off-loop "+ add bucket" persist result. Like
// connSavedMsg it carries no gen (a config mutation, not a load): the subsequent bucket-list
// reload it triggers carries the fresh gen. buckets is the connection's updated pinned list.
type addBucketMsg struct {
	buckets []string
	err     error
}

// connDeletedMsg delivers the off-loop connection-delete result. Carries the
// updated context-name list on success and the gen it was dispatched under.
type connDeletedMsg struct {
	gen   int
	name  string
	names []string
	err   error
}

// contextResolvedMsg delivers the result of resolving a context switch off the event
// loop. Credential resolution (keychain unlock / external command) can be slow, so it
// MUST NOT run synchronously in Update (Constitution II, 005 US6). Carries the gen it
// was dispatched under so a superseded switch is dropped.
type contextResolvedMsg struct {
	gen    int
	target string
	be     Backend
	err    error
}
