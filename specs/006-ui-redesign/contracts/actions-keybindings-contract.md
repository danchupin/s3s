# Contract: Menu-less direct actions + hint bar (US1)

Deletes `modeActionMenu` and `internal/ui/actionmenu.go`. Reuses the selection/
capability gating that built `menuItemsFor()` to drive an always-visible hint bar
and the direct-key dispatch table.

## A1 — Keybindings (FR-002, replaces `defaultKeys()`)

| Action | Key | Scope / availability | Write-gated |
|--------|-----|----------------------|-------------|
| Analyze (`du`) | `a` | folder/level/bucket selection | no (read) |
| Download | `d` | object selected, OR ≥1 marked → bulk download | no (read) |
| Delete object | `x` | object selected (bulk delete when ≥1 marked) | yes |
| Recursive delete | `X` | folder selected | yes |
| Copy | `y` | object selected (bulk copy when ≥1 marked) | yes |
| Move/rename | `m` | object selected | yes |
| Upload here | `u` | any level | yes |
| New folder | `+` | any level | yes |
| Refresh | `r` | any list | no |
| Mark | `space` | object row (objects only) | no |
| Sort / dir | `s` / `S` | any level | no |
| Write toggle | `w` | global | n/a |
| Context switch | `c`, `1`–`9` | global | no |
| Command bar | `:` | global | n/a |
| Help | `?` | global | n/a |
| Back / Quit | `Esc`/`h`/`←` / `q`/`Ctrl+C` | global | n/a |

**Removed**: `Menu` (`a` re-purposed to analyze). The old `Menu` action and
`openActionMenu`/`onMenuKey`/`actionMenuView` are deleted.

## A2 — Direct dispatch reuses existing flows (FR-005)

Each action key invokes the SAME entry point the menu used — `startDownload`,
`startRemoveObject`, `startCopy`, `startMove`, `startRecursiveDelete`,
`startUpload`, `startCreateFolder`, `startAnalyze`, `refresh`. Two-tier
confirmations (simple `y/N` vs typed target/count) are unchanged. No direct-key
path bypasses confirmation.

## A3 — Hint bar (FR-003/FR-004)

- Always visible (above the multi-line footer); rebuilt each render from `mode`,
  `selKind()`, `selCount()`, `writable()`.
- Lists `key label` for each **available** action; segment-by-segment width fit so
  it wraps rather than drops (matches existing footer behavior).
- When `!writable()`, write actions are omitted (or greyed with a "needs --write"
  cue); pressing a write key is a safe no-op with an explanatory status line.
- When `selCount()>0`, `d`/`x`/`y` show the bulk variant + count (FR-006).

## A4 — Migration (FR-007)

`a` now runs analyze (sensible, not dead). Legacy `D` (was recursive delete) → now
`X`; legacy `d` (was delete) → now download. The hint bar is the always-on source
of truth, so a stale-muscle keypress is self-correcting.

**Tests**: pressing each key from the right selection enters its existing flow;
`a` does NOT open a menu; in read-only the hint bar omits write actions and a write
key is a no-op + notice; with marks set, `d`/`x`/`y` route to bulk; destructive
keys still hit confirmation; hint bar contents match selection/capability.
