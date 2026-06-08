# Data Model: UI mode chip dedup, footer breathing room, applied-filter state

**Feature**: 013-ui-mode-footer-filter | **Date**: 2026-06-08

This is a presentation/UX feature. There is **no persisted data, no new storage entity, and no new App state
field**. The "entities" below are render-state derivations and shared render components. All backing fields
already exist; the feature changes how they are *displayed*.

---

## E1 — Mode chip (universal)

- **What**: the single read/write mode indicator, mounted on the top border of every browse box.
- **Derivation**: `modeChip()` (app.go:1294) → `writeBadgeStyle.Render("WRITE")` when `m.writable()`, else
  `roStyle.Render("RO")`. Text carries the state (NO_COLOR-safe).
- **Change this feature**: now mounted on **all three** browse boxes (bucket list, object level, opened
  object), not just the list boxes. The opened-object box (`modeObject`, app.go:1178) switches from `boxView`
  to `boxViewChip`.
- **Backing state**: `m.armed` / `Backend.Writable` / `m.ctxReadOnly` (via `m.writable()`) — unchanged.
- **Invariants**: exactly one mode indicator per browse screen (FR-001); visible in every browse mode
  (FR-003); NO_COLOR-safe (FR-006); rides the border → zero body rows (FR-016).

## E2 — Applied-filter chip (new render-only derivation)

- **What**: a persistent chip showing the committed filter term, on the *filtered pane's* box top border.
- **Derivation** (pure render; no new field):
  - Buckets box: shown iff `m.bucketFilter != "" && !m.searching`; text `filter: <term>`.
  - Objects box: shown iff `m.search != "" && !m.searching`; text `filter: <term>`.
  - `<term>` is capped with an explicit ellipsis before rendering (boxViewWith drops a chip whole, it does not
    elide chip text); full committed term recoverable by re-opening the filter input (`/`), which pre-fills it.
- **Style**: `warnStyle` (`colWarn`) — reuses the typing-input accent (app.go:1428); distinct from the mode
  chip (`writeBadgeStyle`/`roStyle`) and the title (`titleStyle`). No new hue (VII).
- **Scope cue**: implicit from which pane carries the chip (FR-009).
- **Backing state** (all existing, search.go / app.go): `m.search`, `m.bucketFilter`, `m.searching`,
  `m.searchInput`, `m.filterBefore`, `m.searchGen`.
- **Lifecycle**: appears on commit (`onSearchKey` enter, search.go), hidden while typing (`!m.searching`),
  removed automatically when the term is cleared (`goBack`/`objectsBack`/back/context-switch). No clear-side
  code (FR-010/FR-011/R6).
- **Invariants**: distinct from the transient `statusLine` input (FR-008/FR-013, enforced by the `!m.searching`
  gate); never scrolls the footer (border-resident; term capped → FR-012).

## E3 — Border chip slots (shared render component, extended)

- **What**: the top-border composition in `boxViewWith` (styles.go:334-406) and its wrappers.
- **Change this feature**: a **second, inboard chip slot**. Right-to-left order before the `╮` corner:
  `dashes  ‹filterChip›  ‹modeChip› ╮`.
- **Degrade order** (extends styles.go:375-385): center label dropped first → filter chip next → mode chip
  last (mode chip is safety-critical).
- **Wrappers**: `boxViewChip` / `boxViewFocusChip` thread the new param; the objects pane gains a chip-bearing
  variant (today `boxViewFocus`, no chip slot, app.go:1277/1282).
- **Per-box occupancy**:
  | Box | mode chip | filter chip |
  |-----|-----------|-------------|
  | buckets box (Single/Dual/Full) | yes | when `bucketFilter!=""` |
  | objects box (Dual/Full) | no | when `search!=""` |
  | tree/single primary box | yes | when `search!=""` |
  | object view box | yes | n/a |

## E4 — Footer separator token (single-sourced)

- **What**: the inter-element separator in the footer / command bar.
- **Change this feature**: one package-level token, widened ` · ` (w3) → `  ·  ` (w5), replacing the
  hardcoded literals at commandbar.go:63/262/276, styles.go:469/472/518/521, pane.go:54/71.
- **Self-accounting**: every fitter (`fitEntries`, `renderHintRow`, `footerIdentityCompact` cluster-append)
  re-measures via `lipgloss.Width`, so widening the token needs no math change there.

## E5 — Inter-column gap constant (derived)

- **What**: the gap between the three command-bar columns (commandbar.go:179) and its coupled natural-width
  term (commandbar.go:175).
- **Change this feature**: a single `colGap` const (2 → 3 spaces) with `natural := … + 2*len(colGap)` and
  `JoinHorizontal(Top, info, colGap, read, colGap, write)`. Removes the `+4` magic-constant double-count.

## E6 — Key↔label gap

- **What**: the space between a key glyph and its label in a command-bar entry (`entryStyled`,
  commandbar.go:159).
- **Change this feature**: 1 → 2 spaces. Self-accounting (both `blockColumn`/JoinHorizontal natural and
  `fitEntries` measure `entryStyled` width).

---

## Removed / migrated render elements

- **Footer `[RW]/[RO]` tag** in `footerIdentityCompact` (styles.go:513/515) — REMOVED (E1 owns mode state).
  The `writable bool` param is dropped; 3 callers updated (app.go:1382, commandbar.go:185/254).
- **Breadcrumb-embedded filter markers** — `objectsZoneTitle` ` (term*)` (app.go:1354-1356) and
  `resourceTitle` `/term*` (app.go:1478-1479) — REMOVED (E2 owns the term). `[count]` suffix kept.

## State transition (filter, unchanged backing — display only)

```text
no filter ──/──▶ typing (m.searching=true)  ── render: statusLine input ▌filter pane: <buf>
   ▲                │  enter (commit)
   │                ▼
   │            committed (m.searching=false, term set)  ── render: E2 chip on filtered pane border
   │                │  clear (back / esc-clear / ctx switch → term="")
   └────────────────┘
```
