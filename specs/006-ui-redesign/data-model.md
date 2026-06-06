# Phase 1 Data Model: UI Redesign

This feature is UI-state heavy; the only persisted data is the existing config
model (reused, unchanged on disk). Entities below are the in-memory structures the
redesign introduces, plus the config mapping for a new connection.

## Persisted (config — reused, schema unchanged)

The add-connection writer (R6) maps one form draft onto the **existing**
`internal/config` types (`config.go:52/65/98`):

- **Cluster**: `{Name, Endpoint, Region, PathStyle, TLSSkipVerify}` — derived
  `Name` from the connection display name; `Endpoint`/`Region` from the form.
- **User**: `{Name, AccessKeyID, Keychain: true}` — derived `Name`; `Keychain:
  true` is the single credential source (satisfies the exactly-one-source rule,
  005 FR-041). **No plaintext secret** is written (FR-022/FR-005).
- **Context**: `{Name, Cluster, User, ReadOnly}` — `Name` = display name;
  references the derived cluster/user; `ReadOnly` from the form's flag.

The secret access key is stored out-of-band via `secret.StoreKeychain(account,
secret)` (`keychain.go:36`), where `account = KeychainAccount(ctx)` (`resolve.go:
110`, the user name). Only a `keychain: true` reference lives in the config.

**Validation rules** (reuse `config.Validate`, enforce in the writer/form):
- Required: display name, endpoint; endpoint MUST parse as an absolute URL.
- Uniqueness: derived context/cluster/user names MUST NOT collide with existing
  entries (FR-024) — reject without overwrite.
- Exactly-one credential source per non-anonymous user (here: `keychain`).

## In-memory: ConnDraft (new)

The form's working value; UI-agnostic so it crosses the `Connector` seam (R6).

| Field | Type | Notes |
|-------|------|-------|
| `Name` | string | display/context name; required, unique |
| `Endpoint` | string | required; absolute URL |
| `Region` | string | optional |
| `AccessKeyID` | string | non-secret |
| `Secret` | `logging.Secret` | redacted in logs; never persisted to config |
| `ReadOnly` | bool | maps to `Context.ReadOnly` |

**Lifecycle**: empty → field entry (per-field validation) → `Test` (reachability;
result shown) → `Save` (keychain then config) → context list refreshed → optional
immediate switch. On `Test` failure: offer "save anyway" (FR-025a).

## In-memory: Action (new — drives hint bar + direct keys)

Replaces the `menuItem` struct (`actionmenu.go:13`).

| Field | Type | Notes |
|-------|------|-------|
| `key` | string | the single-key binding (display + dispatch) |
| `label` | string | short verb shown in the hint bar |
| `writeOnly` | bool | hidden/disabled when `!writable()` (FR-004) |
| `available` | func(App) bool | selection/mode gating (object/folder/level/bulk) |
| `invoke` | func(App)(tea.Model,tea.Cmd) | the existing `start*`/`refresh` entry point |

**Derivation**: built from the current `mode`, `selKind()`, `selCount()`, and
`writable()` — the same predicates `menuItemsFor()` used (`actionmenu.go:22`). The
hint bar renders available actions; unavailable write actions are omitted or
greyed (FR-003/FR-004). Bulk variants take over `d`/`x`/`y` when `selCount()>0`
(FR-006).

## In-memory: Command (new — `:` registry)

| Field | Type | Notes |
|-------|------|-------|
| `name` | string | canonical command word (e.g. `conn`, `contexts`, `analyze`) |
| `aliases` | []string | e.g. `q`→`quit`, `ctx`→`contexts` |
| `invoke` | func(App)(tea.Model,tea.Cmd) | jump or action |

Prefix-matched as the user types; unknown name → non-destructive `notice`
(FR-018).

## In-memory: pane state (new — App fields)

Added to `App` (`app.go:90`); none change `m.mode`.

| Field | Type | Purpose |
|-------|------|---------|
| `paneGen` | int | pane-load generation; bumped on selection move (supersede) |
| `paneSelKey` | string | the key the in-flight/loaded pane data belongs to |
| `paneMeta` | `*storage.ObjectMetadata` | debounced HeadObject result |
| `panePrev` | `*preview.Payload` | debounced ranged-GET preview |
| `paneVisible` | bool | toggled/collapsed on narrow terminals (FR-013) |

**Messages** (new, `messages.go`): `paneMetaMsg{gen,key,md}`,
`panePreviewMsg{gen,key,payload}` (do NOT set `modeObject`); `connTestedMsg{err}`,
`connSavedMsg{names,err}`. All carry the generation/identifier needed to drop stale
results (Constitution II).

## Mode enum changes (`app.go:23`)

- **Removed**: `modeActionMenu`.
- **Added**: `modeCommand` (`:` bar), `modeConnections` (connection list),
  `modeConnForm` (add-connection form).
- **Unchanged**: `modeBuckets`, `modeTree`, `modeObject`, `modeContextSwitch`,
  `modeHelp`, `modeUsage`.

## State transitions (high level)

```
modeBuckets ─enter→ modeTree ─enter(obj)→ modeObject ─Esc→ modeTree
   │  │                 │  ├─ a → modeUsage (analyze)
   │  │                 │  ├─ d/x/y/m/u/+ → operation flow (confirm) → modeTree
   │  │                 │  └─ (selection move) → debounced pane load (no mode change)
   │  └─ c / 1–9 → modeContextSwitch ─+→ modeConnForm ─save→ modeContextSwitch
   └─ : → modeCommand ─(conn)→ modeConnections ─+→ modeConnForm
```
