# Contract: TUI Confirmation & Operation Framework

**Package**: `internal/ui` | **Feature**: 002-write-foundation

Defines the reusable safety gate and operation lifecycle every mutating action
flows through. Create-folder is the only consumer in 002; 003+ delete/upload reuse
it unchanged.

## Confirmation tiers

| Tier | Used by | Confirm | Abort |
|---|---|---|---|
| `ConfirmSimple` | reversible ops (create-folder) | `y` or `Enter` | `n` or `Esc` |
| `ConfirmTyped` | destructive/irreversible ops (future delete/overwrite) | type the exact target identifier, then `Enter` | `Esc`, or `Enter` on a mismatch |

- The op declares its tier; the UI must not let a `ConfirmTyped` op proceed on a
  non-matching entry (SC-003).
- Typed tier displays the expected identifier and echoes the operator's input.

## Operation lifecycle (messages)

```text
intent ─▶ Pending(confirm overlay) ─▶ Confirmed ─▶ [log start] ─▶ Running(tea.Cmd)
                                                                     │
   operationDone{gen,outcome,err} ◀────────────────────────────────┘
```

- `createFolderCmd(gen, bucket, prefix) tea.Cmd` runs the backend call off the loop.
- Messages (carry `gen`): `operationStarted{gen}` → optional `operationProgress{gen}`
  → `operationDone{gen, outcome, err}`. A message whose `gen` ≠ current op gen is
  dropped (FR-007).
- Spinner/status renders on `operationStarted`, within 100 ms / the next render tick
  (SC-004). `x` cancels via the op's `context.CancelFunc`.
- On `operationDone` success: invalidate the affected level cache and refresh so the
  new folder is visible (SC-006). On failure/cancel: surface a one-line error
  (classified, non-leaking), storage untouched (FR-011).

## Rendering

- The confirmation overlay renders inside the existing bordered layout; it does not
  break the height budget (footer + hints stay visible).
- A read-only refusal (`ErrReadOnly`, or attempting a mutation when not writable)
  shows an explanatory status line ("context is read-only — start with --write")
  rather than a silent no-op (FR-003).

## Keybindings (added)

| Key | Context | Action |
|---|---|---|
| (create-folder key, e.g. `+`) | browsing a writable level | start create-folder intent (name prompt → Simple confirm) |
| `y` / `Enter` | Simple confirm overlay | confirm |
| `n` / `Esc` | Simple confirm overlay | abort |
| `Enter` | Typed confirm overlay | submit typed identifier (confirm iff exact match) |
| `Esc` | Typed confirm overlay | abort |
| `x` | operation Running | cancel in-flight op |

The exact create-folder key is a UI detail finalised in tasks; it MUST be inert
(or show the read-only hint) when the context is not writable.

## Logging hook

Before dispatch (state `Confirmed`→`Running`) the UI emits `mutation.start`; on
terminal state it emits `mutation.done` (see data-model). File handler only;
secrets redacted (FR-008, SC-005).

## Test contract (white-box, `package ui`)

Driven with `deliver`/`press`, asserting on `App.View().Content` and model state:

- **Gate**: no path reaches `Running` without `Confirmed` (SC-001).
- **Simple**: start create-folder → overlay shown → `n` aborts (no command issued) →
  re-start → `y` dispatches.
- **Typed** (driven by a test-only `ConfirmTyped` `MutatingOperation` fixture, since
  002 ships no destructive action): mismatched entry on `Enter` aborts with no
  command; exact match dispatches (SC-003).
- **Read-only**: on a non-writable context the create-folder key shows the read-only
  hint and issues no command (FR-003).
- **Non-blocking**: `operationStarted` renders a spinner; a delayed `operationDone`
  from a superseded generation is dropped (FR-006, FR-007).
- **Logging**: a fake/log sink records `mutation.start` before `mutation.done`, with
  no secret present (SC-005).
