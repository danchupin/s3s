# Research: UI mode chip dedup, footer breathing room, applied-filter state

**Feature**: 013-ui-mode-footer-filter | **Date**: 2026-06-08

All findings are grounded by reading the real `internal/ui` source and tests (4 parallel grounding agents,
every `file:line` verified). No NEEDS CLARIFICATION remain (the two spec clarifications resolved filter-chip
placement and the universal-chip approach).

---

## R1 — Universal read/write mode chip: the only browse box missing it is `modeObject`

**Decision**: Keep the existing `modeChip()` (app.go:1294 → `writeBadgeStyle.Render("WRITE")` /
`roStyle.Render("RO")`, NO_COLOR-safe). Mount the SAME chip on **all three browse boxes**: bucket list, object
level, opened object. The bucket-list and tree boxes already carry it (app.go:1256 `boxViewChip`, :1270
`boxViewFocusChip`, :1286 `boxViewChip`). The single gap is **`modeObject`** (app.go:1177-1178), which renders
via plain `boxView(...)` (chip slot empty). One-line fix: route it through `boxViewChip(m.resourceTitle(),
m.objectKind(), m.modeChip(), m.objectView(w-2,rows), w, rows)`. `boxViewWith` already composes left + center +
chip together, so the object view's center label (`objectKind`) coexists with the chip.

**Rationale**: After R2 removes the footer tag, `modeObject`'s footer (the `footerBlock` else-branch,
app.go:1382) would lose its only write-state signal. Mounting the chip there keeps FR-003 (mode visible in
every browse mode) and constitution V (always-on write signalling) intact with a single render path.

**Scope decision**: "every browse mode" = bucket list, object level, opened object (spec wording).
Overlay/menu modes (`contextSwitch`/`usage`/`connections`/`connForm`/`addBucket`/`filebrowser`) and the help
screen are NOT browse boxes — they stay chip-less. Write state cannot change while inside most of these
transient surfaces, and the help overlay keeps its own `writeBadge` prefix (app.go:1130). This matches the
spec exactly and avoids over-reach.

**Alternatives considered**: (a) chip on *every* `boxView` mode — rejected: over-broad vs the spec's "browse
mode" wording, and pollutes modal/menu surfaces. (b) keep the footer tag only for `modeObject` — rejected:
reintroduces two mechanisms (chip vs tag) by mode (Q2 chose the universal chip).

---

## R2 — Remove the duplicate footer `[RW]/[RO]` tag from `footerIdentityCompact`

**Decision**: Strip the `[RW]`/`[RO]` tag from `footerIdentityCompact` (styles.go:512-524; literals at :513
`[RO]` / :515 `[RW]`). The identity row reduces to `● ctx · cluster` (dot + `segCtxStyle` name + optional
` · ` cluster). Update its three callers: `footerBlock` non-list branch (app.go:1382), `infoColumn`
(commandbar.go:185), `collapsedBarView` (commandbar.go:254). Prefer dropping the now-unused `writable bool`
param for cleanliness (mechanical caller update).

**Exempt — must STAY** (these are the deliberate safety redundancy, NOT the duplicate the user named):
- `writeBadge` (styles.go:501) `[RW]`/`[RO]` used by the modal confirm popup (confirmview.go:36), the inline
  typed-confirm form (confirmview.go:69), and the arm-confirm popup (writemode.go:56).
- The help-overlay badge prefix (`writeBadge(m.writable())`, app.go:1130) — the help screen has no box/chip.

**Rationale**: The two always-on indicators (border chip + footer tag) are the exact duplication the user
flagged ("зачем дублируется … оставь только новый"). The chip is the kept "new" one. Modal/help badges appear
on surfaces where the chip is not shown, so removing them would *lose* signal, not dedup it.

**Alternatives considered**: dim/shorten the footer tag instead of removing — rejected: still two indicators,
fails FR-001.

---

## R3 — Applied-filter chip: per-pane border chip, per-pane state predicate

**Decision**: Render the committed filter term as a **persistent border chip on the filtered pane's box**,
only when that pane's own term is set **and** the typing input is closed:
- Buckets box → show when `m.bucketFilter != "" && !m.searching`.
- Objects box → show when `m.search != "" && !m.searching`.

Use **each pane's own field**, NOT `committedFilterTerm()` (search.go:14), which is *focus-relative* and would
mislabel a pane when focus has crossed away from a still-committed filter. Format: `filter: <term>` (the
literal already exists at app.go:1532), styled with `warnStyle` (`colWarn`) to echo the typing input's accent
bar (app.go:1428) and stay within the palette (VII, no new hue) — distinct from the mode chip's
`writeBadgeStyle`/`roStyle` and the title's `titleStyle`. **Cap the term** with an explicit ellipsis BEFORE
handing it to `boxViewWith` (the border composer drops a chip *whole* when it can't fit — it does not elide
chip text), keeping it short so it survives narrow borders; the full committed term remains recoverable by
re-opening the filter input (`/`), which pre-fills it (`startSearch`, search.go:22-27).

Scope cue is implicit from *which pane* carries the chip — no extra prefix needed (FR-009).

**Move the breadcrumb-embedded markers to the chip** (clarify: NOT in the breadcrumb title): drop the
`objectsZoneTitle` ` (term*)` suffix (app.go:1354-1356) and the `resourceTitle` `/term*` suffix
(app.go:1478-1479). Keep the `[count]` suffix (not the filter term).

**Rationale**: The chip on the pane's own border makes scope unambiguous, survives footer width pressure
(border title elides; footer drops trailing entries), and reuses the established 012 chip idiom (VII).
`!m.searching` makes it mutually exclusive with the transient `statusLine` input (FR-008/FR-013).

**Alternatives considered**: footer/command-bar field (Q1 rejected — footer drops entries → term could
vanish, violating FR-012); append to breadcrumb title (Q1 rejected — duplicates the identifier in the title).

---

## R4 — `boxViewWith` second (inboard) chip slot + explicit degrade order

**Decision**: Extend `boxViewWith` (styles.go:334-406) to accept a **second, inboard chip** (the filter chip)
in addition to the existing right-most chip (the mode chip). Render order on the top border, right-to-left
before the `╮` corner: `… dashes …  ‹filterChip›  ‹modeChip› ╮`. Degrade order when the border is too narrow
(extends the existing rule at styles.go:375-385): drop the **center** label first, then the **filter chip**,
then the **mode chip** last (the mode chip is safety-critical). Thread the new param through the `*Chip`
wrappers (`boxViewChip`, `boxViewFocusChip`); add a chip-bearing variant for the **objects pane**, which today
uses `boxViewFocus` (no chip slot, app.go:1277/1282).

Boxes that carry BOTH chips: the buckets box (mode chip + bucket-filter chip), and the tree/single primary box
(mode chip + objects-filter chip). The objects pane (Dual/Full) carries only the filter chip (mode chip arg
empty there).

**Rationale**: A single chip slot cannot express "drop the filter chip before the safety-critical mode chip".
Path A (a real second slot with an explicit degrade order) is honest about the safety ordering and keeps each
chip's width independent. Chosen over Path B (pre-compose `filterChip + " " + modeChip` into one string),
which couples their widths and drops both together — losing the mode chip with the filter chip under pressure.

**Alternatives considered**: Path B pre-composition (rejected per above); a brand-new border composer
(rejected — duplicates `boxViewWith`, violates VII reuse).

---

## R5 — Footer/command-bar spacing: one separator token + one derived inter-column constant

**Decision**: Widen three gaps and single-source them:
1. **Separator** ` · ` (width 3) → `  ·  ` (width 5). Used by `barGlobals` (commandbar.go:63), `fitEntries`
   (commandbar.go:276), `collapsedBarView` globals (commandbar.go:262), `renderHintRow` (styles.go:469/472),
   `footerIdentityCompact` (styles.go:518/521), and `pane.go` hints (:54/:71). Introduce one package-level
   token and replace the literals.
2. **Key↔label gap** in `entryStyled` (commandbar.go:159) 1 → 2 spaces.
3. **Inter-column gap** in `commandBarView` (commandbar.go:179) 2 → 3 spaces.

**Critical math**: the inter-column gap is the ONLY non-self-measuring site. `commandBarView` computes
`natural := Width(info)+Width(read)+Width(write)+4` (commandbar.go:175) where `+4` == the two 2-space gaps,
then degrades to collapsed if `natural > w`. **Derive the constant** from the gap literal:
`const colGap = "   "; natural := … + 2*len(colGap)` and `JoinHorizontal(Top, info, colGap, read, colGap,
write)`. This permanently kills the double-count hazard. Every OTHER fitter (`fitEntries`, `renderHintRow`,
`footerIdentityCompact` cluster-append) already re-measures its separator via `lipgloss.Width`, so widening
the token is self-accounting there — no math change needed.

**Rationale**: Horizontal-only widening inserts no newline → `footerH` (app.go:1139) and the body budget
(app.go:1142) are unchanged. The single failure mode is a footer *line* exceeding `w` (visual wrap → an
uncounted extra row → footer scrolls off). The self-measuring fitters + the derived inter-column constant
guarantee each tier still fits (constitution VI / FR-016).

**Alternatives considered**: bump each literal independently (rejected — drift, and the `+4` double-count
trap); add vertical padding rows (rejected — costs body rows, risks footer scroll).

---

## R6 — Clear/lifecycle: automatic, no new code

**Decision**: No clear-side code is needed. Every filter-clear path already empties the backing term, so the
chip predicate goes false and the chip vanishes:
- `goBack` (tree.go:154) clears `m.search` in modeTree.
- `objectsBack` (app.go:525) reloads with empty `m.search` in the objects zone.
- bucket-list back (app.go:939) and context switch (app.go:1063) reset the filter.

**Rationale**: The chip is a pure derivation of `m.search` / `m.bucketFilter`; "cleared with the filter"
(FR-011) is automatic. Confirmed by the existing `clear` footer affordance, co-gated on the same
`searchActive() && !m.searching` predicate (commandbar.go:93).

---

## R7 — TDD test plan (constitution III) + existing-test churn

**Decision**: Failing-first white-box tests (`package ui`), one slice per user story, then migrate the
existing assertions the change invalidates.

New failing-first tests:
- **US1a (chip on object box)**: `dualApp`/`treeApp`; open the object (`selectObject` + `press enter`,
  `m.loading=false`); assert the first border line of `viewOf(m)` contains `RO` (and `WRITE` armed). Fails
  today (app.go:1178 plain `boxView`). Pin width ≥ tier so the box renders.
- **US1b (footer tag absent)**: on a chip-bearing view, assert the footer (`footerBlock(w)` /
  `footerIdentityCompact(...)`) no longer contains `[RW]`/`[RO]`.
- **US2 (applied-filter chip)**: `dualApp` + seed; `crossToObjects`; `/` → type → `enter`; assert the objects
  box border contains `filter:`+term AND that it is ABSENT while `m.searching` AND GONE after clear. Bracket
  with `f.ListLevelCalls` to prove presentation-only.
- **US3 (wider spacing)**: `treeApp(f,true)`, `m.width=140`; assert `commandBarView(140)` (on `stripANSI`)
  contains the widened separator / inter-column gap; add a boundary test where old-natural fit but
  new-natural does not → now renders the collapsed 3-row bar (proves the natural-width math was updated).

**Existing-test churn (expected, migrate in the same change — NOT regressions):**
- `[RW]`/`[RO]` asserts on the footer tag → migrate to the chip (`WRITE`/`RO`): `operation_test.go:168/171`
  (TestFooterWriteTag), `visual_test.go:29/39`, `writemode_test.go:132/135/138/144/149`
  (TestWriteBadgeOnEveryScreen — keep its help branch on `writeBadge`), `spec012_test.go:145/148`
  (TestBadgeDoesNotColorAdjacentSpace — move the space-color invariant to the chip).
- No `*_test.go` hard-codes the ` · ` separator literal (separators are asserted via label presence), so US3
  separator widening is source-only churn — but the width guards `footer_test.go:92` `assertWidthSweep`,
  `:150` TestFooterWidthSweepNoOverflow (40..200, ≤9 rows), `:61` TestFooterFitsWidthAndShowsHints (w=60),
  `:163` narrow-drop, and `spec012_test.go:616` TestFooterVisibleAcrossTiers / `polish_test.go:41`
  TestFooterVisibleMinHeight MUST stay green (the no-overflow / no-scroll proof).

**Width pinning** (risk): `commandBarView` renders columns only ≥ `blockColMin=100` (commandbar.go:18); tiers
Single ≤99 / Dual 100–129 / Full ≥130. US3 column-gap tests pin width ≥100 (use 140); `dualApp`=120 (Dual).
Assert on `stripANSI` (us4_test.go:14) or the structured `readEntries`/`writeEntries` fields to avoid ANSI
brittleness. Helpers verbatim: `deliver`/`press`/`pressCmd`/`viewOf`/`stripANSI`; `dualApp`/`treeApp`/
`buildApp`/`withBuckets`/`crossToObjects`/`selectObject`.

**Rationale**: The dedup and spacing changes touch heavily-asserted surfaces; cataloguing the exact churn
sites up front keeps the change test-first and lets the reviewer distinguish migration from regression.
