# Data Model: Filter UX Redesign

No persisted data changes — this is in-memory UI state in the `App` model (`internal/ui`).

## Filter state (per scope) — existing, reused

| Field (App) | Scope | Meaning |
|-------------|-------|---------|
| `bucketFilter` | bucket | committed bucket-name filter term (instant local) |
| `search` | object | committed object filter term (server-side prefix search) |
| `searching` | — | whether the input strip is actively focused for editing |
| `searchInput` | focused | the in-progress text in the strip while editing |
| `filterBefore` | focused | committed term snapshot for Esc-revert |
| `searchGen` | object | debounce generation (coalesces keystroke bursts) |

`filterIsBucketList()` selects the focused scope (bucket vs object); `committedFilterTerm()`
returns the focused scope's committed term. **Lifecycle unchanged** (`/` open → live narrow →
Enter commit → Esc revert → clear/navigate-away clears).

## Match tally (derived at render time)

| Scope | matched | total | Display |
|-------|---------|-------|---------|
| bucket | `len(filteredBuckets())` | `len(buckets)` | `M/T` (e.g. `3/12`) |
| object | `m.level.count()` (loaded dirs+objects after the prefix search) | unknown (paginated) | `N` (e.g. `12`) |

No new network call: bucket counts are local; the object matched count is the already-loaded
level size. The object total is intentionally not fetched (FR-013).

## Indicator chip (per pane) — enhanced

A pane-border chip, term-gated and zone-agnostic:

- **Visible** when its scope has a committed term — independent of which pane is focused.
- **Hidden** only while THAT scope is being actively edited (the live term shows in the strip).
- **Content**: `filter: <term> · <count>` — bucket `M/T`, object `N`.
- **Elision**: the term elides first to fit; the chip drops WHOLE under width pressure (mode chip
  survives); the strip still shows the active filter.

## Filter input strip — NEW (always-visible chrome)

A single always-present row between the body and the footer, in filterable modes only
(`modeBuckets`, `modeTree`).

| State | Rendering |
|-------|-----------|
| active (`searching`) | `▌ filter <pane>: <input>` + caret + live hints (`(live) · Enter · Esc`) |
| idle, scope has term | `▌ filter <pane>: <committed term>` (dim) |
| idle, no term | `/ to filter <pane>` placeholder (dim) |

Bound to the focused pane's scope. Costs exactly one reserved row.

## Layout budget — the invariant

```
rows = height − footerH − filterStripH − 2(border)      ; filterStripH = 1 in filterable modes, else 0
dataRows = rows − 2(table header)
```

Reservation order (never sacrificed): **footer → filter strip → indicator chips (border, 0 body
rows)**. The **list body** (`windowBounds`/`treeView`) absorbs all height loss. This is the
Constitution VI guarantee extended to the filter chrome.
