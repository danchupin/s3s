package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// fakeConnector records calls and returns canned results (no real keychain/network).
type fakeConnector struct {
	testErr     error
	saveErr     error
	deleteErr   error
	names       []string
	tested      bool
	savedName   string
	deletedName string
}

func (c *fakeConnector) Test(_ context.Context, _ ConnDraft) error {
	c.tested = true
	return c.testErr
}

func (c *fakeConnector) Save(_ context.Context, d ConnDraft) ([]string, error) {
	c.savedName = d.Name
	return c.names, c.saveErr
}

func (c *fakeConnector) Delete(_ context.Context, name string) ([]string, error) {
	c.deletedName = name
	return c.names, c.deleteErr
}

func connApp(conn Connector, contexts []string) App {
	f := storage.NewFake()
	return New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x"},
		"ctx", contexts, nil, conn, preview.ProtoNone)
}

// runForm submits the form and drains the test/save command chain synchronously.
func runForm(m App) App {
	mm, cmd := m.submitConnForm()
	m = mm.(App)
	for cmd != nil {
		var nm tea.Model
		nm, cmd = m.Update(cmd())
		m = nm.(App)
	}
	return m
}

// --- US4: form validation (FR-021/FR-024) ---

func TestConnFormRequiresName(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{endpoint: "http://h:9000"}
	mm, _ := m.submitConnForm()
	m = mm.(App)
	if m.form == nil || !strings.Contains(m.form.err, "name") {
		t.Errorf("empty name should block save with a name error; form=%+v", m.form)
	}
}

func TestConnFormRejectsDuplicate(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx", "prod"})
	m.mode = modeConnForm
	m.form = &connForm{name: "prod", endpoint: "http://h:9000"}
	mm, _ := m.submitConnForm()
	if mm.(App).form.err == "" || !strings.Contains(mm.(App).form.err, "already exists") {
		t.Errorf("duplicate name should be rejected; err=%q", mm.(App).form.err)
	}
}

func TestConnFormRejectsBadEndpoint(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "newc", endpoint: "not-a-url"}
	mm, _ := m.submitConnForm()
	if !strings.Contains(mm.(App).form.err, "absolute URL") {
		t.Errorf("bad endpoint should be rejected; err=%q", mm.(App).form.err)
	}
}

// --- US4: test → save (FR-025/FR-025a) ---

func TestConnSaveSuccessUpdatesContexts(t *testing.T) {
	conn := &fakeConnector{names: []string{"ctx", "newc"}}
	m := connApp(conn, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "newc", endpoint: "http://h:9000", accessKey: "AK", secret: "SK"}
	m = runForm(m)

	if !conn.tested || conn.savedName != "newc" {
		t.Errorf("a valid form should Test then Save; tested=%v saved=%q", conn.tested, conn.savedName)
	}
	if len(m.contexts) != 2 || m.contexts[1] != "newc" {
		t.Errorf("contexts should refresh with the new connection; got %v", m.contexts)
	}
	if m.mode != modeConnections {
		t.Errorf("after save the manager should reappear; mode=%v", m.mode)
	}
	if !strings.Contains(m.notice, "saved") {
		t.Errorf("save should surface a confirmation notice; got %q", m.notice)
	}
}

func TestConnTestFailOffersSaveAnyway(t *testing.T) {
	conn := &fakeConnector{testErr: errors.New("dial tcp: timeout"), names: []string{"ctx", "newc"}}
	m := connApp(conn, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "newc", endpoint: "http://h:9000", accessKey: "AK", secret: "SK"}
	m = runForm(m)

	// First submit: test fails → offer "save anyway", NOT saved yet.
	if conn.savedName != "" {
		t.Errorf("a failed test must not save; savedName=%q", conn.savedName)
	}
	if m.form == nil || !strings.Contains(m.form.err, "save anyway") {
		t.Fatalf("a failed test should offer save anyway; form=%+v", m.form)
	}
	// Second submit overrides → saves regardless.
	m = runForm(m)
	if conn.savedName != "newc" {
		t.Errorf("a second Enter should save anyway; savedName=%q", conn.savedName)
	}
}

func TestConnSaveErrorShownInForm(t *testing.T) {
	conn := &fakeConnector{saveErr: errors.New("config: write denied")}
	m := connApp(conn, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "newc", endpoint: "http://h:9000", accessKey: "AK", secret: "SK"}
	m = runForm(m)
	if m.form == nil || !strings.Contains(m.form.err, "save failed") {
		t.Errorf("a save error should surface in the form; form=%+v", m.form)
	}
}

// review #1: ctrl+c quits even inside the modal form (where `q`/`:` are literal text).
func TestCtrlCQuitsFromForm(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "x"}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c in the connection form should quit")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("ctrl+c command produced no message")
	}
}

// review #3: blank credentials are blocked at the form, before the config writer.
func TestConnFormRequiresCredentials(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "newc", endpoint: "http://h:9000"} // no access key / secret
	mm, _ := m.submitConnForm()
	if !strings.Contains(mm.(App).form.err, "access key") {
		t.Errorf("missing access key should block save; err=%q", mm.(App).form.err)
	}
}

// review #4: editing a field after a failed test re-arms the test (no silent save-anyway).
func TestEditAfterFailedTestRetests(t *testing.T) {
	conn := &fakeConnector{testErr: errors.New("timeout"), names: []string{"ctx", "newc"}}
	m := connApp(conn, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: "newc", endpoint: "http://h:9000", accessKey: "AK", secret: "SK"}
	m = runForm(m) // first submit: test fails → save-anyway offered
	if !m.form.tested || m.form.testOK {
		t.Fatal("precondition: form should be in failed-test state")
	}
	// Edit a field → must clear the tested flag.
	m.form.cursor = fldEndpoint
	m.formAppend("9")
	if m.form.tested {
		t.Error("editing a field after a failed test must re-arm the reachability test")
	}
}

func TestOpenConnectionsNilConnectorNotice(t *testing.T) {
	m := connApp(nil, []string{"ctx"})
	mm, _ := m.openConnections()
	m = mm.(App)
	if m.mode == modeConnections {
		t.Error("a nil connector must not open the manager")
	}
	if !strings.Contains(m.notice, "unavailable") {
		t.Errorf("a nil connector should explain it is unavailable; got %q", m.notice)
	}
}

func TestConnectionsListReachableViaCommand(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	bs, _ := storage.NewFake().ListBuckets(context.Background())
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: bs})
	m = press(m, ":")
	m = typeStr(m, "conn")
	m = press(m, "enter")
	if m.mode != modeConnections {
		t.Fatalf("':conn' should open the connection manager; mode=%v", m.mode)
	}
}

// --- 007 US5: delete a connection on the contexts screen ---

// drainCmds runs a command chain to completion against the model.
func drainCmds(m App, cmd tea.Cmd) App {
	for cmd != nil {
		var nm tea.Model
		nm, cmd = m.Update(cmd())
		m = nm.(App)
	}
	return m
}

func TestDeleteConnectionTypedConfirmFlow(t *testing.T) {
	conn := &fakeConnector{names: []string{"ctx"}} // post-delete names
	m := connApp(conn, []string{"ctx", "other"})
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = 1 // select "other" (non-active)

	m = press(m, "ctrl+x")
	if m.op == nil || m.op.kind != "delete_connection" || m.op.tier != confirmTyped || m.op.expect != "other" {
		t.Fatalf("ctrl+x should start a typed connection delete; op=%+v", m.op)
	}
	// Type the exact name and confirm.
	for _, r := range "other" {
		m = press(m, string(r))
	}
	mm2, cmd := m.Update(keyMsgFor("enter"))
	m = drainCmds(mm2.(App), cmd)

	if conn.deletedName != "other" {
		t.Errorf("connector.Delete should be called with %q; got %q", "other", conn.deletedName)
	}
	if len(m.contexts) != 1 || m.contexts[0] != "ctx" {
		t.Errorf("context list should refresh live to [ctx]; got %v", m.contexts)
	}
	if m.op != nil {
		t.Errorf("op should be cleared after delete; got %+v", m.op)
	}
}

func TestDeleteConnectionRefusesActive(t *testing.T) {
	conn := &fakeConnector{names: []string{"ctx", "other"}}
	m := connApp(conn, []string{"ctx", "other"})
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = 0 // "ctx" is the active context

	m = press(m, "ctrl+x")
	if m.op != nil {
		t.Fatalf("deleting the active context must be refused; op=%+v", m.op)
	}
	if conn.deletedName != "" {
		t.Error("connector.Delete must not be called for the active context")
	}
	if m.notice == "" {
		t.Error("refusing the active context should surface a nudge")
	}
}

func TestDeleteConnectionWrongNameAborts(t *testing.T) {
	conn := &fakeConnector{names: []string{"ctx"}}
	m := connApp(conn, []string{"ctx", "other"})
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = 1
	m = press(m, "ctrl+x")
	for _, r := range "wrong" {
		m = press(m, string(r))
	}
	m = press(m, "enter")
	if conn.deletedName != "" {
		t.Error("a mismatched name must abort without calling Delete")
	}
	if m.op != nil {
		t.Errorf("mismatch should clear the op; got %+v", m.op)
	}
}

func TestDeleteLastConnectionEmptyState(t *testing.T) {
	conn := &fakeConnector{names: []string{}} // last one deleted → empty
	m := connApp(conn, []string{"ctx", "solo"})
	m.ctxName = "ctx"
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = 1 // "solo"
	m = press(m, "ctrl+x")
	for _, r := range "solo" {
		m = press(m, string(r))
	}
	mm2, cmd := m.Update(keyMsgFor("enter"))
	m = drainCmds(mm2.(App), cmd)
	if len(m.contexts) != 0 {
		t.Errorf("after deleting the last connection, contexts should be empty; got %v", m.contexts)
	}
	// The connections view must render without panicking on an empty list.
	_ = m.connectionsView(80, 10)
}
