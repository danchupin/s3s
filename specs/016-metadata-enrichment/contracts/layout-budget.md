# Contract: Layout / Height Budget (FR-016/FR-017, constitution VI)

**Surface**: `View()` height budget (`app.go:1138-1168`); `listWithPane`
(`app.go:1264-1312`); `boxViewWith` hard cap (`styles.go:341-350`); the details pane
(`pane.go`).

## The REAL failure mode (the draft mis-modelled this)

`boxViewWith` HARD-CAPS the body to `minRows` (`styles.go:348-350`: `if len(lines) >
minRows { lines = lines[:minRows] }`), and the footer is composed AFTER and outside the
body (`app.go:1222`). So the footer can NEVER be scrolled off — the failure mode is
**silent truncation** of pane content (clipped rows vanish with no reveal). Tests that only
assert "footer present" ALWAYS pass and prove nothing; the budget must be checked by
asserting seeded VALUES survive or are reveal-marked.

## Budget (concrete, at the supported minimum)

```
footerH      = strings.Count(m.footerBlock(w), "\n") + 1   (app.go:1138-1139; multi-row in browse modes)
filterFieldH = 3   in modeBuckets/modeTree                 (app.go:1151-1154)
rows         = m.height - footerH - filterFieldH - 2        (app.go:1159, floored at 3)
dataRows     = rows - 2                                      (app.go:1165, floored at 1)
```

The Full-tier details zone (≥130) receives `rows-2` (`browseDetailsView(detW-2, rows-2)`,
`app.go:1298`); the Dual/Single pane receives `rows-2` (`paneView(paneW-2, rows-2)`,
`app.go:1310`). All added content — enriched object metadata (omit-empty), inline usage
totals, and AT MOST ONE expandable section (breakdown XOR tags XOR config) — MUST fit this
`rows-2` body budget.

## Invariants

- I1 (VI): at 80×24 AND 130×24 with object metadata + one detail section shown, NO seeded
  identifier is silently lost: each is present in `View().Content` OR represented by a
  trailing `… +N more (i to reveal)` line whose clipped rows are recoverable via
  `keys.Reveal`. Verified by a height sweep that seeds ALL enriched optional fields + one
  section and asserts value-presence (NOT footer-presence).
- I2 (FR-003): the object pane omits absent optional fields, so a typical object renders
  few rows and the budget holds comfortably.
- I3 (US3 budget gate): the detail zone shows ONE section at a time (`detailSection`);
  expanding the breakdown collapses the bounded preview first (`pane.go:93` yields
  `max(1, rows-8)`); the breakdown and US4 tags/config are mutually exclusive (they cannot
  coexist in a 24-row detail zone), enforced by `detailSection`.
- I4 (VI): added identifiers are fully visible or revealable (`keys.Reveal`); a width sweep
  (`assertWidthSweep`, `footer_test.go:92`) over the supported range asserts no permanently
  truncated identifier and the footer present at every width.

## Testable assertions

- Height sweep at 130×24 (and 80×24): seed an object with every enriched optional field +
  one detail section; assert every seeded value is present OR reveal-marked, and the footer
  is present.
- One-section gate: pressing `MoreDetail` to open tags after a breakdown is open replaces
  (not stacks) the section; assert only one section's rows appear.
- Width sweep `[lo, hi]` over the object-open + inline-usage builds: footer present,
  `lipgloss.Width(line) ≤ w` for every line, no hidden identifier.
