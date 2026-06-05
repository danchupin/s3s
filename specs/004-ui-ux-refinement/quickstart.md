# Quickstart: UI/UX Refinement (Action Menu + Footer + Help + Feedback)

How to build, run, and verify the keymap reduction and footer/help/status redesign.

## Build & run

```bash
make build
./bin/s3s            # read-only (default)
./bin/s3s --write    # write mode (menu shows write actions)
```

## What changed (user-visible)

- **One actions key.** Press `a` to open a contextual menu of the operations valid for what
  you've selected (delete, copy, move, upload, new folder, recursive delete, refresh). The
  old per-op keys (`+ d u y m D`) and `r` refresh are gone from the top level.
- **Cancel is Esc.** An in-flight load (or running upload/delete) is cancelled with `Esc`;
  the standalone `x` is removed.
- **Arrows are primary.** The footer and menu show arrow keys (`↑/↓`, `Enter`, `Esc`). Vim
  keys (`h/j/k/l`, `g/G`) still work but are listed only in help.
- **Calm footer (≤ 3 rows).** Compact identity row, one contextual hint row (capped at 6,
  shows `a actions`, drops low-priority with `? more`), optional status row. Endpoint/region/
  user/version moved into help.
- **Richer help (`?`).** Categorized — Navigation / Search & View / Actions (the menu) /
  Context / Global / Connection — with all key aliases incl. vim.
- **Clearer status.** Loading names what's loading and says `Esc to cancel`; debounced search
  shows `searching…`; success notices are green, errors red.

## Manual verification

1. `./bin/s3s --write`, enter a bucket, select an **object**, press `a` → menu lists Delete,
   Copy, Move/Rename, Upload here, New folder, Refresh. Select a **folder** → Recursive
   delete replaces Delete/Copy/Move.
2. Press a removed key (`d`, `u`, `+`, `r`, `x`) at top level → nothing happens.
3. Read-only (`./bin/s3s`): press `a` → only Refresh; the bucket list also refreshes via `a`.
4. Start a large load, press `Esc` → load cancels.
5. Footer at ~120 cols → shows `a actions`, no individual write keys; narrow to ~50 cols →
   hint row stays one line, ends with `? more`, keeps `? help`/`q quit`.
6. Press `?` → Navigation/Search & View/Actions/Context/Global/Connection sections; vim keys
   shown next to arrows; Connection lists endpoint/region/user/version.
7. Open a large object → status `loading object…  (Esc to cancel)`.

## Automated verification (TDD-first)

```bash
go test ./internal/ui/ -run 'TestMenu|TestActions|TestFooter|TestHints|TestHelp|TestStatus|TestLoading|TestCancel'
make test            # full unit suite
make fmt vet lint    # gates
make check-readonly  # unchanged guard — must still pass (no SDK writes added)
```

Expected: new white-box tests in `actionmenu_test.go` / `footer_test.go` / `keys_test.go` /
`app_test.go` assert contextual menu items, removed-keys inert, Esc-cancel, footer ≤ 3 rows
with `a actions`, arrow-primary cues, categorized help with Actions + Connection, named
loading. The read-only structural guard is unaffected.

## Out of scope (do not expect)

- No command palette / global fuzzy finder (menu is per-selection contextual).
- No backend, storage, write-semantics, or confirmation-tier changes (menu re-enters the
  existing flows).
- No bucket-level write ops (none exist yet); the bucket-list menu offers Refresh only.
- No colour-blind/low-colour fallback, non-colour cue duplication, or CJK width handling
  (deferred — see spec Assumptions).
