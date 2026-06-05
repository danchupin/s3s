# Phase 1 Data Model: UI/UX Refinement

Presentation feature; "entities" are in-memory view constructs in `internal/ui`. No
storage schema change.

## Entity: MenuItem

A single action in the contextual action menu.

| Field | Type | Notes |
|-------|------|-------|
| `label` | string | Display text, e.g. `delete`, `move / rename`, `refresh`. |
| `invoke` | `func() (tea.Model, tea.Cmd)` | Bound to an existing entry point (`startRemoveObject`, `startCopy`, `startMove`, `startUpload`, `startCreateFolder`, `startRecursiveDelete`, `refresh`). |
| `writeOnly` | bool | True for mutating items (hidden in read-only). |

## Entity: menuCtx (render/build input)

Pure snapshot used to build the item list — no I/O.

| Field | Type | Source |
|-------|------|--------|
| `mode` | mode | `m.mode` (buckets vs tree) |
| `writable` | bool | `m.writable` |
| `selKind` | enum {none, object, folder} | from `m.selected()` (`isDir`) / nil |

Build rules (see contracts/action-menu-contract.md C2): buckets→[Refresh];
tree+RO→[Refresh]; tree+writable gated by `selKind`; Refresh always last/present.

## Entity: Action menu (mode state)

| Field | Type | Notes |
|-------|------|-------|
| `mode == modeActionMenu` | mode flag | Active while the menu is open. |
| `menuItems` | []MenuItem | Built on open from `menuCtx`. |
| `menuSel` | int | Selected index (stateless window like other lists). |
| `prevMode` | mode | Restored on close (reuse the existing help-overlay pattern). |

Invariants: opening/closing does not cancel a background load; choosing an item transitions
into the existing `operation` flow (the menu adds no operation/confirmation behavior).

## Entity: Hint (footer)

| Field | Type | Notes |
|-------|------|-------|
| `key` | string | Display token; nav tokens render arrow glyphs (`↑/↓`, `Enter`, `Esc`), not vim letters. |
| `label` | string | Short verb, e.g. `open`, `actions`, `help`. |
| `prio` | int | Degrade priority; P0 (`? help`, `q quit`) never dropped. |
| `visible` | predicate over `hintCtx` | Applies to current state. |

The write-op hints (`d/u/y/m/D/+`) and `r`/`x` are **removed**; a single `a actions` hint
replaces them.

## Entity: Footer (render output, ≤ 3 rows)

| Row | Content | Condition |
|-----|---------|-----------|
| identity | `● <ctx> [RW|RO]` + optional `· <cluster>` | always (1 row) |
| hints | top-`maxHints`(=6) by priority, single row + optional `? more` | always (1 row) |
| status | loading(named)/search-pending/prompt/notice/error | only when present |

## Entity: HelpSection

| Field | Type | Notes |
|-------|------|-------|
| `title` | string | Navigation / Search & View / Actions / Context / Global / Connection |
| `rows` | []HelpRow | action rows (or metadata rows for Connection) |

### HelpRow

| Field | Type | Notes |
|-------|------|-------|
| `keys` | string | All aliases incl. vim (e.g. `↑/k`, `←/h/Esc`, `q/Ctrl+C`). |
| `desc` | string | Action description. |
| `availability` | enum {always, writeOnly} | Write rows reflect `m.writable`. |

The **Actions** section documents the `a` menu key and lists the menu's items (marking
write-only). The **Connection** section sources only non-secret `Backend` display fields +
`ctxName`/`Version` (redaction guard, FR-021).

## Entity: StatusMessage

| Kind | Hue | Example |
|------|-----|---------|
| loading | accent + dim | `⠙ loading object…  (Esc to cancel)` (named; Esc-cancel per FR-029) |
| searchPending | dim | `searching…` |
| prompt | accent/dim | name/dest entry, typed-confirm (target shown alongside input) |
| notice (success) | `colOK` green (`noticeStyle`) | `recursive delete: 42 deleted, 2 failed` |
| error | `colErr` red (`errStyle`) | `error: Not found …` |

## State transitions

New mode edge: `modeBuckets`/`modeTree` --`a`--> `modeActionMenu` --`Esc`--> back to prev
mode; --`Enter` on item--> existing `operation` flow (phaseName/phaseDest/phaseConfirm/…).
Cancel edge: while `m.loading` or `op.phase==phaseRunning`, Back/Esc → `cancelLoad()`
(replaces the removed `x`). No other state machine changes.
