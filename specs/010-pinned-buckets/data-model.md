# Data Model: Pinned Buckets (010)

Entities, new fields, the form field-index shift, and normalization rules. Citations are to current
code; line numbers are approximate anchors.

## Entities & field additions

### `config.Cluster` (`internal/config/config.go:53-59`)
Add one field:
```go
type Cluster struct {
    Name          string   `yaml:"name"`
    Endpoint      string   `yaml:"endpoint"`
    Region        string   `yaml:"region,omitempty"`
    PathStyle     bool     `yaml:"pathStyle"`
    TLSSkipVerify bool     `yaml:"tlsSkipVerify,omitempty"`
    Buckets       []string `yaml:"buckets,omitempty"` // NEW: pinned bucket names (010)
}
```
- Optional. Empty/nil ⇒ behavior unchanged (list-all). `omitempty` ⇒ existing configs byte-identical.
- Ordered, normalized (see rules). Non-secret.

### `config.NewConnection` (`internal/config/connection.go:13-20`)
Add `Buckets []string`. `AddConnection` (`:50`) copies it into the trial `Cluster`:
`cl := Cluster{Name, Endpoint, Region, PathStyle, Buckets: nc.Buckets}`.

### `ui.Backend` (`internal/ui/app.go:77-87`)
Add `PinnedBuckets []string`. Populated in `cmd/s3s/main.go backendFrom` (`:105-128`) from the
resolved `Cluster.Buckets` (`cfg.Resolve(name)` already returns the cluster). Flows on context switch
automatically (the `resolve` closure → `backendFrom`).

### `ui.App` (model) (`internal/ui/app.go`)
- `pinnedBuckets []string` — seeded in `New()` from `initial.PinnedBuckets`; refreshed from
  `contextResolvedMsg.be.PinnedBuckets` on switch and from `onAddBucket` after a runtime add.
- `bucketsScoped bool` (or derived helper) — true when the current bucket list is scoped (pins exist,
  or the last load errored/was empty); gates the `+ add bucket` row (FR-013a).
- `addForm *bucketAddForm` — non-nil while `modeAddBucket` is active.

### `ui.ConnDraft` (`internal/ui/connections.go:19-27`)
Add `Buckets []string`. Produced by `connForm.draft()` (`:278-289`) from the new `buckets` text field
via the normalization parse. Consumed by `Connector.Save` → `NewConnection.Buckets` and by
`Connector.Test` (probe target).

### `ui.connForm` (`internal/ui/connections.go:61-70`)
Add `buckets textField` between `secret` and `pathStyle`. New const `fldBuckets` and label `"buckets"`.

### `bucketAddForm` (NEW, `internal/ui/connections.go`)
```go
type bucketAddForm struct {
    name textField
    err  string
}
```
Single-field input for the runtime `+ add bucket` flow (`modeAddBucket`). Mirrors `connForm` minimal.

### `addBucketMsg` (NEW, `internal/ui/messages.go`)
```go
type addBucketMsg struct {
    gen     int
    buckets []string // updated pinned list for the active connection
    err     error
}
```
Carries `gen` for staleness drop (Constitution II), mirroring `connSavedMsg`/`connDeletedMsg`.

## Form field-index shift (`connections.go:44-55`)

Adding `fldBuckets` after `fldSecret` shifts the two boolean rows by +1:

| const          | before | after |
|----------------|:------:|:-----:|
| `fldName`      | 0      | 0     |
| `fldEndpoint`  | 1      | 1     |
| `fldRegion`    | 2      | 2     |
| `fldAccessKey` | 3      | 3     |
| `fldSecret`    | 4      | 4     |
| `fldBuckets`   | —      | 5     |
| `fldPathStyle` | 5      | 6     |
| `fldReadOnly`  | 6      | 7     |
| `connFieldCount`| 7     | 8     |

Touch points that MUST stay aligned:
- `connFieldLabels` (`:55`): insert `"buckets"` before `"path-style"` → 8 labels.
- `focusField()` (`:238-252`): add `case fldBuckets: return &f.buckets` (before the boolean
  fall-through `return nil`).
- `connFormView` (`:432+`): the `fields` slice becomes
  `[]textField{name, endpoint, region, accessKey, secret, buckets}` (6 text fields); render
  `fldBuckets` as a normal text row (not a checkbox); the space-toggle switch (`:218-226`) must NOT
  add a `fldBuckets` case (it falls through to text append).
- `connFieldHint()` (`:416-430`): add `case fldBuckets` → e.g.
  `"comma/space-separated bucket names — pin these when credentials can't list all (optional)"`.

## Normalization rules (shared)

Applied by `connForm.draft()` (parse the `buckets` field) **and** `config.AppendBucket`:
1. Split on commas and/or whitespace.
2. `strings.TrimSpace` each token.
3. Drop empty tokens.
4. De-duplicate, **preserving first-seen order** (stable).
5. Result is `[]string` (nil/empty allowed).

`AppendBucket(ctxName, bucket)` additionally: trims the single name, rejects empty
(`ErrInvalid`), and is **idempotent** — appending an already-present name is a no-op that still
returns the current list and succeeds.

## State transitions

- **Connection lifecycle**: a connection is *pinned* iff `len(Cluster.Buckets) > 0`. Adding the first
  bucket (only reachable via the UI when the list is scoped, FR-013a/FR-016) transitions it to the
  pinned model; subsequent loads skip list-all. No automatic transition back (un-pin is out of scope
  v1; user edits config).
- **Bucket-add flow**: `modeBuckets` --(Enter on `+ add bucket`)--> `modeAddBucket` --(Enter, valid
  name)--> `addBucketCmd` → `addBucketMsg` → `onAddBucket` (persist ok) → back to `modeBuckets` with
  the new bucket present; --(Esc)--> `modeBuckets` unchanged.

## Persistence shape (YAML example)

```yaml
clusters:
  - name: avito-staging
    endpoint: https://bucket.avito-sd
    pathStyle: false
    buckets:                       # NEW — pinned set; absent ⇒ list-all
      - st-img-range-bucket-1416
      - some-other-bucket
contexts:
  - name: avito-staging
    cluster: avito-staging
    user: avito-staging
```
A cluster with no `buckets:` key loads with `Buckets == nil` and behaves exactly as before.
