# Contract: Chord Label Format (US2, FR-004)

## Rendering

- `glyph("ctrl+x")` MUST return `"Ctrl+X"`; `glyph("ctrl+o")` MUST return `"Ctrl+O"`.
- `glyph("ctrl+c")` remains `"Ctrl+C"` (the existing precedent the others align to).
- Format is no-space around `+` (`Ctrl+X`, not `Ctrl + X`) — clarified.
- No surface (command bar, connections view, confirm prompts, help, nudges) may display the
  caret shorthand `^x` / `^o`.

## Invariants

- K1: `App.View().Content` never contains `"^x"` or `"^o"` on any chord-advertising surface.
- K2: every advertised chord reads in `Ctrl+KEY` form.
- K3: the bare-key nudge for a dangerous action reads naturally with the new glyph
  (e.g. "press Ctrl+X to delete") without redundant "(Ctrl chord required)" noise.

## Acceptance (tests, written first)

1. `glyph("ctrl+x") == "Ctrl+X"`, `glyph("ctrl+o") == "Ctrl+O"`.
2. Command bar write group on an object → contains `Ctrl+X`, not `^x`.
3. Press bare `x` on a deletable selection → notice contains `Ctrl+X`, not `^x`.
4. Grep-style assert: rendered View of bucket/object/connections modes has no `^x`/`^o`.
