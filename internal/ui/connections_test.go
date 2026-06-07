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
	addErr      error
	names       []string
	addBuckets  []string // returned by AddBucket
	tested      bool
	savedName   string
	savedDraft  ConnDraft // last draft passed to Save (010 US3: assert Buckets)
	deletedName string
	addedBucket string // last bucket passed to AddBucket (010 US2)
}

func (c *fakeConnector) Test(_ context.Context, _ ConnDraft) error {
	c.tested = true
	return c.testErr
}

func (c *fakeConnector) Save(_ context.Context, d ConnDraft) ([]string, error) {
	c.savedName = d.Name
	c.savedDraft = d
	return c.names, c.saveErr
}

func (c *fakeConnector) Delete(_ context.Context, name string) ([]string, error) {
	c.deletedName = name
	return c.names, c.deleteErr
}

func (c *fakeConnector) AddBucket(_ context.Context, _, bucket string) ([]string, error) {
	c.addedBucket = bucket
	return c.addBuckets, c.addErr
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
	m.form = &connForm{endpoint: textField{Value: "http://h:9000"}}
	mm, _ := m.submitConnForm()
	m = mm.(App)
	if m.form == nil || !strings.Contains(m.form.err, "name") {
		t.Errorf("empty name should block save with a name error; form=%+v", m.form)
	}
}

func TestConnFormRejectsDuplicate(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx", "prod"})
	m.mode = modeConnForm
	m.form = &connForm{name: textField{Value: "prod"}, endpoint: textField{Value: "http://h:9000"}}
	mm, _ := m.submitConnForm()
	if mm.(App).form.err == "" || !strings.Contains(mm.(App).form.err, "already exists") {
		t.Errorf("duplicate name should be rejected; err=%q", mm.(App).form.err)
	}
}

func TestConnFormRejectsBadEndpoint(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "not-a-url"}}
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
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "http://h:9000"}, accessKey: textField{Value: "AK"}, secret: textField{Value: "SK"}}
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
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "http://h:9000"}, accessKey: textField{Value: "AK"}, secret: textField{Value: "SK"}}
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
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "http://h:9000"}, accessKey: textField{Value: "AK"}, secret: textField{Value: "SK"}}
	m = runForm(m)
	if m.form == nil || !strings.Contains(m.form.err, "save failed") {
		t.Errorf("a save error should surface in the form; form=%+v", m.form)
	}
}

// review #1: ctrl+c quits even inside the modal form (where `q`/`:` are literal text).
func TestCtrlCQuitsFromForm(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: textField{Value: "x"}}
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
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "http://h:9000"}} // no access key / secret
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
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "http://h:9000"}, accessKey: textField{Value: "AK"}, secret: textField{Value: "SK"}}
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

// --- 009: first-run opens the add-connection form, save enters the connection ---

func TestFirstRunOpensAddForm(t *testing.T) {
	// No store (no config yet) + a connector → the add-connection form opens at start.
	m := New(Backend{}, "", nil, nil, &fakeConnector{}, preview.ProtoNone)
	if m.mode != modeConnForm || m.form == nil {
		t.Fatalf("first run should open the add form; mode=%v form=%v", m.mode, m.form)
	}
	if cmd := m.Init(); cmd != nil {
		t.Error("first-run Init must not load buckets from a nil store")
	}
	// The full View must render (no nil-store panic) and show the form.
	if v := viewOf(m); !strings.Contains(v, "endpoint") || !strings.Contains(v, "secret") {
		t.Errorf("first-run view should render the add-connection form; got:\n%s", v)
	}
}

func TestFirstRunSaveEntersConnection(t *testing.T) {
	f := storage.NewFake()
	resolve := func(string) (Backend, error) {
		return Backend{Store: f, Cluster: "c", User: "u", Endpoint: "x"}, nil
	}
	m := New(Backend{}, "", nil, resolve, &fakeConnector{names: []string{"newc"}}, preview.ProtoNone)
	m.form = &connForm{
		name:      textField{Value: "newc"},
		endpoint:  textField{Value: "http://h:9000"},
		accessKey: textField{Value: "AK"},
		secret:    textField{Value: "SK"},
	}
	mm, cmd := m.onConnSaved(connSavedMsg{names: []string{"newc"}})
	m = mm.(App)
	if m.mode != modeBuckets {
		t.Errorf("first-run save should enter the new connection (modeBuckets); mode=%v", m.mode)
	}
	if cmd == nil {
		t.Error("entering the connection should dispatch a resolve/load cmd")
	}
	if m.form != nil {
		t.Error("form should be cleared after save")
	}
}

// A subsequent add (an existing active context) still returns to the manager list, not the
// browser — first-run behaviour must not leak into normal adds.
func TestSubsequentSaveReturnsToManager(t *testing.T) {
	m := connApp(&fakeConnector{names: []string{"ctx", "newc"}}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{name: textField{Value: "newc"}, endpoint: textField{Value: "http://h:9000"}}
	mm, _ := m.onConnSaved(connSavedMsg{names: []string{"ctx", "newc"}})
	if mm.(App).mode != modeConnections {
		t.Errorf("a normal add (active ctx present) should return to the manager; mode=%v", mm.(App).mode)
	}
}

// --- 008 US1: discoverable connection delete ---

func TestConnDeleteHintVisibleForNonActive(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx", "other"})
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = 1 // "other" — non-active
	view := m.connectionsView(80, 12)
	if !strings.Contains(view, "delete") {
		t.Errorf("non-active selection must show a delete hint; view:\n%s", view)
	}
	if !strings.Contains(view, glyph(m.keys.DeleteChord[0])) {
		t.Errorf("delete hint must show the chord glyph %q; view:\n%s", glyph(m.keys.DeleteChord[0]), view)
	}
}

func TestConnDeleteHintAbsentOnAddRow(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx", "other"})
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = len(m.contexts) // the "+ add connection" row
	view := m.connectionsView(80, 12)
	if strings.Contains(view, "delete") {
		t.Errorf("the +add row must NOT show a delete hint; view:\n%s", view)
	}
}

func TestConnDeleteHintAbsentForActive(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx", "other"})
	m.ctxName = "ctx"
	mm, _ := m.openConnections()
	m = mm.(App)
	m.connSel = 0 // active "ctx"
	view := m.connectionsView(80, 12)
	if strings.Contains(view, "delete") {
		t.Errorf("the active connection must NOT show a delete hint; view:\n%s", view)
	}
}

// --- 008 US3: usable text entry (paste + caret) ---

func formApp() App {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{pathStyle: true}
	return m
}

func TestConnFormPasteIntoField(t *testing.T) {
	m := formApp()
	m.form.cursor = fldEndpoint
	m = deliver(m, tea.PasteMsg{Content: "https://h:9000\n"}) // trailing newline (clipboard)
	if m.form.endpoint.Value != "https://h:9000" {
		t.Errorf("paste should land whole, newline stripped; got %q", m.form.endpoint.Value)
	}
	if m.mode != modeConnForm {
		t.Errorf("paste must not submit the form; mode=%v", m.mode)
	}
}

func TestConnFormCaretMidFieldInsert(t *testing.T) {
	m := formApp()
	m.form.cursor = fldName
	m = typeStr(m, "htps")
	m = press(m, "left")
	m = press(m, "left")
	m = press(m, "t") // insert between 'ht' and 'ps'
	if m.form.name.Value != "https" {
		t.Errorf("mid-field insert at caret: got %q, want https", m.form.name.Value)
	}
}

func TestConnFormBackspaceAtCaret(t *testing.T) {
	m := formApp()
	m.form.cursor = fldName
	m = typeStr(m, "abcd")
	m = press(m, "left") // caret between c and d
	m = press(m, "backspace")
	if m.form.name.Value != "abd" {
		t.Errorf("backspace at caret should remove char before caret: got %q, want abd", m.form.name.Value)
	}
}

func TestConnFormSecretMaskedAndDrafted(t *testing.T) {
	m := formApp()
	m.form.cursor = fldSecret
	m = deliver(m, tea.PasteMsg{Content: "S3CR3Tkey"})
	view := m.connFormView(80)
	if strings.Contains(view, "S3CR3Tkey") {
		t.Errorf("secret must render masked, not in the clear:\n%s", view)
	}
	if !strings.Contains(view, "•") {
		t.Errorf("secret field should render bullets; view:\n%s", view)
	}
	if got := string(m.form.draft().Secret); got != "S3CR3Tkey" {
		t.Errorf("draft must carry the secret value for save; got %q", got)
	}
}

func TestConnFormCaretNoOpOnToggleRow(t *testing.T) {
	m := formApp()
	m.form.cursor = fldPathStyle
	before := m.form.pathStyle
	m = press(m, "left")  // no caret on a toggle row
	m = press(m, "right") // no panic / no change
	m = deliver(m, tea.PasteMsg{Content: "junk"})
	if m.form.pathStyle != before {
		t.Errorf("caret/paste on a toggle row must not flip it")
	}
	m = press(m, "space") // space still toggles
	if m.form.pathStyle == before {
		t.Errorf("space must still toggle the boolean row")
	}
}

// --- 008 US4: secret + per-field guidance ---

func TestConnFormSecretGuidance(t *testing.T) {
	m := formApp()
	m.form.cursor = fldSecret
	view := m.connFormView(100)
	if !strings.Contains(view, "keychain") {
		t.Errorf("secret guidance must name keychain storage; view:\n%s", view)
	}
	if !strings.Contains(view, "config file") {
		t.Errorf("secret guidance must note other sources are config-file-only; view:\n%s", view)
	}
}

func TestConnFormPerFieldGuidance(t *testing.T) {
	m := formApp()
	for _, fld := range []int{fldName, fldEndpoint, fldAccessKey} {
		m.form.cursor = fld
		if m.connFieldHint(fld) == "" {
			t.Errorf("field %d should have a focused-field expectation hint", fld)
		}
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
