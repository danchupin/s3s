# Data Model: 012 UI state

This feature adds no persistent/storage entities — it is a TUI presentation iteration. The "entities" are
UI state held on the `App` model (`internal/ui/app.go`) and small value types. No `storage.Storage` change.

## App state additions

| Field | Type | Purpose | Lifecycle / transitions |
|-------|------|---------|--------------------------|
| `reveal` | `*revealState` (new) | Active reveal/inspect popup payload (R1). | Set when the Reveal key fires on a selection/breadcrumb; nil otherwise. Any key / Esc clears. Suppressed while `op != nil` or `armConfirm`. |
| `filterInput` | reuse `searching`/`searchInput` + a `filterPane` marker | The prominent filter input (R2 + commit-on-Enter flow). | `/` opens (pre-filled with the committed term if any); typing previews live; Enter commits + closes + moves focus to the filtered pane; Esc cancels to last committed state. |
| `committedFilter` | derived from `bucketFilter` (buckets) / `search` (objects) | The applied filter shown as an indicator with a clear affordance. | Cleared via FR-009 back/clear precedence → restores full content. |
| `sortBy` / `sortAsc` | existing (`sort.go`) | Current sort field (name/size/**modified**) + direction. | Unchanged algorithm; now reachable in the objects zone (R3) and advertised in the command bar (R8). |
| `focusZone` | existing (`zoneBuckets`/`zoneObjects`) | Which pane owns input. | Now also gates `selKind()`/`actionCatalog()` (R3) and filter scope (R2). |
| `m.sel` (marks) | existing map | Multi-select set, level-scoped. | **Fix**: cleared in `loadObjectsLevel` (was leaking across bucket/level changes) — R3. |
| keymap `Reveal` | `[]string` (new, default `["i"]`) | Reveal/inspect binding (R1). | Single keymap source → dispatch + hints + help. |
| keymap `Tab` | `[]string` (new, default `["tab"]`) | Focus-toggle binding, was a hardcoded literal (R7-glyphs / FR-037). | Rebindable + advertised. |

## Value types

### revealState (new)

| Field | Type | Notes |
|-------|------|-------|
| `kind` | enum `revealBucket` / `revealObject` / `revealFolder` / `revealPath` | What the value identifies (drives the label). |
| `value` | `string` | The full, un-truncated identifier (or full breadcrumb path). |

Rendering: centered popup reusing `confirmPopupView`/`popupBoxStyle`; wraps long values; always displays the
value; a copy is emitted via `tea.SetClipboard(value)` (best-effort OSC52). No new style.

### Breadcrumb path (computed, not stored)

Built from `ctxName → bucket → split(prefix, "/")` + optional `(search: term)` marker. Rendered as the
objects-zone center label (Dual/Full) or the box title (Single). `elideMiddle(path, maxW)` keeps the bucket
+ deepest segment, drops middle prefixes, falls back to end-truncation; the search marker appends after
elision. Empty prefix → bucket only (no trailing slash). Full path revealable via `revealState{revealPath}`.

### Mode chip (computed, not stored)

Derived from `m.writable()`/`m.armed`/`m.ctxReadOnly`: text `WRITE` (accent: `writeBadgeStyle`/`warnStyle`)
when armed, `RO` (neutral: `roStyle`/`dimCellStyle`) when read-only. Rendered in a new right-aligned slot of
`boxViewWith` (top border) on the PRIMARY list box only — leftmost bucket-list box in multi-zone tiers, the
sole box in Single (one fixed location). NO_COLOR-safe (text). Safety-redundant with the footer badge
(FR-038 exception).

### Sort indicator (computed)

`sortIndicator()` → `"name↑"`/`"size↓"`/`"modified↑"`; surfaced as the first read-block `barEntry`
(`"s "+indicator`) in the command bar. Drops gracefully under `fitEntries` width-trimming.

## State-transition notes (focus & filter)

- **Objects-zone parity** (R3): `(mode==modeBuckets && focusZone==zoneObjects)` is a *level context* — same
  dispatch (`onLevelKey`), same action catalog, same `selKind`, as the full-screen level view; not a mode
  change.
- **Filter lifecycle** (R2 + input flow): open → live preview → Enter (commit + focus to pane + indicator)
  → re-open pre-filled (refine) | Esc (cancel to last committed) | back/clear (remove committed → full).
- **Marks** are level-scoped: cleared on `loadObjectsLevel` and on `enterLevel`/`goBack`.

## Invariants

- No identifier is permanently hidden: every name/key/prefix/path is fully visible OR revealable (VI).
- Footer + command bar never scroll off: every new surface respects `boxView`'s `minRows` cap (FR-022).
- Single keymap source: every on-screen key hint renders via `glyph`/`formatKeys` (no literals) (VII).
- Every cue is NO_COLOR-safe (text in addition to color).
