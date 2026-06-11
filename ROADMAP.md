# Roadmap

Backlog of improvements for s3s, captured from code review. Nothing here is a
known bug — these are enhancements. Ordered roughly by value.

## Done

- **Budgeted usage, insights & details UX (017)** — the ambient usage scan is capped by a
  configurable budget (`usageScanBudget`, default 20 000 objects; honest `≥` lower bounds,
  partial progress cached, the uncapped scan only via an explicit `A`/`:scan`); the details
  pane regrouped into named sections with dual dates, text-distinct field states, a
  multipart-ETag explanation, and per-field copy; a `Y` copy/share menu (S3 URI, style-aware
  HTTPS URL, aws-cli snippet, client-side presigned GET links, CSV/JSON report export — new
  pure `internal/share` package); a full-screen `H` operator health card (age/size/class
  histograms from the same scan pass, incomplete multipart uploads, small-object
  index-pressure warning); payload-aware previews (pretty JSON/NDJSON with a raw toggle,
  transparent capped gunzip, hexdump for binaries). See `specs/017-usage-insights-ux/`.
  (Manual validation at 130×24 + a prefix-wide MPU check on real RGW still TODO; note the
  MinIO `ListMultipartUploads` exact-key quirk in `contracts/storage-read-extension.md`.)
- **Credential sources simplification & config-path override (014)** — narrowed the four
  credential sources to **two**: the OS keychain (the prompted default; macOS Keychain /
  Windows Credential Manager / Linux Secret Service) and an external `cmd`. Removed the
  inline `secretAccessKey` (literal + `${ENV}`), inline `sessionToken`, and `awsProfile`
  sources outright (pre-release, no migration). Headless keychain failures now point
  loudly at `cmd` instead of any plaintext fallback. Added a config-path override
  (`--config` flag > `S3S_CONFIG` env > default) across the TUI, `cred`, and `config init`,
  with keychain accounts namespaced by config identity so multiple configs never collide.
  Constitution amended to v1.2.0. See `specs/014-credentials-config-path/`.
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
- [ ] **Syntax highlighting** for JSON/YAML/code previews (pretty-print + hexdump
  shipped in 017; highlighting needs a palette-role-safe approach).
- [ ] **Visible context numbers for the digit shortcut.** `1-9` switches context,
  but the numbered list was removed from the UI; surface the numbers somewhere
  (e.g. in the context switcher rows or a compact footer hint).
- [ ] **Edit any connection.** The connections manager can switch / add / delete
  (006), but a saved connection cannot be edited — a wrong endpoint, region, or
  credential forces delete-and-re-add. Add an edit action (e.g. `e` on a row in the
  connections manager) that reopens the add-connection form pre-filled with the
  existing cluster/user/context, re-running the same reachability test + save-anyway
  override, and persisting back to the same config entry (secret left untouched unless
  re-entered). Reuse the existing form (`modeConnForm`) rather than a parallel surface.

## Write iteration (next major arc)

- [ ] **Incomplete-multipart-upload cleanup.** 017 surfaces dangling MPUs in the health
  card; aborting them is a mutation and belongs to the write iteration (explicit
  confirmation + pre-execution logging per constitution V).
- [ ] **Bucket administration.** Policy / lifecycle / encryption / CORS management.
- [ ] **Object versioning management.** Browse versions and delete markers (read side),
  restore/permanently-delete (write side).

## Core

- [ ] **Sort spans only the loaded page, not the whole level.** The current sort
  reorders just the objects already fetched into memory for the current level; with
  token-paginated `ListLevel`, entries on later pages aren't part of the comparison, so
  "sort by size/modified" gives a per-page ordering, not a true level-wide one. Decide an
  approach: fetch-all-then-sort for bounded levels (with a cap / progress + cancel,
  honouring the non-blocking-TUI principle), server-side ordering where the backend
  supports it, or an explicit "sorted within loaded page" cue so the scope is honest.
  Needs design — interacts with pagination, the level cache, and large prefixes.
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
