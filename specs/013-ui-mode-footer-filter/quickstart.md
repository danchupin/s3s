# Quickstart: manual verification — 013 UI mode chip dedup, footer spacing, applied-filter state

**Feature**: 013-ui-mode-footer-filter | maps to spec acceptance scenarios.

Build & run:

```bash
make build
./bin/s3s            # against a configured context (read-only or write-capable)
```

Automated gate (run first — TDD):

```bash
make test            # white-box package ui; the 013 failing-first tests + migrated [RW]/[RO]+width tests
make fmt vet lint check-readonly   # check-readonly MUST stay green (no new write-S3 symbol)
```

---

## US1 — one read/write mode indicator, never duplicated

1. Open the browser on a **read-only** context. → The list box top border shows the `RO` chip (right side).
   The footer identity line shows `● ctx · cluster` with **no** `[RO]` tag. *(AS1, SC-001)*
2. Press `w` then `y` to arm write (write-capable context). → The chip reads `WRITE`; footer still carries
   **no** `[RW]` tag; the mode appears exactly once. *(AS2, SC-001)*
3. Navigate: bucket list → open a bucket (object level) → `Enter` on an object (opened object). → The chip is
   present on **every** screen's top border, including the opened-object view. *(AS3, SC-002)*
4. Disable color: run with `NO_COLOR=1`. → The chip still reads `RO`/`WRITE` as text. *(SC-008)*
5. Trigger a delete/overwrite/arm confirmation. → The modal prompt still shows its own loud write badge —
   this is intentional, not a duplicate. *(AS4, FR-005)*

## US2 — applied filter state stays visible

1. With no filter, view a list. → No `filter:` chip anywhere. *(AS1, FR-010)*
2. Press `/`, type a term, press `Enter`. → Input closes; a `filter: <term>` chip appears on the **objects
   pane's** top border, styled distinctly from the mode chip; the list shows the filtered subset. *(AS2)*
3. Re-open the filter (`/`). → The transient input line returns and the persistent chip is hidden while
   typing (no double-show). *(AS2, FR-008/FR-013)*
4. Filter the **bucket list** (focus buckets, `/`, term, `Enter`). → The chip appears on the **buckets pane**
   border (scope clear from placement). *(AS4, FR-009)*
5. Clear the filter (`Esc`/back). → The chip disappears and the full list returns. *(AS3, SC-004)*
6. Apply a very long term on a narrow terminal. → The chip term is capped with `…`; the footer does not wrap
   or scroll; re-open the filter with `/` to see the full term pre-filled in the input. *(AS5, FR-012)*

## US3 — breathing room in the footer / command bar

1. Wide terminal (≥130 cols). → The command bar shows info / read / write columns with visibly larger gaps
   between blocks and between `key label` entries; no two elements appear joined. *(AS1, SC-006)*
2. Narrow terminal (≤99 cols). → The bar collapses to 3 rows; entries are separated by the wider `  ·  `
   separator and stay individually readable. *(AS2)*
3. Resize across widths 40 → 200 and shrink height to ~8 rows. → The footer (incl. the hints/command line)
   is never scrolled off and no line wraps. *(AS3, SC-005, FR-016)*

---

## Regression sweep

- `make test` — the 013 tests plus migrated assertions are green; `assertWidthSweep` (40..200, ≤9 rows) and
  `TestFooterVisibleMinHeight` pass with both chips mounted and spacing widened.
- `Fake.ListLevelCalls` is unchanged across the applied-filter chip render (presentation-only).
- `make check-readonly` green — read-only posture preserved (FR-018 / SC-007).
