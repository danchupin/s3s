# Contract: Pinned bucket list (load + render + navigate)

Covers FR-002, FR-003, FR-004, FR-011, SC-004, SC-005.

## Load
- `loadBuckets(ctx, st, gen, pinned []string)`:
  - `len(pinned) > 0` ⇒ synthesize `[]storage.Bucket{{Name: n}}` (zero `CreationDate`) in order;
    return `bucketsMsg{gen, buckets}` with **no** `st.ListBuckets` call.
  - `len(pinned) == 0` ⇒ call `st.ListBuckets` as today; success ⇒ `bucketsMsg`, error ⇒ `errMsg`.
- Seeded from `m.pinnedBuckets` (from `Backend.PinnedBuckets`), set in `New()` and refreshed on
  context switch (`contextResolvedMsg`).

## Render (`bucketsView`)
- Pinned buckets render as normal name rows; date column blank/`—` (no metadata).
- The visible window, selection, and filter (`filteredBuckets`, `/`) behave identically to a
  list-all set.

## Navigate
- `↑/↓`, top/bottom, `/` filter, `Enter` → `enterLevel()` on the selected bucket — all unchanged.
- Switching from bucket A to bucket B (back to list, open B) issues **no** `ListBuckets` call.

## Domain-style note (FR-011)
- With path-style off + endpoint `https://bucket.avito-sd`, opening pinned bucket `X` addresses
  `X.bucket.avito-sd` (per-bucket vhost). The pinned list never depends on the apex resolving.

## Test assertions (white-box ui)
1. `App` with `pinnedBuckets = ["alpha","beta"]`, Fake with `FailListBuckets = true`: after initial
   load, `viewOf(m)` contains `alpha` and `beta`; Fake records **0** `ListBuckets` calls.
2. `press(m, "enter")` on `alpha` → `m.mode == modeTree`, `m.bucket == "alpha"`, level loads.
3. Back to list, select+enter `beta` → `m.bucket == "beta"`; Fake `ListBuckets` count still 0.
4. `/` + type → filters the pinned names.
5. No pins (Fake with 2 real buckets, `FailListBuckets=false`): unchanged — list-all renders both,
   no `+ add bucket` row.
