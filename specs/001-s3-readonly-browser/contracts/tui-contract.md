# Contract: TUI Behavior & Keybindings

**Feature**: 001-s3-readonly-browser | Bubble Tea (v2) interactive terminal UI.

Defines the user-facing interaction contract (the "API" of a TUI app): launch surface, views,
keybindings, and async/state guarantees. Concrete styling is an implementation detail.

## Launch surface (CLI)

```text
s3s [--context <name>] [--config <path>]
```

- `--context` selects the active context (highest precedence; FR-002).
- `--config` overrides the config file path.
- No mutating subcommands exist (read-only; FR-019).

## Views

| View | Purpose | Source |
|------|---------|--------|
| Bucket list | buckets for active context | `ListBuckets` (FR-006) |
| Tree level | dirs (common prefixes) + objects at current prefix, with breadcrumb | `ListLevel` (FR-007/009) |
| Metadata | object details pane | `HeadObject` (FR-013) |
| Preview | text or image content, scrollable | ranged `GetObjectRange` (FR-014/015/016) |
| Search | prefix input narrowing current level | `ListLevel` with Search (FR-017/018) |
| Context switcher | pick among configured contexts | config (FR-002) |

## Keybindings (default)

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | move selection |
| `→`/`l`/`Enter` | enter selected bucket or directory (drill down) |
| `←`/`h`/`Esc` | go back to parent level |
| `g`/`G` | jump to top/bottom of level |
| `/` | start prefix search at current level; `Esc` clears it |
| `i`/`Enter` (on object) | open metadata for the object |
| `p`/`Space` | preview selected object |
| `r` | refresh current level (discard cache, re-fetch) (FR-011a) |
| `c` | open context switcher |
| `x`/`Ctrl+C` | cancel in-flight load / quit |
| `?` | help overlay |

## Async & state guarantees (Constitution II)

- Every backend call runs in a `tea.Cmd`; `Update`/`View` never block (FR-012, SC-007).
- A spinner + "loading" indicator shows during any in-flight load; the load is cancellable
  (`x`/Esc), which calls the stored `context.CancelFunc`.
- Navigating or searching while a load is in flight cancels the superseded load; stale results are
  dropped via a generation id (no flicker/wrong-level data).
- Returning to an already-loaded level reads from the session cache — no re-fetch (FR-011) — until
  `r` forces a refresh.
- Paging: the next page is requested only when the user scrolls to the end of the loaded entries
  (FR-010, SC-003).

## State contract per view

- **Empty**: explicit "empty bucket/prefix" message, not blank (FR edge).
- **Loading**: spinner; UI still responds to navigation/cancel.
- **Error**: clear, non-technical, secret-free message with retry/back; UI stays responsive
  (FR-020). Distinct copy for access-denied vs unreachable vs timeout.
- **No matches** (search): explicit state (FR-018).
- **Truncated preview**: visible "preview truncated at 5 MiB" notice (FR-016).
- **Image preview fallback**: non-capable terminal shows type/size summary instead of a degraded
  render (FR-015).
- **Resize**: layout reflows; current selection/position preserved (FR edge).

## Test contract

- Model-level unit tests (fake Storage) assert: drill-down/back transitions, breadcrumb updates,
  paging-on-scroll triggers exactly one load, search narrows + clears, refresh invalidates cache,
  cancellation drops stale generation, each error class renders its state.
- Preview classification (text/image/binary) and the 5 MiB truncation flag are unit-tested in
  `internal/preview` independent of the TUI.
