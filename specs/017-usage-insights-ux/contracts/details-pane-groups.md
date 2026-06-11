# Contract: Details-Pane Groups & Field States (US2)

Reorganizes `metaFieldRows` (`internal/ui/metadata.go:35-60`) — the single shared source for
BOTH the Enter object view and the focus details pane (016 invariant preserved).

## Groups (ordered; header = `colHeadStyle`; empty group = fully omitted)

1. **Identity & content** — Key, Size (human + exact bytes, unchanged), Modified
   (`relTime(now,t)` + exact `formatDate`), Type, Class, ETag, Version, Delete marker.
2. **Security & governance** — Encryption, KMS key, Lock, Retain until, Legal hold,
   Replication, Restore.
3. **Delivery** — Expires, Encoding, Cache, Disposition.
4. **User metadata** — existing sorted KV block (header unchanged).

Group 1 always renders (core fields incl. `—` placeholders); groups 2–4 render only when ≥1
field is populated/gated (gated fields keep group 2 visible with `unknown`).

## Field-state rendering (text + role; NO_COLOR-safe)

| State | Text | Role |
|---|---|---|
| populated | value | `metaValStyle` |
| core unset | `—` | dim |
| optional unset | row omitted | — |
| unknown (gated header absent) | `unknown` | warn |
| denied (explicit) | `denied` | warn |
| unsupported | `unsupported` | dim |

`unknown` vs `—` MUST be distinguishable by TEXT alone (constitution VI).

## Multipart ETag

ETag matching `^"?[0-9a-f]{32}-(\d+)"?$` renders the value plus
`(multipart, N parts — not a content hash)` annotation (dim role). Plain ETags unchanged.
Presentation-only (no extra request) — research D15.

## Relative dates

`relTime` units: `just now / Nm / Nh / Nd / Nmo / Ny` + ` ago`; exact timestamp remains in the
same row (`3d ago · 2026-06-08 14:02`). Clock injected (`App.now`) for deterministic tests.
Applies to Modified, Retain until, Expires, and health-card ages (shared helper, VII).

## Per-field copy

- Entry: copy menu → "copy a field…" → field-select state over the visible rows
  (up/down/Enter); Enter emits OSC52 with the FULL untruncated value; footer confirms
  `copied <label> — <first 40 chars…>`.
- Esc exits field-select without copying. Works in both the pane and modeObject.

## Height budget (unchanged guarantees)

Grouping adds header rows: at 130×24 the pane keeps the 016 rules — one expandable section at
a time, `… +N more (i to reveal)` affordance, footer never scrolled off. Group headers count
against the same budget; the height-sweep test (016 `metadata_legibility_test.go` pattern)
re-runs with grouped layout + all states seeded.

## Test obligations (RED first)

1. Grouped render: headers present, order stable, empty groups absent.
2. State matrix: each of the 6 states renders per table; `unknown` ≠ `—` ≠ `denied` by text;
   NO_COLOR run asserts the same.
3. Multipart ETag annotation (32hex-N) vs plain ETag (no annotation).
4. Dual dates: fixed `now` → exact relative strings; row carries both forms.
5. Per-field copy: full value (long KMS ARN) in OSC52 payload, not the truncated render.
6. 130×24 height sweep with groups + a detail section: every value present or revealable.
