package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/preview"
	"github.com/danchupin/s3s/internal/storage"
)

// 010 pinned buckets — white-box UI tests.

// pinnedApp builds a test App whose active connection declares the given pinned buckets.
func pinnedApp(f *storage.Fake, pins []string) App {
	return New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x", PinnedBuckets: pins},
		"ctx", []string{"ctx"}, nil, nil, preview.ProtoNone)
}

// loadInto runs the bucket-load command (synth for pinned, list-all otherwise) and delivers
// the result, exactly as the event loop would after New/refresh.
func loadInto(m App, f *storage.Fake) App {
	cmd := loadBuckets(m.loadCtx, f, m.gen, m.info.PinnedBuckets)
	return deliver(m, cmd())
}

func TestPinnedBucketsRenderWithoutListBuckets(t *testing.T) {
	f := storage.NewFake()
	f.FailListBuckets = true // bucket-scoped creds: list-all denied
	f.Seed("alpha", "f/a.txt")
	f.Seed("beta", "f/b.txt")

	m := pinnedApp(f, []string{"alpha", "beta"})
	m = loadInto(m, f)

	v := viewOf(m)
	if !strings.Contains(v, "alpha") || !strings.Contains(v, "beta") {
		t.Fatalf("pinned buckets not rendered:\n%s", v)
	}
	if f.ListBucketsCalls != 0 {
		t.Errorf("ListBucketsCalls = %d, want 0 (scoped → no list-all)", f.ListBucketsCalls)
	}

	// Enter opens the first pinned bucket; switching to the second opens it too — neither
	// path calls ListBuckets.
	mAlpha := press(m, "enter")
	if mAlpha.mode != modeTree || mAlpha.bucket != "alpha" {
		t.Fatalf("enter: mode=%v bucket=%q, want modeTree/alpha", mAlpha.mode, mAlpha.bucket)
	}
	mBeta := press(press(m, "down"), "enter")
	if mBeta.bucket != "beta" {
		t.Fatalf("down+enter: bucket=%q, want beta", mBeta.bucket)
	}
	if f.ListBucketsCalls != 0 {
		t.Errorf("after open/switch: ListBucketsCalls = %d, want 0", f.ListBucketsCalls)
	}
}

// bucketsConnApp builds a pinned App wired to a Connector (so the "+ add bucket" row shows).
func bucketsConnApp(f *storage.Fake, pins []string, conn Connector) App {
	return New(Backend{Store: f, Cluster: "c", User: "u", Endpoint: "http://x", PinnedBuckets: pins},
		"ctx", []string{"ctx"}, nil, conn, preview.ProtoNone)
}

func TestAddBucketRowShownOnlyWhenScoped(t *testing.T) {
	// Scoped (pinned) → row shown.
	f := storage.NewFake()
	f.FailListBuckets = true
	f.Seed("alpha", "x")
	m := bucketsConnApp(f, []string{"alpha"}, &fakeConnector{})
	m = loadInto(m, f)
	if !strings.Contains(viewOf(m), "+ add bucket") {
		t.Errorf("scoped (pinned) list must show + add bucket:\n%s", viewOf(m))
	}

	// Working list-all with results → row hidden.
	f2 := storage.NewFake()
	f2.Seed("one", "x")
	f2.Seed("two", "y")
	m2 := bucketsConnApp(f2, nil, &fakeConnector{})
	m2 = loadInto(m2, f2)
	if strings.Contains(viewOf(m2), "+ add bucket") {
		t.Errorf("working list-all must NOT show + add bucket:\n%s", viewOf(m2))
	}

	// Scoped via list-all FAILURE (no pins) → row shown so the user can bootstrap.
	f3 := storage.NewFake()
	f3.FailListBuckets = true
	m3 := bucketsConnApp(f3, nil, &fakeConnector{})
	m3 = loadInto(m3, f3) // loadBuckets calls ListBuckets → errMsg → bucketsScoped
	if !strings.Contains(viewOf(m3), "+ add bucket") {
		t.Errorf("list-all failure must show + add bucket:\n%s", viewOf(m3))
	}
}

func TestAddBucketFlowPersistsAndReflects(t *testing.T) {
	conn := &fakeConnector{addBuckets: []string{"alpha", "gamma"}}
	f := storage.NewFake()
	f.FailListBuckets = true
	f.Seed("alpha", "x")
	f.Seed("gamma", "y")
	m := bucketsConnApp(f, []string{"alpha"}, conn)
	m = loadInto(m, f)

	// Move to the "+ add bucket" row (index 1, past the single pinned bucket) and open it.
	m = press(m, "down")
	m = press(m, "enter")
	if m.mode != modeAddBucket || m.addForm == nil {
		t.Fatalf("enter on + add bucket should open modeAddBucket; mode=%v", m.mode)
	}

	// Type a name and submit → connector persists; onAddBucket updates pins + returns to list.
	m = typeStr(m, "gamma")
	m, cmd := pressCmd(m, "enter")
	if cmd == nil {
		t.Fatal("enter on add-bucket form should issue a persist command")
	}
	m = deliver(m, cmd()) // addBucketMsg → onAddBucket
	if conn.addedBucket != "gamma" {
		t.Errorf("AddBucket should receive %q, got %q", "gamma", conn.addedBucket)
	}
	if m.mode != modeBuckets || m.addForm != nil {
		t.Fatalf("after add, should return to bucket list; mode=%v addForm=%v", m.mode, m.addForm)
	}
	// The reload (the batch onAddBucket returned) re-synthesizes from the updated pins.
	m = loadInto(m, f)
	if !strings.Contains(viewOf(m), "gamma") {
		t.Errorf("added bucket should appear in the list:\n%s", viewOf(m))
	}
}

func TestAddBucketEscCancels(t *testing.T) {
	conn := &fakeConnector{}
	f := storage.NewFake()
	f.FailListBuckets = true
	f.Seed("alpha", "x")
	m := bucketsConnApp(f, []string{"alpha"}, conn)
	m = loadInto(m, f)
	m = press(m, "down")  // → + add bucket row
	m = press(m, "enter") // open
	m = typeStr(m, "junk")
	m = press(m, "esc")
	if m.mode != modeBuckets || m.addForm != nil {
		t.Fatalf("esc should cancel back to the bucket list; mode=%v", m.mode)
	}
	if conn.addedBucket != "" {
		t.Errorf("esc must not persist anything; addedBucket=%q", conn.addedBucket)
	}
}

// T030 — edge cases.

func TestPinnedFilterNarrows(t *testing.T) {
	f := storage.NewFake()
	f.FailListBuckets = true
	m := pinnedApp(f, []string{"alpha", "beta", "gamma"})
	m = loadInto(m, f)
	m.bucketFilter = "be" // client-side name filter
	v := viewOf(m)
	if !strings.Contains(v, "beta") || strings.Contains(v, "alpha") || strings.Contains(v, "gamma") {
		t.Errorf("filter %q should show only beta:\n%s", m.bucketFilter, v)
	}
}

func TestRefreshPinnedKeepsNoListBuckets(t *testing.T) {
	f := storage.NewFake()
	f.FailListBuckets = true
	f.Seed("alpha", "x")
	m := pinnedApp(f, []string{"alpha"})
	m = loadInto(m, f)
	// Refresh re-arms the load; the pinned path must still avoid ListBuckets.
	mm, _ := m.refreshBuckets()
	m = mm.(App)
	m = loadInto(m, f)
	v := viewOf(m) // use the reloaded model so the refresh path is fully exercised
	if !strings.Contains(v, "alpha") {
		t.Errorf("refreshed pinned list should still show alpha:\n%s", v)
	}
	if f.ListBucketsCalls != 0 {
		t.Errorf("refresh on a pinned list called ListBuckets %d times, want 0", f.ListBucketsCalls)
	}
}

func TestScopedEmptyListAllShowsAddRow(t *testing.T) {
	// list-all succeeds but returns ZERO buckets → scoped → "+ add bucket" available.
	f := storage.NewFake() // no buckets seeded; FailListBuckets=false
	m := bucketsConnApp(f, nil, &fakeConnector{})
	m = loadInto(m, f)
	if f.ListBucketsCalls != 1 {
		t.Errorf("empty list-all should still call ListBuckets once, got %d", f.ListBucketsCalls)
	}
	if !strings.Contains(viewOf(m), "+ add bucket") {
		t.Errorf("an empty list-all result should offer + add bucket:\n%s", viewOf(m))
	}
}

func TestParseBucketsNormalizes(t *testing.T) {
	got := parseBuckets("a, b  c,,a\tb")
	if strings.Join(got, ",") != "a,b,c" {
		t.Errorf("parseBuckets = %v, want [a b c]", got)
	}
	if parseBuckets("   ") != nil {
		t.Errorf("blank input should yield nil, got %v", parseBuckets("   "))
	}
}

func TestConnFormBucketsFieldSaves(t *testing.T) {
	conn := &fakeConnector{names: []string{"ctx", "newc"}}
	m := connApp(conn, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{
		name:      textField{Value: "newc"},
		endpoint:  textField{Value: "https://bucket.avito-sd"},
		accessKey: textField{Value: "AK"},
		secret:    textField{Value: "SK"},
		buckets:   textField{Value: "alpha, beta"},
	}
	if d := m.form.draft(); strings.Join(d.Buckets, ",") != "alpha,beta" {
		t.Errorf("draft Buckets = %v, want [alpha beta]", d.Buckets)
	}
	m = runForm(m)
	if strings.Join(conn.savedDraft.Buckets, ",") != "alpha,beta" {
		t.Errorf("saved draft Buckets = %v, want [alpha beta]", conn.savedDraft.Buckets)
	}
}

// Regression: adding fldBuckets shifted the boolean rows; space must still toggle them and
// must insert (not toggle) on the buckets field.
func TestConnFormSpaceAfterIndexShift(t *testing.T) {
	m := connApp(&fakeConnector{}, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{}

	m.form.cursor = fldBuckets
	m = press(m, " ")
	if m.form.buckets.Value != " " {
		t.Errorf("space at fldBuckets should insert a space, got %q", m.form.buckets.Value)
	}
	m.form.cursor = fldPathStyle
	m = press(m, " ")
	if !m.form.pathStyle {
		t.Error("space at fldPathStyle should toggle pathStyle on")
	}
	m.form.cursor = fldReadOnly
	m = press(m, " ")
	if !m.form.readOnly {
		t.Error("space at fldReadOnly should toggle readOnly on")
	}
}

// US4 — honest connection-test error + AccessDenied tolerance.

func connFormFor(conn Connector) App {
	m := connApp(conn, []string{"ctx"})
	m.mode = modeConnForm
	m.form = &connForm{
		name:      textField{Value: "newc"},
		endpoint:  textField{Value: "https://bucket.avito-sd"},
		accessKey: textField{Value: "AK"},
		secret:    textField{Value: "SK"},
	}
	return m
}

func TestConnTestAccessDeniedTreatedReachable(t *testing.T) {
	conn := &fakeConnector{testErr: storage.ErrAccessDenied, names: []string{"ctx", "newc"}}
	m := runForm(connFormFor(conn))
	if conn.savedName != "newc" {
		t.Errorf("AccessDenied must be treated reachable and SAVE; savedName=%q", conn.savedName)
	}
	if m.err != nil {
		t.Errorf("m.err must be cleared after save; got %v", m.err)
	}
}

func TestConnTestUnreachableShowsClassifiedAndSavesAnyway(t *testing.T) {
	conn := &fakeConnector{testErr: storage.ErrUnreachable, names: []string{"ctx", "newc"}}
	m := runForm(connFormFor(conn))
	if conn.savedName != "" {
		t.Errorf("unreachable must not save on first submit; savedName=%q", conn.savedName)
	}
	if m.form == nil || !strings.Contains(m.form.err, "Backend unreachable") || !strings.Contains(m.form.err, "save anyway") {
		t.Fatalf("unreachable should show the classified reason + save-anyway; err=%q", m.form.err)
	}
	// Second submit overrides → saves anyway.
	m = runForm(m)
	if conn.savedName != "newc" {
		t.Errorf("second submit should save anyway; savedName=%q", conn.savedName)
	}
}

func TestConnTestNotFoundNotLabelledUnreachable(t *testing.T) {
	conn := &fakeConnector{testErr: storage.ErrNotFound, names: []string{"ctx", "newc"}}
	m := runForm(connFormFor(conn))
	if m.form == nil || !strings.Contains(m.form.err, "Not found") {
		t.Fatalf("not-found should show its own message; err=%q", m.form.err)
	}
	if strings.Contains(m.form.err, "unreachable") || strings.Contains(m.form.err, "Backend unreachable") {
		t.Errorf("not-found must NOT be mislabelled unreachable; err=%q", m.form.err)
	}
}

func TestNonPinnedConnectionListsAll(t *testing.T) {
	f := storage.NewFake()
	f.Seed("one", "x")
	f.Seed("two", "y")

	m := pinnedApp(f, nil) // no pins → unchanged list-all behavior
	m = loadInto(m, f)

	v := viewOf(m)
	if !strings.Contains(v, "one") || !strings.Contains(v, "two") {
		t.Fatalf("list-all buckets not rendered:\n%s", v)
	}
	if f.ListBucketsCalls != 1 {
		t.Errorf("ListBucketsCalls = %d, want 1 (list-all path)", f.ListBucketsCalls)
	}
}
