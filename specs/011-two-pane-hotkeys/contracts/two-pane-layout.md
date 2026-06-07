# Contract: Three-zone master-detail layout (tiers, focus, reflow)

Covers FR-001, FR-007, FR-008, FR-009, FR-013, FR-014, FR-015.

Builds on the existing single-column box composition: `View()` (`app.go:849`) draws a bordered body
over a multi-line footer; the body for `modeBuckets`/`modeTree` already routes through `listWithPane`
(`app.go:935`), which today joins one list box + one `details` box via `lipgloss.JoinHorizontal`
(`app.go:949`). 011 generalizes this to three named width tiers with a third (objects) zone. The
visible window in every zone is still computed statelessly at render via `windowBounds(n, sel, rows)`
(`styles.go:192`); only per-zone selection indices are state, so resize reflow stays trivial.

## Named tiers (normative; total terminal columns `w`)

The existing split constant is `paneSplitMin = 100` (`app.go:929`). The tiers:

| Tier   | Width range   | Box composition (left → right)            | Boxes |
|--------|---------------|-------------------------------------------|-------|
| Full   | `w >= 130`    | `buckets` │ `objects` │ `details`         | 3 |
| Dual   | `100 <= w <= 129` | `buckets` │ `objects` (details collapses) | 2 |
| Single | `w <= 99`     | current single-column mode-stack          | 1 |

- **Single** (`w < paneSplitMin`, i.e. `<= 99`) is unchanged from today: `listWithPane` returns the
  full-width `boxView` of whichever mode is active (`app.go:937`), and the persistent details pane
  collapses (existing `paneVisible`/`w < paneSplitMin` behavior, FR-013). Navigation is the existing
  mode stack: bucket list → enter → tree → enter → object view.
- **Dual** (`100..129`) shows the buckets zone beside a live objects zone; the `details` zone
  collapses. This is the existing two-box split at `paneSplitMin`, but the right box is the OBJECTS
  list (the selected bucket's level), not the details pane.
- **Full** (`>= 130`) adds the third `details` zone on the right, reusing the existing details/preview
  renderer (`paneView`, `pane.go:20`).

Composition reuses `boxView` rounded borders (`styles.go:220`), `lipgloss.JoinHorizontal(lipgloss.Top,
…)` (as `app.go:949`), and per-zone widths budgeted from `w` the same way `listWithPane` budgets the
pane today (`app.go:939`): the buckets and objects zones split the non-details remainder; details is
capped (≈24–40 cols, mirroring `app.go:940`). Each zone's body is hard-capped to its row budget by
`boxView` (`styles.go:233`) so the footer never scrolls off.

## Zone titles + active/dim styling

- `buckets` zone title: the existing `resourceTitle()` bucket form, e.g. `buckets[12]` /
  `buckets[3/12]` when filtered (`app.go:1080`).
- `objects` zone title: the existing tree title for the selected bucket+prefix, e.g.
  `mybucket[42] …` / `mybucket/logs/[7+] …` (`resourceTitle()` tree branch, `app.go:1056`), with the
  centered selection label (`selectionName()`, `app.go:1098`).
- `details` zone title: `details` (as today, `app.go:948`).
- The FOCUSED zone renders its box border + left title in the accent (bold) style; the other zone(s)
  render border + title dim. This reuses the box title styling in `boxView` (`titleStyle`/`ruleStyle`,
  `styles.go:263`); the inactive zone substitutes the dim rule/title styling. Invariant: exactly one
  browse zone is focused at a time; its accent border is the non-color-independent focus cue (the
  selection `▶` gutter, `styles.go:172`, remains the per-row cue inside each zone for NO_COLOR).

## What renders in each zone per focus (FR-008/FR-009)

- **buckets zone**: the filtered bucket table (`bucketsView`, `app.go:1147`), including the scoped
  `+ add bucket` row when applicable (`bucketAddRowVisible`, `app.go:1142`) — unchanged content.
- **objects zone**: the selected bucket's tree level (`treeView`, `tree.go:205`) at the current
  prefix/search. Empty until a bucket is selected/crossed (see lazy-load-cache.md: the level loads
  only on a settled selection). When a bucket has no objects yet loaded, the zone shows the loading or
  empty state the tree view already renders.
- **details zone** (Full only):
  - buckets focused → bucket metadata (name, created + `a analyze · ↵ open` cue) via `paneBucket`
    (`pane.go:33`);
  - objects focused → the selected object's `meta` + bounded preview, or a folder/level summary, via
    `paneTree` (`pane.go:46`). The object meta+preview are the DEBOUNCED pane loads
    (`paneMeta`/`panePrev`, see lazy-load-cache.md), never `modeObject`.

The details zone NEVER changes `m.mode` (it mirrors the existing pane invariant, `pane.go:15`): it is a
passive reflection of the focused zone's selection.

## Focus transitions (FR-007/FR-009)

- `Tab` toggles focus symmetrically buckets↔objects (keymap-contract.md); focus move only, no fetch.
- `→`/`l`/`Enter` with buckets focused crosses into the objects zone for the selected bucket (sets
  `m.bucket`, enters the level via `enterLevel`, `tree.go:116`), and focus moves to objects.
- `←`/`h`/`Esc` with objects focused ascends a prefix (existing `goBack`, `tree.go:153`) or, at the
  level root, returns focus to the buckets zone — without a screen swap in Full/Dual.
- In Single, these keys keep their legacy screen-stack meaning (enter pushes the tree mode; back pops
  it), so behavior is identical to today below 100 cols.

## Footer-always-visible invariant (FR-014)

The footer is composed and height-budgeted exactly as today: `View()` measures `footerBlock`
(`app.go:863`), reserves `footerH + 2` border lines, and `boxView` hard-caps each zone body to the
remaining `rows` (`styles.go:233`). The list-mode three-block command bar (`commandBarView`,
`commandbar.go:161`) remains the footer for `modeBuckets`/`modeTree` at every tier and wraps rather
than drops keys. Invariant: at every tier and every height `>= 24`-ish budget, the full footer
(identity + read/write blocks + help/quit) is present and not clipped; the zone bodies shrink, the
footer does not.

## Resize-reflow invariants (FR-013/FR-015)

- Crossing a tier boundary (e.g. 129→130 or 100→99) re-composes the body on the next `View()` with no
  state migration: zone selection indices are preserved, windows recompute via `windowBounds`.
- `clampSelection()` (`app.go:800`) on `WindowSizeMsg` keeps each zone's selection index in bounds; no
  fetch is triggered by resize.
- Shrinking Full→Dual drops the details zone only (its data — `paneMeta`/`panePrev` — is retained and
  reappears on growing back, no re-fetch). Shrinking Dual→Single collapses to the active mode's
  single column. Growing back is symmetric and side-effect-free.

## Test assertions (white-box `package ui`; assert on `App.View().Content`)

1. **Full tier composition.** `m.width = 140`, a bucket selected with a loaded level:
   `App.View().Content` contains all three zone titles — a `buckets[…]` token, the objects title for
   the selected bucket (e.g. `mybucket[`), AND `details`.
2. **Dual drops details.** `m.width = 110`: content contains the `buckets[…]` token and the objects
   title, but does NOT contain a `details` box title.
3. **Single == today.** `m.width = 90`, `modeBuckets`: content equals the legacy single-column bucket
   list (one bordered box, no objects/details box) — byte-for-byte the pre-011 single-column render.
   In `modeTree` at `width = 90`, content is the legacy single tree box.
4. **Focus styling.** Full tier, buckets focused: the buckets box title is rendered accent/bold and the
   objects box title dim; after `press(m, "tab")` the styling swaps (objects accent/bold, buckets dim).
5. **Details mirrors focus.** Full tier, objects focused on an object whose `paneMeta` is loaded:
   the details zone shows that object's key/size (object meta block, `pane.go:66`). With buckets
   focused, the details zone shows the selected bucket's `Bucket`/`Created` rows (`pane.go:40`).
6. **Footer always present.** At `width` 140, 110, and 90 with a modest height, `View().Content`'s
   footer region still contains the help/quit cue (`? help`, `q quit`) and the `[RW]`/`[RO]` badge —
   no tier clips the footer.
7. **Reflow no-fetch.** Deliver `WindowSizeMsg{140→110→90→140}` with Fake call counters zeroed; after
   the sequence `ListBuckets` and `ListLevel` counts are unchanged from before the resizes (resize
   never lists).
