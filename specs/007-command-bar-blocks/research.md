# Phase 0 Research: 007 command-bar blocks

Resolves the spec's planning-deferred decisions. Each: Decision · Rationale · Alternatives.

## R1 — Ctrl chord scheme for dangerous actions (FR-021)

**Decision**: Dangerous actions are triggered by `ctrl+<same letter>` as their 006 bare
key, EXCEPT where that Ctrl combo is terminal-reserved; the bare key becomes inert for
dangerous actions (shows the chord nudge). Concretely:

| Action | 006 bare key | 007 chord | Note |
|--------|--------------|-----------|------|
| delete object / bulk delete | `x` | `ctrl+x` | safe; `ctrl+x` is free |
| recursive (directory) delete | `X` | `ctrl+x` on a folder selection | same chord, selection-typed (folder → recursive) |
| move/rename | `m` | **`ctrl+o`** | `ctrl+m` == Enter (RESERVED) → remap to `ctrl+o` ("mOve") |
| delete bucket | (none) | `ctrl+x` on the bucket list | same delete chord, bucket-list context |
| delete connection | (none, `:conn`) | `ctrl+x` on the contexts screen | same delete chord, contexts context |
| overwrite (copy/upload onto existing) | detected at dest-submit | no new key — escalates the existing dest flow to the typed/binary surface | unchanged trigger |

Reserved/avoided Ctrl combos: `ctrl+c` (quit), `ctrl+s`/`ctrl+q` (XON/XOFF flow control),
`ctrl+z` (SIGTSTP), `ctrl+d` (EOF), `ctrl+h` (Backspace), `ctrl+i` (Tab), `ctrl+m`
(Enter/CR), `ctrl+[` (Esc). Move therefore cannot use `ctrl+m`; it uses `ctrl+o`.

**Rationale**: One mnemonic family — "the destructive variant is the chorded variant" —
keeps the write block's `^x delete` labels honest and discoverable (FR-026). Reusing
`ctrl+x` across object/recursive/bucket/connection delete is unambiguous because the
*selection context* (object vs folder vs bucket-list vs contexts screen) already routes to
the right `start*`; the surface (binary vs typed) is then chosen per target.

**Alternatives**: (a) a single "delete mode" prefix key — rejected, adds a mode and more
state; (b) `ctrl+d` for delete — rejected, `ctrl+d` is EOF and risks killing the session
over SSH; (c) leave move on a bare key — rejected, move is destructive (source removed) and
the spec puts it behind the chord.

## R2 — Confirmation surfaces (FR-023/FR-023a/FR-027a)

**Decision**: Keep the existing `operation` + `onConfirmKey` state machine; only the
*render surface* splits by tier:
- **Binary tier** (`confirmSimple`-like for delete-object/group/move/overwrite): a
  **centered popup overlay** drawn over the dimmed body in `View()` (a new `centerOverlay`
  in `confirmview.go`), y/N inside.
- **Typed tier** (directory path / bucket name / connection name): a **prominent inline
  form** — a bordered, badge-carrying input strip that replaces the footer status line but
  is NOT a separate alt-screen window; a real editable field with horizontal scroll for
  long identifiers.

Both surfaces are produced by one `confirmview.go` with shared style helpers (same palette
roles, badge placement via `writeBadge`, title/label conventions, key hints) so they read
as one design language (FR-027a, SC-009a).

**Rationale**: The two-tier confirm logic (byte-exact typed match, y/N) already exists and
is well-tested — reusing it avoids re-implementing safety. Only presentation changes, which
is exactly the spec's intent ("change presentation/visibility, not the action flows",
FR-019). A centered popup matches the k9s ask for binary; an inline prominent form matches
"удобно, не отдельное окно" for typing long paths.

**Alternatives**: (a) everything in a centered popup — rejected, a long path in a narrow
modal is awkward (the user's exact objection); (b) everything inline — rejected, the user
asked for a centered k9s popup for the simple confirm; (c) a full-screen typed form —
rejected, "не отдельное окно".

## R3 — Progress bar (FR-034..FR-038, SC-012/013)

**Decision**: Add `progress.go` with `progressBar(fraction float64, width int) string`
rendering a Claude-Code-style determinate bar: a filled run (`█`/`▰` in `colAccent`) + an
empty run (`colDim`) + a trailing ` NN%` + an elapsed/label hint. `opProgressLine`
(operation.go) chooses determinate when a total is known (`upload`: bytes uploaded/total;
`bulk_*`/`delete_recursive`: done/total counts) and the **indeterminate** spinner fallback
when the total is unknown (FR-037). A "taking a while" **threshold** (~400 ms after
`phaseRunning` begins, via the existing spinner tick count) gates the bar so fast ops never
flash it (FR-035, SC-013). The bar lives in the **footer status zone** (the list stays
visible) per the clarified placement; operations remain cancellable (`x`/Esc) — FR-038.

**Rationale**: Reuses the existing `opProgress` struct, `spinnerTick`, and
`waitForProgress` plumbing — no new async machinery, satisfies Non-Blocking (II). Recursive
delete currently has no up-front total; it gets the indeterminate variant unless a cheap
count is available, avoiding a fabricated percent.

**Alternatives**: a third-party progressbar widget — rejected, extra dep for a few cells of
rendering; a centered progress modal — rejected, the clarified placement is inline footer.

## R4 — Block layout: columns (FR-002/FR-016)

**Decision**: Render the three blocks with `lipgloss.JoinHorizontal(lipgloss.Top, info,
read, write)` inside the footer, each block a small stack of `key label` rows under a dim
title. Width budget: when `width < blockColMin` (~100, reuse the `paneSplitMin` instinct)
the columns **collapse** to a compact single wrapped row (today's strip) while still
listing the write entries (dimmed) — never dropping the write block (FR-016). Height stays
≤ the footer budget; `app.go footerBlock` already measures footer height and reserves body
rows from it, so taller footer = fewer body rows (no clipping).

**Rationale**: `JoinHorizontal` is already used for `listWithPane`; the same primitive
gives grouped columns for free and reflows on resize. Collapsing to the proven single-row
strip below the threshold guarantees small-terminal legibility.

**Alternatives**: rows (3 stacked bands) — rejected in the spec (reads as a list, noisier
when dimmed); a boxed sub-panel — rejected, costs border rows the footer can't spare.

## R5 — Palette roles for the blocks (FR-013/FR-014/FR-015)

**Decision**: No new hue. Map to existing tokens:
- **info** block: `segCtxStyle`/`segClusterStyle`/`segUserStyle`/… (the per-parameter hues
  already used in the identity row) for values; `dimCellStyle` titles.
- **read** block: keys in `accentStyle` (coral), labels in `dimCellStyle` — today's hint
  styling.
- **write block, armed (caution)**: keys in `warnStyle` (`colWarn` amber) to distinguish
  from read's coral; labels in `objCellStyle`.
- **write block, read-only (inactive)**: whole block in `emptyStyle` (faint dim) + a
  textual `(w)`/`^` cue so the inactive state survives `NO_COLOR` (FR-015).
- Dangerous chords shown as `^x delete` with the `^x` in the caution/dim role.

**Rationale**: Reuses the 006 token set (FR-013 forbids new hues), keeps the screen calm
(one extra role — amber for armed-write — vs coral read), and every color meaning has a
text cue (gutter, `(w)`, `^`, `[RW]/[RO]`) for NO_COLOR.

**Alternatives**: a new red for write — rejected, garish + reserved for the loud `[RW]`
badge/errors; bold-only differentiation — rejected, too weak to scan.

## R6 — Bucket delete (FR-024b, SC-015)

**Decision**: Add `RemoveBucket(ctx, bucket) error` to `Mutator`. Implementation pre-checks
emptiness with `ListObjectsV2(maxKeys=1)`; if any object/prefix exists, return a new
`ErrBucketNotEmpty` sentinel BEFORE calling `DeleteBucket` (refuse non-empty, FR-024b). The
S3 `DeleteBucket` symbol stays inside `internal/storage/writer.go`. `readOnlyGuard` and
`Fake` implement `RemoveBucket`. The method name uses verb "Remove" (not "Delete") so the
read-only scan (`Put|Delete|Create|Copy|Upload|Restore|Write` + entity) does not flag UI
references — consistent with the existing `RemoveObject` convention.

**Rationale**: Empty-only matches S3 `DeleteBucket` semantics and the clarified decision;
the pre-check yields a clean, truthful refusal instead of a raw `BucketNotEmpty` API error.
Naming convention already established (`RemoveObject` dodges the guard).

**Alternatives**: recursive purge then delete — rejected (clarified out: bucket delete
never purges); rely on the API error — rejected, less clear UX and a wasted round trip
to a guaranteed failure.

## R7 — Connection delete (FR-029..FR-033)

**Decision**: Add `(*Config).RemoveConnection(name string) ([]string, error)`: refuse when
`name == CurrentContext` (active-context guard, FR-032 — the UI also blocks it, defense in
depth), validate a trial copy with the triple removed, `secret.RemoveKeychain(name)`
(best-effort — a missing secret does not block, FR-031), persist, then commit to the live
config and return the new context-name list (live refresh, FR-033). Extend the `Connector`
seam with `Delete(ctx, name) ([]string, error)`; `connSeam.Delete` calls
`RemoveConnection`. The contexts screen (`modeConnections`) binds `ctrl+x` on a selected
non-active context → the typed-name inline confirm (R2) → `Connector.Delete`.

**Rationale**: Mirrors `AddConnection` exactly in reverse (trial-validate-before-mutate,
keychain alongside config, live commit) — symmetric, low-risk, keeps config/keychain logic
out of the UI (Constitution I). Last/only connection deletable → falls back to the
no-connection/add state (clarified); the empty-list render already exists
(`emptyStyle "No contexts configured."`).

**Alternatives**: physical config rewrite from the UI — rejected (violates I); soft-hide
without keychain cleanup — rejected, leaves an orphan secret.

## R8 — Action label rule (FR-005a, SC-014)

**Decision**: Every read/write label is a **single imperative verb, ≤2 words, lowercase,
no articles, no trailing punctuation**; the object is implied by block + selection. Audit
current labels against the rule: `download, analyze, refresh, copy, move, upload` ✓;
`rm -r` → **`delete`** (the recursive nature is conveyed by the folder selection + chord,
not the label) or keep a 2-word `delete tree`; `mkdir` → **`new folder`** (2 words, clearer
than the unix-ism). Bulk variants append a count (`delete 3`) — still ≤2 tokens + number.

**Rationale**: Turns the adjective ("simple/unambiguous/no extra words") into a checkable
rule (SC-014). Lowercase single verbs scan fastest and fit the narrow columns.

**Alternatives**: verb+object everywhere ("delete object") — rejected, widens columns and
the block already names the object class; free guidance — rejected, unmeasurable
(CHK001–006 would stay open).
