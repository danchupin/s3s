# Contract: Layout Budget (footer & forms never sacrificed)

## Reservation order

The vertical budget reserves, in order, then gives the rest to the list:

1. **Footer** (command bar / identity + hints / status) — measured first, never reduced.
2. **Filter forms** — a fixed 3-line band in filterable modes (`modeBuckets`, `modeTree`), stacked
   under the list box(es) by `listWithPane`.
3. **List body** — absorbs ALL remaining height loss.

```
footerH       = lines(footerBlock)
filterFieldH  = 3 in {modeBuckets, modeTree} else 0   // a bordered input box (top, input, bottom)
rows          = height - footerH - filterFieldH - 2(border)   (min 3)
dataRows      = rows - 2(table header)                          (min 1)
```

## Invariants

- The footer/command-hint bar is fully visible at every supported width and height (Constitution
  VI) — unchanged baseline, now also covering the filter forms.
- Each filter form occupies exactly 3 lines and never wraps; under width pressure its content
  elides, it does not add rows.
- Under height pressure the LIST shows fewer rows; the footer and the forms are not sacrificed.

## Tests (extend `assertWidthSweep`)

- Width sweep 40..200 (existing `TestFooterWidthSweepNoOverflow`): footer rows ≤ baseline AND each
  filter-form line is ≤ the form width at every width.
- Height sweep (`assertHeightSweep`): at decreasing heights, the footer + forms stay fully rendered
  (the composed view fills exactly the height); the list-row count decreases, never the footer/forms.

## Acceptance

1. Resizing narrower/shorter never clips the footer or a form; only the list changes row count.
2. In a non-filterable mode (object view, connections, forms) no form band is reserved.
