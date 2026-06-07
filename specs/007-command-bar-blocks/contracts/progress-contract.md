# Contract: progress bar for long operations (US6)

## Behavior (FR-034..FR-038, SC-012/013)

- A long-running operation (download, recursive delete, bulk copy/move/delete, `du`
  analyze) shows a **Claude-Code-style determinate progress bar** inline in the footer
  status zone: a filled/empty horizontal track + a trailing percent + an elapsed/label
  hint, drawn from the existing palette. The list stays visible (not an overlay, not
  replacing the list body).
- Progress advances **monotonically** toward 100%; the bar **clears** on completion or
  cancel, returning to the prior view/result.
- When the total is unknowable up front, an **indeterminate** activity indicator is shown
  instead of a fabricated percent.
- The bar appears only after a brief **"taking a while" threshold** — operations that
  finish faster show NO bar (0% flash).
- The UI stays responsive and the operation cancellable (`x`/Esc) while progress shows
  (non-blocking, Constitution II).

## Rendering

- `progressBar(frac, width)` → `[████████░░░░░░] 57%` style (filled `colAccent`, empty
  `colDim`); a label/elapsed suffix (e.g. `uploading 12 MiB / 21 MiB`).
- Determinate source: upload = bytes uploaded/total; bulk_* / recursive delete = done/total
  counts when a total is known; otherwise indeterminate (spinner + counts, no percent).

## Test checklist

- [ ] determinate op (known total) → bar with monotonic percent, filled track grows
- [ ] indeterminate op (unknown total) → activity indicator, no percent (FR-037)
- [ ] op finishing under the threshold → no bar rendered (SC-013)
- [ ] op crossing the threshold → bar rendered (SC-012)
- [ ] completion/cancel → bar cleared, prior view restored
- [ ] op remains cancellable while the bar shows; UI responsive
- [ ] 80×24 → bar fits the footer width without clipping the list
