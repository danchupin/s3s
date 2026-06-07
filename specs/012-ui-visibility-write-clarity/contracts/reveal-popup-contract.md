# Contract: Reveal/inspect popup + OSC52 copy

**Stories**: US1. FR-002, FR-004, FR-021. Source: new `reveal.go`, reuse `confirmview.go` popup base,
`tea.SetClipboard`.

## Behaviour

- Bound to keymap `Reveal` (default `i`). Available in every browse context (read-only — no write gate).
- Opens a centered popup (reuse `confirmPopupView`/`popupBoxStyle`) showing the FULL identifier of the
  current target:
  - objects zone / level: the highlighted object key or folder/prefix
  - bucket list: the highlighted bucket name
  - on a breadcrumb context: the full path (`revealPath`)
- Always displays the full value (wrapped/scrolled inside the popup if longer than the box).
- Emits `tea.SetClipboard(value)` — best-effort OSC52; silent no-op where unsupported (value still shown).
- Dismiss: any key / Esc.

## Invariants

- Popup height ≤ terminal − footer − borders; never clips the footer (FR-022).
- Suppressed while a higher-priority modal is active (`m.op != nil`, `armConfirm`, `modeCommand`).
- NO_COLOR-safe (text + border, no color-only cue).
- No new style/hue — reuses `popupBoxStyle`, `metaRow`, `entryStyled` (VII).

## Tests

- `TestRevealShowsFullValue`: long key → popup contains the complete value (no `…`).
- `TestRevealEmitsClipboardCmd`: `pressCmd(m, RevealKey)` yields a `tea.SetClipboard` cmd with the value.
- `TestRevealAllZones`: works in bucket list, objects zone, full-screen level.
- `TestRevealFooterNotClipped`: narrow/short terminal + 300-char value → footer still present.
- `TestRevealDismiss`: any key / Back closes it.
