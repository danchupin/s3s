# Contract: Help Surface & Status Feedback

Covers the redesigned help overlay (FR-010..014c) and status-line feedback
(FR-015..018, FR-029). Verified white-box on `App.View().Content` with `m.mode==modeHelp`
(help) and on `statusLine` output (feedback).

## H1 — Reachability & dismissal

- The help surface is openable from every mode via `?` (existing toggle) and renders in
  alt-screen (existing behavior).
- It MUST end with an explicit close instruction: `press any key to close` (FR-012).

## H2 — Categorized layout (FR-011)

Help groups actions under labelled sections, in order:

1. **Navigation** — move up/down, top/bottom, enter/open, back, cancel (Esc when loading).
2. **Search & View** — filter/search.
3. **Actions** — the `a` menu: lists its items (new folder, delete, upload, copy, move,
   recursive delete, refresh), marking which require write mode (FR-014b).
4. **Context** — switch context, numeric quick-switch.
5. **Global** — help, quit.
6. **Connection** — context, cluster, endpoint, region, user, s3s version.

## H3 — Complete keymap with aliases (FR-010, FR-014, FR-014c)

- Every logical action in `defaultKeys()` appears in exactly one section.
- Each action lists **all** bound keys, with the **vim aliases included here** (help is the
  only place vim is advertised, per FR-031): e.g. `↑/k`, `↓/j`, `→/l/Enter`, `←/h/Esc`,
  `g/Home`, `G/End`, `q/Ctrl+C`.
- The key column is derived from `defaultKeys()` (single source of truth) so help can
  never drift from actual bindings.
- The Actions section documents the menu key `a` and its contents; write-only items are
  marked (FR-014b).

## H4 — Capability reflection (FR-013)

- When `m.writable`, the Actions-section write items are shown as available.
- When `!m.writable`, the write items are hidden OR clearly marked unavailable
  (e.g. dimmed with a `(needs --write)` note); Refresh remains available. The section must
  not imply the write actions work when they don't.

## H5 — Connection section (FR-014a, FR-021)

- Presents the metadata removed from the footer: `context`, `cluster`, `endpoint`,
  `region`, `user`, `s3s ver` (`Version`).
- Empty values are omitted or shown as `—`.
- Sources ONLY the non-secret `Backend` display fields + `ctxName` + `Version`. No
  credential/`SecretKey` path is referenced (those never reach `Backend`), so no new secret
  surface is introduced — existing `logging.Secret` redaction in `storage`/`logging` is
  untouched.

## S1 — Named loading (FR-015)

`statusLine` loading text names the in-flight target by current state:
- `modeBuckets` → `loading buckets…`
- `modeTree` → `loading contents…`
- `modeObject` (or op browse) → `loading object…`
Each shows the `(Esc to cancel)` affordance (cancel folded into Esc per FR-029; the old
`x` hint is removed).

## S2 — Search pending (FR-016)

While `m.searching` and a debounced search is scheduled-but-unfired, the status shows a
pending indicator (`searching…`) so the debounce delay reads as intentional.

## S3 — Typed-confirmation visibility (FR-017)

- The typed-confirmation prompt continuously shows the **exact required target** beside
  the user's input (existing `opPromptLine` behavior — locked by a regression test).
- A mismatch on submit cancels safely (`errConfirmMismatch`, no command dispatched) —
  existing two-tier safety preserved (FR-020).

## S4 — Notice vs error distinction (FR-018)

- Success notices render in a success hue (`colOK` green via `noticeStyle`), visually
  distinct from `errStyle` red errors.
- Notices clear on the next interaction (existing behavior).

## S5 — Status-row precedence (FR-018a)

- When multiple status conditions hold at once, the single status row shows **exactly one**,
  in this priority order (highest first): operation prompt (name/dest entry or confirmation)
  > running-op progress > loading > search-pending > success notice > error.
- The row is never split or stacked — lower-priority conditions are suppressed for that frame.

## Test obligations (TDD — write first, must fail before impl)

1. Help in any mode lists all `defaultKeys()` actions, grouped under the 6 section titles
   (Navigation/Search & View/Actions/Context/Global/Connection), with multi-key aliases
   **including vim** (`↑/k`, `←/h/Esc`, `g/Home`) shown (H2/H3, FR-014c).
1a. Help Actions section documents the `a` menu key and lists its items (new folder, delete,
    upload, copy, move, recursive delete, refresh), marking write-only ones (FR-014b).
2. Help contains a Connection section with endpoint/region/user/version (H5).
2a. Redaction guard (FR-021): the Connection section sources ONLY the non-secret `Backend`
    display fields (`Cluster`, `User`, `Endpoint`, `Region` — all plain `string`), plus
    `ctxName` and `Version`. `storage` credentials (`SecretKey`, access keys) are never in
    `Backend` and MUST NOT be referenced by the help renderer — so no new secret surface is
    introduced. Assert structurally: help renders exactly those known fields and pulls
    nothing from a credential/`SecretKey` path.
3. `!writable` help: Write actions hidden or marked unavailable (H4).
4. Help contains `press any key to close` (H1).
5. Loading status differs by mode: contains `buckets` / `contents` / `object`, and shows
   `Esc to cancel` (not `x`) (S1, FR-029).
6. Notice uses green hue, error uses red hue — distinct ANSI (S4).
7. Typed-confirm prompt shows required target alongside input; mismatch → no dispatch
   (S3).
8. Status precedence (S5/FR-018a): with both a loading state and a pending notice set, the
   row shows the loading line (not the notice); with an op prompt active during a load, the
   row shows the op prompt — assert exactly one line, chosen by the priority order.
