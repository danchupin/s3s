# Contract: Level filter (focus-aware scope + commit-on-Enter input)

**Stories**: US7. FR-029, FR-030, FR-039, FR-040, FR-041. Source: `search.go`, `app.go`, reuse
`storage.LevelQuery.Search` (no storage change).

## Scope (FR-029)

| Focus | Filter type | State | Backend |
|-------|-------------|-------|---------|
| `zoneBuckets` (or bucket list) | instant local | `m.bucketFilter` | none |
| `zoneObjects` / full-screen level | server-side current-prefix (non-recursive) | `m.search` → `LevelQuery.Search` | `effPrefix = prefix + search`, `Delimiter:"/"` (`s3client.go:107`); debounced + cache.Key.Search |

`afterFilterEdit`, the Esc/clear branch, and `searchActive()` MUST branch on `focusZone` so bucket-zone `/`
never targets the objects level and objects-zone `/` never targets the bucket list.

## Input surface & lifecycle (FR-039..041)

```
press Filter (/) ─▶ OPEN prominent single-line input
                     (pre-filled with committed term if any; pane label shown)
   │ type ─────────▶ LIVE PREVIEW (instant buckets | debounced server objects); other pane untouched
   │ Enter ────────▶ COMMIT: close input, set committed filter, MOVE FOCUS into filtered pane,
   │                  show "filter: <term>  ✕ clear" indicator
   │ Esc ──────────▶ CANCEL: revert to last committed state
   │                  (committed term preserved; if none, pane returns to unfiltered)
COMMITTED + closed:
   │ press Filter ─▶ RE-OPEN pre-filled (refine); re-Enter replaces the committed term
   │ back/clear ───▶ CLEAR committed filter → restore full pane content (FR-009 precedence:
                      clear-search → ascend → return-to-buckets)
```

Invariants:
- Footer + command bar stay visible while the input is open (`boxView` `minRows` budget recomputed).
- The input reuses the shared field/box style (design system, VII) — no one-off widget.
- Debounce + generation guard drop superseded server searches (existing machinery).

## Tests (white-box + `Fake.ListLevelCalls`)

- `TestFilterScopesToObjectsLevel`: objects-zone `/` narrows objects, bucket list unaffected, exactly one
  new `ListLevelCalls`.
- `TestFilterInputCommitMovesFocus`: Enter commits, input closes, `focusZone==zoneObjects`, indicator shown.
- `TestFilterReopenPrefilled`: re-`/` pre-fills committed term.
- `TestFilterEscRevertsToCommitted`: Esc keeps committed filter; with none committed, returns unfiltered.
- `TestFilterClearRestoresLevel`: back/clear removes committed filter, full level restored.
- `TestBucketFilterStillLocal`: bucket-zone `/` does NOT call `ListLevel`.
