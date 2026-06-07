# Research: Pinned Buckets (010)

Phase 0 decisions. Each: **Decision** / **Rationale** / **Alternatives rejected**. Grounded in a
read-only code map of `internal/config`, `internal/ui`, `cmd/s3s`, `internal/storage` (file:line cited).

## R1 — Where pins are stored in config

**Decision**: add `Buckets []string \`yaml:"buckets,omitempty"\`` to `config.Cluster`
(`internal/config/config.go:53-59`, after `TLSSkipVerify`).

**Rationale**: a `Cluster` already holds endpoint-addressing options (`Endpoint`, `Region`,
`PathStyle`, `TLSSkipVerify`); the set of reachable bucket names is the same kind of metadata. The
add-connection flow maps one connection name onto a same-named cluster+user+context triple
(`connection.go:50-52`), so a cluster field is naturally per-connection. `omitempty` keeps existing
configs byte-identical when empty. `go.yaml.in/yaml/v3` Unmarshal is permissive and `Validate()` does
not reject the new field — **no `Validate()` change, no migration**.

**Alternatives rejected**: (a) `Context.Buckets` — `Context` is a pure cluster+user binding; putting
addressing data there splits it from `Endpoint`. (b) `storage.ClientConfig.Buckets` — storage never
needs bucket names (it's handed a bucket per call); would pollute the storage contract and risk
check-readonly. (c) a new top-level config section — over-engineered for a string list.

## R2 — Pinned vs list-all branch point

**Decision**: a connection is *pinned* iff its resolved `PinnedBuckets` is non-empty. In
`loadBuckets` (`internal/ui/commands.go:35-46`), when pinned, synthesize
`[]storage.Bucket{{Name: n}}` (zero `CreationDate`) and return `bucketsMsg{gen, buckets}` **without**
calling `st.ListBuckets`. Empty → existing `ListBuckets` path. `loadBuckets` gains a `pinned []string`
parameter; the `bucketsMsg` handler (`app.go:344`) is unchanged (same message either way).

**Rationale**: minimal, single branch; honors SC-005 (zero list-all for pinned); the model already
renders `m.buckets` uniformly. The synthesized buckets carry only a name — the view tolerates a zero
date (R5).

**Alternatives rejected**: (a) always call `ListBuckets` then filter to pins — defeats the purpose
(scoped creds get 403). (b) a separate `pinnedBucketsMsg` type — needless; the existing `bucketsMsg`
already carries `[]storage.Bucket`.

## R3 — `+ add bucket` row visibility (FR-013a)

**Decision**: the synthetic `+ add bucket` row is rendered (and selectable) only when the list is
*scoped*: `len(m.pinnedBuckets) > 0` **OR** the last bucket load errored/was denied **OR** returned
zero buckets. Hidden when `ListBuckets` succeeded with ≥1 result. Track a model bool (e.g.
`m.bucketsScoped`) set in the `bucketsMsg`/`errMsg` handlers for `modeBuckets`.

**Rationale**: prevents the footgun where a normal (list-all) connection gains a pin and "loses" its
other buckets with no in-app un-pin (clarify session 2026-06-07). For the motivating scoped case
(list-all 403 / empty), the row appears so the user can bootstrap from nothing.

**Alternatives rejected**: always-visible add row (B in clarify) — footgun; union mode (C) — violates
SC-005 (still calls list-all) and the scoped premise.

## R4 — Add-bucket interaction surface

**Decision**: render a `+ add bucket` row at the end of the bucket list, mirroring
`connRows`/`connectionsView`'s `+ add connection` (`connections.go:85-89`, selection at
`app.go onConnectionsKey:110`). Selecting it (Enter) opens a one-field input via a new
`modeAddBucket` + `bucketAddForm{ name textField; err string }`, reusing `textField`
(`textfield.go`). The row is injected at **render** time in `bucketsView`; `filteredBuckets()` keeps
returning real buckets only, and `onBucketsKey` treats `bucketSel == len(filteredBuckets())` as the
add row (mirror of the `+ add connection` index check).

**Rationale**: maximal consistency with the existing manager idiom; reuses the rune-aware editor and
paste plumbing; the synthetic-row-at-render pattern is already proven for connections.

**Alternatives rejected**: a dedicated keybinding+overlay (needs a new keymap entry + hint-bar slot,
less discoverable); a command-bar `:bucket <name>` command (least discoverable, more parsing).

## R5 — Rendering a pinned bucket row (no creation date)

**Decision**: a synthesized pinned bucket has a zero `CreationDate`; render its date column blank
(or `—`). The `+ add bucket` row renders its name column as the literal `+ add bucket` with a blank
date, exactly like `+ add connection`.

**Rationale**: pinned buckets were never listed, so no real metadata exists; a blank/`—` date is
honest and matches the existing add-row styling.

**Alternatives rejected**: a `HeadBucket`/stat call per pin to fetch a date — extra network, not in
the read interface contract, and pointless for navigation.

## R6 — Reachability probe in `connSeam.Test`

**Decision**: in `cmd/s3s/connection.go` `Test` (`:20-34`), when `d.Buckets` is non-empty, probe
`st.ListLevel(ctx, storage.LevelQuery{Bucket: d.Buckets[0], MaxKeys: 1})`; otherwise keep
`st.ListBuckets`. Return the classified error verbatim (no special-casing in the seam).

**Rationale**: validates exactly what the connection will use (a named bucket), which is what
bucket-scoped creds can actually reach; `ListLevel` is already in the read interface and
integration-tested; `MaxKeys: 1` is a minimal bounded read.

**Alternatives rejected**: `HeadObject` (needs a known key); keeping `ListBuckets` (fails for scoped
creds — the whole problem); probing all pinned buckets (slower; first is a sufficient liveness check).

## R7 — AccessDenied tolerance + honest error message

**Decision**: keep the tolerance + classification in the **UI** (`onConnTested`,
`connections.go:351-362`): treat `msg.err == nil` **or** `errors.Is(msg.err, storage.ErrAccessDenied)`
as success → save; otherwise `m.err = msg.err` and
`m.form.err = m.errorText() + " — press Enter again to save anyway"`. Clear `m.err` on form
cancel/save so a stale test error never leaks into the list footer.

**Rationale**: `errorText()` (`app.go:779-803`) already maps every sentinel to secret-free text;
reusing it removes the lie. Locating the decision in the UI makes it white-box testable with
`fakeConnector{testErr: storage.ErrAccessDenied}` (no live backend). AccessDenied means the host
resolved and answered — reachable, just unprivileged to list — so it must not block save (FR-009).

**Alternatives rejected**: tolerance inside `connSeam.Test` (not UI-testable via the fake; splits the
policy from the message); a brand-new message string (duplicates `errorText`).

## R8 — Persistence of a runtime-added bucket

**Decision**: new `config.AppendBucket(ctxName, bucket string) ([]string, error)` in
`connection.go`, mirroring `AddConnection`/`RemoveConnection`: resolve the context → its cluster,
build a **trial copy** with the normalized bucket appended (`slices.Clone`, trim, dedupe,
order-stable), `Validate()` the trial, `Marshal`+`Save`, then commit to the live `*Config`; log
`connection.bucket-add` (context + bucket + outcome, no secret). Return the cluster's updated bucket
list. Exposed to the UI as `Connector.AddBucket(ctx, ctxName, bucket)`; driven by
`addBucketCmd`→`addBucketMsg{gen, buckets, err}`→`onAddBucket` (mirror
`saveConnCmd`/`connSavedMsg`/`onConnSaved`). `onAddBucket` updates `m.info.PinnedBuckets` +
`m.pinnedBuckets`, closes the add form, and re-runs `beginLoad`+`loadBuckets` to show the new bucket.

**Rationale**: reuses the only blessed config-mutation idiom (trial-validate-persist-commit), so a
disk failure never corrupts the live config; off-loop per Constitution II; additive (no destructive
confirmation needed, Constitution V).

**Alternatives rejected**: session-only (non-persistent) add — contradicts "different buckets, one
connection" persistence (FR-014); direct file write outside the trial idiom — risks corruption;
reusing `AddConnection` — wrong (that creates a new triple).

## R9 — Test-only `storage.Fake` error injection

**Decision**: extend `internal/storage/fake.go`: add `FailListBuckets bool` (→ `ErrAccessDenied` from
`ListBuckets`) and a per-bucket access-denied mechanism for `ListLevel` (e.g. an
`AccessDeniedBuckets map[string]bool`) so a test can model "list-all denied, named bucket reachable."

**Rationale**: needed to test the scoped flow (list-all 403 → show pins + add row) and the probe
(`ListLevel` on a pinned bucket succeeds while `ListBuckets` is denied). These are **read-only** test
knobs — `scripts/check-readonly.sh` only flags write-S3 symbols (`PutObject`, `DeleteObject`, …)
outside `internal/storage`, so adding fields here is permitted and stays green.

**Alternatives rejected**: a full mock package (overkill); driving the real backend in unit tests
(violates the fast-unit posture; integration covers real `ListLevel`).

## Open items deferred to tasks (non-blocking)

- Exact glyph for the blank date column on pinned/add rows (`—` vs empty) — cosmetic, decide in the
  render task.
- Whether `m.bucketsScoped` is a stored bool or a derived helper — implementation detail of R3.
