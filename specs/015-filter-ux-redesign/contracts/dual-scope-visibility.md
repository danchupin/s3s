# Contract: Dual-Scope Filter Visibility

Both filter scopes are kept and both are visible at once.

## Scopes

| Scope | Source of truth | Matching | Network |
|-------|-----------------|----------|---------|
| bucket | `bucketFilter` | case-insensitive substring of bucket names | none (instant local) |
| object | `search` | prefix search within the current prefix | server-side, debounced |

The two are independent: applying or clearing one MUST NOT change the other.

## Simultaneous indicators

In the two-pane browse layout (`listWithPane`), the bucket box carries the bucket chip and the
objects box carries the object chip; each is term-gated (shown whenever its scope has a committed
term) and **focus-agnostic** — moving focus between panes does not hide either committed chip.

The always-visible strip is bound to the **focused** scope (you type into one scope at a time);
the non-focused scope's committed chip + count remain visible.

## Acceptance

1. Filter buckets to `dev`, then move focus to the objects pane and filter objects to `log` →
   both chips visible: `filter: dev · 3/12` (bucket box) and `filter: log · 8` (objects box).
2. Clear the object filter → the object chip disappears; the bucket chip (`dev`) stays.
3. The bucket filter narrows instantly (no spinner); the object filter narrows debounced and the
   UI never freezes while a search is in flight (bursts coalesce).
4. Each scope's term survives focus changes; only navigating up a level / switching context
   clears that level's object filter.
