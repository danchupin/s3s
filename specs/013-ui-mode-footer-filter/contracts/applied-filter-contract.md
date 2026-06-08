# Contract: applied-filter chip

**Feature**: 013 | Governs FR-007..013, SC-003/004/008 | Constitution VI, VII

## Rule

When a filter is committed and the typing input is closed, the active term is shown as a persistent chip on
the **filtered pane's** box top border — NOT in the footer, NOT appended to the breadcrumb title.

## Predicate (per pane — use each pane's OWN field, not focus-relative `committedFilterTerm()`)

- **F1 (buckets box)**: render the chip iff `m.bucketFilter != "" && !m.searching`.
- **F2 (objects box)**: render the chip iff `m.search != "" && !m.searching`.
- **F3**: the tree/single primary box uses F2 (objects-level term `m.search`).

## Rendering

- **F4 (format)**: `filter: <term>` (literal exists at app.go:1532).
- **F5 (style)**: `warnStyle` (`colWarn`), echoing the typing-input accent (app.go:1428). MUST be distinct
  from `writeBadgeStyle`/`roStyle` (mode chip) and `titleStyle` (title). No new hue (VII).
- **F6 (scope cue)**: implicit from which pane carries the chip (FR-009) — no extra scope prefix.
- **C-term (cap)**: cap `<term>` with an explicit trailing `…` BEFORE handing to `boxViewWith` (it drops a
  chip whole, never elides chip text — border-chip-contract C5). Keep it short so it survives narrow borders;
  the full committed term is recoverable by re-opening the filter input (`/`, pre-filled) (FR-012, VI).

## Lifecycle

- **F7 (appears)**: on commit — `onSearchKey` enter sets the term and `m.searching=false` (search.go).
- **F8 (hidden while typing)**: the `!m.searching` gate makes the chip mutually exclusive with the transient
  `statusLine` input (FR-008, FR-013).
- **F9 (cleared)**: no clear-side code — `goBack` (tree.go:154), `objectsBack` (app.go:525), bucket back
  (app.go:939), context switch (app.go:1063) empty the term → predicate false → chip gone (FR-010, FR-011).

## MUST NOT

- **F10**: MUST NOT render in the footer/command bar (clarify; footer drops trailing entries → term could
  vanish, breaking FR-012).
- **F11**: MUST NOT keep the breadcrumb-embedded markers — REMOVE `objectsZoneTitle` ` (term*)`
  (app.go:1354-1356) and `resourceTitle` `/term*` (app.go:1478-1479). Keep the `[count]` suffix.
- **F12**: MUST NOT issue any backend call (pure render over committed state) — asserted via
  `Fake.ListLevelCalls`.

## Tests (failing-first)

- After commit on the objects level (`crossToObjects`, `/`, type, enter): objects box border contains
  `filter:`+term; ABSENT while `m.searching`; GONE after clear. `ListLevelCalls` unchanged across the chip
  render.
- Bucket-list commit: chip on the buckets box backed by `m.bucketFilter`; objects box unaffected.
- SC-003: every committed+closed state shows the term; SC-004: cleared → chip absent + full list.
