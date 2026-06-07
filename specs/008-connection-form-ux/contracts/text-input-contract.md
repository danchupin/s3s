# Contract: Single-line Text Editor (`textField`)

The shared rune-aware editor used by the add-connection form fields and the typed-confirm
input. White-box `package ui` tested.

## State

- `Value string`, `Caret int` (rune index, invariant `0 ≤ Caret ≤ runeLen(Value)`).

## Behaviour

| Operation | Precondition | Effect |
|-----------|--------------|--------|
| `Insert(s)` | any | runes of `s` inserted at `Caret`; `Caret += runeLen(s)` |
| `Backspace()` | `Caret > 0` | rune before caret removed; `Caret--` |
| `Backspace()` | `Caret == 0` | no-op |
| `DeleteFwd()` | `Caret < runeLen` | rune at caret removed; caret unchanged |
| `Left()` | `Caret > 0` | `Caret--` |
| `Right()` | `Caret < runeLen` | `Caret++` |
| `Home()` | any | `Caret = 0` |
| `End()` | any | `Caret = runeLen` |
| `Render(w, masked)` | `w ≥ 1` | window of width `w` containing the caret; masked → `•`×runeLen |

## Invariants

- C1: `Caret` is always a valid rune boundary; no operation splits a multi-byte rune.
- C2: `Render(masked=true)` output contains **no** byte of `Value` (only `•` and caret glyph).
- C3: `Render` always shows the caret position (horizontal scroll when `runeLen > w`).
- C4: `Insert` is atomic — the whole string lands in one call (paste semantics).

## Acceptance (tests, written first)

1. Insert `"abc"`, `Left()`, `Left()`, `Insert("X")` → `Value == "aXbc"`, `Caret == 2`.
2. `End()` then `Backspace()` on `"abc"` → `"ab"`.
3. `Home()` then `DeleteFwd()` on `"abc"` → `"bc"`.
4. `Left()` at `Caret==0` and `Right()` at end are no-ops (no panic, bounds hold).
5. `Insert("héllo")` then caret math uses rune length (5), not byte length (6).
6. `Render(3, false)` on a 10-rune value with caret at end shows the tail incl. caret.
7. `Render(w, true)` length-masks: `•`-count == runeLen, no source bytes present.
