# Contract: Always-Visible Filter Forms

> Design revision (per user feedback after the first cut): the filter is rendered as a **prominent
> bordered FORM per scope**, not a single dim strip. In the two-pane browse BOTH forms (buckets +
> objects) are shown at once, side by side under their panes.

## Placement

A 3-line bordered input box (`filterFieldView`) is rendered BENEATH each list box, present in the
filterable browse modes (`modeBuckets`, `modeTree`) and absent elsewhere. It is reserved chrome:
the height budget subtracts the 3-line band from the LIST, never from the footer.

```
rows := m.height - footerH - filterFieldH - 2   // filterFieldH = 3 in filterable modes, else 0
// listWithPane stacks each list box with its form: JoinVertical(listBox, filterFieldView(...))
view := body + "\n" + footer                    // the forms live inside body
```

In the two-pane `modeBuckets` layout the bucket form (width = bucket column) and the object form
(width = objects column) sit side by side; the non-focusable details zone has no form.

## States (per form)

| Condition | Form body |
|-----------|-----------|
| editing (focused scope, `searching`) | the live input + caret `▏`; the label/border is accent (active) |
| idle, committed term | the committed term (dim border/label, no caret) |
| idle, no term | `/ to filter` placeholder (dim) |

The label is `filter buckets` / `filter objects`. The FOCUSED pane's form is the accent border +
bold label; the other pane's form is calm but still a bordered, labeled box showing its term.

## Rules

- Always exactly 3 lines per form; each line fits the form width (the term/input elides with `…`).
- The editable input keeps a usable minimum width at the narrowest supported terminal (FR-006).
- The forms never cause the footer to grow or scroll (FR-005).
- `statusLine` no longer renders the filter input (it moved here); status messages (loading,
  notice, error, op-prompt) coexist with an active filter.
- The match count is NOT in the form; it rides the list box title (`buckets[M/T]`, `…[N]`).

## Acceptance

1. With no filter and not editing, both forms are still present (idle placeholder) in the two-pane
   browse; the tree view shows the single object form.
2. Opening `/`, typing, committing, and clearing all keep each form on exactly 3 lines.
3. At the minimum supported width and height, the forms and the footer hints are all visible; only
   the list shows fewer rows.
4. Both forms render at once and survive focus changes (the bucket form stays while focus is on
   objects, and vice versa).
