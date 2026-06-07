# Contract: Reachability probe + honest test error

Covers FR-008, FR-009, FR-010, SC-002, SC-003.

## Probe (`cmd/s3s/connection.go` `connSeam.Test`)
- `len(d.Buckets) > 0` ⇒ probe `st.ListLevel(ctx, storage.LevelQuery{Bucket: d.Buckets[0], MaxKeys: 1})`.
- else ⇒ `st.ListBuckets(ctx)` (unchanged).
- Returns the classified error verbatim (`storage.Err*`); no special-casing in the seam.

## Result handling (`internal/ui` `onConnTested`)
- `msg.err == nil` **or** `errors.Is(msg.err, storage.ErrAccessDenied)` ⇒ success: set
  `tested/testOK = true`, return `saveConnCmd(...)`. (Reachable-but-unprivileged = saveable, FR-009.)
- otherwise ⇒ `tested = true, testOK = false`; `m.err = msg.err`;
  `m.form.err = m.errorText() + " — press Enter again to save anyway"`.
- `errorText()` mapping is reused unchanged (`ErrAccessDenied`→"Access denied…",
  `ErrUnreachable`→"Backend unreachable…", `ErrNotFound`→"Not found…",
  `ErrInvalidConfig`→"Invalid configuration…").
- `m.err` is cleared on form cancel (`esc`) and on successful save (`onConnSaved`) so a stale test
  error never leaks into the bucket-list footer.

## Test assertions (white-box ui, `fakeConnector`)
1. `fakeConnector{testErr: nil}` + submit ⇒ saves (existing behavior preserved).
2. `fakeConnector{testErr: storage.ErrAccessDenied}` + submit ⇒ **saves** (treated reachable);
   `m.form.err` not shown as a failure.
3. `fakeConnector{testErr: storage.ErrUnreachable}` ⇒ `m.form.err` contains "Backend unreachable" and
   "press Enter again to save anyway"; second `Enter` saves anyway.
4. `fakeConnector{testErr: storage.ErrNotFound}` ⇒ `m.form.err` contains "Not found", not
   "unreachable".
5. After cancel/save, `m.errorText()` does not bleed into the bucket-list footer (`m.err == nil`).

## Probe assertions (cmd or fake-driven)
6. Draft with `Buckets=["b1"]` ⇒ `Test` calls `ListLevel(b1, MaxKeys:1)`, not `ListBuckets`
   (assert via Fake call counters: `ListBuckets`=0, `ListLevel`≥1).
7. Draft with no buckets ⇒ `Test` calls `ListBuckets` (unchanged).
