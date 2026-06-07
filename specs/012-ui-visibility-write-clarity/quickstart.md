# Quickstart: verifying 012 (UI legibility, hotkey parity, breadcrumbs, write-mode)

Manual walkthrough mapped to the acceptance scenarios. Build: `make build` → `./bin/s3s`. Use a wide
terminal (≥130 cols) for the multi-pane tiers unless a step says otherwise.

## Setup

1. Point at a context with a bucket whose name is long (longer than ~24 chars) and a level with many
   objects, ideally one with a long object key. (MinIO/RGW or a seeded test context.)
2. `make test` is green before you start; run `make fmt vet lint check-readonly` after changes.

## US1 — names fully visible (FR-001/003/004)

- Open the browse. The buckets column grows to fit the long bucket name (objects zone gives up slack) —
  **no `…` truncation** while there is room. (FR-001)
- Highlight a long object key in the objects zone: the active row wraps in place to show the full key
  without pushing the footer off-screen. (FR-003)
- Press `i` (Reveal) on it: a centered popup shows the complete key and copies it to the clipboard
  (paste elsewhere to confirm; on an unsupported terminal the value is still fully shown). Any key closes.
  (FR-004) Works on bucket names and the breadcrumb path too.

## US2 — write state legible (FR-006/007/008/009/038)

- Look at the footer identity: only `[RO]`/`[RW]` is colored — the space before it is neutral. (FR-006)
- Look at the box top border: a `RO`/`WRITE` chip is inset at the right (like an editor mode label),
  neutral read-only, accent when armed. (FR-038)
- In the command bar the write affordance reads "`w` enable write" when disarmed and "`w` → read-only"
  when armed — symmetric. (FR-007/008/009)
- `NO_COLOR=1 ./bin/s3s` — badge, chip and labels remain distinguishable by text.

## US4 — prominent arm confirmation (FR-014..017)

- Press `w` in a writable context: a **centered popup** asks to arm write mode (consequence + y/N, cancel
  default) — not a faint bottom line. The badge and border chip stay visible. (FR-014/017)
- Press `y` → armed (chip/badge flip). Press `w` again → disarms **instantly**, no popup. (FR-016)

## US3 — breadcrumb (FR-010..013)

- Drill several prefixes deep: the objects-zone center label shows `ctx → bucket/a/b/c`. (FR-010/011)
- Ascend / switch bucket / filter: it updates each time. (FR-012)
- Narrow the terminal until the path is too long: it elides the middle (`ctx → bucket/…/c`), keeping bucket
  + deepest segment; `i` reveals the full path. (FR-013)

## US6 — objects-zone hotkey parity (FR-026..028) — regression

- Tab into the objects zone. Verify each works exactly as in the full-screen level view:
  mark (space/`m`) + the marked count appears; sort `s` and direction `S` reorder; context `c` opens the
  switcher; per-item actions `d`/`a`/`y`/`u`/`+`/`r` and the delete chord `^x` run their normal flow,
  write-gated identically. No key is a silent dead key. (FR-026/027/028)
- Mark objects, switch bucket, Tab back: marks are cleared (level-scoped).

## US7 — filter the current level via the input (FR-029/039/040/041)

- Tab into the objects zone, press `/`: a prominent input opens (labeled for the objects pane); typing
  previews the narrowed level live; the bucket list is unaffected. (FR-029/039/040)
- Press Enter: input closes, focus is in the objects zone, an indicator shows `filter: <term>` + clear.
  (FR-040)
- Press `/` again: it re-opens pre-filled to refine. (FR-041)
- Press Esc while typing: reverts to the last committed state. Back/clear on the committed filter restores
  the full level. (FR-041/030)
- On the bucket list, `/` still filters buckets instantly (no backend call).

## US8 — sort surfaced (FR-031/032)

- The command bar shows the sort affordance + current field/direction (`s name↑`). Cycle to `modified` and
  verify the level reorders and the indicator updates — including in the objects zone.

## US5/US9 — consistency & declutter (FR-018..020/033..037)

- Open the details pane on a folder: the hint shows the bound delete glyph (not a literal `^x`).
- Rebind an action key (keybindings) and confirm every surface (command bar, help, details pane, confirm
  dialog, status) shows the new key.
- Confirm there are no duplicated on-screen hints; every action is still advertised at least once.

## Gate

`go test ./...` green · `make fmt vet lint check-readonly` clean · footer/command bar visible at 60×10,
120×8, 140×12 · `NO_COLOR=1` run legible.
