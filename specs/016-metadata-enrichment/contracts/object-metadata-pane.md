# Contract: Object Metadata Pane (US1 — FR-001/FR-002/FR-003)

**Surface**: the SHARED `metaFieldRows` (`internal/ui/metadata.go:28-37`) + the new
`omitEmpty` helper, consumed by BOTH `metaPane` (`metadata.go:42-68`, the `modeObject`
Enter view) AND `paneTree` (`pane.go:79`, the focus pane reached on the wide layout via
`browseDetailsView`, `pane.go:38-43`). White-box testable on `App.View().Content` from
both render paths.

## Inputs

- A `storage.ObjectMetadata` with the enriched fields (data-model §1), populated from the
  **existing** `HeadObject` (no extra round-trip; FR-002).
- Render width `w`.

## Rendered shape (in order, ALL inside metaFieldRows)

1. Core block, ALWAYS, via `metaRow` (keep `orDash`): `Key`, `Size` (`<human> (<bytes>
   B)`), `Modified`, `Type`, `Class`, `ETag` (`metadata.go:30-35` unchanged).
2. Optional block, each via `omitEmpty(label, value, gated)`:
   - non-gated, value present → `metaRow(label, sanitizeLabel(value), w)`
   - non-gated, value empty → **no line emitted** (FR-003 compact)
   - gated (object-lock mode, legal-hold), value empty → `metaRow(label, "unknown", w)`
   - gated, value present → `metaRow(label, value, w)`
   Fields: Version, Delete-marker, Encryption (type + key ref), Replication, Restore,
   Object-lock mode, Retain-until, Legal-hold, Lifecycle expiration, Content-encoding,
   Cache-control, Content-disposition.
3. User-metadata block (existing, `metadata.go:56-66`, only in `metaPane`), only when
   non-empty.

## Invariants

- I1 (FR-003): an absent optional field emits ZERO lines — no blank gap. Asserted from
  BOTH paths by seeding only a version id and asserting the replication/restore/lock rows
  are absent while core rows are present.
- I2 (SC-004): a permission-gated field with no value renders `unknown`, NEVER `none` and
  never absent.
- I3 (VI): the pane height stays within the corrected `View()` budget (layout-budget
  contract); a height sweep at 80×24 AND 130×24 with all optional fields present keeps the
  footer on-screen and emits the `… +N more (i to reveal)` affordance for any clipped row
  (no silent truncation).
- I4 (VI): long values (KMS key ARN, lifecycle date) are truncated in the cell but
  revealable via `keys.Reveal` (`i`); no identifier is permanently hidden.
- I5 (edge `spec.md:254`): a multipart ETag `"<hash>-<n>"` renders verbatim, not labeled as
  an MD5.
- I6 (shared path): because the optional block lives in `metaFieldRows` (not `metaPane`),
  the focus pane (`paneTree:79`) renders the enriched fields too — asserted from the
  `browseDetailsView`/`paneTree` path, not only the Enter view.

## Degrade order (narrow width / overflowing height)

Value cell truncates to `w - metaKeyWidth` (`metaRow`, `metadata.go:17-23`); the label
column (14) is fixed; reveal recovers the full value. No row is dropped by width — only
empty optional rows are omitted (by I1). When ROW COUNT exceeds the body budget, the pane
appends `… +N more (i to reveal)` and the clipped rows are reveal-recoverable (I3).
