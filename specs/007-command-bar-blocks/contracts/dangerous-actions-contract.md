# Contract: dangerous-action gating + tier-chosen confirmation (US4)

## Chord gating (FR-021/FR-022, SC-008)

- A bare (un-modified) key NEVER triggers a dangerous action (delete object, bulk delete,
  recursive delete, delete bucket, move, overwrite). The bare key is inert for that action
  and the write block advertises the chord (`^x delete`, `^o move`).
- The Ctrl chord triggers it (R1): `ctrl+x` for delete (object/group/recursive/bucket/
  connection — routed by selection context), `ctrl+o` for move. Overwrite escalates at
  dest-submit (no new key).
- Safe writes keep their bare key (no chord): new folder `+`, copy `y` (to a new key),
  upload `u`, download `d`.

## Surface per tier (FR-023/FR-023a/FR-024/FR-027a)

| Target | Tier | Surface | Typed identifier |
|--------|------|---------|------------------|
| single-object delete, bulk delete | binary | centered popup | — |
| move/rename | binary | centered popup | — |
| overwrite (copy/upload onto existing) | binary | centered popup | — |
| recursive (directory) delete | typed | inline form | exact **path** |
| bucket delete | typed | inline form | exact **bucket name** |
| connection delete | typed | inline form | exact **connection name** |

- **Binary**: a centered popup over the dimmed body, y/N (k9s-style).
- **Typed**: a prominent inline form (NOT a separate window) with a real editable field;
  exact, case-sensitive full match; wrong entry → abort, no mutation, retry allowed
  (FR-024a).
- Both surfaces carry the loud write badge, share one visual style (FR-027a, SC-009a),
  cancel on Esc with no change (FR-025), confirm runs the existing flow unchanged.
- Neither surface clips on 80×24; long identifiers scroll in the inline field (SC-009).

## Bucket delete (FR-024b, SC-015)

- Chord-gated (`ctrl+x` on the bucket list), typed bucket-name confirm.
- Requires an EMPTY bucket: a non-empty bucket is refused with a "purge first" status
  line; no recursive purge is performed (`ErrBucketNotEmpty`, refused before `DeleteBucket`).

## Read-only fall-through (FR-028)

- A dangerous chord in a read-only context opens NO surface; it falls through to the same
  read-only nudge as the bare key.

## Test checklist

- [ ] bare `x` on an object → no delete, no surface (SC-008)
- [ ] `ctrl+x` on an object → centered binary popup; y deletes, n/Esc aborts
- [ ] `ctrl+x` on a folder → inline typed form requiring the path; wrong path aborts
- [ ] `ctrl+x` on the bucket list (empty bucket) → inline typed form requiring bucket name
- [ ] `ctrl+x` on a non-empty bucket → refused, "purge first", nothing deleted
- [ ] `ctrl+o` on an object → binary popup (move); `ctrl+m` does NOT move (it is Enter)
- [ ] overwrite at dest-submit → binary popup
- [ ] dangerous chord in read-only → nudge only, no surface
- [ ] both surfaces show the write badge and share style; Esc cancels with no change
- [ ] storage: RemoveBucket empty→ok, non-empty→ErrBucketNotEmpty (unit + MinIO integration)
- [ ] guard: readOnlyGuard.RemoveBucket → ErrReadOnly
