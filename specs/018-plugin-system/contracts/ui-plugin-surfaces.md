# Contract: UI Surfaces for Plugins

**Feature**: 018-plugin-system

## Keys & commands

| Binding | Action | Notes |
|---------|--------|-------|
| `P` | open plugin status surface | free key (017 audit: a/A/Y/H/p taken); shift-pair convention like y/Y |
| `:plugins` | command-bar alias for the same surface | |
| `Esc` | leave the surface → previous mode | modeHealth pattern |
| `↑/↓`, `j/k` | row selection inside the surface | shared list navigation |
| `Enter` | reveal full (sanitized) last-error detail for the selected plugin | constitution VI reveal idiom |
| `space` | toggle enable/disable for the selected plugin | persists via `Connector.SetPluginEnabled`; immediate effect |
| `r` | inside the surface: re-invoke the selected plugin's last failed target | convenience retry |

Hint exposure: the footer hints line advertises `P plugins` **only when ≥ 1 plugin is
declared**. Zero-config users never see plugin chrome. Help (`?`) gains a Plugins
section under the same condition.

## Plugin status surface (`modePlugins`, full-screen)

Patterned on the operator health card: bordered full-screen box, height-budgeted so the
footer and hints line stay visible at every supported size (130×24 reference; degrades
by dropping detail columns, never the footer).

```text
┌ Plugins ──────────────────────────────────────────────────────────────┐
│ NAME                    CAPABILITY        SCOPE             STATE     │
│ avito-bucket-discovery  bucket-discovery  prod-rgw,stage    ok 120ms · 2m ago
│ image-storage-meta      object-metadata   prod-rgw images-* failed: timeout · 5m ago
│ old-plugin              bucket-discovery  prod-rgw          disabled  │
│ broken-path             object-metadata   prod-rgw          unavailable: executable not found
└────────────────────────────────────────────────────────────────────────┘
 Enter detail · Space enable/disable · r retry · Esc back
```

State vocabulary (text-distinct, NO_COLOR-safe — color is accent only, per palette
roles): `ok <dur> · <age>` / `failed: <reason> · <age>` / `running` / `disabled` /
`unavailable: <reason>` / `incompatible: contract v<N>`.

Long values (scope list, error reason) are truncated with the existing reveal affordance
(`Enter` shows the full sanitized detail) — no identifier is permanently hidden.

## Bucket list integration (discovery)

- While a discovery invocation is in flight, the existing spinner segment shows
  `discovering…`; the list is already interactive with pinned/listed entries.
- Merged list = `pinned ∪ listed (when available) ∪ discovered`, dedup, sorted — visually
  identical rows (no per-row origin badge; provenance lives in the status surface).
- Discovery failure: transient footer notice
  `discovery failed: <plugin> (<reason>) — P for details`, first failure per plugin per
  session only; pinned/listed content untouched.
- Partial result: notice `discovery: <n> buckets (<d> invalid discarded)` when `d > 0`
  or truncation applied.
- Manual refresh (`r`) re-invokes discovery for the current connection (cache
  invalidation, existing semantics).

## Details pane integration (enrichment)

- A new named group appended after the existing metadata groups, header `From
  <plugin-name>` — provenance always on-screen.
- Group states (reuse the 017 text-distinct conventions):
  - `pending` — invocation in flight (debounced with the pane's selection load,
    gen-guarded);
  - populated — fields in plugin order, each wired into the existing per-field
    reveal/copy machinery;
  - `— (empty)` — valid empty answer, distinct from failure;
  - `failed: <reason>` — sanitized, truncated;
  - group absent entirely — object out of every plugin's scope (no invocation made).
- Truncated values carry an explicit marker and remain fully revealable/copyable
  (constitution VI).
- Multiple matching plugins ⇒ one group per plugin, declaration order.

## Non-negotiable UI invariants (tested)

1. No plugin activity may block input: navigation during `pending`/`discovering` always
   responds; superseded results are dropped by generation.
2. Footer + hints line never scroll off on any plugin surface.
3. Every plugin-supplied string is sanitized before rendering (no control/CSI/OSC
   passthrough).
4. All plugin states distinguishable with NO_COLOR=1 (text, not color, carries state).
5. Zero plugins declared ⇒ zero visible UI change anywhere.
