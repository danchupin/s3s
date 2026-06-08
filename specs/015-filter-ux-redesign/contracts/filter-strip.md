# Contract: Always-Visible Filter Strip

## Placement

A single row rendered BETWEEN the list body and the footer, present in the filterable browse
modes (`modeBuckets`, `modeTree`) and absent elsewhere. It is reserved chrome: the height budget
subtracts its row from the LIST, never from the footer.

```
rows := m.height - footerH - filterStripH - 2   // filterStripH = 1 in filterable modes, else 0
view := body + "\n" + filterStripView(w) + "\n" + footer
```

## States

| Condition | Strip content |
|-----------|---------------|
| `searching` (active edit) | `▌ filter <pane>: <input>` + caret + `(live) · Enter apply · Esc cancel` (object: live; bucket: instant) |
| idle, focused scope has a committed term | `▌ filter <pane>: <term>` (dim, no caret) |
| idle, no committed term | `/ to filter <pane>` placeholder (dim) |

`<pane>` is `buckets` or `objects`, from the focused scope (`filterIsBucketList`).

## Rules

- Always exactly one row; never wraps. A long input/term elides with `…`.
- The input field retains a usable minimum width at the narrowest supported terminal (FR-006).
- The strip never causes the footer to grow or scroll (FR-005).
- `statusLine` no longer renders the filter input (it moved here); status messages (loading,
  notice, error, op-prompt) coexist with an active filter.

## Acceptance

1. With no filter and not editing, the strip is still present (idle placeholder) in buckets/tree.
2. Opening `/`, typing, committing, and clearing all keep the strip on exactly one row.
3. At the minimum supported width and height, the strip, the chips, and the footer hints are all
   visible; only the list shows fewer rows.
