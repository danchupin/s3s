# Roadmap

Backlog of improvements for s3s, captured from code review. Nothing here is a
known bug — these are enhancements. Ordered roughly by value.

## UI / UX

- [ ] **Full-quality image preview via external viewer.** Half-block is bounded
  by the terminal cell grid; Bubble Tea v2's cell renderer strips terminal
  graphics protocols (kitty/iTerm2), so crisp inline images aren't possible
  in-frame. Add a key (e.g. `o`) that opens the selected image in the OS viewer
  (`open` on macOS, `xdg-open` on Linux) from a temp file, or shells out to
  `imgcat`/`kitten icat`. Keep half-block as the inline default.
- [ ] **Sortable lists.** Sort buckets/objects by name, size, or last-modified
  (k9s-style), with a key to cycle the sort column and toggle direction.
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
