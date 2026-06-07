# Quickstart: exercising the 007 blocked command bar

Prereqs: built `bin/s3s` (`make build`) and a reachable backend (local MinIO is easiest —
see CLAUDE.md). For destructive steps, arm write with `w` (or start with `--write`).

## US1 — three blocks, write visible-but-dimmed in read-only

1. `bin/s3s` → bucket list. Look at the footer: three columns — **info** (context ·
   cluster · user · region · s3s ver + add-connection key), **read** (download · analyze ·
   filter · refresh · open), **write** (delete · copy · move · rm · upload · new folder).
2. Read-only (default): the whole **write** block is shown but **dimmed**. Press a write
   key (e.g. `x`) → nothing mutates; the footer nudges "read-only — press `w`".
3. Press `w`, confirm to arm: the write block switches to its **active (amber/caution)**
   style. Read and info blocks are unchanged.
4. Narrow the terminal below ~100 cols: the columns collapse to a compact row that still
   lists the write entries (dimmed) and keeps the loud `[RW]/[RO]` badge.

## US2 — add a connection from a visible affordance

1. In the info block, note the add-connection key (e.g. `+ new connection`). Press it (or
   open the contexts screen).
2. The connection form opens; fill name/endpoint/region/access key/secret, save → it tests
   reachability, stores the secret in the keychain, writes the triple, and appears live in
   the contexts list.

## US3 — palette

- In read-only vs armed, confirm info/read/write/dimmed/caution are visually distinct using
  only the existing palette; under `NO_COLOR` the active-vs-inactive write distinction is
  still legible by text cue (`(w)`/`^`/dim).

## US4 — dangerous actions need a chord + tier-chosen surface

1. Arm write. Highlight an object: bare `x` does **nothing** (no delete, no prompt).
2. `Ctrl+x` → a **centered popup** asks y/N (binary tier). `y` deletes; `n`/Esc aborts.
3. Highlight a folder, `Ctrl+x` → a **prominent inline form** asks you to type the exact
   **path**. A wrong path aborts with no change; the exact path deletes recursively.
4. On the bucket list, `Ctrl+x` on an EMPTY bucket → inline form asks the **bucket name**;
   on a NON-empty bucket it is refused ("purge first") — nothing is deleted.
5. `Ctrl+o` moves the selected object (binary popup); `Ctrl+m` does NOT move (it is Enter).
6. Safe writes keep their bare key: `+` new folder, `y` copy, `u` upload, `d` download.

## US5 — delete a connection

1. Open the contexts screen. Select a **non-active** connection, press `Ctrl+x` → the
   inline typed form asks the exact **connection name**.
2. Confirm → the connection vanishes from the list (config triple removed, keychain secret
   deleted), live. Select the **active** context + `Ctrl+x` → refused ("switch first").

## US6 — progress bar

1. Start a long op (large download, or a bulk delete of many objects). A **determinate
   progress bar with a percent** appears inline in the footer and advances; the list stays
   visible and the op is cancellable (`x`/Esc).
2. A fast op shows **no** bar (no flash). An op with an unknown total shows an
   indeterminate activity indicator instead of a percent.

## Regression smoke

Confirm still present: context quick-switch `1`–`9`, object view via Enter, `du`
drill-down, sort `s`/`S`, the loud `[RW]`/`[RO]` badge, structured logs of destructive ops,
the read-only structural guard (`make check-readonly`).
