# Contract: delete connection on the contexts screen (US5)

## Behavior (FR-029..FR-033, SC-010/011)

- The contexts screen (`modeConnections`) exposes a discoverable delete-connection key
  (`^x`) alongside add/switch.
- Delete is a highest-tier dangerous action: chord-gated + a centered/inline **typed
  confirmation requiring the exact connection name** (shares the US4 typed surface).
- On confirm: the connection's config triple (cluster + user + context) is removed and
  persisted, AND its keychain secret is deleted (best-effort — a missing secret does NOT
  block removal). The contexts list updates live (no restart).
- The **active/current** connection cannot be deleted: the attempt is refused with a
  status line telling the operator to switch context first.
- The **last/only** connection IS deletable: with zero contexts remaining, the app falls
  back to its no-connection / add-connection state and does not crash.

## Seam (Constitution I)

- `ui.Connector` gains `Delete(ctx, name) ([]string, error)`; `connSeam.Delete` calls
  `(*config.Config).RemoveConnection(name)`. The UI never imports config/keychain.
- `RemoveConnection`: refuse `name == CurrentContext`; trial-validate the triple-removed
  copy; `secret.RemoveKeychain(name)` best-effort; persist; commit live; return new
  `ContextNames()`. Logged `connection.delete` (non-secret fields only).

## Test checklist

- [ ] contexts screen shows the delete key; selecting a non-active context + `^x` opens
      the typed-name confirm
- [ ] confirm with exact name → triple removed from config, keychain secret removed, list
      refreshed live
- [ ] wrong name typed → abort, nothing removed
- [ ] active context selected + `^x` → refused with "switch first" nudge
- [ ] deleting the last connection → empty-state render, no crash
- [ ] config unit: RemoveConnection drops the triple, refuses CurrentContext, tolerates a
      missing keychain secret
- [ ] Esc cancels with no change
