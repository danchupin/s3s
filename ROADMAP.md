# Roadmap

Backlog of improvements for s3s, captured from code review. Nothing here is a
known bug — these are enhancements. Ordered roughly by value.

## Done

- **UI redesign (006)** — k9s-style rework: the modal action menu is gone; every item
  action is a single direct key (`d` download, `a` analyze, `x`/`X` delete/recursive,
  `y` copy, `m` move, `u` upload, `+` mkdir, `r` refresh) with an always-visible
  contextual hint bar. A persistent details/preview pane sits beside the list (debounced
  per-selection load; collapses on narrow terminals). A `:` command bar jumps between
  views. Cluster connections can be added from inside the app (`:conn`) — persisted to
  config as a cluster+user+context triple with the secret in the OS keychain (never
  plaintext), reachability-tested with a save-anyway override. See `specs/006-ui-redesign/`.
  (Integration test against MinIO — `connections_integration_test.go` — still TODO.)
- **Storage operations & analytics (005)** — download objects to local disk, `du`
  storage analytics (ranked breakdown + drill-down), multi-select bulk
  download/delete/copy, sortable lists, a runtime read-only↔write toggle with a loud
  always-on indicator, and pluggable secure credential sources (keychain / command /
  AWS profile / `${ENV}` / prompt) with an `s3s cred` subcommand. See
  `specs/005-storage-ops-analytics/`.
- **UI/UX refinement (004)** — contextual action menu, footer declutter, key
  discoverability. See `specs/004-ui-ux-refinement/`.
- **Object write operations (003)** — delete, upload, copy, move/rename, recursive
  delete. See `specs/003-object-write-ops/`.
- **Write foundation & safety (002)** — the first mutating capability, the two-tier
  confirmation framework, and create-folder. See `specs/002-write-foundation/`.

## UI / UX

- [ ] **Full-quality image preview via external viewer.** Half-block is bounded
  by the terminal cell grid; Bubble Tea v2's cell renderer strips terminal
  graphics protocols (kitty/iTerm2), so crisp inline images aren't possible
  in-frame. Add a key (e.g. `o`) that opens the selected image in the OS viewer
  (`open` on macOS, `xdg-open` on Linux) from a temp file, or shells out to
  `imgcat`/`kitten icat`. Keep half-block as the inline default.
- [ ] **Richer content preview.** Syntax highlighting for JSON/YAML/code; a
  hex-dump view for binary objects instead of just a summary line.
- [ ] **Visible context numbers for the digit shortcut.** `1-9` switches context,
  but the numbered list was removed from the UI; surface the numbers somewhere
  (e.g. in the context switcher rows or a compact footer hint).
- [ ] **Copy-to-clipboard.** Yank the selected key / full s3 URI / ETag.

## Core

- [ ] **Use the authoritative content-type for preview classification.**
  `loadPreview` currently passes an empty content-type and relies on byte
  sniffing, even though `HeadObject` (loaded concurrently in the object view)
  returns `ContentType`. Thread it through so classification matches the backend.

## Testing / tooling

- [ ] **Testable entrypoint.** `cmd/s3s` is at 0% coverage; extract the wiring in
  `run()` into a thin, testable seam (config → backend → model) so the happy path
  and error branches can be unit-tested.
- [ ] **golangci-lint pin.** Document/pin a golangci-lint version built with the
  module's Go toolchain to avoid "targeted Go version" mismatches.

## Notes

- Image protocol code (kitty/iTerm2 escapes) exists in `internal/preview` behind
  `S3S_IMAGE_PROTOCOL=kitty|iterm2|auto` but does not survive Bubble Tea v2's
  renderer; treat as experimental until the external-viewer path lands.
