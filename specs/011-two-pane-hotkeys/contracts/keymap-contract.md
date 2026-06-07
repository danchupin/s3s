# Contract: Reviewed keymap (mnemonics, bold glyphs, focus keys)

Covers FR-016, FR-017, FR-018, FR-019, FR-020, FR-021, FR-022, FR-023, FR-024, FR-025.

The keymap is the single source of truth: `defaultKeys()` (`internal/ui/keys.go:42`). Dispatch
(`onKey`/`onBucketsKey`/`onTreeKey`, `dispatchActionKey`/`dispatchChord` in `commands.go`), the
list-mode command bar (`commandbar.go`), and help (`helpLines` in `keys.go:124`) all derive their
keys from this one struct. A rebind there propagates to dispatch, the bar, and help together — they
can never drift (FR-024, the single-source invariant).

## Action → key(s) → mode availability (the reviewed map)

`A` = always (any list/tree mode + most overlays). `B` = bucket list (`modeBuckets`). `T` = tree
(`modeTree`). `O` = object view (`modeObject`). `W` = write-only (dropped from dispatch/bar when
`!writable()`). Keys are the live values from `defaultKeys()`.

| Action          | Key(s)                 | Source field   | Availability | Notes |
|-----------------|------------------------|----------------|--------------|-------|
| Up / Down       | `↑`/`k`, `↓`/`j`        | `Up`,`Down`    | A (per-zone) | moves the FOCUSED zone's cursor (focus-aware below) |
| Top / Bottom    | `g`/`Home`, `G`/`End`  | `Top`,`Bottom` | A (per-zone) | |
| Enter / cross   | `Enter`,`→`,`l`         | `Enter`        | A            | focus-aware: open / cross buckets→objects (below) |
| Back / return   | `Esc`,`←`,`h`           | `Back`         | A            | focus-aware: ascend / return objects→buckets (below) |
| Search / filter | `/`                    | `Search`       | B,T          | bucket-name filter (B) / level prefix search (T) |
| Mark            | `Space`                | `Mark`         | T            | multi-select toggle |
| Sort / SortDir  | `s`, `S`               | `Sort`,`SortDir`| T           | |
| Download        | `d`                    | `Download`     | B,T (object/marks) | read; works read-only |
| Analyze (du)    | `a`                    | `Analyze`      | B,T (non-object) | read |
| Refresh         | `r`                    | `Refresh`      | A            | invalidates the level cache (see lazy-load-cache.md) |
| Copy (yank)     | `y`                    | `Copy`         | T `W`        | KEPT (FR-022) |
| Move chord      | `Ctrl+O`               | `MoveChord`    | T `W`        | KEPT (FR-022); `Ctrl+M` is Enter so the chord is `Ctrl+O` |
| New folder      | `+`                    | `NewFolder`    | T `W`        | |
| Upload          | `u`                    | `Upload`       | T `W`        | |
| Delete          | `x` (bare = inert)     | `Delete`       | T `W`        | fires only via `Ctrl+X` chord |
| Delete recursive| `X` (bare = inert)     | `DeleteAll`    | T `W`        | fires only via `Ctrl+X` chord |
| Delete chord    | `Ctrl+X`               | `DeleteChord`  | B,T,conns `W`| dangerous: object/group/recursive/bucket/connection |
| Write toggle    | `w`                    | `WriteToggle`  | A            | arm prompts y/N; disarm is instant |
| Context switch  | `c`, `1`–`9`           | `Context`      | B,T          | |
| Command bar     | `:`                    | `Command`      | B,T          | |
| Help            | `?`                    | `Help`         | A            | |
| Quit            | `q`, `Ctrl+C`          | `Quit`         | A            | `Ctrl+C` is the universal escape even inside modal text input |

`AddConn` (`n`) is RETAINED in `defaultKeys()` as the field that drives the row-only affordance
glyph, but it is no longer a global keypress action — see the single change below.

## The single change: AddConn `n` removed as a keypress, row-only affordance kept (FR-016/FR-017)

- TODAY `app.go:604` dispatches `matches(key, m.keys.AddConn)` in `modeBuckets`/`modeTree`, so a bare
  `n` opens the connection manager. This is the mnemonic collision the review removes: `n` reads as
  "next/new-object" and is the only advertised single letter that leaves the browse view unexpectedly.
- AFTER 011: the `n`-keypress branch at `app.go:604` is DELETED. Pressing `n` in the bucket list or
  tree is inert (falls through to `dispatchActionKey`, which has no `n` binding → no-op).
- The connection manager stays reachable by the EXISTING row-only affordances, unchanged:
  - the `+ add connection` row in the connection list (`connections.go:103`, opened via the bar's
    "connections" entry), and
  - the command bar's `connections` entry (`commandbar.go:191` / `:244`), which still renders
    `glyph(m.keys.AddConn[0])` = `n` as its label key. The bar entry remains the discoverable way in.
- `m.keys.AddConn` therefore stays defined (single-source) so the bar label and `helpLines`
  ("add a new connection", `keys.go:173`) keep advertising the same glyph; only the GLOBAL keypress
  dispatch is dropped.

Help text update (FR-017): the help "Context" section row for `AddConn` is rephrased from a global
hotkey to "open the connection manager (via the bar / `+ add connection` row)" so help never advertises
a key that no longer dispatches.

## Bold-glyph rendering rule (FR-018/FR-019)

Every advertised key glyph is rendered BOLD. Today the advertised keys carry the accent FOREGROUND
only and are NOT bold:
- hint bar: `accentStyle.Render(h.key)` (`styles.go:337`);
- command bar read entries: `roleStyle[roleRead] = accentStyle` (`commandbar.go:39`), plus the
  `connections`/`help`/`quit` globals at `commandbar.go:62`/`:191`/`:244`;
- help rows: `accentStyle.Render(pad(keys, 17))` (`keys.go:128`).

`accentStyle = lipgloss.NewStyle().Foreground(colAccent)` (`styles.go:47`) gains `.Bold(true)` at the
key-glyph render sites (a dedicated bold accent style, e.g. `keyStyle`, applied to the KEY token only —
NOT the label, so labels stay calm dim text). Invariant: any string that advertises a dispatchable key
to the user is rendered with a bold-weight style; labels/descriptions remain non-bold.

## NO_COLOR non-color cue (FR-020)

Under `NO_COLOR` lipgloss strips foreground/background color but KEEPS the SGR bold attribute. The bold
weight is therefore the redundant, color-independent cue that distinguishes a key glyph from its label
when color is gone — consistent with the existing NO_COLOR posture in `styles.go:70` (the `▶` gutter,
`✓` mark, `[RW]`/`[RO]` text, `error:`/`loading…` prefixes). Invariant: with `NO_COLOR=1` set, key
glyphs remain visually distinguished from labels via bold alone; no meaning is carried by color only.

## Locked keys (kept verbatim, FR-022)

`y` (`Copy`) and `Ctrl+O` (`MoveChord`) are LOCKED — unchanged in binding, mode, and write-gate. The
review explicitly keeps them; only `n` changes. `Ctrl+X` (`DeleteChord`), `Ctrl+C`/`q` (`Quit`), and
`?` (`Help`) are likewise unchanged.

## Focus keys and their focus-aware meaning (FR-009, three-zone browse)

In the three-zone master-detail layout (see two-pane-layout.md) browse focus is per-zone: either the
buckets zone or the objects zone owns the cursor. `Up`/`Down`/`Top`/`Bottom` move the FOCUSED zone's
cursor only. The directional/cross keys are focus-aware:

| Key(s)         | Buckets focused                         | Objects focused                                  |
|----------------|-----------------------------------------|--------------------------------------------------|
| `Tab`          | focus → objects (if a bucket is selected) | focus → buckets (symmetric toggle)             |
| `→`/`l`/`Enter`| move focus INTO the objects zone (the level is **already lazy-loaded** on bucket selection — no load on cross) | open: dir → descend (loads the sub-level), object → object view |
| `←`/`h`/`Esc`  | (search active) clear filter; else top of bucket list / no-op | ascend a prefix, or RETURN focus to buckets at the level root |

- `Tab` is a NEW binding, a symmetric focus toggle between buckets and objects; it never lists or
  fetches by itself (focus move only).
- **Crossing never loads.** The highlighted bucket's level is loaded *lazily on bucket selection*
  (the bucket-scroll debounce → `m.level`, see lazy-load-cache.md); so both `Tab` and `→`/`l`/`Enter`-on-a-bucket
  only set `focusZone = zoneObjects` — the level is already present (or in-flight). Neither issues a
  `ListLevel` by the act of crossing (FR-007/FR-008).
- `→`/`l`/`Enter` reuse the existing `Enter` action field; the focus-aware behavior is resolved at
  dispatch by which zone is focused. A load happens only when **descending a folder** while focus is
  already in the objects zone (existing `enterLevel`, `tree.go:116`, operating on `m.level`).
- `←`/`h`/`Esc` reuse the existing `Back` field and existing `goBack` semantics (`tree.go:153`):
  clear search → ascend prefix → return to the bucket zone at the level root. In the three-zone
  layout "return to bucket list" means "return focus to the buckets zone", not a screen swap.
- The active zone shows the accent (bold) border/title; the inactive zone is dim — see
  two-pane-layout.md for the rendering contract.

## Test assertions (white-box `package ui`; `deliver`/`press`/`viewOf`)

1. **`n` no longer opens connections.** Bucket list active, `connect != nil`:
   `press(m, "n")` ⇒ `m.mode == modeBuckets` (still), connection manager NOT opened. Same in
   `modeTree`: `press(m, "n")` ⇒ `m.mode == modeTree`, no mode change.
2. **Row-only path still works.** The command bar still advertises `n connections`
   (`viewOf(m)` contains "connections"); selecting the `+ add connection` row + `Enter` opens the
   form (`connections.go:124`), unchanged.
3. **Bold glyph.** The styled `viewOf(m)` for a list mode renders advertised key glyphs (e.g. `/`,
   `d`, `a`, `?`, `q`) with the bold SGR attribute on the KEY token; the matching label token is not
   bold. (Assert via the bold-style helper / ANSI bold code presence on the key segment.)
4. **NO_COLOR cue survives.** With color stripped (NO_COLOR render path), the key glyph segment still
   carries bold while the label does not — the key stays distinguishable without color.
5. **`y` / `Ctrl+O` locked.** In `modeTree` on an object selection, write armed: `press(m, "y")`
   starts copy; `press(m, "ctrl+o")` starts move — unchanged from pre-011 behavior.
6. **Tab toggles focus, no fetch.** Buckets focused, a bucket selected (its level already loaded by
   the bucket-scroll debounce), Fake call counters zeroed: `press(m, "tab")` ⇒ objects zone focused;
   `ListLevel` count unchanged — the level load is driven by *bucket selection*, never by `Tab` or by
   crossing (see lazy-load-cache.md assertion set).
7. **Cross vs return.** Buckets focused, level already loaded, counters zeroed: `press(m, "l")` moves
   focus into objects with **no** new `ListLevel` (`focusZone == zoneObjects`). Objects focused at the
   level root: `press(m, "h")` returns focus to buckets (no screen swap in the three-zone tier). A
   `ListLevel` fires only on a folder descend inside objects, not on the cross.
8. **Single-source.** Every key advertised by `helpLines()` and by `commandBarView` resolves through
   a `defaultKeys()` field (no literal key strings in the bar/help except the synthetic `1-9`); a
   table-driven test rebinds a field on a copy of `keyMap` and asserts the bar label, the help row,
   AND `dispatchActionKey` all follow the new binding.
