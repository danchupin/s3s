# Contract: Preview Rendering Upgrades (US5)

Extends `internal/preview` (pure transforms; no new requests; `Limit` = 5 MiB stays,
`text.go:15`).

## Pipeline order

```
fetched range (≤ Limit)
  → gzip? (magic 1f 8b; hints: Content-Encoding, .gz)  → gunzip capped at Limit out
  → Classify (existing, text.go:74)                     → text | json | ndjson | image | binary
  → render form: pretty (default for json/ndjson) | raw | hexdump (binary)
```

## gzip

- Detection: magic bytes first; `Content-Encoding: gzip` / `.gz` suffix corroborate (any one
  suffices for the attempt; failed gunzip → fall back to raw bytes silently).
- Cap: `io.LimitReader(gz, Limit)` on OUTPUT — compression-bomb-safe; `Truncated=true` when
  the cap is hit.
- Indicator line: `gzip: 1.2 MiB compressed → 5.0 MiB shown (truncated)` (dim role).
- Decompressed bytes re-enter `Classify` (gzipped JSON pretty-prints).

## JSON / NDJSON

- `KindJSON`: whole payload parses as ONE value (object/array). `KindNDJSON`: ≥2
  newline-delimited parseable values (per-line `json.Valid` over non-empty lines).
- Pretty: `json.Indent` (2 spaces) for JSON; per-line indent for NDJSON, record order
  preserved. Computed once per payload (5 MiB max — instant); scroll machinery unchanged.
- Parse failure anywhere → silent raw fallback (no error banner; FR-025). Truncated payloads
  (ranged cap) usually fail the parse → raw, by design.
- Toggle: `p` (`keys.RawToggle`) in `modeObject` flips pretty↔raw; state resets on each new
  object; hint advertised in the modeObject hintbar; no-op for non-JSON kinds.

## Hexdump (binary)

- Body: `encoding/hex.Dumper` format — `00000000  xx xx … |printable…|`, 16 bytes/row, within
  the already-fetched range; existing one-line binary summary stays as the header line.
- Width: rows are 78 chars — fits the ≥80-col minimum; no wrap handling needed.

## Test obligations (RED first)

1. Pretty JSON golden; raw toggle returns byte-identical original; invalid JSON → raw, no
   error text in `View().Content`.
2. NDJSON: 3 records → 3 indented blocks in order; one bad line → whole payload raw.
3. gzip: golden small payload; bomb (high-ratio) capped at Limit with `Truncated` indicator;
   bad magic after `.gz` hint → raw fallback.
4. gzipped JSON → pretty (re-classify path).
5. Hexdump golden incl. non-printable bytes; header summary preserved.
6. `p` resets between objects; ignored for text/image kinds.
