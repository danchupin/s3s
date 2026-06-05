# Contract: Footer & Contextual Hints

Defines the observable behavior of the redesigned footer. Verified white-box by
asserting on `App.View().Content` at chosen widths/modes. Maps to FR-001..009,
FR-019, FR-031, SC-001..003, SC-005, SC-007, SC-009.

## C1 — Footer height

- The footer renders **at most 3 rows**: identity, hints, and (conditionally) status.
- The standalone separator rule row and the endpoint row are **removed**.
- Identity and hints are each **exactly one row** — they degrade (drop content) rather
  than wrap onto extra rows.
- Status row appears **only** when a status exists (loading / search / prompt / notice /
  error).

## C2 — Identity row

- Always shows `● <context> [RW]` (writable) or `● <context> [RO]` (read-only).
- `[RW]` uses the accent hue; `[RO]` uses the muted/OK hue (unchanged tags).
- Appends `· <cluster>` only if it fits after context+tag; otherwise omitted.
- Never shows endpoint, region, user, or version (those move to help — C-help).
- Width-capped so the row never exceeds terminal width.

## C3 — Hint catalog & priority

Hints are filtered by `hintCtx`, sorted by descending `prio`, **capped to the top
`maxHints = 6`** (highest priority first), then packed into one row. The count cap is
applied before width-packing and is independent of width: at any width, at most 6 hints
appear (FR-001, SC-003).

| key (glyph) | label | prio | visible when |
|-------------|-------|------|--------------|
| `?` | help | P0 | always (never dropped) |
| `q` | quit | P0 | always (never dropped) |
| `↵`/`enter` | open | P1 | list modes (buckets/tree/context) |
| `esc` | back | P1 | tree (not at root) / object / context |
| `/` | filter | P2 | buckets |
| `/` | search | P2 | tree |
| `esc` | clear | P2 | when `searchActive` (overrides `esc back` label; see C5) |
| `a` | actions | P3 | list modes (buckets/tree) — see note |
| `c` | context | P4 | `multiContext` |
| `1-9` | switch | P4 | `multiContext` |

The per-operation write hints (`d/u/y/m/D/+`) and the `r refresh` / `x cancel` hints are
**removed** — they are replaced by the single `a actions` hint (the menu holds those
operations + refresh). Navigation cues use **arrow glyphs**, not vim letters (FR-031,
SC-009); vim aliases never appear in the footer.

Rules:
- **FR-001/FR-028**: the footer advertises `a actions` instead of individual write keys.
- **FR-002**: in read-only the menu still exists (Refresh only), so `a actions` MAY appear
  but MUST NOT imply write capability; no individual write hint ever appears.
- **FR-003**: `c`/`1-9` hidden when `!multiContext`.
- **FR-005**: `? help` and `q quit` are P0 and survive every degrade.
- **FR-031**: nav hint tokens render `↑/↓`, `↵`/`enter`, `esc` (arrow-family), never
  `h/j/k/l`/`g`/`G`.

## C4 — Cap, degrade & "? more" cue

- **Count cap**: at most `maxHints = 6` hints are ever shown (top priority first). When
  the catalog yields >6 applicable hints, the surplus is dropped by lowest `prio` and a
  trailing `? more` cue is shown (the keymap continues in help).
- **Width degrade**: of the ≤6 capped hints, when width forces dropping ≥1, drop **lowest
  `prio` first**, and append a trailing `? more` (dim) cue. The cue's width is reserved so
  it always fits alongside the surviving P0 hints.
- The `? more` cue appears whenever hints were dropped for EITHER reason (cap or width);
  when all applicable hints (≤6) fit, no cue is shown.
- At any width in 40–200 cols, the hint row never wraps and never overflows (FR-019).

## C5 — Search disambiguation (FR-009)

- While a search/filter is active, the hint row MUST show `esc clear` and MUST NOT show
  `esc back` — the rendered cue itself signals that the next back-key press clears the
  search rather than ascending a level.
- When no search/filter is active, the hint row shows `esc back` (mode-appropriate) and
  not `esc clear`.
- Pressing the back key while search is active clears the search (existing behavior —
  unchanged); only the advertised hint label switches.

## C6 — Mode/selection reactivity (FR + Edge cases)

- Returning from object view to tree immediately restores the `a actions` hint.
- The `a actions` hint shows in both buckets and tree list modes (the menu opens in both).

## Test obligations (TDD — write first, must fail before impl)

1. `modeTree` writable at width 80 → hint row is **one line**, contains `a` (actions),
   `? help`, `q quit`, and does NOT contain `d`/`u`/`y`/`m`/`D`/`+`/`r`/`x`; footer ≤ 3 rows
   (SC-001).
2. Read-only context, any mode/selection → **zero** individual write hints (`d/u/y/m/D/+`)
   anywhere in the footer (SC-005); `a actions` may still appear.
3. Single context (`len==1`) → `1-9` and `c` hint absent.
4. Width swept 40→200 → every footer line width ≤ width; footer rows ≤ 3 (SC-002).
5. Narrow width that forces a drop → trailing `? more` present and `? help`+`q quit`
   still present (C4, FR-004/005).
6. Footer nav cues contain arrow glyphs (`↑`/`↓`/`↵`/`esc`) and NOT vim letters
   (`h`/`j`/`k`/`l`/`g`/`G`) (FR-031, SC-009).
7. Search active in tree → `esc clear` present AND `esc back` absent; search inactive →
   `esc back` present AND `esc clear` absent (C5, FR-009).
8. A state yielding >6 applicable hints (multi-context writable tree) at a wide width
   (e.g. 200 cols) → hint row shows **at most 6** hints AND a `? more` cue if any dropped
   (C3/C4 count cap, SC-003). Count rendered key tokens to assert ≤ 6.
