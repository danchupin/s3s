# Contract: Add-connection form `buckets` field

Covers FR-005, FR-006 (parse/normalize at creation).

## Field
- New text field `buckets` at cursor index `fldBuckets = 5` (between `secret` and `path-style`);
  label `"buckets"`. Boolean rows shift: `fldPathStyle = 6`, `fldReadOnly = 7`, `connFieldCount = 8`
  (see data-model.md table).
- Renders as a normal text input (not a checkbox). `focusField()` returns `&f.buckets` for
  `fldBuckets`. Space at `fldBuckets` inserts a space (falls through to text append, NOT a toggle).
- Hint (`connFieldHint`): "comma/space-separated bucket names — pin these when credentials can't list
  all (optional)".

## Behavior
- Optional: empty is valid; `validateForm()` adds **no** check for it (and only runs after the
  existing name/endpoint/credential checks).
- `draft()` sets `ConnDraft.Buckets = parseBuckets(f.buckets.Value)` using the shared normalization
  (split on comma/space, trim, drop empty, dedupe order-stable).
- `Connector.Save` maps `ConnDraft.Buckets` → `NewConnection.Buckets` → `Cluster.Buckets`.
- Paste (`tea.PasteMsg`) into the focused `buckets` field inserts via `textField.Insert` like any
  other field (interior newlines → space, trailing stripped — existing paste rule).

## Test assertions (white-box ui)
1. Navigate to `fldBuckets` (5 × `down` from name) ⇒ typing appends to `f.buckets.Value`; `viewOf`
   shows the text on the `buckets` row.
2. `f.buckets.Value = "a, b  c,,a"` ⇒ `draft().Buckets == ["a","b","c"]`.
3. Empty `buckets` ⇒ `draft().Buckets` is nil/empty; form still validates/saves.
4. `space` on `fldBuckets` inserts a space (does not toggle); `space` on `fldPathStyle`/`fldReadOnly`
   still toggles (regression guard for the index shift).
5. Saving with buckets ⇒ `fakeConnector.savedDraft.Buckets` matches normalized list.
