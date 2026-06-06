# Quickstart: Exercising the redesigned UI

Prereqs: a built `bin/s3s` and a reachable backend (local MinIO is easiest — see
`make test-integration` notes in CLAUDE.md). Build: `make build`.

## Browse with the new layout (US2)

1. `bin/s3s` → bucket list. Enter a bucket.
2. Move the selection (`j`/`k`/↑/↓). The **right pane** updates to show the
   highlighted item's details; for an object it shows metadata + a bounded preview
   after a brief pause (debounced — fast scrolling does not hammer the backend).
3. Narrow the terminal below ~100 cols: the pane stacks/collapses; the hint bar and
   footer stay visible.

## Act with one key, no menu (US1)

- Highlight an object → `d` downloads it immediately (read; works read-only).
- `a` analyzes a folder/level (`du`).
- In a writable session (`w` to arm), `x` deletes, `X` recursively deletes a
  folder, `y` copies, `m` moves, `u` uploads, `+` makes a folder — each goes
  straight into its confirmation/flow. No action menu appears.
- The **hint bar** at the bottom always lists the keys valid for the current
  selection and write state. In read-only, write keys are absent/greyed.
- Multi-select with `space`, then `d`/`x`/`y` act on the marked set.

## Command bar (US3)

- `:` opens the command bar. Type `conn` + Enter → connection manager; `ctx` →
  contexts; `du` → analyze; `q` → quit. `Esc` closes it with no effect; an unknown
  command shows a notice.

## Add a connection from the app (US4)

1. `:conn` (or open the context switcher and press `+`).
2. Fill name, endpoint, region, access key id, secret (no echo), read-only flag.
3. Save: s3s runs a reachability test. On success it stores the secret in the OS
   keychain and writes the connection to the config; on failure it offers "save
   anyway".
4. The new context appears in the list and is switchable immediately (no restart).
5. Verify: open the config file — the new context/cluster/user are present and the
   secret is **not** in plaintext (only `keychain: true`). On the next launch the
   secret resolves from the keychain.

## Regression smoke (US/FR-028)

Confirm still present: context quick-switch `1`–`9`, object view via Enter, `du`
drill-down, bulk download/delete/copy, sort `s`/`S`, write toggle `w` with the loud
badge, structured logs of destructive ops.
