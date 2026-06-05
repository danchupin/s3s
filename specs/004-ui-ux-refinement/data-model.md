# Phase 1 Data Model: UI/UX Refinement

This is a presentation feature; "entities" are in-memory view constructs in
`internal/ui`, not persisted data. No storage schema changes.

## Entity: Hint

A single advertised footer action.

| Field | Type | Notes |
|-------|------|-------|
| `key` | string | Display key token, e.g. `enter`, `/`, `d`, `?`. May be a group (`1-9`). |
| `label` | string | Short verb, e.g. `open`, `del`, `help`. |
| `prio` | int | Degrade priority. Higher = kept longer under width pressure. P0 (help/quit) never dropped. |
| `visible` | predicate over `hintCtx` | Returns whether the hint applies to current state. |

**Catalog** (static, see contracts/footer-hints-contract.md for exact priorities &
labels). Derived key tokens reference `defaultKeys()` so they cannot drift from real
bindings.

## Entity: hintCtx (render input)

Pure snapshot the footer hint builder reads — no I/O.

| Field | Type | Source |
|-------|------|--------|
| `mode` | mode | `m.mode` |
| `writable` | bool | `m.writable` |
| `selKind` | enum {none, object, folder} | from `m.selected()` (`isDir`) / nil |
| `searchActive` | bool | `m.search != ""` (tree) or `m.bucketFilter != ""` (buckets) |
| `searching` | bool | `m.searching` (input open) |
| `multiContext` | bool | `len(m.contexts) > 1` |
| `opActive` | bool | `m.op != nil` |
| `width` | int | terminal width |

## Entity: Footer (render output)

Composite bottom region, ≤ 3 rows.

| Row | Content | Condition |
|-----|---------|-----------|
| identity | `● <ctx> [RW|RO]` + optional `· <cluster>` | always (1 row, never wraps) |
| hints | top-`maxHints`(=6) by priority, packed to a single row + optional trailing `? more` | always (1 row, never wraps) |
| status | loading / search / confirm / notice / error | only when a status exists |

Invariant: total rendered rows ≤ 3; every row width ≤ terminal width.

## Entity: HelpSection

A labelled group in the help surface.

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | One of: Navigation, Search & View, Context, Write, Global, Connection |
| `rows` | []HelpRow | action rows (or metadata rows for Connection) |

### HelpRow

| Field | Type | Notes |
|-------|------|-------|
| `keys` | string | All aliases for the action, e.g. `↑/k`, `→/l/Enter`, `q/Ctrl+C`. |
| `desc` | string | Action description. |
| `availability` | enum {always, writeOnly} | Write rows marked/hidden per `m.writable` (FR-013). |

Connection section rows carry metadata (`endpoint`, `region`, `user`, `s3s ver`,
`context`, `cluster`) instead of keybindings; secret-bearing values redacted (FR-021).

## Entity: StatusMessage

Transient status-row content with a visual category.

| Kind | Hue | Example |
|------|-----|---------|
| loading | accent + dim | `⠙ loading object…  (x to cancel)` (named per D6) |
| searchPending | dim | `searching…` |
| prompt | accent/dim | name/dest entry, typed-confirm (target shown alongside input) |
| notice (success) | `colOK` green (`noticeStyle`) | `recursive delete: 42 deleted, 2 failed` |
| error | `colErr` red (`errStyle`) | `error: Not found — the bucket or object does not exist.` |

`notice` and `error` MUST be visually distinct (FR-018): green vs red.

## State transitions

No new state machine. Footer/help/status are pure derivations of existing `App` state;
they add no fields beyond the optional `noticeStyle` definition and (if needed) a
transient "search scheduled" flag already implied by `m.searching` + `m.searchGen`.
