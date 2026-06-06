# Contract: Runtime write toggle & loud signalling (US5)

The session's write capability becomes runtime state in the UI, guarded dynamically. The
read-only guard remains the single runtime enforcement point.

## C1 — Model & derivation

- `App` holds `raw storage.Storage` (unguarded client), `ctxReadOnly bool`, `armed bool`.
- Derived `writable = armed && !ctxReadOnly`.
- `activeStore() = storage.Guard(raw, writable)`; **every** operation MUST use `activeStore()`,
  never `raw` directly. While `writable` is false, a mutating call returns `ErrReadOnly` without a
  network call (guard unchanged).
- `Backend`/`Resolver` returns the **raw** store + `ReadOnly bool`; `main.go` stops pre-guarding.

**Tests**: with `armed=false` a bulk/single mutation through `activeStore()` returns `ErrReadOnly`;
with `armed=true` and `ctxReadOnly=false` it reaches the client.

## C2 — Toggle behavior (FR-025/026)

- A dedicated hotkey toggles state at runtime (no restart).
- **Arm** (RO→write): requires a simple deliberate confirm; only on confirm does `armed=true`.
- **Disarm** (write→RO): immediate, no confirm; `armed=false`.
- Arming is refused when `ctxReadOnly` — state stays read-only with a clear reason (FR-028).
- `--write` launch flag sets initial `armed=true` (FR-031); default launch `armed=false` (RO).

**Tests**: RO + toggle → confirm prompt → confirm → writable; writable + toggle → instant RO;
`readonly:true` ctx + toggle → refused, stays RO; `--write` → starts writable.

## C3 — Loud indicator (FR-027)

- While `writable`, a high-contrast `WRITE` badge (`writeBadgeStyle`) MUST render on **every**
  screen — normal views and the alt-screen overlays (action menu, help, object view) — and MUST
  NOT be the first element dropped when width is tight.
- Read-only renders a calm `RO`. The badge reflects the **current context's** `writable`.

**Tests**: `App.View().Content` contains the loud WRITE marker on each mode while armed and the RO
marker while disarmed; narrow-width render still includes the badge; menu/help overlays include it.

## C4 — Context switch & in-flight (FR-029)

- Switching context re-derives `ctxReadOnly`/`writable` from the new context: switching to a
  writable context preserves `armed`; switching to a `readonly:true` context forces `writable=false`
  (the indicator must never be stale).
- Disarming while a committed mutation is in flight does not abort it (existing cancellation path
  governs); it only prevents starting new mutations.

**Tests**: armed → switch to RO ctx → indicator/derivation show RO; armed → switch to writable ctx
→ stays armed; toggle during `phaseRunning` does not corrupt the running op.

## C5 — Logging (FR-032)

Every RO↔write transition MUST be logged as a security-relevant event (new state + context) via
`log/slog` (file only). Secrets never logged.

**Tests**: a toggle emits a log record with the new state and context name.
