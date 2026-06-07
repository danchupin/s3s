# Contract: Post-Mutation Visibility (US6, FR-015/FR-016/FR-018)

The view must reflect every completed mutation without a manual refresh, including a
SAME-BUCKET cross-PREFIX copy/move/bulk-copy. UI-only (cache invalidation + reload);
no storage/config change. There is NO cross-bucket copy/move (storage `CopyKey`/
`MoveObject` take a single bucket), so no dst-bucket key is ever invalidated.

## Behaviour

| Mutation | Levels invalidated | Result |
|----------|--------------------|--------|
| new folder / upload / delete object / recursive delete | current level (as today) | current view shows change |
| bucket delete | bucket list (as today) | list shows removal |
| **copy** (same bucket) | source prefix key + **destination prefix key** (precise, same bucket) | destination shows the new object on navigation |
| **move** (same bucket) | source prefix key + **destination prefix key** (precise, same bucket) | source loses it, destination shows it |
| **bulk copy** (same bucket) | source level + **destination prefix key** (the bulk target prefix) | destination shows the copied objects on navigation |

- Destination/source keys share the bucket and use empty Search (a cached filtered view of either is also stale). Prefix computed via `parentPrefix`.
- Precise invalidation only — NOT a whole-cache `Clear()` (clarified). The per-session cache otherwise persists; manual refresh (`r`) still works.
- All within `internal/ui` — `make check-readonly` stays green (FR-018).

## Invariants

- V1: After any successful mutation, no stale cached level for an affected location remains.
- V2: Visibility is uniform — no action needs a manual refresh that another auto-performs.
- V3: No storage/config package change (cross-prefix invalidation lives in the UI); no cross-bucket case.

## Acceptance (tests, written first; same bucket throughout)

1. Seed object `a/x`; copy `a/x` → `b/x`; navigate to `b/` → `x` present (no manual refresh).
2. Move `a/x` → `b/x`; view `a/` → absent; view `b/` → present, both without refresh.
3. Pre-visit `b/` (cache it empty); copy into `b/`; re-navigate `b/` → shows the new object
   (proves the destination key was invalidated, not just the current view).
4. Bulk copy marked objects to prefix `b/`; navigate `b/` → copied objects present (no refresh).
5. New folder / upload / delete in the current level → reflected immediately (no regression).
