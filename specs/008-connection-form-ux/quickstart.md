# Quickstart: Connection Management UX Fixes

Manual + automated verification of the nine user stories (US1–US9). All in `internal/ui`.

## Build & test

```bash
make test            # unit (white-box ui + textfield)
make fmt vet lint
make check-readonly  # must stay green (no storage change)
make build && ./bin/s3s
```

## US1 — discoverable connection delete

1. Launch, open connections (`c` / the add-conn key surface).
2. Highlight an existing **non-active** connection → the view shows `Ctrl+X delete` inline.
3. Press `Ctrl+X` → typed-name confirm appears.
4. Highlight the **active** connection, press `Ctrl+X` → message: cannot delete the active
   connection (switch context first).
5. Highlight `+ add connection` → no active delete hint.

## US2 — Ctrl+X labels

1. In the bucket/object list, the write group shows `Ctrl+X delete` (not `^x`).
2. Press bare `x` on a deletable selection → nudge reads "press Ctrl+X to delete".
3. No surface anywhere shows `^x` / `^o`.

## US3 — usable form text entry

1. Open `+ add connection`.
2. Focus endpoint, **paste** a URL from the clipboard → the whole value lands.
3. Move the caret with `←/→/Home/End`; insert a character mid-field → it lands at the caret.
4. `Backspace` mid-field removes the char before the caret (not always the last).
5. Focus secret, paste a 40-char key → shown as 40 `•`; saved to keychain, never plaintext.
6. On a toggle row, `←/→`/paste do nothing; `space` still toggles.
7. Repeat paste + caret edits in the **delete-connection** typed-confirm input.

## US4 — secret guidance

1. In the form, focus the secret field → hint names the secret access key, says it is stored
   in the OS keychain, and that env var / cmd / AWS profile are config-file-only.
2. Other fields show a one-line expectation when focused.

## US5 — quieter command bar

1. Wide terminal → no `INFO` / `READ` / `WRITE` headings; info / read / write entries still in distinct columns.
2. Read-only context → no headings, but literal `w to arm` still shown.
3. Narrow terminal → collapsed bar readable, no orphan title text.

## US6 — changes visible after every action (same bucket)

1. Copy an object to a **different prefix** (same bucket); navigate there → it is present (no manual `r`).
2. Move an object to another prefix in the same bucket → gone from source, present at destination, both
   without manual refresh.
3. Bulk-copy marked objects to a destination prefix; navigate there → copied objects present (no `r`).
4. New folder / upload / delete / recursive delete in the current level → reflected at once
   (no regression).

## US7 — connection affordance

1. The command bar connection entry reads "connections" (not "new conn").
2. Triggering it opens the manager where the active connection can be switched (and added/deleted).
3. Narrow terminal that drops read entries → the "connections" entry survives (not dropped first).

## US8 — reset an active filter

1. Apply a bucket filter / level search → the command bar shows `Esc clear`.
2. Trigger it → the full unfiltered list returns.
3. No filter active → no clear affordance.

## US9 — no duplicate delete labels

1. Object cursor → write group shows the object delete only; folder cursor → recursive delete
   only. Never two "delete" at once.

## Regression guards

- `make check-readonly` green (no new write S3 symbol; no storage edits).
- Existing add/test/save/delete connection flow unchanged (validation, save-anyway,
  keychain, active-context refusal).
