# Contract: Layout — list + persistent details pane (US2)

Replaces the full-width single box. `App.View()` (`internal/ui/app.go:616`)
composes the bordered body as a horizontal split on wide terminals.

## L1 — Split (FR-008/FR-013)

- Wide (`width >= 100`): body = `JoinHorizontal(list, pane)`. Pane width =
  `min(40, width/3)`; list gets the remainder (minus borders). `paneVisible=true`.
- Narrow (`width < 100`): pane stacks below the list OR collapses to a toggle;
  list spans full width. The hint bar, footer, and write badge always render and
  are never clipped (verified at 80×24).
- List renderers (`treeView`, `bucketsView`) receive the **reduced** list width;
  `windowBounds(n, sel, rows)` keeps windowing stateless (selection index is the
  only state).

## L2 — Pane content (FR-010/FR-011)

| Selection | Pane shows |
|-----------|-----------|
| object | size, content-type, last-modified, ETag, storage class + bounded inline preview (`panePrev`) |
| folder | folder summary line + hint "`a` analyze" |
| level/none | level counts + sort indicator; no object preview |
| loading | spinner + "loading…" until the debounced fetch returns |

## L3 — Debounced pane load (FR-009/FR-012)

- Selection move renders instantly-known list fields (name/size/modified) with **no
  backend call**.
- A `paneTick` (~150–250 ms) fires `loadMetadata`+`loadPreview` under `paneGen`
  ONLY if the selection key is unchanged when it fires.
- Moving the selection bumps `paneGen`; `paneMetaMsg`/`panePreviewMsg` whose `gen`
  ≠ current `paneGen` are dropped — the pane never shows data for a row already
  scrolled past.
- Pane messages MUST NOT set `m.mode = modeObject` (distinct from the Enter view).

**Tests**: split renders both columns ≥100 cols; stacks/toggles <100; pane shows
object metadata for an object row, summary for a folder; fast successive selection
moves cause at most one fetch for the settled row (drive the tick); a stale
`paneMetaMsg` is ignored; 80×24 shows hint bar + footer + badge uncliped.
