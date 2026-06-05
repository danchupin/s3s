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
	phaseRunning                // dispatched; awaiting the outcome
)

// confirmTier is the strength of confirmation an operation requires (FR-005).
type confirmTier int

const (
	confirmSimple confirmTier = iota // reversible: y/Enter
	confirmTyped                     // destructive: type the exact target
)

// operation is the in-flight mutating intent. For 002 the only production kind is
// "create_folder" (confirmSimple). The confirmTyped tier ships for future
// destructive operations (US3) and is exercised by a test-only fixture.
type operation struct {
	kind   string // "create_folder" (plus test-only kinds)
	bucket string
	parent string // parent prefix of the level the op started in
	name   string // typed folder name (name phase)
	target string // resolved key acted on (parent+name); for confirm + logging
	tier   confirmTier
	expect string // typed tier: the exact string the operator must type
	input  string // typed tier: the operator's entry
	phase  opPhase
}

// UI-local hint sentinels (rendered as secret-free status lines).
var (
	errFolderExists    = errors.New("ui: folder already exists")
	errConfirmMismatch = errors.New("ui: confirmation did not match")
)

// startCreateFolder begins a create-folder intent at the current tree level. On a
// read-only context it issues no command and shows the read-only hint, never a
// silent no-op (FR-003).
func (m App) startCreateFolder() (tea.Model, tea.Cmd) {
	if !m.writable {
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

// onOpKey routes a keypress while an interactive operation is active (name or
// confirm phase). phaseRunning is handled by the global cancel path, not here.
func (m App) onOpKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.op.phase {
	case phaseName:
		return m.onNameKey(key, msg)
	case phaseConfirm:
		return m.onConfirmKey(key, msg)
	}
	return m, nil
}

// onNameKey edits the folder name. Only Esc cancels and Enter submits; every other
// printable key is text (so "h"/"l" are valid name characters, not navigation).
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

// submitName validates the typed name and advances to the simple confirmation, or
// surfaces a hint and stays/aborts. Collision is an advisory check against the
// loaded level (no silent overwrite — FR-010).
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

// dispatchOp logs the intent and runs the mutation off the event loop under a fresh
// generation (FR-006/FR-008). The backend must be a Mutator; a read-only guard
// would refuse with ErrReadOnly, but the UI already gates on m.writable.
func (m App) dispatchOp() (tea.Model, tea.Cmd) {
	op := m.op
	mut, ok := m.store.(storage.Mutator)
	if !ok {
		m.op = nil
		m.err = storage.ErrReadOnly
		return m, nil
	}
	op.phase = phaseRunning
	switch op.kind {
	case "create_folder":
		logMutationStart(op.kind, "bucket", op.bucket, "key", op.target, "context", m.ctxName)
		ctx := (&m).beginLoad()
		return m, tea.Batch(createFolderCmd(ctx, mut, op.bucket, op.target, m.gen), spinnerTick())
	default:
		// Test-only kinds (e.g. the typed-tier fixture): no real action; complete
		// immediately as success so the confirmation flow can be exercised end-to-end.
		(&m).beginLoad()
		gen := m.gen
		return m, func() tea.Msg { return operationDoneMsg{gen: gen} }
	}
}

// onOperationDone applies the terminal outcome (FR-006/FR-007).
func (m App) onOperationDone(msg operationDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil // superseded — drop
	}
	m.loading = false
	m.op = nil
	if msg.err != nil {
		// A cancelled in-flight mutation has an indeterminate storage outcome: never
		// reported as success; re-fetch so the view shows ground truth (FR-007).
		if errors.Is(msg.err, context.Canceled) {
			return m.refresh()
		}
		m.err = msg.err
		return m, nil
	}
	// Success: the new folder exists; invalidate the cached level and re-fetch so it
	// becomes visible (SC-006).
	return m.refresh()
}

// opPromptLine renders the transient operation prompt in the footer status line.
func (m App) opPromptLine(w int) string {
	op := m.op
	switch op.phase {
	case phaseName:
		const label = "new folder: "
		// Surface a validation hint inline so an invalid name is never silent (FR-003).
		suffix, style := "▏  (Enter create · Esc cancel)", dimCellStyle
		if txt := m.errorText(); txt != "" {
			suffix, style = "▏  "+txt, errStyle
		}
		input := truncate(op.name, max(1, w-len(label)-len(suffix)))
		return accentStyle.Render(label) + objCellStyle.Render(input) + style.Render(suffix)
	case phaseConfirm:
		if op.tier == confirmTyped {
			const label = "type to confirm: "
			input := truncate(op.input, max(1, w-len(label)-24))
			return accentStyle.Render(label) + objCellStyle.Render(input) +
				dimCellStyle.Render("▏  (need "+sanitizeLabel(op.expect)+" · Esc)")
		}
		msg := fmt.Sprintf("create folder %s/ ?", sanitizeLabel(op.target))
		return accentStyle.Render(truncate(msg, max(1, w-12))) + dimCellStyle.Render("  (y / N)")
	}
	return ""
}
