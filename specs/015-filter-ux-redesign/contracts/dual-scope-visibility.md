# Contract: Dual-Scope Filter Visibility

Both filter scopes are kept and both are visible at once.

## Scopes

| Scope | Source of truth | Matching | Network |
|-------|-----------------|----------|---------|
| bucket | `bucketFilter` | case-insensitive substring of bucket names | none (instant local) |
| object | `search` | prefix search within the current prefix | server-side, debounced |

The two are independent: applying or clearing one MUST NOT change the other.

## Simultaneous indicators

In the two-pane browse layout (`listWithPane`), the bucket box stacks the **bucket filter form**
and the objects box stacks the **object filter form**; both forms render at once and are
**focus-agnostic** — moving focus between panes does not hide either form or its committed term.
The match count for each scope rides its list box title (`buckets[M/T]`, `…[N]`).

You type into one scope at a time (the focused pane's form, which is drawn active with a caret);
the non-focused scope's form keeps showing its committed term, and both title counts stay.

## Acceptance

1. Filter buckets to `dev`, then move focus to the objects pane and filter objects to `log` →
   both chips visible: `filter: dev · 3/12` (bucket box) and `filter: log · 8` (objects box).
2. Clear the object filter → the object chip disappears; the bucket chip (`dev`) stays.
3. The bucket filter narrows instantly (no spinner); the object filter narrows debounced and the
   UI never freezes while a search is in flight (bursts coalesce).
4. Each scope's term survives focus changes; only navigating up a level / switching context
   clears that level's object filter.
