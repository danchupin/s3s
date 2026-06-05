# Phase 0 Research: Write Foundation & Safety

All Technical Context unknowns resolved. No outstanding NEEDS CLARIFICATION.

## 1. Runtime read-only enforcement

**Decision**: Wrap every constructed backend in a `readOnlyGuard` (in
`internal/storage`). The guard implements the same `storage.Storage` +
`storage.Mutator` interfaces but, when read-only, returns `ErrReadOnly` from every
mutating method **before** any network call. Reads pass through untouched. The
resolver decides at construction time whether to wrap, from the `WriteMode`
(global `--write` AND not per-context `readonly`).

**Rationale**: A single enforcement point inside the storage layer cannot be
bypassed by UI code (Constitution I) and guarantees "unchanged storage" for refused
mutations (FR-003, FR-011, FR-012) without a round-trip. Construction-time wrapping
means the UI holds either a writable or a read-only backend and never re-checks
policy itself.

**Alternatives considered**:
- *UI-side checks before calling*: rejected — duplicates policy in the UI, easy to
  forget on a new write feature, violates single-source enforcement.
- *Two separate interfaces resolved by capability sniffing*: more complex; the
  guard still needed for runtime refusal. The guard subsumes it.
- *Build-tag a read-only binary*: doesn't model per-context read-only at runtime
  (FR-002 needs the same binary to mix writable + protected contexts).

## 2. Keep `scripts/check-readonly.sh` unchanged

**Decision**: Retain the guard script as-is. It scans only files **outside**
`internal/storage/` and forbids the SDK `service/s3` import plus mutation symbols
there. Adding `CreateFolder`'s `PutObject` call **inside** `internal/storage` does
not trip it.

**Rationale**: The invariant we still want is "SDK mutations live only in the
storage layer". The script already enforces exactly that. (Earlier notes that the
guard would be "relaxed/inverted" were inaccurate — corrected here and in CLAUDE.md.)

**Caveat for later features**: the script's symbol regex matches
`(Put|Delete|Create|Copy|Upload|Restore|Write)(Object|Bucket|…)`. The 002 interface
method `CreateFolder` is safe (`Folder` is not in the entity list). But a future 003
interface method literally named e.g. `DeleteObject` would match and trip the guard
when the **UI** calls it. Name UI-facing mutating interface methods to avoid the
verb+entity fusion (e.g. `RemoveObject`, `Delete(...)`) or extend the script's
allow-list deliberately. Noted for 003; out of scope here.

## 3. Create-folder semantics

**Decision**: A "folder" is a zero-length object whose key is the target prefix
ending in `/` (e.g. `reports/`). `CreateFolder(ctx, bucket, prefix)` issues
`PutObject` with an empty body and that key.

**Rationale**: This is the de-facto convention MinIO and Ceph RGW (and the AWS
console) use to represent an empty folder; listing with the `/` delimiter then
surfaces it as a common prefix, matching the existing tree navigation (001).

**Alternatives considered**:
- *No-op "virtual" folders* (S3 has no real folders): rejected — the user needs a
  persistent, visible folder after refresh (SC-006); a virtual one vanishes.

**Validation** (FR-010): reject empty/whitespace names; normalise to exactly one
trailing `/`; reject keys containing control characters; surface a clear "already
exists" message if the prefix or a colliding object key is already present (do not
overwrite).

## 4. Two-tier confirmation framework

**Decision**: A reusable confirmation step modeled as UI state, not per-action ad
hoc prompts. Each mutating intent declares a `ConfirmTier`:
- **Simple** (reversible, e.g. create-folder): a yes/no overlay; `y`/`Enter`
  confirms, `n`/`Esc` aborts.
- **Typed** (destructive/irreversible): the operator must type the exact target
  identifier (bucket or key); a non-match aborts with no action.
The overlay renders within the existing bordered layout; the model exposes the
pending operation so white-box tests can assert tier + outcome.

**Rationale**: Centralising the tiers now means 003's delete/overwrite plug into a
proven guardrail (US3) instead of reinventing prompts. Typed confirmation is the
standard "type the name to confirm" pattern (GitHub/AWS) that defeats reflexive
keypresses (SC-003).

**Alternatives considered**:
- *Single yes/no for everything*: rejected — fails SC-003 (destructive needs a
  stronger gate).
- *OS-level dialogs*: impossible/inappropriate in a TUI.

## 5. Non-blocking operation + progress model

**Decision**: Reuse the established pattern — the mutation runs in a `tea.Cmd`
(goroutine) under a bumped generation id and a per-op `context.CancelFunc`;
messages `operationStarted` → (optional `operationProgress`) → `operationDone`
carry the generation so superseded/cancelled results are dropped (FR-007). A
spinner/status line renders immediately on `operationStarted` (next render tick,
≤100 ms — SC-004). `x` cancels an in-flight op.

**Rationale**: Identical to the read-load model already proven in 001 (Constitution
II); no new concurrency primitive. Create-folder is a single fast call, but the
framework must generalise to slow ops (uploads) in 003.

**Alternatives considered**:
- *Synchronous call in Update*: rejected — freezes the frame, violates II and SC-004.

## 6. Enabling writes: `--write` flag + `readonly` context field

**Decision**: A global boolean flag `--write` (default false) on `cmd/s3s`. A new
optional `readonly: true` field on a **context** in config. A new
`config.WriteModeFor(name, writeFlag)` method produces a `WriteMode{Writable:
bool}` per active context as `writeFlag && !context.ReadOnly` — the existing
`Resolve`/`ClientConfig` methods are left unchanged (a separate method, not an
overload, so no current caller breaks). `main.go` threads `writeFlag` into the
resolver; the resolver wraps the backend in the read-only guard unless `Writable`.

**Rationale**: Matches the clarified opt-out model (Session 2026-06-05): deliberate
global enablement, with sensitive contexts (prod) protected by `readonly: true`
regardless (FR-001, FR-002). Putting the field on the context (not cluster/user)
lets the same cluster be writable under one context and protected under another.

**Alternatives considered**:
- *`writable: true` opt-in per context*: rejected at clarify (Option B) — more
  friction for the common single-context case.
- *Env var only*: a flag is more discoverable and ephemeral (not persisted), which
  suits a safety gate; env can be added later if needed.

## 7. Logging of mutations

**Decision**: Each mutation logs a structured `slog` record **before** execution
(`event=mutation.start`, action, bucket, key, context) and **after**
(`event=mutation.done`, outcome, error-class). Reuse the file-only handler and
`logging.Secret` so credentials never appear (FR-008, SC-005). Error classes reuse
the existing `storage.classify` sentinels; `ErrReadOnly` is added.

**Rationale**: Directly implements Constitution V ("logged before execution") and
keeps the TUI frame clean. Pre-execution logging means even a crash mid-op leaves a
trace of intent.

**Alternatives considered**:
- *Log only on completion*: rejected — loses the intent record V requires.

## Summary of new/changed surfaces

| Surface | Change |
|---|---|
| `storage.Storage` | unchanged read methods |
| `storage.Mutator` (new) | `CreateFolder(ctx, bucket, prefix) error` |
| `storage` sentinels | `+ ErrReadOnly` |
| `storage.readOnlyGuard` (new) | wraps backend; refuses mutations when read-only |
| `config.Context` | `+ ReadOnly bool` (yaml `readonly`) |
| `config.WriteModeFor` (new) | `+ WriteMode` (existing `Resolve`/`ClientConfig` unchanged) |
| `cmd/s3s` | `+ --write` flag |
| `ui` | confirmation overlay (simple/typed) + operation/progress flow |
| `scripts/check-readonly.sh` | unchanged |
