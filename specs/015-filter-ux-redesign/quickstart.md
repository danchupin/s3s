# Quickstart: The Redesigned Filter

s3s has two filter scopes, both always visible and both kept:

- **Buckets** — an instant local filter of bucket names (no network).
- **Objects** — a filter/search of objects within the current prefix (server-side, debounced).

## What you see

- A **filter strip** is always on screen (one line, just above the footer) in the bucket and
  object browse views. Idle it reads `/ to filter buckets` (or `objects`); press `/` to type.
- The **active filter** for each scope is shown as a chip on that pane's border, with a match
  count:
  - buckets: `filter: dev · 3/12` (3 of 12 match)
  - objects: `filter: log · 8` (8 matched; the level total is not fetched)
- Both chips show at once when both scopes are filtered — they do not depend on which pane is
  focused.

## Using it

```
/            open the filter for the focused pane (pre-filled with the current term)
<type>       narrow live — buckets instantly, objects debounced (the UI never freezes)
Enter        commit (the chip + count stay; the strip returns to idle)
Esc          cancel the edit (revert to the last committed term)
```

Clearing the term (empty it and commit) removes the chip and restores the full view; navigating
up a level or switching context clears that level's filter automatically.

## Always fits

On any terminal size, the filter strip, the indicator chips, and the footer/command-hint bar stay
fully visible — the browse **list** is what shows fewer rows when the screen is small. A long
filter term is shown elided (`…`); re-open the filter (`/`) to see/edit the full term.
