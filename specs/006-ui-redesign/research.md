# Phase 0 Research: UI Redesign

All spec clarifications were resolved in `/speckit-clarify` (see spec
`## Clarifications`). No `NEEDS CLARIFICATION` markers remain. This document
records the design decisions that turn the resolved spec into an implementable
plan, grounded in the existing codebase.

## R1 — TUI patterns adopted (which tools, what we take)

**Decision**: Adopt k9s as the structural base (full-width table + `:` command bar
+ direct single-key actions + `?` help + breadcrumb header), and borrow the
**persistent preview/details pane** from terminal file managers (ranger / lf /
yazi). Keep the existing `ncdu`-style `du` view.

**Rationale**: The user explicitly likes k9s and dislikes the modal action menu.
k9s's defining trait is that actions are single keys on the highlighted row with a
discoverable command bar — exactly the "no extra buttons before an action" ask.
The file-manager pane addresses "object area too big": the freed width shows
per-selection detail continuously.

**Alternatives considered**: Miller-columns (ranger/yazi 3-pane) — rejected by the
user in clarify (further from k9s, larger rewrite). Pure single-table k9s with no
pane — rejected: doesn't reclaim the over-large list area the user complained
about.

## R2 — Details pane load strategy (FR-009)

**Decision**: Debounced lazy load. On selection move, render instantly-known list
fields (name, size, last-modified) immediately; schedule a `paneTick` (~150–250 ms)
that, if the selection is unchanged when it fires, dispatches `HeadObject` +
ranged `GetObject` under a dedicated **pane generation**. A new selection bumps the
pane generation, so in-flight pane loads for scrolled-past rows are dropped.

**Rationale**: Clarify answer B. Mirrors the existing `debounceSearch` /
`searchFireMsg` pattern (`commands.go:270`) and the generation-drop mechanism
(`m.gen`), so it is idiomatic and already proven in this codebase.

**Implementation notes**: Reuse `loadMetadata` / `loadPreview` (`commands.go:63/76`)
but emit **new** messages (`paneMetaMsg` / `panePreviewMsg`) that do **not** set
`m.mode = modeObject` (the existing `metadataMsg`/`previewMsg` handlers force
`modeObject` at `app.go:282/292`). Add `m.paneGen`, `m.paneMeta`, `m.panePrev`,
`m.paneSelKey` fields. The full-screen `modeObject` (Enter) is retained unchanged
(FR-015).

**Alternatives considered**: Eager per-row (A) — network storm on fast scroll.
On-demand key (C) — loses the "see it as you move" value. Hybrid was effectively
folded into B (instant list fields + debounced fetch).

## R3 — Menu-less direct actions + the hint bar (US1, FR-001..FR-007)

**Decision**: Delete `modeActionMenu` and the `Menu` (`a`) binding. Reuse the
*selection/capability gating logic* from `menuItemsFor()` (`actionmenu.go:22`) to
build two things instead: (1) a `hintbar` catalog of `{key, label, available}` for
the current view/selection/writability, rendered always-visible above the footer;
(2) the dispatch table for the direct keys. Each action key calls the **same**
`start*` entry point the menu used (`startDownload`, `startRemoveObject`,
`startCopy`, `startMove`, `startRecursiveDelete`, `startUpload`,
`startCreateFolder`, `startAnalyze`, `refresh`), so confirmations and operation
flows are unchanged (FR-005).

**Rationale**: Reuses tested logic; the menu was only an indirection over these
entry points. Removing it is subtractive plus a rebind.

**Key rebinding** (resolved here; the spec left specific keys to planning):

| Action | Old | New | Note |
|--------|-----|-----|------|
| Analyze (`du`) | menu-only | `a` | reuse the freed menu key; mnemonic "analyze" |
| Download | menu-only | `d` | mnemonic "download"; read — available read-only |
| Delete object | `d` | `x` | k9s-style delete; frees `d` for download |
| Recursive delete | `D` | `X` | matches `x` family |
| Copy | `y` | `y` | unchanged |
| Move/rename | `m` | `m` | unchanged |
| Upload here | `u` | `u` | unchanged |
| New folder | `+` | `+` | unchanged |
| Refresh | menu-only | `r` | bind directly (was menu-only after 004) |
| Mark (multi-select) | `space` | `space` | unchanged |
| Sort / dir | `s` / `S` | `s` / `S` | unchanged |
| Write toggle | `w` | `w` | unchanged |
| Context switch | `c`, `1`–`9` | `c`, `1`–`9` | unchanged |
| Command bar | — | `:` | new (R5) |
| Connections | — | via context view `+` and `:conn` | new (R6) |

`x`/`X` for delete is a deliberate safety choice: the most destructive keys are
**not** on the same letter as the most common read action (download), avoiding
muscle-memory mistakes. Bulk variants: when `selCount()>0`, `d`/`x`/`y` act on the
marked set (FR-006) — same rule the menu used.

**Migration (FR-007)**: keep `a` mapped (now to analyze) — pressing it does
something sensible; a former-`a`-as-menu user gets analyze, not a dead key. The old
`Menu` action is removed; if a user presses an unbound legacy key, the hint bar is
the always-visible source of truth.

## R4 — Layout split: list + persistent pane (US2, FR-008..FR-015)

**Decision**: In `View()` (`app.go:616`), split the bordered body horizontally on
wide terminals into a **list column** (left) and a **details/preview pane** (right,
~36–40 cols or ~⅓ width, whichever smaller). Below a width threshold (e.g.
`< 100` cols) stack the pane under the list or collapse it to a toggle; the hint
bar and footer always render. The list renderer (`treeView`, `bucketsView`)
already takes an explicit width — pass the reduced list width; `windowBounds`
keeps windowing stateless.

**Rationale**: Minimal structural change — the existing `boxView` + width-parametric
table renderers compose into two columns with Lipgloss `JoinHorizontal`. Selection
indices remain the only state (resize stays trivial, per the architecture note in
CLAUDE.md).

**Pane content** (FR-010/FR-011): object selection → metadata lines (size, content
type, last-modified, ETag, storage class) + bounded preview from `paneMeta`/
`panePrev`; folder/level selection → level summary (counts; offer `a` to analyze).
The write badge (`writeBadge`, FR-027/FR-014) and the multi-line footer stay.

## R5 — Command bar `:` (US3, FR-016..FR-019)

**Decision**: New `modeCommand`. `:` opens a one-line input (reuse the search-input
rendering path in `statusLine`). A static command **registry** maps names →
handlers: `buckets`, `contexts`/`ctx`, `conn`/`connect` (connection manager),
`analyze`/`du`, `refresh`, `help`, `quit`/`q`. Typing shows matching command names
(prefix filter); Enter on a unique/selected match dispatches; Esc closes with no
effect; unknown → a non-destructive `notice` (FR-018). `:` and `/` are distinct
modes and never coexist (FR-019); operation prompts own keys first (existing
precedence in `onKey`).

**Rationale**: k9s parity; reuses the search input and `notice` plumbing.

## R6 — In-app connection manager + writer (US4, FR-020..FR-027, FR-022a, FR-024, FR-025a)

**Decision**:
- **UI**: a connection **list** mode (reachable from the context switcher via `+`
  "add connection" and from `:conn`) and an **add-form** mode with fields: display
  name, endpoint, region, access key id, secret access key (no-echo), read-only
  flag. Field-level validation before save (required name/endpoint; endpoint must
  parse as an absolute URL — reuse the rule in `config.Validate`).
- **Seam**: `cmd/s3s/main.go` injects a new `Connector` closure into `ui.New`
  (alongside `Resolver`), nil-able to disable the feature. Signature (UI-agnostic
  types only):

  ```go
  type ConnDraft struct {
      Name, Endpoint, Region, AccessKeyID string
      Secret   logging.Secret
      ReadOnly bool
  }
  type Connector interface {
      Test(ctx context.Context, d ConnDraft) error          // reachability (ListBuckets)
      Save(ctx context.Context, d ConnDraft) ([]string, error) // persist; returns new context-name list
  }
  ```

- **Save mapping (FR-022a)**: derive a `Cluster{Name, Endpoint, Region}`, a
  `User{Name, AccessKeyID, Keychain:true}`, and a `Context{Name, Cluster, User,
  ReadOnly}` from one draft (names derived from the display name; collisions
  rejected, FR-024). Then `secret.StoreKeychain(KeychainAccount, secret)`
  (`keychain.go:36`), then `config.Upsert(cl,u,cx,false)` (`generate.go:51`) +
  `config.Save(path, Marshal(cfg))` (`generate.go:24/91`). The on-disk schema is
  unchanged; the `User` carries `keychain: true` (no plaintext secret), satisfying
  the exactly-one-source rule (`config.Validate`, FR-041 from 005) and FR-022/
  FR-005.
- **Order & atomicity (FR-026/FR-025a)**: `Test` first (off-loop; on failure the
  form offers "save anyway"). On save, **store the secret in the keychain before**
  writing config; if the config write fails, the orphaned keychain entry is
  harmless and overwritten on retry; if the keychain write fails, **abort before**
  touching config so no context points at a missing secret. Report a clear,
  secret-free error either way; never claim success on failure.
- **Session refresh (FR-025)**: `Save` returns the updated context-name slice; the
  UI replaces `m.contexts`, and because `main.go`'s `resolve` closure captures the
  same `*config.Config` (mutated by `Upsert`), switching to the new context
  resolves immediately without restart.

**Rationale**: Every backend/keychain/config primitive already exists and is
UI-agnostic; the feature is wiring + a form. The `Connector` interface keeps S3 and
config-marshalling out of `internal/ui` (Constitution I).

**Alternatives considered**: Let the UI hold `*config.Config` directly — rejected
(leaks config logic into UI, violates I). Reuse/create cluster+user pickers
(clarify option B) — rejected (more form); auto-derive triple (option A) chosen.
Flattened self-contained context (option C) — rejected (new schema variant).

## R7 — Testing approach

**Decision**: White-box `package ui` tests using the existing `deliver`/`press`
helpers and `App.View().Content` assertions for: hint-bar contents by
selection/capability; each direct key routing into the right `start*`; `a` no
longer opens a menu; the pane updating on selection + debounce/supersede behavior
(drive the tick); `:` command parse/dispatch/unknown; connection-form validation,
duplicate rejection, save-anyway path. `config`/`secret` get unit tests for the
`AddConnection`/`Upsert` mapping and keychain store (mock keyring). An
`//go:build integration` test exercises add→test→switch against MinIO.

**Rationale**: Matches the constitution's Test-First + Integration principles and
the project's established UI test style (CLAUDE.md "Testing conventions").

## R9 — Visual design: palette reuse, emphasis, restraint (FR-031..FR-046)

**Decision**: Treat the existing `internal/ui/styles.go` 256-color token set as the
authoritative palette and codify it (spec FR-031 table). New surfaces (pane, hint
bar, command bar, connection form) map onto existing roles — no new hues. The design
language is already restrained (single coral accent on muted grays; the `[RW]` badge
is the only loud element; the footer uses a bounded 3-hue param set; `maxHints=6`
caps advertised actions). The redesign **preserves and bounds** this, rather than
adding color.

**Rationale**: Closes the `ux.md` checklist gaps (CHK001–CHK046) and resolves the
tension between "color-code important elements" and "stay calm, not gaudy": color is
bounded (≤4 accent/param hues per screen, FR-037), the baseline is neutral
(FR-038), emphasis is a defined finite set with redundant non-color cues (FR-033/
FR-034), and exactly one loud element is allowed (the badge, FR-035). All of these
are objectively countable → measurable Success Criteria SC-009..SC-013.

**Implementation notes**: `styles.go` gains pane/hint-bar/command-bar/form styles
that REUSE existing tokens (`colAccent`, `colDim`, `selRowStyle`, etc.). No new
`lipgloss.Color(...)` literals outside the FR-031 table. Honor `NO_COLOR` and rely
on glyph/weight cues (`▶`, `✓`, bold, `error:`/`[RW]` text) so meaning survives
without color.

## R8 — Things explicitly out of scope (kept simple)

- Editing or deleting an **existing** connection from the UI (only *add*); `s3s
  cred` remains for rotation. (Spec Assumptions.)
- Non-keychain credential sources in the add-form (cmd/awsProfile/inline) — the
  form stores to keychain only; power users still hand-edit or use existing
  sources. (Clarify Q3 secrets = keychain.)
- Removing the full-screen `modeObject` view — retained as the richer preview
  (FR-015).
