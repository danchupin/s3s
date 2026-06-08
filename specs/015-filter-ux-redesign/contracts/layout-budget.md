# Contract: Layout Budget (footer & strip never sacrificed)

## Reservation order

The vertical budget reserves, in order, then gives the rest to the list:

1. **Footer** (command bar / identity + hints / status) — measured first, never reduced.
2. **Filter strip** — one row in filterable modes (`modeBuckets`, `modeTree`).
3. **Indicator chips** — ride the box border (0 body rows).
4. **List body** — absorbs ALL remaining height loss.

```
footerH       = lines(footerBlock)
filterStripH  = 1 in {modeBuckets, modeTree} else 0
rows          = height - footerH - filterStripH - 2(border)   (min 3)
dataRows      = rows - 2(table header)                          (min 1)
```

## Invariants

- The footer/command-hint bar is fully visible at every supported width and height (Constitution
  VI) — unchanged baseline, now also covering the filter strip.
- The filter strip occupies exactly one line and never wraps; under width pressure its content
  elides, it does not add rows.
- Under height pressure the LIST shows fewer rows; the footer, strip, and chips are not sacrificed.

## Tests (extend `assertWidthSweep`)

- Width sweep 40..200 (existing `TestFooterWidthSweepNoOverflow`): footer rows ≤ baseline AND the
  filter strip is present and ≤ 1 line at every width.
- Height sweep: at decreasing heights, the footer + strip stay fully rendered; the list-row count
  decreases monotonically and never the footer/strip.

## Acceptance

1. Resizing narrower/shorter never clips the footer or the strip; only the list changes row count.
2. In a non-filterable mode (object view, connections, forms) no strip row is reserved.
