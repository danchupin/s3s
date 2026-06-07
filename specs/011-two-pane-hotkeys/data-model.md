# Data Model: 011 — Three-Zone Master-Detail Browse + Hotkey Mnemonic Review

> Scope reminder. This feature adds **NO new SDK types and NO new `storage` methods**. The objects zone lists via the existing `storage.Storage.ListLevel(bucket, prefix="", …)` (FR-026/027), so the read-only guard and `check-readonly` stay green. Everything below maps to concrete Go state in `internal/ui` (`app.go`, `keys.go`, `pane.go`, `tree.go`, `commands.go`) and `internal/cache`. Where a field already exists it is cited verbatim; where it is new it is marked **(NEW)**.

The three zones are **buckets** (left), **objects** (middle — the highlighted bucket's top level), and **details** (right — the existing debounced pane). Buckets and objects each own an independent cursor and focus state; details is a passive projection of the active zone's selection. Layout collapses zones by width.

---

## Entity: BrowseFocus

**Purpose.** Names which of the two interactive zones (`buckets` / `objects`) currently owns keyboard input, so navigation keys (`Up/Down/Top/Bottom`), `Enter`, and direct-action dispatch route to the correct cursor. Drives the accent-vs-dim border styling of the two zone boxes (FR-007/008). It is a *focus* selector layered on top of the existing `mode` machine — it does **not** replace `mode`; it only matters while `m.mode == modeBuckets` (the Full/Dual three-zone screen).

**Fields.**
- `focusZone zone` **(NEW)** — `iota` enum `zoneBuckets` / `zoneObjects`. New `App` field on the struct in `app.go` (alongside `bucketSel int` / `bucketFilter string`, around app.go:115-117).

```go
type zone int

const (
    zoneBuckets zone = iota // default
    zoneObjects
)
```

**Relationships.**
- Selects between the two cursors: `zoneBuckets → m.bucketSel`, `zoneObjects → the objects-zone cursor` (see *Zone cursors*).
- Only meaningful in the Full/Dual tiers (`m.mode == modeBuckets`). In Single tier the zones are stacked and `mode` already disambiguates (`modeBuckets` vs `modeTree`), so `focusZone` is dormant there (always treated as `zoneBuckets`).
- Read by `View()`/`listWithPane` (app.go:935) to pick which box gets the accent border/title and which is dimmed.

**Validation / invariants.**
- Always exactly one of `{zoneBuckets, zoneObjects}`. Zero value = `zoneBuckets`, matching the default landing zone (left) and the existing `mode: modeBuckets` default in `New()` (app.go:217).
- `zoneObjects` is only reachable when a bucket is highlighted **and** the tier renders the objects zone (Full/Dual). On a layout collapse to Single, or when the highlighted bucket changes such that no objects level is loadable, focus falls back to `zoneBuckets` (see lifecycle).

**Lifecycle / transitions.** (FR-007/008/009)
- `New()` / context switch / first render: `zoneBuckets` (default).
- `Tab` (new binding, see *Keymap*): symmetric toggle `zoneBuckets ↔ zoneObjects`. Only crosses to `zoneObjects` if the objects zone is currently shown and a bucket is highlighted; otherwise no-op.
- `Right` / `l` / `Enter` **on a highlighted bucket** while `zoneBuckets`: cross right into `zoneObjects` (does not yet drill into a sub-prefix — it just moves input to the objects zone; the live top level is already loaded). In Single tier this remains the existing behavior: `enterLevel()` → `modeTree`.
- `Left` / `h` / `Esc` while `zoneObjects`: return to `zoneBuckets` (ascend-or-return per FR-009). When `zoneObjects` is at the objects top level, Left/Esc returns focus to buckets; deeper navigation inside the objects zone is the existing `Back`/`enterLevel` ascend.
- Width collapse to Single (`<=99` cols): reset to `zoneBuckets`.

---

## Entity: Zone cursors (bucketSel + objectsSel)

**Purpose.** Two independent selection indices, one per interactive zone, so the user's place in the bucket list is preserved while they move through the highlighted bucket's objects, and vice-versa (FR-008 — cursors are independent).

**Fields.**
- `bucketSel int` — **existing** (app.go:116). Global cursor into `filteredBuckets()` (plus the synthetic `+ add bucket` row when `bucketAddRowVisible()`). Unchanged by this feature; it now *also* determines which bucket the objects zone lists.
- `treeSel int` — **existing** (app.go:124). The level cursor into the rendered tree `entries`; read by `selected()` (tree.go:106-111) and windowed by `windowBounds(len(entries), m.treeSel, rows)` (tree.go:220). **Reused as the objects-zone cursor** — no new field. In the three-zone screen `treeSel` indexes the *objects zone's* loaded level (`m.level`), exactly as it indexes the full-screen `modeTree` level today; this keeps `selected()`, `afterSelectionMove()`, and the pane debounce working verbatim.

**Relationships.**
- `focusZone` selects which cursor the nav keys mutate: `zoneBuckets → bucketSel`, `zoneObjects → treeSel`.
- `bucketSel` → identifies the highlighted bucket → drives `ObjectsZoneState` (the objects zone lists *that* bucket's top level).
- `treeSel` → `selected() *treeEntry` → drives the details pane (`afterSelectionMove`/`onPaneTick`, app.go:291-320) and direct-action gating (`selKind`).
- Independence: changing `bucketSel` (scrolling buckets) reloads the objects zone and resets `treeSel = 0` (matching `enterLevel()`/tree.go:118,146) but does **not** otherwise couple the two; changing `treeSel` never touches `bucketSel`.

**Validation / invariants.**
- Both are clamped by `clampSelection()` (app.go:800-811): `bucketSel ≤ len(buckets)` (or `len(buckets)` incl. the add-row), `treeSel < m.level.count()`. Resize never invalidates them because the visible window is recomputed statelessly at render via `windowBounds` (app.go:798-799, styles.go).
- `treeSel == 0` whenever a *fresh* objects level loads (a new highlighted bucket), per the existing reset in `enterLevel`.

**Lifecycle / transitions.**
- `bucketSel`: `Up/Down/Top/Bottom` while `zoneBuckets` (`onBucketsKey`, app.go:636-687). On each settle (post-debounce) the objects zone reloads for the new bucket and `treeSel` resets to 0.
- `treeSel`: `Up/Down/Top/Bottom` while `zoneObjects` (the existing `onTreeKey` cursor logic, tree.go:46-65). Each move triggers `afterSelectionMove()` → re-arms the details-pane debounce (app.go:291-301).

---

## Entity: ObjectsZoneState

**Purpose.** The loaded, cached, paged top level of the **highlighted** bucket, shown in the middle zone. It is the same shape as a full-screen tree level — by design it **reuses** the existing `levelState` + `*App.level` + generation + cache machinery rather than introducing a parallel state container. The "objects zone" is conceptually "the `modeTree` level for `(highlighted bucket, prefix="")`, rendered beside the bucket list instead of replacing it."

**Fields** (all existing, repurposed for the live side-by-side render):
- `bucket string` — **existing** (app.go:122). The highlighted bucket's name (`filteredBuckets()[bucketSel].Name`); the objects zone lists this bucket.
- `prefix string` — **existing** (app.go:123). `""` for the objects-zone top level (FR-026: `ListLevel(bucket, prefix="")`). Deeper navigation inside the zone reuses the existing prefix descent.
- `search string` — **existing** (app.go:124). Empty for the live objects zone unless a level search is applied; part of the cache key.
- `level *levelState` — **existing** (app.go:124). The accumulated level for the highlighted bucket. `levelState{dirs []string; objects []storage.ObjectRef; nextToken *string; complete bool}` (app.go:64-69); `count()` = `len(dirs)+len(objects)` (app.go:71).
- `gen int` — **existing** (app.go:160). The monotonic load generation from `beginLoad()` (app.go:265-276). Every objects-zone load carries the gen it was issued under; `onLevel`/`bucketsMsg`/`errMsg` drop any message whose `msg.gen != m.gen` (app.go:359,370; tree.go:188). This is the supersede mechanism for fast bucket scrolling (constitution II).
- `loading bool` — **existing** (app.go:161). True while the settled objects level is in flight; rendered as the explicit *loading* status (FR-005). `bucketsView` already shows `"Loading buckets…"`; the objects zone reuses `m.loading` + the spinner the same way.
- `err error` — **existing** (app.go:165). Set on a failed objects load (gated by gen, app.go:393-398). Surfaces via `errorText()` (app.go:822-845) as the explicit *error* status (FR-005). **Errors are not cached** (see *Cache entry*), so revisiting the bucket re-attempts (FR-006b/c).

**Derived status (FR-005)** — not a stored field; computed at render from the above:
- *loading* = `m.loading && m.level == nil` (or page-in-flight),
- *empty* = `m.level != nil && m.level.count() == 0 && m.level.complete`,
- *error* = `m.err != nil` for this gen,
- *loaded* = `m.level != nil && m.level.count() > 0`.

**In-flight dedup (FR-006d).** A settled bucket selection must not issue a second `ListLevel` for a coordinate already loading or loaded:
- `objectsInFlightKey cache.Key` **(NEW, optional)** — the cache `Key` of the objects load currently in flight (zero `Key` when idle). Before issuing `loadLevel`, compare against `objectsInFlightKey` and against a cache hit (`m.cache.Get(key)`); skip the fetch on a match. This mirrors the existing pattern where `paneSelKey string` (app.go:137) guards duplicate pane loads. If the implementation prefers, dedup can be derived purely from `(loading && levelKey()==key) || cache hit` without a new field — the field is documented as the explicit, testable form.

**Relationships.**
- Keyed by the highlighted bucket (`bucketSel`). Bucket scroll → debounce → reload (see lifecycle).
- Cached under `levelKey()` = `cache.Key{Context: m.ctxName, Bucket: m.bucket, Prefix: m.prefix, Search: m.search}` (app.go:323-325) in the **shared** `m.cache *cache.Cache[*levelState]` (app.go:108). Same cache instance and key space as the full-screen `modeTree` view, so entering a bucket full-screen after previewing it in the zone is a cache hit (FR-006e).
- Feeds the details zone: `treeSel` selects within `m.level` → `selected()` → pane.

**Validation / invariants.**
- The objects zone calls **only** `ListLevel` (never a new method); read-only guard intact (FR-026/027).
- A loaded, complete, empty level is a cached success (FR-006b). An errored level is **never** written to `m.cache` (FR-006c) — `onLevel` only `Put`s on a successful page (tree.go onLevel path).
- First page only is fetched for the preview; further pages load on demand via `nextToken`/`complete` (FR-006a) — the existing paging path, not changed.
- A stale page (`msg.gen != m.gen`) is dropped before any merge/cache write (tree.go:188), so a superseded bucket selection cannot corrupt the zone.

**Lifecycle / transitions.** See the state machine in *Objects-zone load* below.

---

## Entity: LayoutTier (Full | Dual | Single) — derived, NOT stored

**Purpose.** Chooses the column composition from terminal width. It is **computed at render time**, never persisted — the same stateless-window philosophy as `windowBounds(n, sel, rows)` (styles.go) and `boxView` height budgeting (app.go:798-799, 863-876). No `App` field.

**Values (normative widths).**
- `Full` — `width >= 130`: `buckets | objects | details` (three boxes joined via `lipgloss.JoinHorizontal`, app.go:949).
- `Dual` — `100 <= width <= 129`: `buckets | objects` (details collapses). Aligns with the existing `paneSplitMin = 100` threshold (app.go:929): at/above 100 a second column is affordable.
- `Single` — `width <= 99`: the current single-column mode-stack (`modeBuckets`→`modeTree`→`modeObject`), exactly today's behavior. Below `paneSplitMin` `listWithPane` already returns the full-width list with the pane collapsed (app.go:936-938).

**Computation.** A pure helper, e.g. `layoutTier(w int) tier`, called inside `View()`/`listWithPane`. It reads `clampW(m.width)` (app.go:850) only. Because it is derived, a resize (`tea.WindowSizeMsg`, app.go:341-344) needs no tier bookkeeping — it updates `m.width/m.height`, clamps the cursors, and the next `View()` recomputes the tier and reflows. (`paneVisible bool`, app.go:139, remains the user/width collapse toggle for the details column and composes with the tier: details shows only in Full **and** `paneVisible`.)

**Relationships.**
- Full/Dual ⇒ `focusZone` is live (two interactive zones). Single ⇒ `focusZone` dormant; `mode` drives the stack.
- Width thresholds are the **only** input; cursors/state are tier-independent (resize never loses place).

**Validation / invariants.**
- Exactly one tier per render; monotonic in width (`Single < Dual < Full`).
- The box body must not exceed its row budget or the footer scrolls off (app.go:863-876); each tier's boxes share the same `rows`/`dataRows` budget, with table views reserving 2 header rows.

---

## Entity: Keymap (defaultKeys) — single source of truth

**Purpose.** The one keybinding table feeding dispatch (`matches(key, m.keys.X)`), the hint bar, and the help overlay, so they can never drift (FR-019..025). It is the existing `keyMap` struct + `defaultKeys()` (keys.go:11-72), held as `m.keys keyMap` (app.go:107).

**Fields.** Existing `keyMap` action→bindings (keys.go:11-39). Changes for 011:
- `AddConn []string` — **CHANGED**: the `'n'` hotkey binding is **removed**. The add-connection affordance survives as the row-only `"+ add connection"` entry already rendered in the connection manager list (`connections.go:103`), mirroring the `"+ add bucket"` row (app.go:1142-1173). Concretely, the global `AddConn` dispatch branch in `onKey` (app.go:604-606) is dropped (or `AddConn` set empty), so `'n'` is no longer a global hotkey. `y`/`ctrl+o` (`Copy`/`MoveChord`, keys.go:58,62) are **kept** unchanged.
- `Tab []string` **(NEW)** — symmetric focus toggle for `focusZone` (`{"tab"}`). New field on `keyMap`; new branch in `onKey`/`onBucketsKey`/`onTreeKey` dispatch. Single-sourced like every other binding so the hint bar and help advertise it without drift.

**Bold render attribute (FR-022/023).** Every *advertised* key glyph must render **bold**. Today the hint bar/help render keys with `accentStyle` foreground only (e.g. `accentStyle.Render(...)`, `row(...)` in `helpLines`, keys.go:128). The change: add `.Bold(true)` to the key-glyph styling path (the `accentStyle`/`hintLabelStyle` used for keys in `hintbar.go`, `helpLines`, and `paneBucket`/`paneTree` hint strings, pane.go:42,59). This is a **style attribute, not state** — it lives in `styles.go`, not on `App`.
- **NO_COLOR invariant (FR-024):** when color is disabled, bold (an SGR attribute, not a color) still applies; if even bold is unavailable, a non-color cue (e.g. brackets/`·` separators already present in the hint segments) keeps the advertised keys legible. The cue must not depend on color alone.

**Relationships.**
- Read by: `onKey`/`onBucketsKey`/`onTreeKey` dispatch (`matches`, app.go:579-633), the hint bar (`commandBarView`/`footerHints`), and `helpLines` (keys.go:124-192, which derives every key column from `m.keys`).
- `keyGlyph`/`glyph`/`formatKeys` (keys.go:86-114) render bindings to display glyphs; the bold attribute wraps their output at the call sites.

**Validation / invariants.**
- One source: dispatch, hint bar, and help all read `m.keys` — adding `Tab` or removing the `AddConn` hotkey updates all three at once (FR-021/025).
- An advertised key is always dispatchable, and a dispatchable key is advertised somewhere (hint bar or help). Removing `'n'` from dispatch ⇒ it is no longer advertised as a hotkey; the `"+ add connection"` row remains the discoverable path.

---

## Entity: Cache entry — `cache.Key → *levelState`

**Purpose.** The per-session, TTL-free level cache shared by the objects zone and the full-screen tree (FR-006e). Returning to a level reads from cache; only manual refresh (`r`, `m.keys.Refresh`) invalidates.

**Fields** (`internal/cache/cache.go`):
- `Key{Context, Bucket, Prefix, Search string}` (cache.go:9-14) — the full coordinate. Built by `levelKey()` (app.go:323-325) / `levelKeyFor()` (app.go:330-332).
- Value `*levelState` (app.go:64-69) — the accumulated, paged level.
- `Cache[V]` is a plain `map[Key]V`, single-threaded ownership by the Bubble Tea loop (cache.go:16-19). `m.cache` is `*cache.Cache[*levelState]` (app.go:108).

**Relationships.**
- Same instance and key space as `modeTree`, so a bucket previewed in the objects zone is a cache hit when entered full-screen, and vice-versa (FR-006e).
- `Context` segment scopes entries per context; `m.cache.Clear()` on context switch (app.go:785).

**Validation / invariants.** (FR-006b/c/e)
- **Success and empty-success are cached** (`Put` on a successful `ListLevel` page, incl. a complete empty level) → no re-fetch on revisit.
- **Errors are NOT cached** → a revisit re-attempts (matches `errMsg` setting `m.err` without any cache write, app.go:393-398).
- No expiry: freshness is user-driven (cache.go package doc). Invalidation is precise: `Invalidate(key)` for one level (app.go:336), `InvalidateBucket(ctx, bucket)` for a bucket's nested levels (cache.go:43-49), `Clear()` only on context switch.
- `'r'` (`m.keys.Refresh`) invalidates the current level's entry, forcing one re-fetch (FR-006e / cache.go:38-41).

---

## State transition: BrowseFocus (Full/Dual tiers)

```
            Tab                         Right / l / Enter-on-bucket
zoneBuckets ───────────────► zoneObjects ◄─────────────── zoneBuckets
     ▲                            │  │
     └──── Tab / Left / h / Esc ──┘  └── (deeper) Back/Enter = existing prefix
                                         ascend/descend inside the objects level

Guards:
- Cross to zoneObjects only if objects zone is rendered (Full/Dual) AND a bucket is highlighted.
- Width collapse to Single (<=99) OR context switch OR New(): reset to zoneBuckets.
- Active zone → accent border + title; the other zone dimmed (FR-007/008).
```

In **Single** tier this collapses to today's behavior: `modeBuckets --Enter/l/Right--> modeTree --Esc/h/Left--> modeBuckets`; `focusZone` is not consulted.

---

## State transition: Objects-zone load (per settled bucket selection)

```
                         scroll bucket (bucketSel changes)
                                     │
                                     ▼
   ┌────────┐  paneTick/debounce  ┌──────────┐  cache HIT (success/empty)  ┌────────┐
   │  idle  │ ──~180ms, ≤200ms──► │ debounced│ ──────────────────────────► │ loaded │
   └────────┘   gen-armed          └──────────┘                            │/empty  │
        ▲            │  settled selection still current (gen+bucket match)  └────────┘
        │            ▼  AND not already in flight (dedup, FR-006d)               ▲
        │       ┌──────────┐  loadLevel(ListLevel, prefix="", gen)               │
        │       │ loading  │ ──────────────────────────────────────────────────►│ levelMsg, gen==m.gen → merge+Put(cache)
        │       └──────────┘                                                      │
        │            │  errMsg (gen==m.gen)                                  ┌────────┐
        │            └─────────────────────────────────────────────────────►│ error  │ (NOT cached → re-attempt on revisit)
        │
        └── supersede: a faster scroll calls beginLoad() → m.gen++ → any in-flight
            levelMsg/errMsg with the old gen is DROPPED (msg.gen != m.gen). The new
            settled selection starts its own debounce→load. (constitution II)
```

**Notes tying the diagram to code.**
- **Debounce** reuses the details-pane tick mechanism: `paneTickCmd(gen, key)` (~180 ms, commands.go:311-314) fired from `afterSelectionMove()` (app.go:291-301), gated in `onPaneTick` by `(gen, key)` equality (app.go:307-320) so only the *settled* selection fetches; fast scroll past a bucket drops its tick (FR-003).
- **Gen suppression / supersede** is `beginLoad()` bumping `m.gen` (app.go:269) and every handler's `if msg.gen != m.gen { return m, nil }` guard (app.go:359,370,393).
- **Dedup** (FR-006d): before `loadLevel`, skip if `m.cache.Get(levelKey())` hits or `objectsInFlightKey == levelKey()`.
- **Cache policy:** success/empty → `Put`; error → no write (FR-006b/c). Shared key space with `modeTree` (FR-006e).
- **No new storage call:** the load path is the existing `loadLevel(ctx, m.activeStore(), key, q, m.gen)` → `ListLevel` (commands.go:62-71; tree.go:129,138,149), preserving the read-only guard.
