# Quickstart: The Redesigned Filter

s3s has two filter scopes, both always visible (each as its own prominent form) and both kept:

- **Buckets** — an instant local filter of bucket names (no network).
- **Objects** — a filter/search of objects within the current prefix (server-side, debounced).

## What you see

- A **filter form** is always on screen for each filterable pane: a bordered, labeled input box
  (`filter buckets` / `filter objects`) sitting directly under its list box, just above the
  footer. In the two-pane browse BOTH forms render side by side at once. Idle, a form reads
  `/ to filter`; press `/` to type into the focused pane's form.
- The **focused** pane's form is drawn in the accent border + bold label (active) and shows a
  caret while editing; the other pane's form is calmer but still a clearly bordered box showing
  its committed term — so both filters are visible regardless of focus.
- The **match count** rides each list box's title above its form:
  - buckets: `buckets[3/12]` (3 of 12 match)
  - objects: `…[8]` (8 matched; the level total is not fetched)

## Using it

```
/            open the filter for the focused pane (pre-filled with the current term)
<type>       narrow live — buckets instantly, objects debounced (the UI never freezes)
Enter        commit (the form keeps showing the term; the caret goes away)
Esc          cancel the edit (revert to the last committed term)
```

Clearing the term (empty it and commit) restores the full view and the form returns to its
`/ to filter` placeholder; navigating up a level or switching context clears that level's filter
automatically.

## Always fits

On any terminal size, the filter forms and the footer/command-hint bar stay fully visible — the
browse **list** is what shows fewer rows when the screen is small (the forms reserve a fixed
3-line band). A long filter term is shown elided (`…`); re-open the filter (`/`) to see/edit the
full term.
