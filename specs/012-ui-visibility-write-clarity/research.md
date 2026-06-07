# Phase 0 Research: 012 UI Legibility, Hotkey Parity, Breadcrumbs & Write-Mode Clarity

Code-grounded design decisions (10-agent investigation + adversarial-verified regression findings).
All anchors are `file:line` in the current tree. No storage-contract change; no new write-S3 symbol
outside `internal/storage` (read-only structural guard stays green).

---

## R1 — Reveal/inspect popup + OSC52 copy (US1, FR-003/FR-004)

**Decision**: A dedicated, centered reveal popup (read-only) shows the full identifier (bucket / object
key / folder / breadcrumb path) and copies it best-effort to the terminal clipboard. Reuse the
`confirmPopupView` centered-overlay pattern (`confirmview.go:35-45`, `popupBoxStyle` `:26-30`) layered in
`View()` exactly like the binary confirm popup (`app.go:1146-1149`). Use Bubble Tea v2's built-in
`tea.SetClipboard(s)` Cmd (`charm.land/bubbletea/v2@v2.0.6/clipboard.go`) — it emits OSC52 at the protocol
layer (bypasses the cell renderer) and degrades silently where unsupported. The full value is always
displayed for manual selection, so copy failure never loses the value. Bind a new `Reveal` keymap field;
default key **`i`** (inspect) — free in `defaultKeys` (`keys.go:41-69`). Any key / Esc dismisses.

**Rationale**: Clarified decision = wrap **and** popup; the popup is the canonical surface for values too
long to wrap and for copy. `tea.SetClipboard` is the contract API (capability detection + no renderer
stripping). Reusing `confirmPopupView` satisfies design-system unity (FR-018/020).

**Alternatives**: hand-written OSC52 escape (rejected — bypasses the SDK contract, risks state corruption);
a full `modeReveal` (rejected — transient overlay is simpler than a sustained mode); OS-specific
`pbcopy/xclip/wl-copy` subprocess (rejected — fragile, permissions, slow); display-only (rejected — misses
the OSC52 clarification).

**Risks**: popup height on narrow terminals (cap body height = total − footerH − 2, wrap/scroll inside);
precedence vs other modals (reveal is transient, suppress while `m.op != nil`/`armConfirm`); rebind
propagation (Reveal goes in the keymap → `glyph()`/`formatKeys()` everywhere). Anchors: `confirmview.go:26-45`,
`app.go:1146-1149`, `keys.go:41-69`, `charm.land/bubbletea/v2@v2.0.6/clipboard.go:30-34`.

## R2 — Server-side current-prefix filter for the objects zone (US7, FR-029)

**Decision**: Route `/` in the objects zone (`focusZone==zoneObjects`) through the EXISTING search
plumbing — no storage change. `LevelQuery.Search` already exists and is documented for server-side
prefix narrowing (`storage.go:162-169`); `s3client.go:107` computes `effPrefix := q.Prefix + q.Search`
with `Delimiter:"/"`, i.e. a non-recursive current-level filter (exactly the clarified behaviour). The
debounce (`commands.go:293-296`), generation guard, and `cache.Key.Search` (`cache.go:9-14`,
`levelKey()` `app.go:522-524`) all already participate. Fix is in the UI dispatch only: `afterFilterEdit`
(`search.go:51-60`) and the Esc/clear branch must check `focusZone` so that bucket-zone `/` stays the
instant local bucket filter while objects-zone `/` runs the server search; `searchActive()`
(`app.go:1278-1283`) gains the same focus-aware branch.

**Rationale**: The contract was built for this (FR-017 of 010); reuse keeps one debounce/cache/gen path
and zero storage churn. Matches the clarification (server-side, current prefix, finds unloaded-page matches).

**Alternatives**: client-side filter of the loaded page (rejected — misses unloaded matches, contradicts
clarification); new `SearchLevel()` method (rejected — duplicates existing `LevelQuery.Search`).

**Risks**: `Search` is concatenated as a literal prefix (no glob/regex — correct, UI types literals);
`objectsBack()` (`app.go:491-492`) already clears search before ascend (FR-009). Anchors:
`storage.go:162-169`, `s3client.go:102-150`, `search.go:17-86`, `app.go:425,522-524,820-828`.

## R3 — Objects-zone hotkey parity (US6, FR-026/027/028) — regression fix

**Decision**: Factor a shared `onLevelKey(focusZone)` from `onTreeKey` (`tree.go:41-103`); both `onTreeKey`
and `onObjectsKey` (`app.go:445-486`) delegate to it. Add the five missing branches (Mark, Sort, SortDir,
Context, and the default `dispatchChord`/`dispatchActionKey`). Make `selKind()` (`app.go:60-73`) and
`actionCatalog()` (`hintbar.go:44-86`) treat `(mode==modeBuckets && focusZone==zoneObjects)` as a level
context so the OBJECT catalog + real `selObject/selFolder` kinds are used (not the bucket catalog /
`selNone`). Marks stay level-scoped; **bug to fix**: `loadObjectsLevel` (`app.go:394-408`) resets `treeSel`
but NOT `m.sel` — add `m.sel = nil` so marks clear on bucket/level change. Context switch from the objects
zone returns focus to buckets (global mode change, uses `prevMode`-style restore, not `objReturn`).

**Rationale**: `onObjectsKey` was shipped with navigation only (7 cases) vs `onTreeKey`'s 11 + dispatch;
18 keys verified dead in the objects zone. Shared handler = single dispatch source (aligns FR-035), no
synthetic mode (preserves the modeBuckets/modeTree boundary).

**Alternatives**: inline-duplicate the 11 cases (rejected — DRY/single-source violation); synthetic
`modeLevel` in multi-pane (rejected — erases the modeBuckets invariant, complicates restore).

**Risks**: `selKind` shift (selNone→selFolder) must not break `avail` predicates — audited, they test
`!=selFolder`/`==selObject` correctly (`hintbar.go:60-86`); focus-return after openObject/upload/confirm
(in-place model mutation preserves `focusZone`); context-switch restore must use prevMode not `objReturn`.
Anchors: `app.go:445-486,60-73,820-839,170-188`, `tree.go:41-103`, `hintbar.go:44-86`, `selection.go:13-28`.

## R4 — Bucket-column auto-grow + active-row wrap (US1, FR-001/FR-003)

**Decision**: (a) In `listWithPane` (`app.go:1200-1225`), grow the buckets column to fit the longest
**visible** bucket name, bounded by a max and by measured objects-zone slack (`slack = objW − objMinW`,
allocate up to ~half), per tier; if no slack, keep the base `paneW` clamp `[24,40]`. Measure visible
window only (lazy-load model, FR-002a). (b) Active-row wrap: a `renderTable` variant wraps ONLY the
selected row across multiple display lines when a cell overflows, within the existing `rows` budget;
`windowBounds` stays stateless (it slices data rows; one data row rendering as N lines doesn't change the
window). Pre-measure wrapped height; if `header(2)+wrapped+others > minRows`, fall back to truncation +
the reveal popup so `boxView`'s `minRows` hard-cap (`styles.go:255-256`) is never exceeded → footer never
scrolls off (FR-022). Wrap active only in Dual/Full; Single tier keeps single-line truncation.

**Rationale**: zones compete for width, so grow buckets-only (never shrink objects below legible). Active-
row-only wrap keeps scrolling stable. Reuses the `paneW` ratio+clamp idiom and `renderTable`'s `pad`.

**Alternatives**: grow all columns / measure all buckets / wrap all rows / horizontal scrollbar — all
rejected (squeeze objects zone, latency, scroll jitter, new component vs no-scroll philosophy).

**Risks**: wrapped-row height vs `minRows` (progressive truncation → popup fallback); keep `windowBounds`
unchanged. Anchors: `app.go:1193-1231,1441-1468`, `styles.go:151-256` (renderTable/boxView/windowBounds).

## R5 — Location breadcrumb (US3, FR-010/011/012/013)

**Decision**: Full path `context → bucket → prefix-chain` rendered as the objects-zone box CENTER label
in Dual/Full (replacing `objectsZoneTitle`'s bucket name, `app.go:1235`) and as the box title via
`resourceTitle` (`app.go:1346-1380`) in Single. New `elideMiddle(path, maxW)` (sibling of `truncate`
`styles.go:118`) keeps the bucket + deepest segment, drops middle prefixes, falls back to end-truncate;
the search marker `(search: …)` appends AFTER elision. Respect the `boxViewWith` center-width budget
(`styles.go:259-289`). Full path revealable via R1 popup. Prepend `ctxName` (`breadcrumb()` currently
starts at bucket — `app.go:1412-1431`). Empty prefix → no trailing slash.

**Rationale**: center label marks exactly where the user is; middle-elision preserves orientation
(start+end) better than end-truncation; reuse existing title plumbing.

**Risks**: title width collision (center cap already enforced); count `[n]` + breadcrumb coexist in Single
title; sort indicator moves to the command bar (R8) so it doesn't fight the breadcrumb for the title.
Anchors: `app.go:1233-1244,1346-1380,1411-1431,1219/1224`, `styles.go:118-137,259-289`, `tree.go:255-267`.

## R6 — Write-mode UX (US2/US4, FR-006/007/008/016/017)

**Decision**: (1) Badge space-color bug (`styles.go:411`): render the separator space with `dimCellStyle`
and only the tag text with `tagStyle`. (2) Symmetric labels in `writeColumn` (`commandbar.go:213-222`):
disarmed→"`w` enable write", armed→"`w` → read-only" (disable write), key sourced from `m.keys.WriteToggle`
(rebind-safe). (3) Prominent arm confirmation: new `armConfirmPopupView` reusing `confirmPopupView`,
overlaid in `View()` like the binary confirm; the `statusLine` armConfirm branch (`app.go:1294`) yields to
it; the write badge stays visible (FR-017); disarm stays instant (`writemode.go` `onArmConfirmKey`/
`toggleWrite` unchanged, FR-016).

**Rationale**: all three are safety-critical legibility fixes; reuse the proven popup → consistency
(FR-018/020); NO_COLOR-safe via text.

**Risks**: precedence (dispatcher already gates `armConfirm` after ops/forms); badge not clipped on narrow
terminals; NO_COLOR. Anchors: `styles.go:405-417`, `commandbar.go:213-222`, `writemode.go:17-54`,
`app.go:1294-1296`, `confirmview.go:32-45`.

## R7 — Mode chip on the box border (US2, FR-038) — *new request*

**Decision**: Render the read/write mode as a chip inset into the main browse box's TOP border, modeled
on the editor-style border-mounted mode label (the Claude Code `ultracode` chip). Add a right-label slot
to the box-border renderer `boxViewWith` (`styles.go:259-289`) — it already lays out a left label + a
centered label between border dashes; a right-aligned chip occupies the right dashes (mirror of the left
slot). Style via existing roles only: `writeBadgeStyle`/`warnStyle` (accent) when armed, `roStyle`/
`dimCellStyle` (neutral) when read-only; text `WRITE`/`RO` so it survives NO_COLOR. Rendered on the PRIMARY
list box only (clarified) — the leftmost bucket-list box in multi-zone tiers, the sole box in Single — one
fixed, always-visible location glanceable at the frame edge. The footer `[RW]`/`[RO]` badge MAY remain
(safety redundancy — explicit exception to the FR-033 dedup, encoded in FR-038).

**Rationale**: user wants the prominent, always-on border highlight like `ultracode`; the box border is
the most glanceable always-present chrome, and the title renderer already supports inset labels — a right
slot is the minimal, design-system-consistent extension (no new component, no new hue).

**Alternatives**: footer-only badge (status quo — rejected, not prominent enough per the request); a new
full-width banner (rejected — costs a row, fights footer-visibility invariant). 

**Risks**: width math — the right chip competes with the centered breadcrumb (R5) for border dashes; cap
center label = inner − leftW − rightW − dashes; on a very narrow border drop the centered label before the
mode chip (mode chip is safety-critical). NO_COLOR. Anchors: `styles.go:225-238,259-289,385-399`,
`app.go:1124-1140` (box callers).

## R8 — Sort surfaced in the command bar + date sort reachable (US8, FR-031/032)

**Decision**: Add sort as the first read-block `barEntry` in `readEntries` (`commandbar.go:65-95`):
`s name↑` (key glyph + `sortIndicator()` field+direction, `sort.go:48-55`). It inherits `fitEntries`
width-trimming (drops gracefully with `…` on narrow bars, footer-visibility preserved). `sortModified`
already exists in the cycle (`sort.go`); reachability in the objects zone comes free from R3. Remove the
duplicate sort indicator from the box title (it moves to the bar; the title is reserved for the breadcrumb,
R5) — keeps US9 declutter (one advertisement).

**Rationale**: the bar is where affordances live (open/search); reuses `barEntry`/`entryStyled`, no new
component/hue; makes a hidden capability discoverable.

**Alternatives**: put it in `infoFields` (rejected — info block is identity, not actionable); a 4th bar
block (rejected — over-engineered); symbol-only `s ↑` (rejected — field name needed for discoverability).

**Risks**: `modified↑` (9 chars) is the longest variant — acceptable to drop first on a tiny bar; coordinate
with R5 so title=breadcrumb, bar=sort (no double-display). Anchors: `commandbar.go:18,65-95,173-178`,
`sort.go:48-55`, `app.go:1365`, `keys.go:33-34,64-65`.

## R9 — Shared-component inventory / design system (US5, FR-018/019/020)

**Decision**: Canonical reusable set to map every new surface onto (no parallel styling): palette tokens
+ roles (`styles.go:14-65`), `boxView`/`boxViewFocus`/`boxViewWith` (`:225-289`), `popupBoxStyle` +
`confirmPopupView` + `typedConfirmForm` (`confirmview.go:25-83`), `metaRow`/`metaFieldRows`
(`metadata.go:11-36`), `barEntry`/`entryStyled`/`roleStyle` (`commandbar.go:25-158`), `truncate`/`pad`/
`padLine`/`layoutWidths` (`styles.go:117-146`, `app.go:214-220`), key rendering `glyph`/`formatKeys`/
`keyStyle.Bold` (`keys.go:82-112`). Mapping: reveal popup→popupBoxStyle+confirmPopupView+metaRow;
arm confirm→confirmPopupView+writeBadge; breadcrumb→titleStyle/dimCellStyle+elide; mode chip→
boxViewWith right slot + writeBadgeStyle/roStyle; sort→barEntry/entryStyled; decluttered hints→
footerHints+keyStyle. Governance: code review flags any new `lipgloss.NewStyle()`/`Color()` outside
`styles.go` (constitution VII).

**Rationale**: codifies what exists; each PR cites the component it reuses → consistency enforced.

**Risks**: drift if a dev adds a hue; NO_COLOR discipline must carry to new surfaces. Anchors as above.

## R10 — TDD test strategy (constitution III)

**Decision**: White-box `package ui` tests, failing-first, per user story; reuse existing helpers
(`newApp`/`deliver`/`press`/`viewOf`/`dualApp`/`crossToObjects`/`drillObjects`/`treeApp`/`buildApp`/
`selectObject`/`viaMenu`), assert on public `App` fields + `View().Content`/`commandBarView`/`paneView`/
`helpView`/`statusLine`. Use `storage.Fake.ListLevelCalls` (`fake.go`) to prove US7 triggers exactly one
server list and US6 marks don't. No new helpers. Representative tests: `TestBucketsColumnGrowsWithSlack`,
`TestBadgeDoesNotColorAdjacentSpace`, `TestModeChipOnBorder`, `TestBreadcrumbUpdatesOnDrill`,
`TestArmConfirmationIsCenteredPopup`, `TestNoCaretLiteralsInAnyView` (+rebind propagation across all
surfaces), `TestMarkObjectsInObjectsZone`, `TestSortCycleInObjectsZone`, `TestFilterScopesToObjectsLevel`,
`TestSortIndicatorInCommandBar`.

**Rationale**: matches the established convention (`app_test.go`, `focus_test.go`, `lazyload_test.go`,
`keys_bold_test.go`, `write_ops_test.go`); string-contains on `View().Content` is the ground truth.

**Risks**: ANSI in output (string-contains is robust); NO_COLOR + narrow-terminal edge tests follow once
core passes. Anchors: `app_test.go:22-77`, `focus_test.go:12-45`, `writemode_test.go:16-179`,
`lazyload_test.go:15-137`, `keys_bold_test.go:13-40`, `tier_test.go:1-27`, `write_ops_test.go:22-100`,
`storage/fake.go:17-97`.

---

### Cross-cutting

- **No storage-contract change** → constitution IV N/A (justified); read-only guard + integration surface
  untouched. The only storage touch is the test-only `ListLevelCalls` counter already on `Fake`.
- **Constitution VI/VII** (added v1.1.0) are the governance gates for legibility + design-system reuse.
- **Layout invariant (FR-022)**: every new surface respects `boxView`'s `minRows` cap so the footer/command
  bar never scrolls off — verified per-surface in R1/R4/R6/R7/R8.
