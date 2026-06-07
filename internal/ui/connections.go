package ui

import (
	"context"
	"errors"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/logging"
	"github.com/danchupin/s3s/internal/storage"
)

// In-app connection manager (006 US4). A connection list (modeConnections) and an add
// form (modeConnForm) reach the config writer + keychain ONLY through the injected
// Connector seam, so the UI never imports the SDK or config marshalling (Constitution I).

// ConnDraft is the UI-agnostic working value of the add-connection form. The secret is
// held redacted and never persisted to config (FR-022).
type ConnDraft struct {
	Name        string
	Endpoint    string
	Region      string
	AccessKeyID string
	Secret      logging.Secret
	Buckets     []string // pinned bucket names (010 US3) — empty ⇒ list-all
	PathStyle   bool
	ReadOnly    bool
}

// Connector is the injected seam (built in cmd/s3s/main.go). nil disables the in-app add
// feature, like a nil Resolver disables context switching.
type Connector interface {
	// Test runs a live reachability check (storage.New + ListBuckets) off the event loop.
	Test(ctx context.Context, d ConnDraft) error
	// Save persists the connection (secret→keychain, then config triple) and returns the
	// updated context-name list. Never writes the secret to config in plaintext.
	Save(ctx context.Context, d ConnDraft) ([]string, error)
	// Delete removes the named connection (config triple + keychain secret) and returns
	// the updated context-name list. Refuses the active context (007 US5 / FR-031/032).
	Delete(ctx context.Context, name string) ([]string, error)
	// AddBucket pins a bucket name to the named connection and returns the updated pinned
	// bucket list (010 US2). A CONFIG mutation (no keychain, no S3 write); idempotent.
	AddBucket(ctx context.Context, ctxName, bucket string) ([]string, error)
}

// connField indexes the editable form fields (the read-only flag and submit are handled
// by cursor position past the text fields).
const (
	fldName = iota
	fldEndpoint
	fldRegion
	fldAccessKey
	fldSecret
	fldBuckets // pinned bucket names (010 US3) — optional, comma/space separated
	fldPathStyle
	fldReadOnly
	connFieldCount
)

var connFieldLabels = []string{"name", "endpoint", "region", "access key id", "secret", "buckets", "path-style", "read-only"}

// connForm is the in-flight add-connection form state (modeConnForm). The secret field is
// masked BY THE FORM (held as a raw string, rendered as bullets, wrapped in logging.Secret
// only at save) — never via secret.Prompt (x/term), which only works before the TUI starts
// (005 R12) and would corrupt the alt-screen.
type connForm struct {
	name, endpoint, region, accessKey textField
	secret                            textField // masked on render, never logged
	buckets                           textField // pinned bucket names (010 US3), comma/space separated
	pathStyle                         bool
	readOnly                          bool
	cursor                            int    // 0..fldReadOnly
	err                               string // field-level / test error (secret-free)
	tested                            bool   // a reachability test has run
	testOK                            bool   // the last test succeeded
}

// bucketAddForm is the in-flight "+ add bucket" input (modeAddBucket, 010 US2): a single
// rune-aware field for a bucket name plus a field-level error. Mirrors connForm, minimal.
type bucketAddForm struct {
	name textField
	err  string
}

// openConnections opens the connection manager. Disabled (a notice) when no Connector was
// injected — e.g. a non-interactive run.
func (m App) openConnections() (tea.Model, tea.Cmd) {
	if m.connect == nil {
		m.notice = "in-app connections unavailable (no config writer)"
		m.mode = modeBuckets
		return m, nil
	}
	m.mode = modeConnections
	m.connSel = 0
	return m, nil
}

// connRows is the connection-list contents: existing contexts then an "add" row.
func (m App) connRows() []string {
	rows := append([]string{}, m.contexts...)
	return append(rows, "+ add connection")
}

// onConnectionsKey handles the connection-list navigation: pick a context to switch, or
// the last "add" row to open the form. ctrl+x deletes the selected (non-active)
// connection (007 US5). Esc returns to the bucket list.
func (m App) onConnectionsKey(key string) (tea.Model, tea.Cmd) {
	rows := m.connRows()
	switch {
	case matches(key, m.keys.Up):
		if m.connSel > 0 {
			m.connSel--
		}
	case matches(key, m.keys.Down):
		if m.connSel < len(rows)-1 {
			m.connSel++
		}
	case matches(key, m.keys.DeleteChord):
		return m.startRemoveConnection()
	case matches(key, m.keys.Back):
		m.mode = modeBuckets
	case matches(key, m.keys.Enter):
		if m.connSel == len(rows)-1 {
			m.form = &connForm{pathStyle: true} // path-style default (Ceph RGW / MinIO)
			m.mode = modeConnForm
			return m, nil
		}
		if m.connSel >= 0 && m.connSel < len(m.contexts) {
			return m.applyContext(m.contexts[m.connSel])
		}
	}
	return m, nil
}

// startRemoveConnection begins deleting the selected connection (007 US5). It is the
// highest-tier dangerous action: typed confirmation of the exact connection name (the
// shared typed surface). The ACTIVE context is refused (FR-032); the "+ add" row and
// out-of-range selections are no-ops.
func (m App) startRemoveConnection() (tea.Model, tea.Cmd) {
	if m.connSel < 0 || m.connSel >= len(m.contexts) {
		return m, nil // the "+ add connection" row or an empty list
	}
	name := m.contexts[m.connSel]
	if name == m.ctxName {
		m.notice = "cannot delete the active connection — switch context first"
		return m, nil
	}
	m.err = nil
	m.op = &operation{
		kind:   "delete_connection",
		target: name,
		tier:   confirmTyped,
		expect: name,
		phase:  phaseConfirm,
	}
	return m, nil
}

// connDeleteCmd removes a connection off the event loop via the injected Connector
// (007 US5). Returns the updated context-name list on success.
func connDeleteCmd(c Connector, name string, gen int) tea.Cmd {
	return func() tea.Msg {
		names, err := c.Delete(context.Background(), name)
		return connDeletedMsg{gen: gen, name: name, names: names, err: err}
	}
}

// onConnDeleted applies the delete outcome: success → refresh the live context list and
// stay in the manager; failure → surface the error (007 US5 / FR-033).
func (m App) onConnDeleted(msg connDeletedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.gen {
		return m, nil // superseded
	}
	m.loading = false
	m.op = nil
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}
	m.contexts = msg.names
	if m.connSel > len(m.contexts) { // keep selection within the (contexts + add) rows
		m.connSel = len(m.contexts)
	}
	m.notice = "connection deleted: " + msg.name
	return m, nil
}

// onConnFormKey edits the add-connection form. up/down move between fields; space toggles
// read-only; Enter submits (test, then save; a second Enter saves anyway after a failed
// test — FR-025a); Esc cancels.
func (m App) onConnFormKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	f := m.form
	switch key {
	case "esc":
		m.form = nil
		m.err = nil // clear any test error so it doesn't leak into the list footer (010 US4)
		m.mode = modeConnections
		return m, nil
	case "up":
		if f.cursor > 0 {
			f.cursor--
		}
		return m, nil
	case "down", "tab":
		if f.cursor < connFieldCount-1 {
			f.cursor++
		}
		return m, nil
	case "enter":
		return m.submitConnForm()
	case "left":
		m.formCaret((*textField).Left)
		return m, nil
	case "right":
		m.formCaret((*textField).Right)
		return m, nil
	case "home":
		m.formCaret((*textField).Home)
		return m, nil
	case "end":
		m.formCaret((*textField).End)
		return m, nil
	case "backspace":
		m.formBackspace()
		return m, nil
	case "delete":
		if p := f.focusField(); p != nil {
			p.DeleteFwd()
			f.tested, f.testOK = false, false
		}
		return m, nil
	case " ", "space":
		switch f.cursor {
		case fldPathStyle:
			f.pathStyle = !f.pathStyle
		case fldReadOnly:
			f.readOnly = !f.readOnly
		default:
			m.formAppend(" ")
		}
		return m, nil
	default:
		if msg.Text != "" {
			m.formAppend(msg.Text)
		}
		return m, nil
	}
}

// focusField returns a pointer to the editable textField under the cursor, or nil for the
// boolean toggle rows. The single field selector shared by all editing ops.
func (f *connForm) focusField() *textField {
	switch f.cursor {
	case fldName:
		return &f.name
	case fldEndpoint:
		return &f.endpoint
	case fldRegion:
		return &f.region
	case fldAccessKey:
		return &f.accessKey
	case fldSecret:
		return &f.secret
	case fldBuckets:
		return &f.buckets
	}
	return nil
}

func (m App) formAppend(s string) {
	f := m.form
	if p := f.focusField(); p != nil {
		p.Insert(s)
	}
	f.tested, f.testOK = false, false // editing invalidates a prior reachability test (FR-025a)
}

func (m App) formBackspace() {
	f := m.form
	if p := f.focusField(); p != nil {
		p.Backspace()
	}
	f.tested, f.testOK = false, false // editing invalidates a prior reachability test (FR-025a)
}

// formCaret applies a caret-movement op to the focused text field (008 US3, FR-006). A
// no-op on the boolean toggle rows (FR-008). Caret moves never invalidate the test.
func (m App) formCaret(move func(*textField)) {
	if p := m.form.focusField(); p != nil {
		move(p)
	}
}

// draft builds the ConnDraft from the form.
func (f *connForm) draft() ConnDraft {
	return ConnDraft{
		Name:        strings.TrimSpace(f.name.Value),
		Endpoint:    strings.TrimSpace(f.endpoint.Value),
		Region:      strings.TrimSpace(f.region.Value),
		AccessKeyID: strings.TrimSpace(f.accessKey.Value),
		Secret:      logging.Secret(f.secret.Value),
		Buckets:     parseBuckets(f.buckets.Value),
		PathStyle:   f.pathStyle,
		ReadOnly:    f.readOnly,
	}
}

// parseBuckets normalizes a free-text bucket list (010 US3/FR-006): split on commas and
// whitespace, trim, drop empties, de-duplicate preserving first-seen order. nil when empty.
func parseBuckets(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	var out []string
	seen := map[string]bool{}
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// validate enforces required/format rules + name uniqueness before any backend call.
func (m App) validateForm() string {
	f := m.form
	name := strings.TrimSpace(f.name.Value)
	if name == "" {
		return "name is required"
	}
	for _, c := range m.contexts {
		if c == name {
			return "a context named " + name + " already exists"
		}
	}
	ep := strings.TrimSpace(f.endpoint.Value)
	if ep == "" {
		return "endpoint is required"
	}
	if u, err := url.Parse(ep); err != nil || u.Scheme == "" || u.Host == "" {
		return "endpoint must be an absolute URL (https://host:port)"
	}
	// Credentials are required: the connection is persisted with a keychain source, which
	// the config validator rejects without an access key id (and an empty secret is
	// useless). Catch it here so a bad draft never reaches the config writer (FR-021).
	if strings.TrimSpace(f.accessKey.Value) == "" {
		return "access key id is required"
	}
	if f.secret.Value == "" {
		return "secret access key is required"
	}
	return ""
}

// submitConnForm validates then runs Test (or saves anyway after a prior failed test).
func (m App) submitConnForm() (tea.Model, tea.Cmd) {
	if e := m.validateForm(); e != "" {
		m.form.err = e
		return m, nil
	}
	// Override path: a prior test failed and the user pressed Enter again → save anyway.
	if m.form.tested && !m.form.testOK {
		return m, saveConnCmd(m.connect, m.form.draft())
	}
	m.form.err = ""
	return m, testConnCmd(m.connect, m.form.draft())
}

// testConnCmd runs the reachability test off the event loop (FR-025a/FR-030).
func testConnCmd(c Connector, d ConnDraft) tea.Cmd {
	return func() tea.Msg { return connTestedMsg{err: c.Test(context.Background(), d)} }
}

// saveConnCmd persists the connection off the event loop (FR-030).
func saveConnCmd(c Connector, d ConnDraft) tea.Cmd {
	return func() tea.Msg {
		names, err := c.Save(context.Background(), d)
		return connSavedMsg{names: names, err: err}
	}
}

// onConnTested applies the reachability result: success → save; failure → offer
// "save anyway" (FR-025a).
func (m App) onConnTested(msg connTestedMsg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		return m, nil
	}
	// nil OR access-denied counts as reachable → save (010 US4/FR-009): the endpoint resolved
	// and the server answered; bucket-scoped creds simply may lack list-all. Other failures
	// (unreachable/not-found/invalid-config) show the CLASSIFIED reason (not a blanket
	// "unreachable") and keep the "save anyway" affordance (FR-010).
	if msg.err == nil || errors.Is(msg.err, storage.ErrAccessDenied) {
		m.form.tested, m.form.testOK = true, true
		m.err = nil
		return m, saveConnCmd(m.connect, m.form.draft())
	}
	m.form.tested, m.form.testOK = true, false
	m.err = msg.err
	m.form.err = m.errorText() + " — press Enter again to save anyway"
	return m, nil
}

// onConnSaved applies the save outcome: success → refresh the context list and switch back
// to the manager; failure → show the error in the form (FR-025/FR-026).
func (m App) onConnSaved(msg connSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if m.form != nil {
			m.form.err = "save failed: " + msg.err.Error()
		}
		return m, nil
	}
	if msg.names != nil {
		m.contexts = msg.names
	}
	name := ""
	if m.form != nil {
		name = m.form.draft().Name
	}
	m.form = nil
	m.err = nil // a stale test error must not bleed into the bucket-list footer (010 US4)
	m.notice = "connection saved: " + name
	// Enter the just-saved connection so the user lands in its bucket list — where a scoped
	// connection (no pinned buckets, list-all denied) shows the "+ add bucket" row. This makes
	// "create the connection, then choose buckets" a single flow (010), and unifies first-run
	// (009) with subsequent adds. Switching disabled (resolve == nil, e.g. tests) → manager.
	if m.resolve != nil {
		return m.applyContext(name)
	}
	m.mode = modeConnections
	return m, nil
}

// openAddBucket opens the "+ add bucket" input (010 US2). No-op without a Connector.
func (m App) openAddBucket() (tea.Model, tea.Cmd) {
	if m.connect == nil {
		return m, nil
	}
	m.addForm = &bucketAddForm{}
	m.mode = modeAddBucket
	m.err = nil
	return m, nil
}

// addBucketCmd persists a pinned bucket off the event loop via the injected Connector
// (010 US2), mirroring saveConnCmd. The result list flows back via addBucketMsg.
func addBucketCmd(c Connector, ctxName, bucket string) tea.Cmd {
	return func() tea.Msg {
		buckets, err := c.AddBucket(context.Background(), ctxName, bucket)
		return addBucketMsg{buckets: buckets, err: err}
	}
}

// onAddBucketKey edits the "+ add bucket" input (modeAddBucket, 010 US2). Enter submits a
// non-empty name; Esc cancels; the rest drive the single textField (incl. caret + paste).
func (m App) onAddBucketKey(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	f := m.addForm
	switch key {
	case "esc":
		m.addForm = nil
		m.mode = modeBuckets
		return m, nil
	case "enter":
		name := strings.TrimSpace(f.name.Value)
		if name == "" {
			f.err = "bucket name required"
			return m, nil
		}
		f.err = ""
		return m, addBucketCmd(m.connect, m.ctxName, name)
	case "left":
		f.name.Left()
	case "right":
		f.name.Right()
	case "home":
		f.name.Home()
	case "end":
		f.name.End()
	case "backspace":
		f.name.Backspace()
	case "delete":
		f.name.DeleteFwd()
	case " ", "space":
		f.name.Insert(" ")
	default:
		if msg.Text != "" {
			f.name.Insert(msg.Text)
		}
	}
	return m, nil
}

// onAddBucket applies the persist outcome (010 US2): on success update the live pinned list
// and reload the bucket view so the new bucket appears; on failure surface it on the form.
func (m App) onAddBucket(msg addBucketMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if m.addForm != nil {
			m.addForm.err = "add failed: " + msg.err.Error()
		}
		return m, nil
	}
	m.info.PinnedBuckets = msg.buckets
	m.addForm = nil
	m.mode = modeBuckets
	m.notice = "bucket added"
	ctx := (&m).beginLoad()
	return m, tea.Batch(loadBuckets(ctx, m.activeStore(), m.gen, m.info.PinnedBuckets), spinnerTick())
}

// addBucketView renders the "+ add bucket" input body (010 US2).
func (m App) addBucketView(w int) string {
	f := m.addForm
	if f == nil {
		return ""
	}
	avail := max(1, w-20)
	var b strings.Builder
	b.WriteString("▶ " + formActiveStyle.Render(pad("bucket", 16)) + objCellStyle.Render(f.name.Render(avail, false)) + "\n\n")
	b.WriteString(dimCellStyle.Render(truncate("name of a bucket your credentials can access", max(1, w-1))) + "\n")
	if f.err != "" {
		b.WriteString(formErrStyle.Render(truncate(f.err, max(1, w-1))) + "\n")
	}
	b.WriteString(dimCellStyle.Render("Enter add · Esc cancel"))
	return b.String()
}

// connDeleteHint is the connection-list help line (008 US1). It always advertises
// navigation; the delete segment is appended ONLY for a deletable (non-active, existing)
// selection — absent on the "+ add connection" row, the empty list, and the active
// connection (FR-001/FR-003). The delete keystroke is sourced from the keymap so it tracks
// rebinds and the chord-label format (FR-004).
func (m App) connDeleteHint() string {
	base := "↑/↓ select · Enter open/switch · Esc back"
	if m.connSel >= 0 && m.connSel < len(m.contexts) && m.contexts[m.connSel] != m.ctxName {
		return base + " · " + glyph(m.keys.DeleteChord[0]) + " delete"
	}
	return base
}

// connectionsView renders the connection-list body (existing contexts + an add row) plus the
// inline help line carrying the delete hint (008 US1).
func (m App) connectionsView(w, rows int) string {
	hint := m.connDeleteHint()
	body := ctxTable(w, max(1, rows-2), m.connSel, "connection", m.connRows(), m.ctxName)
	return body + "\n\n" + dimCellStyle.Render(truncate(hint, max(1, w-1)))
}

// connFieldHint returns the focused-field guidance line (008 US4, FR-009/FR-010). The secret
// field names what to enter (the secret access key, stored in the OS keychain) and that the
// other credential sources are config-file-only — it deliberately does NOT promise ${ENV}
// resolution, since the form stores the secret verbatim to the keychain.
func (m App) connFieldHint(cursor int) string {
	switch cursor {
	case fldName:
		return "unique context name"
	case fldEndpoint:
		return "absolute URL, e.g. https://host:9000"
	case fldRegion:
		return "region, e.g. us-east-1 (optional)"
	case fldAccessKey:
		return "access key id"
	case fldSecret:
		return "secret access key — stored in your OS keychain · env var / cmd / AWS profile via config file"
	case fldBuckets:
		return "comma/space-separated bucket names — pin these when credentials can't list all (optional)"
	}
	return ""
}

// connFormView renders the add-connection form body.
func (m App) connFormView(w int) string {
	f := m.form
	if f == nil {
		return ""
	}
	fields := []textField{f.name, f.endpoint, f.region, f.accessKey, f.secret, f.buckets}
	checkbox := func(on bool) string {
		if on {
			return "[x] (space toggles)"
		}
		return "[ ] (space toggles)"
	}
	avail := max(1, w-20)
	var b strings.Builder
	for i, label := range connFieldLabels {
		marker := "  "
		lblStyle := dimCellStyle
		focused := i == f.cursor
		if focused {
			marker = "▶ "
			lblStyle = formActiveStyle
		}
		var val string
		switch i {
		case fldPathStyle:
			val = checkbox(f.pathStyle)
		case fldReadOnly:
			val = checkbox(f.readOnly)
		case fldSecret:
			if focused {
				val = fields[i].Render(avail, true) // masked, with caret
			} else {
				val = strings.Repeat("•", len([]rune(fields[i].Value)))
			}
		default:
			if focused {
				val = fields[i].Render(avail, false) // windowed, with caret
			} else {
				val = truncate(fields[i].Value, avail)
			}
		}
		b.WriteString(marker + lblStyle.Render(pad(label, 16)) + objCellStyle.Render(val) + "\n")
	}
	b.WriteString("\n")
	if hint := m.connFieldHint(f.cursor); hint != "" {
		b.WriteString(dimCellStyle.Render(truncate(hint, max(1, w-1))) + "\n")
	}
	if f.err != "" {
		b.WriteString(formErrStyle.Render(truncate(f.err, max(1, w-1))) + "\n")
	}
	b.WriteString(dimCellStyle.Render("↑/↓ field · ←/→ move · space toggle · Enter test+save · Esc cancel"))
	return b.String()
}
