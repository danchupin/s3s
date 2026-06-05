# Phase 1 Data Model: Write Foundation & Safety

Entities derived from the spec (Key Entities + FRs). Types are illustrative Go
shapes; field names finalise during implementation.

## WritePolicy

Resolved per active context; decides whether the backend is wrapped in the
read-only guard.

| Field | Type | Notes |
|---|---|---|
| `Writable` | `bool` | `globalWriteFlag && !context.ReadOnly` (FR-001, FR-002) |

- **Derivation**: produced by `config.WritePolicyFor(name, writeFlag)` (a new
  method; the existing `Resolve`/`ClientConfig` are unchanged).
- **Invariant**: `context.ReadOnly == true` ⇒ `Writable == false`, regardless of
  `writeFlag` (read-only always wins, SC-002).
- Lives in core (`internal/config`), UI-agnostic, unit-tested without Bubble Tea.

## Context (config delta)

Existing kubectl-style context gains one optional field.

| Field | Type | Notes |
|---|---|---|
| `Name` | `string` | existing |
| `Cluster` | `string` | existing |
| `User` | `string` | existing |
| `ReadOnly` | `bool` | **new**; yaml `readonly`; default `false`; when `true`, context refuses all mutations even under `--write` |

- **Validation**: `readonly` is optional; absent ⇒ `false`. No other config change;
  `ClientConfig` is untouched (the guard, not the client, enforces read-only).

## MutatingOperation

In-memory UI state for one requested change. For this feature `Kind` has a single
value (`CreateFolder`); the type is shaped to generalise to 003.

| Field | Type | Notes |
|---|---|---|
| `Kind` | enum | `CreateFolder` (only value in 002) |
| `Bucket` | `string` | target bucket |
| `Target` | `string` | prefix/key the operation acts on (e.g. `reports/`) |
| `Context` | `string` | active context name (for logging) |
| `Tier` | enum | `ConfirmSimple` \| `ConfirmTyped` (CreateFolder ⇒ Simple) |
| `Status` | enum | see state machine |
| `Err` | `error` | set on failure; classified sentinel, never raw secret |
| `Gen` | `uint64` | generation id for stale-result dropping (FR-007) |

### Status state machine

```text
Pending ──confirm──▶ Confirmed ──dispatch──▶ Running ──┬─ok──▶ Succeeded
   │                     │                              ├─err─▶ Failed
   └──abort──▶ Cancelled └──abort──▶ Cancelled         └─cancel/x──▶ Cancelled
```

- `Pending`: operation built, confirmation overlay shown, nothing dispatched.
- `Confirmed`: confirmation satisfied (simple yes, or typed match) — about to log +
  dispatch.
- `Running`: `tea.Cmd` in flight; spinner visible (≤100 ms after entry, SC-004);
  cancellable via `x`.
- `Succeeded`/`Failed`/`Cancelled`: terminal; outcome logged; affected level cache
  invalidated only on `Succeeded`.
- **Invariant**: no transition into `Running` without passing `Confirmed`
  (SC-001 — 100% of mutations confirmed).
- **Invariant**: `Failed`, and `Cancelled` *before dispatch* (from `Pending`/
  `Confirmed`), ⇒ storage unchanged (FR-011, SC-007).
- **Invariant**: `Cancelled` *during* `Running` ⇒ **indeterminate** storage outcome
  (the backend call may already have applied); never reported as success, and the
  next refresh reflects ground truth (FR-007).

## Confirmation

Transient gate state bound to a `Pending`/`Confirmed` operation.

| Field | Type | Notes |
|---|---|---|
| `Tier` | enum | `ConfirmSimple` \| `ConfirmTyped` |
| `Expect` | `string` | exact identifier required by the typed tier (e.g. bucket/key); empty for simple |
| `Input` | `string` | operator's typed entry (typed tier only) |
| `Confirmed` | `bool` | simple: `y`/`Enter`; typed: `Input == Expect` |

- **Simple tier**: `y`/`Enter` ⇒ confirmed; `n`/`Esc` ⇒ abort.
- **Typed tier**: confirmed only when `Input == Expect` **byte-for-byte exactly**
  (no whitespace trimming, no case folding); any mismatch on submit ⇒ abort, no
  change (SC-003, FR-005).
- CreateFolder uses **Simple** (reversible). The **Typed** tier ships and is
  unit-tested in 002 but has no UI-triggerable destructive action yet (US3) — it is
  exercised by tests and consumed by 003.

## ErrReadOnly (storage sentinel)

New sentinel alongside `ErrNotFound`/`ErrAccessDenied`/`ErrUnreachable`/`ErrInvalidConfig`.

- Returned by `readOnlyGuard` mutating methods when the backend is read-only.
- Carries no network detail (the call never left the process).
- UI maps it to a clear "context is read-only" message (FR-003), distinct from an
  access-denied from the backend.

## Log records

Two structured `slog` events per mutation (file handler only; secrets redacted).

| Event | When | Fields |
|---|---|---|
| `mutation.start` | before dispatch (after `Confirmed`) | `action`, `bucket`, `key`, `context` |
| `mutation.done` | terminal | `action`, `bucket`, `key`, `context`, `outcome` (`ok`/`failed`/`cancelled`), `error_class` |
