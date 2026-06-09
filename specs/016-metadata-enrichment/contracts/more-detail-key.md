# Contract: Context-Aware "More Detail" Key (FR-019, FR-008)

**Surface**: keymap field rename `Analyze` → `MoreDetail` (`keys.go:21, 54`, binding stays
`"a"`); help row; `:` command (`command.go:33`); hint catalog (`hintbar.go:52, 70`); pane
hint labels (`pane.go:54, 67, 71`); new `startMoreDetail` dispatcher.

## Behavior (one key, context-aware — and one shared invoke target)

| Focus | `a` (MoreDetail) / `:detail` does |
|---|---|
| bucket list (zoneBuckets) | toggle `detailSection` breakdown (`sectBreakdown`) + lazily load `GetBucketConfiguration` (`sectConfig` on a second concern) |
| tree prefix/folder (selFolder) | toggle breakdown + (for the bucket) config |
| tree level (selNone) | toggle the level's breakdown |
| object (selObject) | load `GetObjectTagging` (`sectTags`) + render governance detail |

The `:detail`/`:info` command-bar entry (`command.go:33`) sets `invoke: App.startMoreDetail`
— the SAME function the `a` key calls — so the command bar and the key share ONE target and
cannot diverge (FR-019 "one mental model").

## Invariants

- I1 (FR-008): the former `analyze` destination is gone; pressing `a` opens NO full-screen
  view; `modeUsage` removed from `canOpenCommand` (`command.go:57`); `:analyze`/`:du` no
  longer resolve (replaced by `:detail`/`:info`).
- I2 (FR-019): exactly ONE key drives all on-demand affordances — no new keybinding added
  (clarification `spec.md:69-74`); at most ONE detail section is open at a time
  (`detailSection`, budget gate).
- I3 (VII): the rename propagates to every hint automatically via `keyHint`/`firstBind`
  (`keys.go:101-113`) ONLY after `hintbar.go:52/70` AND `pane.go:54/67/71` are migrated to
  `m.keys.MoreDetail` with the `detail` label; no hardcoded `"a"` literal in hints; no new
  palette role.
- I4 (II/FR-016): tag/config loads run in a `tea.Cmd` carrying `detailGen`; a result whose
  `detailKey`/`detailGen` no longer matches the focus is dropped (wiring mirrors
  `onPaneTick`, `app.go:344-357`).

## Testable assertions

- `m.keys.MoreDetail` exists (not `m.keys.Analyze`); `firstBind == "a"`.
- help shows `a detail` (not `a analyze`); the command registry has `detail`/`info` (not
  `analyze`/`du`) and both invoke `startMoreDetail`.
- `pane.go` hints render `a detail` (the migration of `pane.go:54/67/71` is asserted, since
  forgetting it is a compile error first, then a stale-label failure).
- Hint shows on bucket/prefix focus (`avail` true) and dispatches tags on object focus.
