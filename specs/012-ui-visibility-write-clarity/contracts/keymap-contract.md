# Contract: Keymap single-source & hint rendering

**Principle**: VII (UI Consistency / Design System). FR-019, FR-033..037. Source: `keys.go`.

## Bindings

- `keyMap` gains `Reveal` (default `["i"]`) and `Tab` (default `["tab"]`).
- `Search` remains the single filter/search binding (mode/zone-aware label "filter"/"search"); NO separate
  Filter field (the filter reuses Search — confirmed in research).
- All other fields unchanged.

## Single-source guarantees (testable)

1. Every on-screen key hint MUST render via `glyph(m.keys.X[0])` or `formatKeys(m.keys.X)` — **no string
   literal** key (`"^x"`, `"d/x/y"`, `"Enter"`, `"Esc"`, `"↑/↓"`, `"tab"`).
2. A rebind of any action key MUST update every surface: command bar, help overlay, details pane,
   confirmation dialogs, status/operation/connection/file-browser hints. (Test: rebind → assert old glyph
   absent and new glyph present across `View().Content`, `helpView()`, `commandBarView()`, `paneView()`,
   `statusLine()`.)
3. Key dispatch in confirmation/prompt surfaces MUST use the keymap: `confirm.go onConfirmKey` cancels via
   `matches(key, m.keys.Back)` (not literal `"esc"`); `connections.go` field nav uses
   `matches(key, m.keys.Tab)` (not literal `"tab"`); focus toggle in `app.go` uses `m.keys.Tab`.
4. Key glyphs render bold (`keyStyle.Bold`), labels in their role style (`entryStyled` pattern).

## Sites to convert (from research, non-exhaustive — grep gate at the end)

`pane.go:54,71` · `app.go:822,1304,1312-1314,1330,1341` · `keys.go:152` · `confirm.go:15,57` ·
`confirmview.go:42,78,80` · `connections.go:206,525,535,623` · `commandbar.go:76` · `operation.go:552,563`
· `filebrowser.go:90`.

**Acceptance**: `grep -rn '\^x\|\^o\|d/x/y\|"esc"\|"tab"\|Enter \|Esc ' internal/ui/*.go | grep -v _test.go`
returns only keymap definitions / glyph map — zero hardcoded hint or dispatch literals.
