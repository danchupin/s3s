# Feature Specification: Two-Pane Browse + Hotkey Mnemonic Review

**Feature Branch**: `011-two-pane-hotkeys`

**Created**: 2026-06-07

**Status**: Draft

**Input**: User description: "Два улучшения UI. (1) Мелкое — ревью хоткеев, назначить более логичные (например непонятно почему `n` = connections), по тому же принципу остальные; плюс выделить хоткеи жирным. (2) Глобальное — разделить основной экран на два блока: левый — список бакетов, правый — объекты в бакете; как сейчас, но объекты первого уровня видны без явного открытия бакета, просто листая. Блоки выделить красивыми рамками в нашем стиле."

## Clarifications

### Session 2026-06-07

- Q: How does navigation work in the new multi-pane browse screen? → A: Focus crosses between panes (Miller-columns / ranger style): buckets stay on the left; `→`/`Tab` moves focus into the objects pane (navigate + drill folders there); `←`/`Esc` returns focus toward buckets.
- Q: The current right pane shows metadata + preview of the highlighted item (feature 006). The new design wants the bucket's contents there. What happens to metadata/preview? → A: Keep an adaptive details pane as a third zone — when a bucket is focused it shows the bucket's metadata; when an object is focused it shows the object's metadata + bounded preview. (Motivation: bucket metadata is expected to grow in future.)
- Q: Three zones only fit a wide terminal (~130+ columns). Behaviour on medium/narrow? → A: Details collapses first. The normative tiers (see "Layout tiers"): Full ≥ 130 cols = three zones (buckets | objects | details); Dual 100–129 cols = two zones (buckets | objects), object details reachable via the full-screen Enter view; Single ≤ 99 cols = one column, current behaviour (buckets → Enter → objects → Enter → object) unchanged.
- Q: How deep should the hotkey remap go? → A: Full mnemonic review of every binding; rebind the illogical ones with a migration table; bold key glyphs in hints/help regardless.
- Q: After dropping `n`, keep a global accelerator key for "add connection"? → A: No. Drop the global key entirely; "add connection" is reached only via the existing "+ add connection" row (connections list, mirroring feature 010's "+ add bucket"). Migration record: `n` → removed (row-only).
- Q: In multi-pane mode, what does Enter on a highlighted bucket do? → A: It crosses focus into the objects zone (same as `→`/`Tab`) — Miller-columns model. There is NO full-width single-column level view in multi-pane; a full-width level listing exists only in the Single tier (≤ 99 cols). Full-screen view is reached only by Enter on an object.
- Q: Is `Tab` a one-way cross or a symmetric focus toggle? → A: Symmetric toggle. `Tab` flips focus between the buckets and objects zones in BOTH directions (from a deep objects level it jumps straight back to buckets), preserving each zone's cursor. `←`/`h`/`Esc` remain the ascend/back path (FR-009); `Tab` is the direct pane switch.
- Q: Lazy-load + caching policy for bucket contents (directive: no eager all-bucket listing at startup)? → A: Startup lists only bucket NAMES; a bucket's first level is listed lazily only when its selection settles (FR-002a). Load fetches the first page, paging on demand (FR-006a). Successful and empty listings are cached; failed/denied listings are NOT cached and are re-attempted on revisit (FR-006b/c). In-flight loads are de-duplicated (FR-006d). Cache = the existing per-session TTL-free level cache keyed by (context,bucket,prefix,search), invalidated only by `r` (FR-006e).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Peek a bucket's contents without opening it (Priority: P1)

While browsing the bucket list, the operator wants to see what is inside each bucket without committing to open it. As the selection moves over the bucket list, a second pane to the right shows that bucket's first-level entries (folders and objects), updating live as the operator scrolls. Each pane is drawn inside the project's rounded border style, so the two blocks read as distinct zones.

**Why this priority**: This is the headline of the "global" improvement and the single most valuable change — it turns bucket browsing from "open, look, go back, open the next" into a continuous scan. It is independently shippable as a passive preview even before cross-pane navigation exists (Enter still drills in full-screen as today).

**Independent Test**: Open the browser with several buckets. Move the bucket cursor down one row. Without pressing Enter, confirm the right pane now lists the newly-highlighted bucket's top-level entries. Move again; confirm it updates. Confirm scrolling the bucket list stays responsive (no freeze) on a slow backend.

**Acceptance Scenarios**:

1. **Given** a connection with ≥2 buckets in multi-pane mode, **When** the operator highlights a bucket, **Then** the right pane shows that bucket's first-level folders and objects.
2. **Given** the operator scrolls the bucket list quickly across N buckets, **When** selections change faster than listings complete, **Then** at most one backend listing is issued (for the settled selection) and input dispatch is not blocked while a listing is in flight.
3. **Given** a highlighted bucket that is empty, **When** its contents load, **Then** the right pane shows the exact `(empty)` marker, not a blank or stale pane.
4. **Given** a highlighted bucket whose contents are still loading, **When** the load is in flight, **Then** the right pane shows a `loading…` marker and the bucket list remains usable.
5. **Given** the operator returns to a previously-viewed bucket within the same session, **When** it is re-highlighted, **Then** its contents appear from cache with no additional backend listing call.
6. **Given** multi-pane mode, **When** the screen renders, **Then** each zone is drawn with the project's rounded border and a labelled title, and the focused zone's border/title use the accent (active) style while unfocused zones use the dim style — so the blocks read as distinct, consistently-styled areas.

---

### User Story 2 - Navigate into the objects pane and drill without leaving the bucket list (Priority: P1)

Having seen a bucket's contents in the right pane, the operator wants to act on those objects — move the cursor among them, enter sub-folders, open an object — while the bucket list stays visible on the left. Focus crosses from the bucket pane into the objects pane and back, Miller-columns style.

**Why this priority**: Without focus crossing, the contents pane is read-only scenery. This is what makes the two-pane layout a real browsing model rather than a preview gimmick, and it is the interaction the user explicitly described ("листать объекты первого уровня").

**Independent Test**: With contents shown for a highlighted bucket, press `→` (or `Tab`); confirm focus moves into the objects pane (active-zone indicator switches) and the cursor now moves among objects. Enter a sub-folder; confirm the objects pane descends while the bucket list stays put. Press `←` (or `Esc`) at the root; confirm focus returns to the bucket list.

**Acceptance Scenarios**:

1. **Given** the objects pane shows a bucket's contents and focus is on the bucket list, **When** the operator presses `→`/`Tab`, **Then** focus moves into the objects pane and the active zone is visually distinct (border/title highlight), with its own cursor.
2. **Given** focus is in the objects pane on a folder, **When** the operator presses Enter/`→`, **Then** the objects pane descends into that folder's level and the bucket list stays unchanged on the left.
3. **Given** focus is in the objects pane on an object, **When** the operator presses Enter, **Then** the full-screen object view opens (current behaviour), and leaving it returns to the same objects-pane position.
4. **Given** focus is in the objects pane at the bucket's root level, **When** the operator presses `←`/`Esc`, **Then** focus returns to the bucket list without changing the highlighted bucket.
5. **Given** focus is in the objects pane several levels deep, **When** the operator presses `←`/`Esc`, **Then** the objects pane ascends one level (and only returns focus to buckets from the root).

---

### User Story 3 - Preserve the details pane as an adaptive third zone (Priority: P2)

The operator wants the existing metadata/preview pane (feature 006) kept — not removed by the new layout — and made to adapt to what is focused: a bucket's metadata when a bucket is highlighted, an object's metadata plus a bounded content preview when an object is focused. This is preservation and relocation of an existing pane, not a brand-new capability.

**Why this priority**: It safeguards an existing feature rather than building something new, and it is additive — US1+US2 deliver the core value without it. The operator asked to keep it because bucket metadata is expected to grow later; *what* bucket metadata contains is out of scope here — this story only guarantees the pane survives and adapts. The Full-tier (three-zone) width threshold is an engineering accommodation for fitting the preserved pane, not a user-stated requirement.

**Independent Test**: On a wide terminal (≥130 cols), highlight a bucket and confirm the third pane shows bucket metadata (name, creation date, …). Cross into the objects pane and highlight an object; confirm the third pane switches to that object's metadata + a short preview. Shrink the terminal below the three-zone threshold; confirm the details pane collapses and object details remain reachable via the full-screen Enter view.

**Acceptance Scenarios**:

1. **Given** a wide terminal showing all three zones and focus on the bucket list, **When** a bucket is highlighted, **Then** the details pane shows that bucket's metadata.
2. **Given** focus has crossed into the objects pane, **When** an object is highlighted, **Then** the details pane shows that object's metadata and a bounded preview (reusing the existing details-pane fields).
3. **Given** the terminal is narrowed below the three-zone threshold, **When** the layout reflows, **Then** the details pane collapses (buckets | objects remain) and object details stay reachable via the full-screen object view.

---

### User Story 4 - Hotkey clarity: mnemonic review + bold key glyphs (Priority: P2)

The operator wants the keybindings to be predictable and the keys themselves to stand out visually. Every advertised action key is reviewed for a defensible mnemonic; clearly illogical bindings are rebound (the operator called out `n` = "connections"); and every key glyph in the hint bar and help surface is rendered bold so it is unmistakable.

**Why this priority**: The operator called this the "minor" improvement — *minor in scope, not in value*. It is fully decoupled from the layout work (touches only the keymap, hint bar, and help) and is the cheapest, quickest win here, so it CAN and arguably SHOULD ship first/independently. It is tagged P2 only to mark the layout (US1/US2) as the larger, operator-stated headline — not to park this behind it.

**Independent Test**: Open the help surface; confirm every action lists a key whose mnemonic is explained in one line, and every key glyph renders bold. Confirm the previously-illogical binding is changed and a migration note documents old→new. Confirm navigation keys (arrows + vim `hjkl`/`gg`/`G`) and the locked keys (Enter, Esc, `q`, `?`, `:`, Space) are unchanged.

**Acceptance Scenarios**:

1. **Given** the hint bar or help surface is visible, **When** it renders an action key, **Then** the key glyph is bold and visually distinct from its label.
2. **Given** the previously-illogical binding, **When** the operator consults help, **Then** the action is on a mnemonic key and a migration note records the old→new change.
3. **Given** `$NO_COLOR` is set, **When** the hints render, **Then** keys remain distinguishable from labels via a non-color cue (so bold/emphasis is not the only signal).
4. **Given** the full keymap, **When** reviewed, **Then** navigation (arrows + vim aliases) and the locked global keys (Enter/Esc/`q`/`?`/`:`/Space) are unchanged from today.

---

### Edge Cases

- **Scoped / pinned connections (feature 010)**: when credentials are bucket-scoped (no `s3:ListAllMyBuckets`), the bucket list is the pinned set. Highlighting a pinned bucket lists its first level normally. If the objects-pane listing of a bucket is denied (403), the objects pane shows an honest error in-pane and the bucket list stays usable — no crash, no mislabeling.
- **Rapid scrolling**: holding a movement key changes the highlighted bucket faster than listings can complete; only the settled selection is fetched, and superseded listings are dropped (never render under the wrong bucket).
- **Empty bucket / empty level**: explicit "(empty)" state in the objects pane.
- **Filtered bucket list (`/` search active on buckets)**: the objects pane reflects the highlighted bucket within the filtered subset; the cursor indexes the filtered list.
- **Search semantics by focus**: `/` filters buckets when focus is on the bucket list, and searches the current level (prefix) when focus is in the objects pane — preserving today's per-context search behaviour.
- **Multi-select / marks**: marks belong to the focused objects level; changing the highlighted bucket or ascending out of the level **clears** those marks (level-scoped, never carried across buckets — per FR-012), consistent with today's `goBack`/`enterLevel` behaviour.
- **Terminal resize across thresholds**: crossing the 100 / 130-column boundaries reflows the layout without losing the highlighted bucket, the objects cursor, or the focused zone; borders re-wrap cleanly.
- **Very small terminals**: below the single/two-column thresholds the experience degrades to the current single-column mode-stack with no loss of function.
- **Height pressure**: with few rows available, the panes shrink but the footer (identity + the always-visible hint line incl. help/quit) must never scroll off.
- **Stale-vs-cache alignment**: if the highlighted bucket changes while a prior listing is still loading, the objects pane must not briefly show the previous bucket's contents under the new bucket's title.

## Requirements *(mandatory)*

### Layout tiers (normative)

Three named tiers govern the browse screen. The column boundaries below are the **normative defaults**; the exact pixel/column values may be tuned during planning, but the three-tier structure and their ordering are fixed.

- **Full tier — terminal width ≥ 130 columns**: three zones side by side — *buckets | objects | details*.
- **Dual tier — width 100–129 columns**: two zones — *buckets | objects*; the details zone is collapsed (object details remain reachable via the full-screen object view).
- **Single tier — width ≤ 99 columns**: one column — today's mode-stack (buckets → Enter → objects → Enter → object), unchanged.

"Multi-pane mode" means the Full or Dual tier. Boundaries are inclusive as written (130 = Full; 129 = Dual; 100 = Dual; 99 = Single).

### Functional Requirements

#### Multi-pane browse (US1 / US2)

- **FR-001**: In multi-pane mode, the browse screen MUST present the bucket list and the highlighted bucket's first-level entries as two side-by-side zones, each drawn in the project's existing rounded-border style, with visually distinct zone titles so the blocks read as separate areas.
- **FR-002**: The objects (middle) zone MUST show the first-level folders and objects of the currently-highlighted bucket, and MUST update as the bucket selection changes — without the operator pressing Enter or otherwise "opening" the bucket.
- **FR-002a** (lazy, no eager listing): At startup and on entering the browse screen, the app MUST list only bucket NAMES (the bucket list). It MUST NOT list any bucket's object level eagerly, and MUST NOT pre-list the contents of all buckets. A bucket's first-level contents are listed only when that bucket becomes the *settled selection*, where "settled" means the highlighted bucket has not changed for the debounce interval of FR-003. (Scoped/pinned connections per FR-018 still synthesise the bucket list from pins without an object-level listing.)
- **FR-003**: Listing a highlighted bucket's first level MUST NOT block input. Listings MUST be debounced (named ceiling: ≤ 200 ms after the selection settles — exact value tuned in planning) so that fast scrolling does not issue one backend listing per intermediate selection; only the settled selection is fetched.
- **FR-004**: A listing that is superseded by a newer selection MUST be discarded; the objects zone MUST never display one bucket's contents under another bucket's title.
- **FR-005**: The objects zone MUST show explicit states without disturbing the bucket list: an empty bucket renders the exact marker `(empty)`; an in-flight load renders a `loading…` marker; a denied/failed listing renders an `error:`-prefixed line (the `(empty)` text is normative; `loading…` / `error:` prefixes follow the existing UI cue convention).
- **FR-006**: Re-highlighting a bucket already listed in the current session MUST serve its contents from the existing per-session cache with no additional backend listing call.
- **FR-006a** (lazy extent): A lazy first-level load fetches the first page of that level for the objects-zone preview; further pages MUST be fetched on demand only when the operator scrolls within or enters that level (reusing the existing level paging), never eagerly and never for unfocused buckets.
- **FR-006b** (cache success incl. empty): A successful listing — *including an empty level* — MUST be cached so that re-highlighting that bucket within the session issues zero additional backend listings.
- **FR-006c** (errors not cached): A failed or denied listing (e.g. a `403` on a scoped bucket per FR-018) MUST NOT be cached. Re-highlighting that bucket MUST re-attempt the listing, so a transient or permission error can recover without a manual refresh.
- **FR-006d** (in-flight de-duplication): Re-selecting a bucket whose first-level listing is already in flight MUST NOT issue a duplicate backend call; the existing in-flight load serves the re-selection.
- **FR-006e** (cache scope & invalidation): The objects-zone cache MUST be the existing per-session, TTL-free level cache keyed by (context, bucket, prefix, search) — the same cache the full-screen level view uses, not a separate one. It persists for the session and is invalidated only by manual refresh (`r`) of the focused level. Entries are not separately capped (consistent with today's level cache): each entry is a small listing and the cache lifetime is bounded by the session.
- **FR-007**: Focus MUST be crossable between zones and the focused zone MUST be rendered with a single deterministic active-zone indicator: the focused zone's border and title use the active (accent) style, every unfocused zone uses the dim style. Crossing INTO the objects zone (from the bucket list) is bound to `→` / `l` / `Tab` / Enter-on-a-bucket; returning toward the buckets is governed by FR-009. In multi-pane mode, Enter on a bucket ONLY crosses focus — it never opens a full-width single-column level view (that view exists only in the Single tier); the bucket's level always renders inside the objects zone, and full-screen view is reached only by Enter on an object (FR-010). `Tab` is a symmetric focus toggle: it flips focus between the buckets and objects zones in both directions (from any objects-level depth straight back to buckets), preserving each zone's cursor; it does not ascend levels (that is `←`/`h`/`Esc`, FR-009).
- **FR-008**: Each zone MUST maintain its own selection cursor; moving the cursor in one zone MUST NOT move the cursor in another.
- **FR-009**: Multi-pane back/descend semantics MUST be focus- and level-aware, with this precedence when focus is in the objects zone:
  - Enter / `→` / `l` on a folder → descend the objects zone into that folder's level; the bucket list stays unchanged.
  - `←` / `h` / `Esc` → (1) if a search is active in the objects level, clear it; else (2) if below the bucket root, ascend one level; else (3) at the root, return focus to the bucket list (the highlighted bucket does not change).

  When focus is on the bucket list, `←` / `h` / `Esc` keep their current meaning (clear bucket filter / quit-or-back), unchanged from today.
- **FR-010**: With focus in the objects zone, Enter on an object MUST open the existing full-screen object view; returning from it MUST restore the prior objects-zone position and focus.
- **FR-011**: The objects zone MUST honour the same search, marking, sorting, and per-item action semantics the current full-screen level view provides, scoped to the focused level (search filters/searches the focused level; marks belong to that level). *Carrying the full level toolset into the zone is derived consistency-preservation, not a new user ask — planning MAY defer marks/sort scoping if the two-pane core slips, as long as cursor movement + drill + open work.*
- **FR-012**: Changing the highlighted bucket or ascending out of an objects level MUST clear any marks made in that level (marks are level-scoped and do not survive leaving the level — consistent with today's `goBack`/`enterLevel` behaviour). Marks are never carried across buckets.

#### Adaptive details zone — preserve feature 006 (US3)

- **FR-013**: In the Full tier, the existing details pane (feature 006) MUST be **preserved and relocated** into the layout as a third, non-focusable zone that adapts to focus: bucket metadata when a bucket is focused; object metadata plus the existing bounded preview when an object is focused. This is preservation of an existing capability, not a new pane. (Extending what bucket metadata contains is explicitly out of scope for this feature; the zone simply hosts whatever the bucket-metadata view provides now and later.)
- **FR-014**: The details zone's metadata/preview load MUST be non-blocking and debounced, and superseded loads MUST be dropped (identical to the existing pane behaviour — no new load machinery).
- **FR-015**: The layout MUST adapt across the tiers of "Layout tiers" above: details collapses first (Full → Dual), then the objects zone collapses (Dual → Single). Resizing across tier boundaries MUST reflow without losing the highlighted bucket, the objects cursor, or the focused zone.
- **FR-016**: The Single-tier (narrow) experience MUST remain behaviourally equivalent to today's buckets → objects → object navigation, including Enter-on-a-bucket drilling full-screen (no regression for narrow terminals).

#### Cross-cutting

- **FR-017**: The footer (identity line and the always-visible hint line, including help and quit) MUST remain visible at all supported sizes; panes MUST yield height to keep it on screen.
- **FR-018**: For scoped/pinned connections (feature 010), the objects zone MUST list a pinned bucket's first level normally and MUST surface an honest in-pane error if a listing is denied, without crashing or mislabeling the connection state.

#### Hotkeys (US4)

- **FR-019**: Every advertised action MUST have a key documented by a one-line mnemonic in the help surface. Write-family actions (upload, copy, move, delete, recursive delete, new folder, write-toggle) MAY be advertised while write is disarmed but MUST keep their existing capability tag (e.g. "(needs --write)") so a bold, advertised key is never mistaken for a live read-only action.
- **FR-020**: The "add connection" action MUST NOT have a global single-key binding. The current `n` binding MUST be removed and no replacement global accelerator added. "Add connection" is reached only via the existing "+ add connection" row in the connections list (`connections.go`, mirroring feature 010's "+ add bucket"). The migration note records `n` → removed (row-only).
- **FR-021**: The full keymap MUST be reviewed and the outcome recorded as a migration note listing every changed binding as old→new. The review's only hard, testable requirement is FR-020 (add-connection off `n`); the broader "make every key mnemonic" outcome is delivered as the migration-table artifact, not as a per-key pass/fail.
- **FR-022**: The set of keys MUST NOT shrink or be re-lettered for navigation/locked actions: arrow keys, the vim aliases `h`/`j`/`k`/`l`/`g`/`G`, and the locked global keys (Enter, Esc, `q`, `?`, `:`, Space) keep their identities. In multi-pane mode, `→`/`←`/`h`/`l`/`Esc`/`Tab`/Enter additionally gain the focus-aware meanings defined in FR-007/FR-009/FR-010; no NEW global letter key is introduced for pane navigation.
- **FR-023**: Every action key advertised in the hint bar and the help surface MUST carry a bold text attribute on the key glyph (verifiable: the rendered key string includes the bold ANSI attribute, distinct from its label).
- **FR-024**: Key emphasis MUST NOT rely on color alone. Under `$NO_COLOR` (color stripped) each advertised key MUST remain set apart from its label by a deterministic non-color cue (the existing bold attribute and/or a stable delimiter), so the key is still identifiable in a plain-text render.
- **FR-025**: The hint bar, the help surface, and the key-dispatch logic MUST stay derived from one keymap source, so a rebind updates dispatch, hints, and help together (no drift).

#### Posture & non-regression

- **FR-026**: The feature MUST NOT introduce any write-capable storage operation into the UI layer; the structural read-only guard MUST stay green (the objects zone uses only existing read methods).
- **FR-027**: The feature MUST NOT add a new backend listing capability; the highlighted bucket's first level uses the existing first-level listing.

### Proposed keymap changes *(reference for planning; final keys confirmed in /plan)*

The current scheme is already largely mnemonic (k9s/vim heritage). A full review found exactly one clearly-illogical binding plus two "weak-but-keep" cases:

| Action | Current | Proposal | Verdict & rationale |
|--------|---------|----------|---------------------|
| Add connection | `n` | **removed** → "+ add connection" row only | **Illogical** → unbind (FR-020). `n` ("new what?") surprises from the bucket list. No global accelerator replaces it; the existing "+ add connection" row (mirrors 010's "+ add bucket") is the sole affordance. |
| Copy (yank) | `y` | `y` (keep) | **Weak** but familiar (vim yank). Rebinding costs muscle memory for little gain. *Write-gated.* |
| Move chord | `ctrl+o` | `ctrl+o` (keep) | **Weak** (`o` ≈ "move"?), but `ctrl+m` is Enter (reserved); `ctrl+o` is the least-bad free chord. *Write-gated.* |
| All others | — | unchanged | **Good** mnemonics: `a` analyze, `d` download (read); `/` search, `r` refresh, `c` context, `s`/`S` sort, `:` command, `?` help, `w` write-toggle; and the *write-gated* family `u` upload, `x`/`X`/`ctrl+x` delete, `m` move, `+` new folder. Write-gated keys keep their "(needs --write)" tag (FR-019). |

Cross-pane focus reuses the existing navigation set in a focus-aware way (FR-022) — no new global letters: `→`/`l`/`Tab`/Enter-on-a-bucket cross into the objects zone, `←`/`h`/`Esc` ascend/return per FR-009.

### Key Entities *(include if feature involves data)*

- **Browse zone**: a bordered region of the browse screen. Three roles — *buckets* (left, lists buckets/pins), *objects* (middle, lists the highlighted bucket's current level), *details* (right, adaptive metadata/preview). Visibility is width-driven.
- **Focus state**: which zone currently owns keyboard input (buckets vs objects). Drives the active-zone indicator and the meaning of cross/back keys.
- **Zone cursor**: the per-zone selection index (one for buckets, one for the objects level), independent across zones.
- **Level listing**: the first-level (or deeper) entries of a bucket at a prefix — folders + objects — keyed by (context, bucket, prefix, search) and shared with the cache.
- **Keybinding map**: the single source mapping logical actions to keys, consumed by dispatch, the hint bar, and help; carries the migration record for changed bindings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In multi-pane mode, moving the bucket cursor (zero Enter/open actions) shows the highlighted bucket's first-level contents, and the contents shown always match the highlighted bucket (no stale/mismatched render, per FR-004).
- **SC-002**: From a state where a bucket is highlighted (Full or Dual tier), an operator opens a first-level object of that bucket in ≤ 2 keypresses (cross focus via `→`/`Tab`, then Enter) — no full-screen bucket-drill detour required.
- **SC-003**: During fast scrolling of the bucket list, N intermediate bucket selections produce ≤ 1 backend listing (only the settled selection is fetched); after the selection settles, its listing is requested within the FR-003 ceiling (≤ 200 ms). Input dispatch is never blocked on a listing.
- **SC-004**: Revisiting a bucket already viewed in the session shows its contents with no additional backend listing call (served from cache).
- **SC-005**: Every action key advertised in the hint bar and help (100%) renders its key glyph with a bold ANSI attribute, and every action has a one-line mnemonic in help; with `$NO_COLOR` set, each key remains identifiable next to its label via the bold attribute and/or a stable delimiter (no color dependency).
- **SC-006**: Exactly the bindings listed in the migration note change; the navigation set and locked global keys (arrows, `hjkl`/`gG`, Enter, Esc, `q`, `?`, `:`, Space) are unchanged in identity, verified against the single keymap source.
- **SC-007**: In the Single tier (≤ 99 cols) the browse experience is behaviourally identical to today's single-column flow, including Enter-on-a-bucket drilling full-screen (no regression).
- **SC-008**: Resizing across the Full/Dual/Single tier boundaries preserves the highlighted bucket, the objects cursor, and the focused zone, and never breaks a border or hides the footer (identity + hint line incl. help/quit stay visible).
- **SC-009**: The structural read-only guard (`make check-readonly`) stays green and no new backend listing capability is added (no new `storage.Storage` method).
- **SC-010**: Entering the browse screen with K buckets issues exactly one bucket-name listing and zero object-level listings; an object-level listing occurs only after a bucket selection settles (lazy), and a denied object-level listing is re-attempted (not cached) on the next revisit.

## Assumptions

- **Debounce + supersession reuse**: the objects-zone live listing reuses the existing non-blocking pattern (generation id + debounce, superseded loads dropped) already used by the details pane. The debounce ceiling is ≤ 200 ms (FR-003); the precise value (the existing pane uses ~180 ms) is confirmed in planning. Intermediate fast-scroll selections are not fetched.
- **Cache reuse**: the per-session level cache (keyed by context/bucket/prefix/search) is shared between the objects zone and the existing full-screen level view, so revisits are free.
- **Width tiers**: governed by the normative "Layout tiers" section (Full ≥ 130, Dual 100–129, Single ≤ 99). These boundaries are normative defaults that planning MAY re-tune, but the three-tier structure and collapse order (details first, then objects) are fixed. The current single/multi split point is the existing `paneSplitMin` (100); each zone keeps a minimum width set in planning.
- **Height budget**: with three bordered boxes plus the footer, panes shrink before the footer; a viable minimum row budget is preserved, otherwise the layout falls back to fewer zones.
- **Details pane fields**: the adaptive details zone reuses feature 006's existing metadata/preview field set; bucket-metadata may be extended later without changing this feature's contract.
- **Scoped connections**: pinned/scoped-connection behaviour (feature 010) is reused for both the bucket list and the objects-zone listing; a denied listing surfaces as an honest in-pane error.
- **Keymap is already mostly mnemonic**: the review changes one binding (`n`) and keeps two weak-but-familiar ones (`y`, `ctrl+o`) with documented rationale, rather than churning the whole scheme.
- **No backend/storage change**: the highlighted bucket's first level uses the existing first-level listing method; no new storage capability and no write capability are introduced (read-only guard stays green; no constitution amendment).
- **Focus starts on buckets**: when the browse screen opens, focus is on the bucket list (left zone).
