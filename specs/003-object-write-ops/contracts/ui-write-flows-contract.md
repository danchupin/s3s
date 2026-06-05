# Contract: UI Write Flows — File Browser, Destination Entry, Progress (003)

**Packages**: `internal/ui`, `internal/localfs` | **Feature**: 003-object-write-ops

Defines the UI-side contract for the five object operations: how each is triggered,
which confirmation tier it uses, the local file browser, destination-key entry, and
the streaming-progress mechanism. Reuses the 002 confirmation overlay and
operation/generation model unchanged where possible.

## Triggers & tiers

| Op | Trigger (on selection) | Pre-confirm phase | Tier | Typed `expect` |
|----|------------------------|-------------------|------|----------------|
| Delete object | `d` on an object | — | typed | object key |
| Upload | `u` in a level | `phaseBrowse` | simple, or typed if target key exists | target key (overwrite) |
| Copy | `c` on an object | `phaseDest` | simple, or typed if dst exists | dst key (overwrite) |
| Move/rename | `m` on an object | `phaseDest` | typed (source removed) | dst key |
| Recursive delete | `D` (shift-d) on a folder/prefix | — | typed | the prefix |

A read-only context (or `--write` off) refuses the trigger immediately with the
read-only hint and issues no command (FR-012) — same guard pattern as 002
`startCreateFolder` (`if !m.writable { m.err = storage.ErrReadOnly; return m, nil }`).

## Local file browser (`internal/localfs` + `filebrowser.go`)

`internal/localfs` (UI-agnostic, unit-tested):

```go
package localfs

type Entry struct {
    Name  string
    Path  string // absolute
    IsDir bool
    Size  int64
}

// ReadDir lists dir's entries, directories first then files, each alphabetical.
// Returns a classifiable error on unreadable directories.
func ReadDir(dir string) ([]Entry, error)

// IsReadableFile returns nil if path is an existing, readable regular file;
// otherwise a classifiable error. Called before dispatching an upload.
func IsReadableFile(path string) error
```

`internal/ui/filebrowser.go` (thin renderer + key handling), active during
`phaseBrowse`:
- Starts at the process working directory.
- `up/down` move selection; `enter` on a dir descends (`ReadDir(entry.Path)`),
  `enter` on a file selects it (sets `op.localPath`, `op.localSize`, advances to
  overwrite-check → confirm); `esc`/`backspace` ascends to the parent (or cancels
  at the root); `left`/`h` ascends, `right`/`l` descends.
- Renders within the existing bordered-box budget; long listings window like the
  object list (`windowBounds`).

## Destination-key entry (`phaseDest`)

- Reuses the create-folder name-input rendering. Prefilled with the source key so
  rename only edits the tail.
- On `enter`: validate (non-empty, no control chars, `dst != src`); invalid → show
  guidance, stay in `phaseDest` (FR-013). Valid → overwrite-check → confirm.
- `esc` cancels the operation.

## Overwrite check (advisory)

After the source/destination is known (upload target = `parent + filename`; copy/move
= `dstKey`), the UI checks the **current loaded level** for that key. If present, set
`op.tier = confirmTyped`, `op.expect = <target key>`, and the confirm overlay shows
an "overwrite" message. If absent, upload/copy use `confirmSimple`. Move is always
typed regardless (source removal). This mirrors 002's advisory "already exists"
check; it is best-effort, not a server precondition (see R6).

## Streaming progress (upload, recursive delete)

Pattern (idiomatic Bubble Tea, non-blocking):

```go
// dispatch: start work in a goroutine, return the first waitForProgress.
// (Method names are guard-safe — UploadFile, not UploadObject — so this UI file
// passes scripts/check-readonly.sh.)
func uploadCmd(ctx context.Context, mut storage.Mutator, bucket, key string,
    r io.Reader, size int64, ch chan opProgress, gen int) tea.Cmd {
    go func() {
        defer close(ch)
        cr := &countingReader{R: r, total: size, ch: ch} // throttled sends
        err := mut.UploadFile(ctx, bucket, key, cr, size)
        ch <- opProgress{done: true, err: err}            // terminal marker
    }()
    return waitForProgress(ch, gen)
}

// waitForProgress reads ONE update and re-issues itself until the terminal marker.
func waitForProgress(ch chan opProgress, gen int) tea.Cmd {
    return func() tea.Msg {
        p, ok := <-ch
        if !ok || p.done {
            return operationDoneMsg{gen: gen, err: p.err, summary: p.summary, partial: p.partial}
        }
        return operationProgressMsg{gen: gen, progress: p}
    }
}
```

- Update handles `operationProgressMsg` by storing `op.progress` and re-issuing
  `waitForProgress(ch, gen)`; it handles `operationDoneMsg` as terminal.
- Throttle: the counting reader / batch callback sends at most ~1 update / 50 ms so
  a fast op does not flood the loop; the first update still appears ≤100 ms (SC-007).
- Generation drop: a progress/done msg whose `gen` != `m.gen` is ignored (superseded
  navigation/cancel), per Constitution II / FR-010.
- Recursive delete uses the same shape: the `onProgress(DeleteSummary)` callback
  feeds `ch`; the terminal marker carries `summary` + `partial = summary.Failed>0`.

## Outcome handling

`operationDoneMsg` (extended with `summary *storage.DeleteSummary`, `partial bool`):
- Success (`err == nil && !partial`): clear `op`, log outcome, invalidate the
  affected level(s) (R10), reload.
- Partial (`partial == true`, e.g. recursive `Failed>0` or `ErrMovePartial`): clear
  `op`, show a partial message (e.g. "deleted 12, 3 failed" / "copied; source
  remains"), log the partial outcome, invalidate + reload — never a clean-success
  message (FR-011, SC-005/006).
- Error (`err != nil`, incl. `context.Canceled`): clear `op`, show a non-leaking
  error, log the outcome, reload the level so the view is truthful (FR-015/FR-016).
- `ErrReadOnly` never reaches here (refused at trigger), but is handled defensively.

## Logging (reuse 002 `logMutationStart`/`logMutationDone`)

Before dispatch: log `action` (delete/upload/copy/move/delete_recursive), `bucket`,
`source`, `destination` (where applicable), `context`. After: log outcome; for
recursive delete include `deleted`/`failed` counts. File log only, secrets redacted
(FR-014/SC-009).

## Test contract (white-box `package ui`, `deliver`/`press`)

- Delete: `d` → typed overlay; wrong text aborts (no command); exact match dispatches;
  done → object gone after reload; read-only context refuses with hint.
- Upload: `u` → file browser; navigate + select a fixture file; non-colliding →
  simple confirm; colliding target → typed overwrite; progress messages update the
  view; cancel mid-flight → not a success.
- Copy: `c` → dest entry prefilled with source key; dst==src rejected; free dst →
  simple; existing dst → typed overwrite; done duplicates in the listing.
- Move: `m` → dest entry; typed confirm; clean move shows only the destination;
  injected `ErrMovePartial` → partial message, both keys still listed.
- Recursive delete: `D` → typed confirm of the prefix; progress shows running
  deleted/failed; partial (`Failed>0`) → partial message, not clean success; cancel
  → partial/cancelled, never success.
- Generation: a progress/done msg from a superseded gen is dropped (no view change).
- `localfs`: unit tests for `ReadDir` ordering/hidden-files/errors and
  `IsReadableFile` on file/dir/missing.
