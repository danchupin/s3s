package ui

import (
	"context"
	"net/url"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/logging"
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
}

// connField indexes the editable form fields (the read-only flag and submit are handled
// by cursor position past the text fields).
const (
	fldName = iota
	fldEndpoint
	fldRegion
	fldAccessKey
	fldSecret
	fldReadOnly
	connFieldCount
)

var connFieldLabels = []string{"name", "endpoint", "region", "access key id", "secret", "read-only"}

// connForm is the in-flight add-connection form state (modeConnForm). The secret field is
// masked BY THE FORM (held as a raw string, rendered as bullets, wrapped in logging.Secret
// only at save) — never via secret.Prompt (x/term), which only works before the TUI starts
// (005 R12) and would corrupt the alt-screen.
type connForm struct {
	name, endpoint, region, accessKey string
	secret                            string
	readOnly                          bool
	cursor                            int    // 0..fldReadOnly
	err                               string // field-level / test error (secret-free)
	tested                            bool   // a reachability test has run
	testOK                            bool   // the last test succeeded
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
// the last "add" row to open the form. Esc returns to the bucket list.
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
	case matches(key, m.keys.Back):
		m.mode = modeBuckets
	case matches(key, m.keys.Enter):
		if m.connSel == len(rows)-1 {
			m.form = &connForm{}
			m.mode = modeConnForm
			return m, nil
		}
		if m.connSel >= 0 && m.connSel < len(m.contexts) {
			return m.applyContext(m.contexts[m.connSel])
		}
	}
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
	case "backspace":
		m.formBackspace()
		return m, nil
	case " ", "space":
		if f.cursor == fldReadOnly {
			f.readOnly = !f.readOnly
			return m, nil
		}
		m.formAppend(" ")
		return m, nil
	default:
		if msg.Text != "" {
			m.formAppend(msg.Text)
		}
		return m, nil
	}
}

func (m App) formAppend(s string) {
	f := m.form
	switch f.cursor {
	case fldName:
		f.name += s
	case fldEndpoint:
		f.endpoint += s
	case fldRegion:
		f.region += s
	case fldAccessKey:
		f.accessKey += s
	case fldSecret:
		f.secret += s
	}
}

func (m App) formBackspace() {
	f := m.form
	chop := func(s string) string {
		if n := len(s); n > 0 {
			return s[:n-1]
		}
		return s
	}
	switch f.cursor {
	case fldName:
		f.name = chop(f.name)
	case fldEndpoint:
		f.endpoint = chop(f.endpoint)
	case fldRegion:
		f.region = chop(f.region)
	case fldAccessKey:
		f.accessKey = chop(f.accessKey)
	case fldSecret:
		f.secret = chop(f.secret)
	}
}

// draft builds the ConnDraft from the form.
func (f *connForm) draft() ConnDraft {
	return ConnDraft{
		Name:        strings.TrimSpace(f.name),
		Endpoint:    strings.TrimSpace(f.endpoint),
		Region:      strings.TrimSpace(f.region),
		AccessKeyID: strings.TrimSpace(f.accessKey),
		Secret:      logging.Secret(f.secret),
		ReadOnly:    f.readOnly,
	}
}

// validate enforces required/format rules + name uniqueness before any backend call.
func (m App) validateForm() string {
	f := m.form
	if strings.TrimSpace(f.name) == "" {
		return "name is required"
	}
	for _, c := range m.contexts {
		if c == strings.TrimSpace(f.name) {
			return "a context named " + strings.TrimSpace(f.name) + " already exists"
		}
	}
	ep := strings.TrimSpace(f.endpoint)
	if ep == "" {
		return "endpoint is required"
	}
	if u, err := url.Parse(ep); err != nil || u.Scheme == "" || u.Host == "" {
		return "endpoint must be an absolute URL (https://host:port)"
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
	if msg.err == nil {
		m.form.tested, m.form.testOK = true, true
		return m, saveConnCmd(m.connect, m.form.draft())
	}
	m.form.tested, m.form.testOK = true, false
	m.form.err = "unreachable — press Enter again to save anyway"
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
	m.mode = modeConnections
	m.notice = "connection saved: " + name
	return m, nil
}

// connectionsView renders the connection-list body.
func (m App) connectionsView(w, rows int) string {
	items := m.connRows()
	off, end := windowBounds(len(items), m.connSel, rows)
	data := make([][]string, 0, end-off)
	for i := off; i < end; i++ {
		status := ""
		if i < len(m.contexts) && m.contexts[i] == m.ctxName {
			status = "active"
		}
		data = append(data, []string{items[i], status})
	}
	return renderTable(w, []column{{"connection", 0}, {"status", 10}}, data, nil, m.connSel-off)
}

// connFormView renders the add-connection form body.
func (m App) connFormView(w int) string {
	f := m.form
	if f == nil {
		return ""
	}
	vals := []string{f.name, f.endpoint, f.region, f.accessKey, strings.Repeat("•", len(f.secret))}
	var b strings.Builder
	for i, label := range connFieldLabels {
		marker := "  "
		lblStyle := dimCellStyle
		if i == f.cursor {
			marker = "▶ "
			lblStyle = formActiveStyle
		}
		var val string
		if i == fldReadOnly {
			box := "[ ]"
			if f.readOnly {
				box = "[x]"
			}
			val = box + " (space toggles)"
		} else {
			val = vals[i]
		}
		b.WriteString(marker + lblStyle.Render(pad(label, 16)) + objCellStyle.Render(truncate(val, max(1, w-20))) + "\n")
	}
	b.WriteString("\n")
	if f.err != "" {
		b.WriteString(formErrStyle.Render(truncate(f.err, max(1, w-1))) + "\n")
	}
	b.WriteString(dimCellStyle.Render("↑/↓ field · space toggle · Enter test+save · Esc cancel"))
	return b.String()
}
