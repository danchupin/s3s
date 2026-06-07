# Contract: In-app "+ add bucket" (runtime add + persist)

Covers FR-013, FR-013a, FR-014, FR-015, FR-016, SC-006.

## Visibility (FR-013a) — the `+ add bucket` row is shown iff the list is *scoped*:
`len(m.pinnedBuckets) > 0` **OR** the last bucket load errored/was denied **OR** returned 0 buckets.
Hidden when `ListBuckets` succeeded with ≥1 result. Rendered as the last row of `bucketsView`
(mirror `+ add connection`); not part of `filteredBuckets()` (injected at render).

## Trigger + input
- `Enter` with selection on the `+ add bucket` row (`bucketSel == len(filteredBuckets())`) ⇒
  `m.mode = modeAddBucket`, `m.addForm = &bucketAddForm{}`.
- `modeAddBucket` edits a single `textField` (`name`): runes/paste insert, backspace/delete, caret
  moves — same plumbing as `connForm` (paste routed in `onPaste`).
- `Esc` ⇒ cancel: `m.addForm = nil`, `m.mode = modeBuckets`, list unchanged.
- `Enter` with empty name ⇒ inline `addForm.err`, no command.

## Submit → persist → reflect
- Valid name ⇒ `addBucketCmd(connect, ctxName, name, gen)` → `Connector.AddBucket` (off-loop).
- `addBucketMsg{gen, buckets, err}`:
  - stale (`gen != m.gen`) ⇒ dropped.
  - `err != nil` ⇒ `m.err = err` (rendered via `errorText`), stay in `modeAddBucket` (or surface and
    let user retry/cancel).
  - success ⇒ `m.pinnedBuckets = buckets`, `m.info.PinnedBuckets = buckets`, `m.addForm = nil`,
    `m.mode = modeBuckets`, re-run `beginLoad`+`loadBuckets` so the new bucket appears.
- Normalization (FR-015): dup name ⇒ no duplicate row; trim applied.

## Persistence (FR-014)
- `Connector.AddBucket` → `config.AppendBucket` writes to disk (trial-validate-persist) so the bucket
  is present after restart. Off the UI loop.

## Test assertions (white-box ui + config)
1. Scoped list (`FailListBuckets=true`, no pins): `viewOf(m)` shows `+ add bucket`.
2. List-all success with results: `viewOf(m)` does **not** show `+ add bucket`.
3. Select add row + `Enter` ⇒ `m.mode == modeAddBucket`; `typeStr(m, "alpha")` then `Enter` with a
   `fakeConnector{names/buckets: ["alpha"]}` ⇒ `m.mode == modeBuckets`, `viewOf` contains `alpha`,
   `fakeConnector.addedBucket == "alpha"`.
4. `Esc` in `modeAddBucket` ⇒ list unchanged.
5. Add duplicate ⇒ single row (no dup).
6. (config) After `AppendBucket`, reload config ⇒ bucket persisted (cross-ref config-schema.md).
