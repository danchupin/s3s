# Contract: Command bar `:` (US3)

New `modeCommand`. Opened with `:`; a one-line input reusing the search-input
render path in `statusLine` (`app.go:711`).

## CB1 — Registry (FR-016/FR-017)

| Command | Aliases | Effect |
|---------|---------|--------|
| `buckets` | — | jump to the bucket list |
| `contexts` | `ctx` | open the context switcher |
| `connect` | `conn` | open the connection manager (US4) |
| `analyze` | `du` | analyze current selection/level |
| `refresh` | — | reload current list |
| `help` | `?` | open help |
| `quit` | `q` | quit |

As the user types, command names are prefix-filtered and shown (discovery). Enter
on a unique/selected match dispatches and closes the bar.

## CB2 — Dismiss & errors (FR-018)

- `Esc` closes the bar with **no** side effect.
- An unknown command on Enter → a non-destructive `notice` ("unknown command: X");
  no action taken; bar closes.

## CB3 — Disambiguation vs filter (FR-019)

- `:` (command) and `/` (filter) are **distinct modes**; opening one is impossible
  while the other's input is active.
- An in-progress operation prompt (name/dest/confirm) owns keys first (existing
  precedence in `onKey`, `app.go:329`); `:` does not interrupt it.

**Tests**: `:` opens the bar; typing a known name + Enter jumps/acts; prefix shows
candidates; Esc is a no-op; unknown name → notice, no action; `:` is inert while a
search or operation prompt is active.
