package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/storage"
)

// opPhase is the interactive stage of a mutating operation.
type opPhase int

const (
	phaseName    opPhase = iota // typing the folder name (create-folder only)
	phaseConfirm                // confirmation overlay (simple or typed tier)
	phaseRunning                // dispatched; awaiting progress/outcome
	phaseBrowse                 // local file browser open (upload source)
	phaseDest                   // destination-key entry (copy / move)
)

// confirmTier is the strength of confirmation an operation requires (FR-005).
type confirmTier int

const (
	confirmSimple confirmTier = iota // reversible: y/Enter
	confirmTyped                     // destructive: type the exact target
)

// opProgress is the live progress of a streaming operation, rendered during
// phaseRunning. Upload uses uploaded/total (bytes); recursive delete uses
// deleted/failed (object counts).
type opProgress struct {
	uploaded int64
	total    int64
	deleted  int
	failed   int
}

// operation is the in-flight mutating intent. kind selects the backend action and
// the interactive flow; the confirm tier is simple for reversible actions and typed
// for destructive ones (delete, move, recursive delete) or a detected overwrite.
type operation struct {
	kind      string // create_folder | delete_object | upload | copy | move | delete_recursive
	bucket    string
	parent    string // parent prefix of the level the op started in
	name      string // typed folder name (create-folder name phase)
	srcKey    string // source object key (delete/copy/move)
	dstKey    string // destination key buffer (copy/move dest entry)
	prefix    string // target prefix (recursive delete)
	localPath string // chosen upload source path
	localSize int64  // upload source size (progress total)
	target    string // resolved identifier acted on; for confirm + logging
	tier      confirmTier
	expect    string    // typed tier: the exact string the operator must type
	input     textField // typed tier: the operator's entry (caret + paste, 008 US3)
	overwrite bool      // target already exists in the loaded level (advisory) — clobber warning
	bulkKeys  []string  // marked object keys for a bulk_* operation (005 US3)
	phase     opPhase
	progress  opProgress
	ticks     int // spinner ticks elapsed while phaseRunning — gates the progress bar (007 FR-035)
}

// UI-local hint sentinels (rendered as secret-free status lines).
var (
	errFolderExists    = errors.New("ui: folder already exists")
	errConfirmMismatch = errors.New("ui: confirmation did not match")
)

// startCreateFolder begins a create-folder intent at the current tree level. On a
// read-only context it issues no command and shows the read-only hint (FR-003).
func (m App) startCreateFolder() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	m.err = nil
	m.op = &operation{
		kind:   "create_folder",
		bucket: m.bucket,
		parent: m.prefix,
		tier:   confirmSimple,
		phase:  phaseName,
	}
	return m, nil
}

// startRemoveObject begins a single-object delete (typed tier — destructive). Only
// valid on an object selection; refused on a read-only context (FR-001/FR-012).
func (m App) startRemoveObject() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	e := m.selected()
	if e == nil || e.isDir {
		return m, nil // delete targets an object; a folder uses recursive delete (D)
	}
	m.err = nil
	// Single-object delete is the BINARY tier (007 FR-024): a centered y/N popup, no
	// typed identifier (only container-removal — recursive/bucket/connection — types).
	m.op = &operation{
		kind:   "delete_object",
		bucket: m.bucket,
		parent: m.prefix,
		srcKey: e.full,
		target: e.full,
		tier:   confirmSimple,
		phase:  phaseConfirm,
	}
	return m, nil
}

// startRemoveBucket begins a whole-bucket delete (typed tier — type the exact bucket
// name) on the selected bucket. Only valid in the bucket list; refused read-only. The
// backend requires the bucket be EMPTY (007 FR-024b).
func (m App) startRemoveBucket() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	fb := m.filteredBuckets()
	if m.bucketSel < 0 || m.bucketSel >= len(fb) {
		return m, nil
	}
	name := fb[m.bucketSel].Name
	m.err = nil
	m.op = &operation{
		kind:   "delete_bucket",
		bucket: name,
		target: name,
		tier:   confirmTyped,
		expect: name,
		phase:  phaseConfirm,
	}
	return m, nil
}

// startUpload opens the local file browser to pick an upload source for the current
// level. Refused on a read-only context (FR-002/FR-012).
func (m App) startUpload() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	m.err = nil
	m.op = &operation{
		kind:   "upload",
		bucket: m.bucket,
		parent: m.prefix,
		phase:  phaseBrowse,
	}
	return (&m).openBrowser(".")
}

// startCopy / startMove begin a destination-key entry for the selected object.
// Copy is reversible (simple confirm unless overwrite); move removes the source and
// is always typed (FR-004/FR-006).
func (m App) startCopy() (tea.Model, tea.Cmd) { return m.startCopyMove("copy") }
func (m App) startMove() (tea.Model, tea.Cmd) { return m.startCopyMove("move") }
func (m App) startCopyMove(kind string) (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	e := m.selected()
	if e == nil || e.isDir {
		return m, nil
	}
	m.err = nil
	m.op = &operation{
		kind:   kind,
		bucket: m.bucket,
		parent: m.prefix,
		srcKey: e.full,
		dstKey: e.full, // prefill with the source key so rename only edits the tail
		phase:  phaseDest,
	}
	return m, nil
}

// startRecursiveDelete begins a recursive prefix delete (typed tier — highest risk).
// Only valid on a folder/common-prefix selection (FR-008/FR-012).
func (m App) startRecursiveDelete() (tea.Model, tea.Cmd) {
	if !m.writable() {
		m.err = storage.ErrReadOnly
		return m, nil
	}
	e := m.selected()
	if e == nil || !e.isDir {
		return m, nil // recursive delete targets a folder; single objects use d
	}
	m.err = nil
	m.op = &operation{
		kind:   "delete_recursive",
		bucket: m.bucket,
		parent: m.prefix,
		prefix: e.full,
		target: e.full,
		tier:   confirmTyped,
		expect: e.full,
		phase:  phaseConfirm,
	}
	return m, nil
}

// onOpKey routes a keypress while an interactive operation is active. phaseRunning
// is handled by the global cancel path, not here.
func (m App) onOpKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.op.phase {
	case phaseName:
		return m.onNameKey(key, msg)
	case phaseConfirm:
		return m.onConfirmKey(key, msg)
	case phaseBrowse:
		return m.onBrowseKey(key)
	case phaseDest:
		return m.onDestKey(key, msg)
	}
	return m, nil
}

// onNameKey edits the folder name. Only Esc cancels and Enter submits; every other
// printable key is text.
func (m App) onNameKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.op = nil
		return m, nil
	case "enter":
		return m.submitName()
	case "backspace":
		if n := len(m.op.name); n > 0 {
			m.op.name = m.op.name[:n-1]
		}
		return m, nil
	default:
		if msg.Text != "" {
			m.op.name += msg.Text
		}
		return m, nil
	}
}

// submitName validates the typed folder name and advances to the simple confirm, or
// surfaces a hint. Collision is an advisory check against the loaded level (FR-010).
func (m App) submitName() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.op.name)
	if name == "" {
		m.err = storage.ErrInvalidName
		return m, nil // stay in the name phase
	}
	target := m.op.parent + strings.TrimRight(name, "/")
	if m.levelHasDir(target + "/") {
		m.err = errFolderExists
		m.op = nil
		return m, nil
	}
	m.op.name = name
	m.op.target = target
	m.op.tier = confirmSimple
	m.op.phase = phaseConfirm
	return m, nil
}

// onDestKey edits the destination key for copy/move. Enter validates and advances to
// confirm (escalating to the typed overwrite tier when the destination already
// exists in the loaded level); Esc cancels.
func (m App) onDestKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.op = nil
		return m, nil
	case "enter":
		return m.submitDest()
	case "backspace":
		if n := len(m.op.dstKey); n > 0 {
			m.op.dstKey = m.op.dstKey[:n-1]
		}
		return m, nil
	default:
		if msg.Text != "" {
			m.op.dstKey += msg.Text
		}
		return m, nil
	}
}

// submitDest validates the destination key (non-empty, no control chars, != source)
// and routes to the confirmation phase with the correct tier (FR-013).
func (m App) submitDest() (tea.Model, tea.Cmd) {
	dst := m.op.dstKey
	if strings.TrimSpace(dst) == "" || dst == m.op.srcKey || hasControl(dst) {
		m.err = storage.ErrInvalidName
		return m, nil // stay in the dest phase
	}
	// Bulk copy: dst is a destination PREFIX (normalized to a trailing "/"); copy is
	// reversible, so dispatch directly without a confirmation (005 US3).
	if m.op.kind == "bulk_copy" {
		if !strings.HasSuffix(dst, "/") {
			dst += "/"
		}
		m.op.dstKey = dst
		return m.dispatchOp()
	}
	m.op.target = dst
	m.op.overwrite = m.levelHasKey(dst)
	// Move and overwrite are now the BINARY tier (007 FR-024): a centered y/N popup, not
	// a typed identifier. Only container-removal (recursive/bucket/connection) types.
	m.op.tier = confirmSimple
	m.op.phase = phaseConfirm
	return m, nil
}

// levelHasDir reports whether the loaded level already lists the given directory.
func (m App) levelHasDir(dir string) bool {
	if m.level == nil {
		return false
	}
	for _, d := range m.level.dirs {
		if d == dir {
			return true
		}
	}
	return false
}

// levelHasKey reports whether the loaded level already lists the given object key
// (advisory overwrite detection — best-effort, from the current listing; FR-003/005).
func (m App) levelHasKey(key string) bool {
	if m.level == nil {
		return false
	}
	for i := range m.level.objects {
		if m.level.objects[i].Key == key {
			return true
		}
	}
	return false
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// dispatchOp logs the intent and runs the mutation off the event loop under a fresh
// generation (FR-006/FR-008/FR-014). Streaming ops (upload, recursive delete) set up
// a progress channel consumed by a waitForProgress command.
func (m App) dispatchOp() (tea.Model, tea.Cmd) {
	op := m.op
	// Download and bulk-download are READS (US1/US3) — no Mutator, work read-only.
	if op.kind == "download" {
		return m.dispatchDownload(op)
	}
	if op.kind == "bulk_download" || op.kind == "bulk_delete" || op.kind == "bulk_copy" {
		return m.dispatchBulk(op)
	}
	// Connection delete is a CONFIG mutation via the injected Connector, not an S3 call —
	// it does not touch the storage Mutator (007 US5).
	if op.kind == "delete_connection" {
		if m.connect == nil {
			m.op = nil
			m.notice = "in-app connections unavailable"
			return m, nil
		}
		name := op.target
		op.phase = phaseRunning
		(&m).beginLoad() // bump gen; the live config write logs the delete (Constitution V)
		// A single cmd (not a Batch): the delete is near-instant and the result clears the
		// op. Batching a spinner here would emit a BatchMsg the linear drain can't follow.
		return m, connDeleteCmd(m.connect, name, m.gen)
	}
	mut, ok := m.activeStore().(storage.Mutator)
	if !ok {
		m.op = nil
		m.err = storage.ErrReadOnly
		return m, nil
	}
	op.phase = phaseRunning
	op.progress = opProgress{total: op.localSize}
	switch op.kind {
	case "create_folder":
		logMutationStart(op.kind, "bucket", op.bucket, "key", op.target, "context", m.ctxName)
		ctx := (&m).beginLoad()
		return m, tea.Batch(createFolderCmd(ctx, mut, op.bucket, op.target, m.gen), spinnerTick())
	case "delete_object":
		logMutationStart(op.kind, "bucket", op.bucket, "key", op.srcKey, "context", m.ctxName)
		ctx := (&m).beginLoad()
		return m, tea.Batch(removeObjectCmd(ctx, mut, op.bucket, op.srcKey, m.gen), spinnerTick())
	case "copy":
		logMutationStart(op.kind, "bucket", op.bucket, "src", op.srcKey, "dst", op.target, "context", m.ctxName)
		ctx := (&m).beginLoad()
		return m, tea.Batch(copyKeyCmd(ctx, mut, op.bucket, op.srcKey, op.target, m.gen), spinnerTick())
	case "move":
		logMutationStart(op.kind, "bucket", op.bucket, "src", op.srcKey, "dst", op.target, "context", m.ctxName)
		ctx := (&m).beginLoad()
		return m, tea.Batch(moveObjectCmd(ctx, mut, op.bucket, op.srcKey, op.target, m.gen), spinnerTick())
	case "upload":
		logMutationStart(op.kind, "bucket", op.bucket, "key", op.target, "src", op.localPath, "context", m.ctxName)
		ctx := (&m).beginLoad()
		ch := make(chan progressEvent, 8)
		m.opCh = ch
		return m, tea.Batch(uploadCmd(ctx, mut, op.bucket, op.target, op.localPath, op.localSize, ch, m.gen), spinnerTick())
	case "delete_recursive":
		logMutationStart(op.kind, "bucket", op.bucket, "prefix", op.prefix, "context", m.ctxName)
		ctx := (&m).beginLoad()
		ch := make(chan progressEvent, 8)
		m.opCh = ch
		return m, tea.Batch(recursiveDeleteCmd(ctx, mut, op.bucket, op.prefix, ch, m.gen), spinnerTick())
	case "delete_bucket":
		logMutationStart(op.kind, "bucket", op.bucket, "context", m.ctxName)
		ctx := (&m).beginLoad()
		return m, tea.Batch(removeBucketCmd(ctx, mut, op.bucket, m.gen), spinnerTick())
	default:
		// Test-only kinds: complete immediately as success.
		(&m).beginLoad()
		gen := m.gen
		return m, func() tea.Msg { return operationDoneMsg{gen: gen} }
	}
}

// onOperationProgress stores one progress tick and re-arms the wait command.
func (m App) onOperationProgress(msg operationProgressMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen || m.op == nil {
		return m, nil // superseded — drop
	}
	m.op.progress = msg.progress
	return m, waitForProgress(m.opCh, m.gen)
}

// onOperationDone applies the terminal outcome (FR-007/FR-011/FR-016).
func (m App) onOperationDone(msg operationDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil // superseded — drop
	}
	// Download is a read: it changes nothing remote, so it reports a notice and skips
	// the post-mutation refresh (005 US1).
	isDownload := m.op != nil && m.op.kind == "download"
	dlDest := ""
	if isDownload {
		dlDest = m.op.localPath
	}
	isBulk := m.op != nil && strings.HasPrefix(m.op.kind, "bulk_")
	bulkVerb := ""
	if isBulk {
		bulkVerb = strings.TrimPrefix(m.op.kind, "bulk_")
	}
	isBucket := m.op != nil && m.op.kind == "delete_bucket"
	bucketName := ""
	if isBucket {
		bucketName = m.op.bucket
	}
	// US6 (FR-015/FR-016): a copy/move/bulk-copy whose destination prefix differs from the
	// current view leaves that level cached-stale. Precisely invalidate the SOURCE and
	// DESTINATION prefix keys (same bucket — copy/move are single-bucket) so a later
	// navigation shows fresh contents. Same-level mutations are covered by refresh() below.
	if m.op != nil {
		// A partial move still copied to the destination (source remains) — its dest level is
		// now stale too, so invalidate on ErrMovePartial as well as on clean success.
		copyMoveOK := msg.err == nil || errors.Is(msg.err, storage.ErrMovePartial)
		switch m.op.kind {
		case "copy", "move":
			if copyMoveOK {
				m.invalidateLevel(m.levelKeyFor(m.op.bucket, parentPrefix(m.op.srcKey)))
				m.invalidateLevel(m.levelKeyFor(m.op.bucket, parentPrefix(m.op.target)))
			}
		case "bulk_copy":
			if msg.err == nil {
				// bulk dstKey is the destination PREFIX itself (not a key), source is op.parent.
				m.invalidateLevel(m.levelKeyFor(m.op.bucket, m.op.parent))
				m.invalidateLevel(m.levelKeyFor(m.op.bucket, m.op.dstKey))
			}
		}
	}
	m.loading = false
	m.op = nil
	m.opCh = nil
	switch {
	case isBucket && msg.err == nil:
		// Bucket removed — return to a fresh bucket list (the bucket is gone).
		m.notice = "bucket deleted: " + bucketName
		return m.refreshBuckets()
	case isBucket && errors.Is(msg.err, storage.ErrBucketNotEmpty):
		m.notice = "bucket not empty — empty it first (bucket delete does not purge)"
		return m, nil
	case isBucket && msg.err != nil:
		m.err = msg.err
		return m, nil
	case isBulk:
		// Truthful per-batch summary; the selection is consumed (005 FR-018/FR-019).
		m.sel = nil
		done, failed := 0, 0
		if msg.summary != nil {
			done, failed = msg.summary.Deleted, msg.summary.Failed
		}
		if msg.err != nil && errors.Is(msg.err, context.Canceled) {
			m.notice = fmt.Sprintf("bulk %s cancelled: %d done, %d failed", bulkVerb, done, failed)
		} else {
			m.notice = fmt.Sprintf("bulk %s: %d done, %d failed", bulkVerb, done, failed)
		}
		if bulkVerb == "download" {
			return m, nil // read — nothing remote changed
		}
		return m.refresh()
	case msg.err != nil && errors.Is(msg.err, context.Canceled):
		// Indeterminate outcome: never success (FR-004/FR-007).
		if isDownload {
			m.notice = "cancelled — partial download removed"
			return m, nil
		}
		m.notice = "operation cancelled — nothing reported as done"
		return m.refresh()
	case isDownload && msg.err != nil:
		m.err = msg.err
		return m, nil
	case isDownload:
		m.notice = "downloaded to " + dlDest
		return m, nil
	case msg.err != nil && errors.Is(msg.err, storage.ErrMovePartial):
		// No data loss: copied to the destination, source remains (FR-007).
		m.notice = "moved partially: copied to destination, source still present"
		return m.refresh()
	case msg.err != nil:
		m.err = msg.err
		return m, nil
	case msg.partial && msg.summary != nil:
		m.notice = fmt.Sprintf("recursive delete: %d deleted, %d failed", msg.summary.Deleted, msg.summary.Failed)
		return m.refresh()
	case msg.summary != nil:
		m.notice = fmt.Sprintf("recursive delete: %d deleted", msg.summary.Deleted)
		return m.refresh()
	default:
		return m.refresh()
	}
}

// opPromptLine renders the transient operation prompt in the footer status line for
// the name, dest-entry, and confirm phases.
func (m App) opPromptLine(w int) string {
	op := m.op
	switch op.phase {
	case phaseName:
		const label = "new folder: "
		suffix, style := "▏  (Enter create · Esc cancel)", dimCellStyle
		if txt := m.errorText(); txt != "" {
			suffix, style = "▏  "+txt, errStyle
		}
		input := truncate(op.name, max(1, w-len(label)-len(suffix)))
		return accentStyle.Render(label) + objCellStyle.Render(input) + style.Render(suffix)
	case phaseDest:
		label := "copy to: "
		if op.kind == "move" {
			label = "move to: "
		}
		suffix, style := "▏  (Enter · Esc cancel)", dimCellStyle
		if txt := m.errorText(); txt != "" {
			suffix, style = "▏  "+txt, errStyle
		}
		input := truncate(sanitizeLabel(op.dstKey), max(1, w-len(label)-len(suffix)))
		return accentStyle.Render(label) + objCellStyle.Render(input) + style.Render(suffix)
	case phaseConfirm:
		// Typed-identifier tier → the prominent inline form (007 FR-023a). Binary tier is
		// rendered as a centered popup by View() (confirmPopupView), so the footer line is
		// empty for it.
		if op.tier == confirmTyped {
			return m.typedConfirmForm(w)
		}
		return ""
	}
	return ""
}

// simpleConfirmText is the prompt for a reversible (simple-tier) confirmation.
func (m App) simpleConfirmText() string {
	op := m.op
	switch op.kind {
	case "create_folder":
		return fmt.Sprintf("create folder %s/ ?", sanitizeLabel(op.target))
	case "copy":
		if op.overwrite {
			return fmt.Sprintf("OVERWRITE %s ?", sanitizeLabel(op.target))
		}
		return fmt.Sprintf("copy to %s ?", sanitizeLabel(op.target))
	case "upload":
		if op.overwrite {
			return fmt.Sprintf("OVERWRITE %s ?", sanitizeLabel(op.target))
		}
		return fmt.Sprintf("upload to %s ?", sanitizeLabel(op.target))
	case "download":
		return fmt.Sprintf("overwrite local %s ?", sanitizeLabel(op.target))
	case "delete_object":
		return fmt.Sprintf("delete %s ?", sanitizeLabel(op.srcKey))
	case "bulk_delete":
		return fmt.Sprintf("delete %s ?", op.target)
	case "move":
		if op.overwrite {
			return fmt.Sprintf("move %s → OVERWRITE %s ?", sanitizeLabel(op.srcKey), sanitizeLabel(op.target))
		}
		return fmt.Sprintf("move %s → %s ?", sanitizeLabel(op.srcKey), sanitizeLabel(op.target))
	default:
		return fmt.Sprintf("%s %s ?", op.kind, sanitizeLabel(op.target))
	}
}

// opProgressLine renders live progress for a running streaming operation (FR-010, 007
// US6). When the total is known AND the op has run past the "taking a while" threshold it
// shows a Claude-Code-style determinate bar with a percent; otherwise it shows the
// indeterminate spinner (no fabricated percent, FR-037). The cancel hint is always shown.
func (m App) opProgressLine() string {
	op := m.op
	var label string
	switch op.kind {
	case "upload":
		label = fmt.Sprintf("uploading %s / %s", humanSize(op.progress.uploaded), humanSize(max64(op.progress.total, op.progress.uploaded)))
	case "download":
		label = fmt.Sprintf("downloading %s / %s", humanSize(op.progress.uploaded), humanSize(max64(op.progress.total, op.progress.uploaded)))
	case "bulk_download", "bulk_delete", "bulk_copy":
		label = fmt.Sprintf("%s… %d/%d done, %d failed", strings.TrimPrefix(op.kind, "bulk_"), op.progress.deleted, op.progress.total, op.progress.failed)
	case "delete_recursive":
		label = fmt.Sprintf("deleting… %d removed, %d failed", op.progress.deleted, op.progress.failed)
	default:
		label = "working…"
	}
	cancel := dimCellStyle.Render("  (x to cancel)")
	if frac, ok := op.progress.determinate(); ok && op.ticks >= progressThreshold {
		return progressBar(frac, 24) + dimCellStyle.Render(" "+label) + cancel
	}
	return accentStyle.Render(m.spinnerView()) + dimCellStyle.Render(" "+label) + cancel
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
