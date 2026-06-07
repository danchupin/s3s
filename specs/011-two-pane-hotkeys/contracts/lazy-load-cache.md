# Contract: Lazy load + per-session level cache (three-zone browse)

Covers FR-002, FR-002a, FR-003, FR-006, FR-006a, FR-006b, FR-006c, FR-006d, FR-006e,
FR-026, FR-027.

The objects zone is fed by the EXISTING read path — `storage.Storage.ListLevel(ctx, LevelQuery{Bucket,
Prefix:"", …})` (`storage.go:109`) via `loadLevel` (`commands.go:62`) — and the EXISTING per-session
level cache (`internal/cache`, keyed `(Context,Bucket,Prefix,Search)`, `cache.go:9`). No new storage
method is added; the read-only guard (`scripts/check-readonly.sh`) stays green (FR-026/FR-027).
Non-blocking is the existing pattern: each fetch runs in a `tea.Cmd`, results carry the generation
`m.gen` they were issued under, and `beginLoad()` (`app.go:265`) cancels the prior context and bumps
`m.gen`; a message whose `gen != m.gen` is dropped (`onLevel`, `tree.go:188`; Constitution II).

## Startup: names only (FR-002/FR-002a)

- On launch `loadBuckets` runs (`Init`, `app.go:255`). It fetches bucket NAMES only:
  - scoped/pinned connection (`len(pinned) > 0`) ⇒ synthesizes `[]storage.Bucket{{Name}}` with NO
    `ListBuckets` call (`commands.go:40`);
  - otherwise one `ListBuckets` (`commands.go:51`).
- Startup performs ZERO object listings: no `ListLevel` is issued for any bucket until a settled
  selection crosses into it. Invariant: `ListLevelCalls == 0` immediately after the initial
  `bucketsMsg` is delivered.

## Settled selection triggers exactly one ListLevel (FR-002a/FR-003)

- Scrolling the buckets zone changes only `bucketSel`; it does NOT list. The objects-zone fetch fires
  only for the SETTLED selection, debounced and gen-suppressed exactly like the details pane:
  - reuse `paneTickCmd(gen, key)` (`commands.go:312`) with `paneDebounce = 180 * time.Millisecond`
    (`commands.go:302`, ceiling `<= 200ms`), armed by the per-selection rearm `afterSelectionMove`
    (`app.go:291`);
  - a settle tick is honored only when `gen`+`key` still match the current selection (`onPaneTick`,
    `app.go:307` / `onContextResolved`-style gen check); a tick for a scrolled-past bucket is dropped.
- On a honored settle, the objects level is served from cache on a hit (no fetch) or fetched with ONE
  `loadLevel` first-page call on a miss — the existing `enterLevel` logic (`tree.go:116`): cache hit ⇒
  `m.level = cached`, no command; miss ⇒ `beginLoad()` + one `loadLevel`.
- Fast scroll across many buckets issues AT MOST ONE `ListLevel` (for the bucket the cursor lands on),
  because intermediate selections never settle past the debounce and any in-flight load is superseded
  by `beginLoad()` bumping `m.gen` (FR-003).

## First page only; paging on demand (FR-006a)

- The settle fetch requests the FIRST page only: `LevelQuery{Bucket, Prefix:"", Search:""}` with the
  default page size (`tree.go:128`). Subsequent pages load only on demand — scrolling to the bottom of
  the objects zone calls `fetchNextPage` (`tree.go:133`), exactly one additional `ListLevel` per page
  (the existing paging-on-scroll behavior, `tree.go:55`). Preview/details for the first page's
  selection use the debounced pane loads, not an eager full listing.

## Cache semantics (FR-006b/FR-006c/FR-006d/FR-006e)

- **Success + empty are cached.** `onLevel` (`tree.go:188`) puts the merged level into the cache
  (`tree.go:200`) on any successful page — including an empty level (zero dirs/objects). Revisiting a
  cached bucket's level serves from cache with NO `ListLevel` call (`tree.go:121`).
- **Errors are NOT cached.** An `errMsg` from `loadLevel` (`commands.go:67`) sets `m.err` and leaves
  the level uncached, so the next visit to that bucket RE-ATTEMPTS the listing (FR-006b/FR-006c). A
  denied bucket (`AccessDeniedBuckets`, `fake.go:101` → `ErrAccessDenied`) is therefore retried on
  every revisit, never silently stuck.
- **In-flight dedup (FR-006d).** A second settle on the same bucket while its load is in flight does
  not issue a duplicate fetch: `beginLoad()` cancels the prior context and the generation check drops
  the superseded result; the cache (filled on completion) absorbs the repeat. Re-selecting the same
  settled bucket re-uses the in-flight/loaded level rather than firing a parallel `ListLevel`.
- **Cache key (FR-006e).** The key is `(Context, Bucket, Prefix, Search)` (`cache.go:9`,
  `levelKey()` `app.go:323`) — the SAME cache instance shared with the full-screen tree view. A level
  fetched in the three-zone objects zone is a cache hit when later opened full-screen in Single, and
  vice-versa.

## Refresh invalidation (FR-006e)

- `r` (`Refresh`, `keys.go:52`) invalidates the current level and re-fetches: `refresh`
  (`tree.go:142`) calls `m.cache.Invalidate(key)` then one `loadLevel`. There is NO TTL — the cache is
  invalidated only by manual `r` (or context switch `Clear`, `app.go:785`). Refresh is the single way
  a successfully-cached level is re-fetched.

## Test assertions (white-box `package ui` + `storage.Fake` call counters)

`storage.Fake` exposes `ListBucketsCalls` and `ListLevelCalls` (`fake.go:30`), `FailListBuckets`
(`fake.go:25`), and `AccessDeniedBuckets` (`fake.go:28`). Drive with `deliver`/`press`/`viewOf`.

1. **Startup lists names only.** Fake seeded with 3 buckets each holding objects. After the initial
   `bucketsMsg`: `ListLevelCalls == 0`; the buckets zone shows all 3 names (and for a pinned
   connection `ListBucketsCalls == 0`).
2. **One ListLevel per settle.** Buckets focused, select bucket B and let the settle tick fire (gen+key
   match): `ListLevelCalls == 1`; the objects zone shows B's first page.
3. **Fast scroll ≤ 1 fetch.** From a zeroed counter, press `down` rapidly across buckets B1→B5 without
   letting intermediate selections settle, then settle on B5: `ListLevelCalls <= 1` (only B5's load
   survives; superseded loads are dropped by the gen check).
4. **Revisit = 0 fetches.** After B is loaded and cached, navigate away and re-settle on B:
   `ListLevelCalls` is unchanged (cache hit, no new `ListLevel`); the objects zone re-renders from
   cache.
5. **Empty cached.** A seeded-but-empty bucket E: settle on E ⇒ `ListLevelCalls == 1`, objects zone
   shows the empty state; re-settle on E ⇒ count unchanged (empty result was cached).
6. **Denied re-attempted.** `AccessDeniedBuckets["d"] = true`: settle on `d` ⇒ `ListLevelCalls == 1`,
   `m.err` is `ErrAccessDenied` (footer shows "Access denied…"); navigate away and re-settle on `d` ⇒
   `ListLevelCalls == 2` (error was NOT cached — re-attempted).
7. **In-flight dedup.** Settle on B, then before its `levelMsg` arrives settle on B again: the second
   settle issues no extra `loadLevel` for the same in-flight key (count for B stays 1 once delivered);
   the superseded generation is dropped.
8. **`r` re-fetches.** After B is cached (`ListLevelCalls == 1`), `press(m, "r")` ⇒
   `ListLevelCalls == 2` (cache invalidated, level re-fetched); a non-refresh revisit stays at the
   cached count.
9. **Shared cache.** A level fetched in the objects zone (Full/Dual) is a cache hit when the same
   `(Context,Bucket,Prefix,Search)` is opened full-screen in Single: opening it issues no additional
   `ListLevel`.
