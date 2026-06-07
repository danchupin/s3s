# Phase 1 Data Model: 007 command-bar blocks

UI-layer state + the two thin core additions. No on-disk schema change (config triple
unchanged; deletion removes a triple). Entities below are Go-ish sketches, not final code.

## Command bar (US1/US3)

```go
// blockKind identifies the three labelled groups laid out as columns.
type blockKind int
const ( blockInfo blockKind = iota; blockRead; blockWrite )

// barEntry is one row in a block.
//   - info:  a labelled field (context/cluster/user/region/version) or the add-connection affordance
//   - read:  an action with key+label (always active)
//   - write: an action with key+label + a per-target chord + enabled/dimmed/inapplicable state
type barEntry struct {
    key     string      // displayed key glyph (e.g. "d", "^x"); chord entries show the "^x" form
    label   string      // FR-005a: single imperative verb, ≤2 words, lowercase
    role    styleRole   // info|read|writeActive|writeDimmed|caution (maps to existing palette tokens)
    state   entryState  // active | dimmed (read-only) | inapplicable (wrong selection)
}

type entryState int
const ( entryActive entryState = iota; entryDimmed; entryInapplicable )
```

**Derivation** (no stored block state — computed at render like `availableActions` today):
- `blockInfo` entries from `m.info` + `m.ctxName` + `Version`, plus an add-connection
  affordance whenever `m.connect != nil` (FR-011) — always present.
- `blockRead` entries from the read actions (download, analyze, filter/search, refresh,
  open) — always `entryActive`.
- `blockWrite` entries from the write actions (delete, copy, move, recursive delete,
  upload, new folder). State:
  - `entryDimmed` when `!m.writable()` (read-only or disarmed) — **shown, not hidden**
    (FR-007, reverses 006 FR-004).
  - `entryInapplicable` when the action does not apply to the current selection (e.g.
    recursive delete with no folder selected) — distinct from dimmed (FR-018).
  - `entryActive` otherwise.

**Validation / invariants**:
- All three blocks always rendered (subject to responsive collapse, FR-006/FR-016).
- Dimmed write key press → no mutation + read-only nudge (FR-009) — reuses the existing
  `dispatchActionKey` `writeOnly && !writable()` branch (already sets `ErrReadOnly`).
- Label rule (FR-005a) enforced by a table-driven test over the catalog (SC-014).

## Confirmation surface × tier (US4)

```go
// confirmTier already exists (operation.go): confirmSimple | confirmTyped.
// NEW: the render surface is chosen from the tier + action, not stored as a 3rd enum
// on the op — a pure function of (op.tier, op.kind):
func confirmSurface(op *operation) surface // popupBinary | inlineTyped

type surface int
const ( surfacePopupBinary surface = iota; surfaceInlineTyped )
```

Mapping (FR-024):
| op.kind / situation | tier | surface | typed identifier |
|---------------------|------|---------|------------------|
| delete_object, bulk_delete | binary | popup | — |
| move | binary | popup | — |
| copy/upload overwrite | binary | popup | — |
| delete_recursive | typed | inline | directory **path** (`op.expect = prefix`) |
| delete_bucket (NEW) | typed | inline | **bucket name** (`op.expect = bucket`) |
| delete_connection (NEW, config) | typed | inline | **connection name** |

> NOTE: this REVISES 006 behavior where delete_object/move/recursive all used
> `confirmTyped`. Per spec FR-024, single-object/group/move/overwrite drop to the binary
> tier; only container-removal (recursive/bucket/connection) stays typed. `startRemoveObject`
> and `startMove` change `tier: confirmTyped → confirmSimple`; `startRecursiveDelete` keeps
> typed; new bucket/connection ops use typed.

**Both surfaces** carry the loud `writeBadge`, share palette/label conventions (FR-027a),
cancel on Esc with no mutation (FR-025), and never clip on 80×24 (SC-009). The inline typed
field supports horizontal scroll for long identifiers (FR-023a).

## Progress state (US6)

```go
// opProgress (operation.go) EXTENDED with a determinacy helper — no new fields needed
// beyond what exists (uploaded/total bytes; deleted/failed/total counts):
func (p opProgress) determinate() (frac float64, ok bool) // ok=false → indeterminate
```

- Determinate when a total is known (`upload`: uploaded/total; `bulk_*`/`delete_recursive`
  when a total count is available). `frac` clamped 0..1, monotonic (FR-036).
- Indeterminate (`ok=false`) → spinner/activity fallback, no fabricated percent (FR-037).
- The bar renders only after the "taking a while" threshold (R3) — fast ops show nothing
  (FR-035, SC-013). Cleared when the op leaves `phaseRunning` (FR-036).

## Bucket delete (US4 / storage)

```go
// Mutator gains:
RemoveBucket(ctx context.Context, bucket string) error
// storage.go: new sentinel
var ErrBucketNotEmpty = errors.New("storage: bucket is not empty")
```

- `(*s3Client).RemoveBucket`: `ListObjectsV2(maxKeys=1)`; if non-empty → `ErrBucketNotEmpty`
  (no `DeleteBucket` call); else `DeleteBucket` (FR-024b, SC-015).
- `readOnlyGuard.RemoveBucket` → `ErrReadOnly` (single runtime enforcement point).
- `Fake.RemoveBucket`: removes an empty bucket; `ErrBucketNotEmpty` otherwise (unit tests).
- UI op kind `delete_bucket`; `dispatchOp` routes to `RemoveBucket`; logged via
  `logMutationStart("delete_bucket", "bucket", …)` before execution (Constitution V).

## Connection delete (US5 / config + seam)

```go
// config: symmetric inverse of AddConnection
func (c *Config) RemoveConnection(name string) ([]string, error)
// ui.Connector seam gains:
Delete(ctx context.Context, name string) ([]string, error)
```

- Refuse `name == c.CurrentContext` (FR-032) — also blocked in the UI before dispatch.
- Trial copy with the triple (cluster+user+context) removed → `Validate()` →
  `secret.RemoveKeychain(name)` (best-effort, FR-031) → persist → commit live → return new
  `ContextNames()` (FR-033).
- UI: `modeConnections` binds the delete chord on a non-active row → inline typed-name
  confirm → `connDeleteCmd` → `connDeletedMsg{names, err}`; on success refresh `m.contexts`
  (live, no restart). Logged `connection.delete` (non-secret fields).
- Last/only connection deletable → empty-state render (clarified); no crash on zero
  contexts.

## Keys (US4 additions)

```go
// keyMap gains:
DeleteChord []string // {"ctrl+x"} — object/group/recursive/bucket/connection delete
MoveChord   []string // {"ctrl+o"} — move (ctrl+m is Enter, reserved)
AddConn     []string // visible add-connection affordance key (info block / contexts)
// (overwrite escalation needs no new key)
```

Bare `x`/`X`/`m` become inert for the dangerous action (no mutation) and surface the chord
nudge (FR-021); the write block advertises `^x delete`, `^o move` (FR-026).
