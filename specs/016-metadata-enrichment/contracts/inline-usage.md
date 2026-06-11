# Contract: Inline Usage (US2/US3 — FR-004..FR-010, FR-016)

**Surface**: `paneBucket` (`pane.go:45-56`) and `paneTree` folder/level branches
(`pane.go:58-96`) render totals + ONE expandable breakdown section; the off-loop scan
reuses `storage.UsageOf` (`storage.go:123-127`) via the retained `analyzeCmd`/`waitForUsage`
plumbing (`analyze.go:73-98`). `modeUsage`/`usageView`/`runAnalyze`/`onUsageKey`/`usageTitle`
are DELETED.

## Inputs

- Focused target `(bucket, prefix)` resolved by `usageTarget`/`focusedUsageTarget`
  (`analyze.go:37-52`).
- `usageResults *cache.Cache[*storage.UsageReport]` keyed by
  `cache.Key{Context, Bucket, Prefix, Search:""}` (FR-007).
- `usageGen`, `usageCancel`, `usageProg`, `detailSection` (data-model §6).

## Rendered shape

- Scanning: `scanning… N objects, <size> so far` — driven SOLELY by `usageProgressMsg`
  ticks (R6; no `spinnerTick` for the inline scan), inline in the details pane, input
  responsive.
- Complete: `total <size> · N objects`; `(partial)` appended when `Complete==false`
  (FR-006).
- Expanded (`detailSection == sectBreakdown`, US3): ranked largest-first children, each
  `name`, `humanSize(Size)`, and `usageBar(share)` (`analyze.go:214-223`); collapse returns
  to the compact totals. Only ONE detail section renders at a time (budget gate,
  layout-budget contract).

## Behavior

- B1 (FR-004): totals appear in the details area — NO full-screen mode (`modeUsage` gone,
  `app.go:30, 1190-1191` deleted; `command.go:57` no longer lists it).
- B2 (FR-005 dwell): focus move (in `afterBucketMove` for buckets; in the EXTENDED
  `afterSelectionMove` for dir/level — today it returns early for non-objects) calls
  `usageCancel()` + bumps `usageGen` together, then schedules `usageTickCmd{gen, bucket,
  prefix}`; `onUsageTick` fires `loadUsage` ONLY if `msg.gen==usageGen` AND
  `focusedUsageTarget()==(msg.bucket,msg.prefix)` AND not cached → rapid transit spawns no
  scan (edge `spec.md:263-265`). A cached target shows immediately, bypassing the dwell.
- B3 (FR-006): navigating away (and `beginLoad`) calls `usageCancel()` (the scan's OWN ctx
  — NOT `loadCancel`) AND bumps `usageGen` together; the late `usageDoneMsg` is stamped the
  old `usageGen` so its report is NOT applied, while its channel still drains to `close`; a
  cancelled scan yields `Complete=false` rendered `(partial)`.
- B3a (II, no leak): the channel pump (`waitForUsage` re-arm in `onUsageProgress`) keeps
  draining a live `m.usageCh` REGARDLESS of gen (only `usageProg` assignment is gen-gated),
  mirroring the documented guard at `analyze.go:100-108`; combined with `usageCancel()`
  making `UsageOf` return promptly + `close(ch)`, no producer goroutine leaks under rapid
  navigation.
- B4 (FR-007): completed reports cached in `usageResults`; tree `r` invalidates the focused
  level entry beside `m.cache.Invalidate` (`tree.go:144`); bucket-list `r`
  (`refreshBuckets`, `hintbar.go:175`, which invalidates NOTHING today) calls
  `usageResults.InvalidateBucket(ctxName, highlightedBucket)`; context switch
  `usageResults.Clear()` beside `m.cache.Clear()` (memory parity — `Context` in the key
  already prevents bleed).
- B5 (FR-009/FR-010): the `MoreDetail` key toggles `detailSection` to/from `sectBreakdown`;
  Enter on a child sub-prefix re-targets navigation into that prefix (its usage shows via
  B2).
- B6 (edge `spec.md:256-257`): empty prefix resolves to `0 B · 0 objects` quickly, not an
  infinite scan.

## Invariants

- I1 (II/FR-016): every scan runs in a `tea.Cmd`; the (gen bump, ctx cancel, drop-check)
  triple all key off `usageGen`+`usageCancel`, NEVER `m.gen`/`loadCancel`.
- I2 (VI): only one detail section renders at a time; totals + the one section stay within
  the corrected `View()` budget (layout-budget contract); the footer never scrolls off.
- I3 (VII): reuses `usageBar`/share + `accentStyle`/`dimCellStyle`/`warnStyle` — no new hue,
  no parallel layout.
