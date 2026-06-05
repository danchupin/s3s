# Phase 0 Research: UI/UX Refinement (Action Menu + Footer + Help + Feedback)

All unknowns are design decisions internal to `internal/ui`. Each resolved below.

## D1 — Action menu as a new modal mode

**Decision**: Add `modeActionMenu` (a mode like `modeHelp`) with menu state on the model:
the built item list + a selection index. Opened by `a` from `modeBuckets`/`modeTree`;
rendered as a centered overlay box (reusing `boxView` styling) titled
`actions: <selection>`. `onMenuKey` handles ↑/↓ (+ vim `j`/`k`) to move, Enter/→ to invoke
the selected item, Esc/← to close. The menu is modal (owns keys while open) but does not
block background loads.

**Rationale**: Mirrors the existing `modeHelp` pattern (minimal new machinery, stateless
render from a selection index — matches the project's style). A mode keeps routing in the
existing `onKey` switch.

**Alternatives considered**:
- *Inline popup anchored to the row* — more layout math, no real benefit. Rejected.
- *Reuse the `operation` struct* — the menu precedes an operation; conflating them muddies
  the op state machine. Rejected.

## D2 — Menu items dispatch the existing `start*` functions

**Decision**: Each menu item is `{label, invoke func() (tea.Model, tea.Cmd)}` bound to an
existing entry point: `startCreateFolder`, `startRemoveObject`, `startUpload`, `startCopy`,
`startMove`, `startRecursiveDelete`, `refresh`. Invoking an item calls that function, which
already performs selection/writability validation and drives the existing name/dest entry +
two-tier confirmation.

**Rationale**: FR-026/FR-020 — zero change to operation semantics or safety. The menu is a
pure new entry point; the `start*` funcs are already factored and reused verbatim.

**Alternatives considered**: re-implement op dispatch inside the menu — duplicates logic,
risks diverging confirmation tiers. Rejected.

## D3 — Contextual item set (FR-024/FR-025)

**Decision**: Build the item list from a `menuCtx` snapshot (mode, writable, selKind):

| Context | Items (in order) |
|---------|------------------|
| buckets (any) | Refresh |
| tree, read-only | Refresh |
| tree, writable, object selected | Delete, Copy, Move/Rename, Upload here, New folder, Refresh |
| tree, writable, folder selected | Recursive delete, Upload here, New folder, Refresh |
| tree, writable, empty/none | Upload here, New folder, Refresh |

Refresh is always last and always present (sole refresh entry point after `r` removal).

**Rationale**: FR-023/024/025 + clarified menu-scope answer (buckets+tree; buckets =
Refresh only). Object-only ops gated on `selKind==object`; recursive delete on folder.

**Alternatives considered**: show all items always, disabling invalid ones — noisier, and
inconsistent with the footer's "only what applies" principle. Rejected.

## D4 — Keymap reduction & cancel folding

**Decision**: In `keys.go`, add `Menu: []string{"a"}` and remove the `Cancel: []string{"x"}`
binding. In `tree.go` `onTreeKey`, delete the `case`s for NewFolder/Delete/Upload/Copy/Move/
DeleteAll/Refresh and add `case matches(key, m.keys.Menu): return m.openActionMenu()`.
`onBucketsKey` gains the same Menu case. In `app.go`, replace the two cancel paths
(`Cancel && m.loading` at the global level, and the `phaseRunning` modal's `Cancel`) with
the Back key: when a load or a running op is in flight, `Esc`/Back cancels; otherwise Back
navigates. The `start*`/`refresh` functions remain defined and are now called only from the
menu. The keyMap fields for write ops stay (used by the menu items + help text) but are no
longer matched at top level — so a stray `d`/`u`/`y`/`m`/`D`/`+`/`r`/`x` does nothing.

**Rationale**: FR-028/FR-029/FR-030. Reuses the existing `cancelLoad()` and `start*` plumbing;
the only behavior delta is the routing table. Net top-level interactive actions: Up, Down,
Top, Bottom, Enter, Back(+cancel), Search, Context, Menu, Help, Quit (+ digit context
switch) ≈ 11–12 ≤ 12 (SC-008).

**Alternatives considered**: keep `x` as an alias for cancel — contradicts "fewer keys"
(FR-029). Rejected. Unbinding write keys entirely (removing keyMap fields) — loses the
single-source-of-truth the help text reads. Rejected; keep fields, drop top-level routing.

## D5 — Arrow-primary, vim secondary (FR-031)

**Decision**: Navigation keeps BOTH arrow and vim bindings in `defaultKeys()` (functional).
Only the *advertising* changes: footer hints and the menu's nav cue render arrow glyphs
(`↑/↓`, `Enter`, `Esc`); the help surface lists the vim aliases (`h/j/k/l`, `g/G`) alongside
the arrows. No binding is removed.

**Rationale**: FR-031/FR-014c/SC-009 — power users keep vim; the advertised surface stays
clean and beginner-legible. Pure presentation change (hint label text), no routing change.

**Alternatives considered**: drop vim bindings — breaks muscle memory, user explicitly wants
them kept (just hidden). Rejected.

## D6 — Footer composition & 3-row budget

**Decision**: `footerBlock` → identity row + hints row + optional status row. Remove the
separator rule and the endpoint row. Hints come from a catalog filtered by `hintCtx`, sorted
by priority, **capped at `maxHints = 6`**, packed to one row, dropping lowest-prio first with
a `? more` cue. Write-op hints are replaced by a single `a actions` hint; `r`/`x` hints are
gone. Identity = `● <ctx> [RW|RO]` + optional `· <cluster>`.

**Rationale**: FR-001/004/006/007 + prior clarifications (3-row budget, cap 6, `? more`,
metadata→help). The `a actions` hint replaces six, making the cap easy to satisfy.

**Alternatives considered**: keep per-op hints (status quo) — the clutter being removed.
Rejected.

## D7 — Help surface (categorized + Actions + Connection)

**Decision**: `helpLines()` → method `m.helpLines()` returning categorized sections —
Navigation, Search & View, **Actions** (the menu key + its items, marking write-only),
Context, Global — plus a **Connection** section (context, cluster, endpoint, region, user,
version). Key column derived from `defaultKeys()` (no drift); vim aliases listed alongside
arrows. Ends with an explicit close instruction.

**Rationale**: FR-010..014c. Help is now the discovery surface for both the full keymap
(incl. vim) and the menu's contents. Connection houses the footer-evicted metadata.

**Alternatives considered**: command palette / fuzzy finder — out of scope (Assumptions).
Rejected.

## D8 — Named loading, Esc-cancel hint, search-pending, notice hue

**Decision**: `statusLine` loading text names the target by state (`loading buckets…` /
`loading contents…` / `loading object…`) and shows `(Esc to cancel)` (was `x`). Add a
`searching…` pending branch while a debounced search is scheduled-but-unfired. Render
`m.notice` with a green `noticeStyle` distinct from red `errStyle`. Typed-confirm prompt
keeps showing the required target (existing `opPromptLine`).

**Rationale**: FR-015/016/017/018 + FR-029 (cancel now Esc). Pure functions of existing
state.

**Alternatives considered**: generic "loading…" / keep `x` hint — fail FR-015/FR-029.
Rejected.

---

**All NEEDS CLARIFICATION resolved.** No open unknowns blocking Phase 1.
