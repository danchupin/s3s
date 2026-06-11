# Research: Plugin System for External Capability Providers

**Feature**: 018-plugin-system | **Date**: 2026-06-11

All Technical Context unknowns resolved. Decisions numbered D1–D15.

## D1 — Invocation channel: argv-exec subprocess, JSON over stdin/stdout

**Decision**: A plugin is an external executable declared in config. s3s splits the
declared command line with POSIX shell-words rules (`github.com/google/shlex`, already a
dependency), executes it as argv via `exec.CommandContext` — **never** `sh -c` — writes
one JSON request to stdin, reads one JSON response from stdout, and applies a deadline.
Before any plugin runs, the config file must pass the owner-only-writable check, exactly
like the credential `cmd` source (`internal/secret/command.go`): group/world-writable
config refuses to execute declared commands.

**Rationale**: This is the established, reviewed trust model in the codebase (014
credential `cmd` source). One process per invocation gives failure isolation for free
(crash = exit code, hang = deadline kill), needs no lifecycle management, and works for
any language the user writes plugins in. stdin/stdout JSON is trivially testable with
`/bin/sh` fixtures and keeps the contract transport-visible.

**Alternatives considered**:
- **Go `plugin` package / shared objects** — rejected: platform-fragile (no Windows,
  brittle on macOS), locks plugin authors to the exact Go toolchain, crashes take the
  host down (violates failure isolation, FR-007).
- **Long-running sidecar with JSON-RPC** — rejected for v1: process lifecycle,
  restart/backoff and handshake state machines add complexity with no v1 payoff at
  O(few) invocations per user action. The envelope (D2) doesn't preclude it later.
- **HTTP endpoint plugins** — rejected for v1: s3s would gain arbitrary outbound network
  calls, colliding with the "no network calls beyond the configured S3 endpoint"
  constraint. A subprocess making its own calls keeps that constraint intact for s3s
  itself. The channel-agnostic envelope keeps this door open.

## D2 — Contract envelope: versioned, capability-tagged, channel-agnostic

**Decision**: One request/response envelope for all capabilities (full schema in
`contracts/plugin-exec-contract.md`):

```json
// request (stdin)
{"contractVersion": 1, "capability": "bucket-discovery", "connection": {...}, "target": {...}}
// response (stdout)
{"contractVersion": 1, "buckets": [...]}            // bucket-discovery
{"contractVersion": 1, "fields": [{"name": "...", "value": "..."}]}  // object-metadata
{"contractVersion": 1, "error": "human-readable reason"}             // soft failure
```

`contractVersion` is an integer, currently `1`. A response declaring a different major
version, or unparsable output, is a contract failure: the invocation fails and the plugin
is marked incompatible/disabled in status (FR-015). Request and response shapes contain
nothing exec-specific — the same JSON objects can ride any future channel (MCP bridge
maps a tool call to the same envelope), satisfying FR-016.

**Rationale**: A single versioned envelope is the smallest thing that survives contract
evolution; capability tagging keeps one runner code path.

**Alternatives considered**: per-capability ad-hoc formats (rejected: N parsers, no
shared version gate); semver strings (rejected: integer compare suffices for v1).

## D3 — v1 capabilities: `bucket-discovery`, `object-metadata`

**Decision**: Exactly two capability values (clarified in spec session 2026-06-11).
Actions and MCP are future capabilities/channels on the same envelope.

**Rationale**: Both motivating tasks covered; smallest coherent contract.

## D4 — Config schema: top-level `plugins:` list with scope rules

**Decision**: New top-level config section (full schema in `contracts/config-plugins.md`):

```yaml
plugins:
  - name: avito-bucket-discovery
    capability: bucket-discovery
    cmd: "s3s-avito-discovery --cluster prod"
    timeout: 5s            # optional, default 5s
    enabled: true          # optional, default true
    connections: [prod-rgw]            # discovery scope: connection names
  - name: image-storage-meta
    capability: object-metadata
    cmd: "s3s-image-meta"
    match:                              # enrichment scope
      connections: [prod-rgw]
      buckets: ["images-*"]             # glob, optional
      keyPattern: "^[0-9a-f]{32}"       # RE2 regex, optional
```

Validation at load: unique plugin names; known capability; non-empty `cmd`; discovery
entries require `connections`, metadata entries require `match` with at least
`connections`; `keyPattern` must compile; unknown connection names are a load-time
warning surfaced in plugin status (`unavailable`), not a fatal error.

**Rationale**: Mirrors existing config idioms — `cmd` string like the credential source
(shlex rules documented there), glob matching like pinned-bucket conventions, explicit
per-connection assignment like context wiring. Warning-not-error for unknown connections
keeps one shared config usable across machines.

**Alternatives considered**: per-connection nested plugin blocks (rejected: duplicates
declarations when one plugin serves several connections); auto-discovery of executables
on PATH (rejected outright: violates explicit opt-in, FR-001).

## D5 — Timeout and caps defaults

**Decision**: Default per-invocation timeout **5 s** (per-plugin `timeout:` override).
Caps: **5 000** discovered names per invocation, **64** metadata fields, **4 096** bytes
per field value, **200** chars of sanitized stderr/error retained for status display.
Exceeding a cap truncates with an explicit `… (+N more / truncated)` indicator (FR-010).
Output read is hard-capped at **1 MiB** — larger stdout is a contract failure (defends
against runaway producers before JSON parse).

**Rationale**: 5 s aligns with SC-001 ("buckets within 5 s"); the credential cmd source
uses 10 s but is launch-time-only — interactive paths want tighter. 1 MiB covers any
sane discovery list (5 000 names × ~64 B ≪ 1 MiB) while bounding memory.

**Alternatives considered**: unbounded-with-spinner (rejected: violates FR-006/SC-002);
global-only timeout (rejected: slow corporate discovery endpoints are real, per-plugin
override is cheap).

## D6 — Discovery merge: additive union in the bucket load path

**Decision**: `loadBuckets` grows a discovery leg: for a connection with assigned,
enabled discovery plugins, the bucket-list load dispatches the storage listing (or pinned
synthesis — existing behavior) **and** one `tea.Cmd` per discovery plugin in the same
generation. The final list is the deduplicated, sorted union: pinned ∪ listed (when
listing succeeds) ∪ discovered (clarified 2026-06-11: always additive). Discovered names
failing S3 bucket-name validation are discarded and counted in the failure/partial
notice (FR-019). Discovery failure leaves the pinned/listed result intact and posts a
transient footer notice (`m.notice` mechanism) naming the plugin and reason. The
"+ add bucket" affordance keeps its existing scoped-connection rule — discovery results
do not change `bucketsScoped` semantics.

**Rationale**: Union-of-sources is the rule that answers the original problem (listing
shows owned, provider shows granted-not-owned) with one mental model; riding the existing
generation guard means superseded discoveries can never corrupt a newer view.

**Alternatives considered**: replace-listing and fallback-only modes (rejected in
clarification Q3 — both lose buckets in common cases).

## D7 — Enrichment: joins the debounced details load, renders as an attributed group

**Decision**: When the details pane (or full-screen object view) loads metadata for a
selected object that matches an enabled metadata plugin's scope, the same debounced,
gen-guarded load dispatches an enrichment `tea.Cmd`. The result renders as a new named
group in the details pane — `From <plugin-name>` — after the existing groups,
using the 017 grouping and text-distinct state conventions: `pending` while in flight,
fields when populated, `failed: <reason>` on error (distinct from an empty result), and
nothing at all for non-matching objects (no invocation, FR-003). Fields plug into the
existing per-field reveal/copy machinery unchanged. Session cache keyed
`(context, plugin, bucket, key)`; manual refresh re-invokes; late results for a
superseded selection are dropped by the generation check.

**Rationale**: Reuses every relevant 017 mechanism (groups, tri-states, per-field copy,
debounced pane load) — zero new UI idioms, which is exactly what constitution VII wants.

**Alternatives considered**: separate key to fetch enrichment on demand (rejected as the
default: hidden data nobody discovers; automatic-with-pending matches US2 acceptance and
SC-004. The cache plus scope matching keeps invocation volume tiny).

## D8 — Plugin status surface: full-screen `modePlugins` on `P` / `:plugins`

**Decision**: New full-screen mode patterned on the 017 health card (`modeHealth`):
opened with `P` or `:plugins`, `Esc` returns to the previous mode. One row per declared
plugin: name, capability, scope summary, enabled state, and last invocation outcome
(`ok 120ms · 2m ago` / `failed: timeout · 5m ago` / `disabled` / `unavailable:
executable not found` / `incompatible: contract v2`). `Enter` reveals the full sanitized
last-error detail; `space` toggles enable/disable. All states are text-distinct
(NO_COLOR-safe). The footer hints line advertises `P plugins` only when at least one
plugin is declared — zero-config users never see the feature (discoverability without
noise; the user's UX directive).

**Rationale**: `P` is free in the keymap (017 audit: a/A/Y/H/p taken; P is not), reads as
"Plugins", and the yank-family precedent (`y`/`Y`) already establishes shift-pairing.
Full-screen card reuses an existing component family rather than inventing a popup.

**Alternatives considered**: plugins-as-section inside connections manager (rejected:
plugins scope across connections; status is operational, not connection-edit, data);
footer-only status (rejected: fails US3 acceptance — no room for per-plugin detail).

## D9 — Enable/disable persistence: `Connector.SetPluginEnabled`

**Decision**: The in-surface toggle takes effect immediately on the model and persists
by extending the `Connector` port with `SetPluginEnabled(ctx, name string, enabled bool)
([]PluginInfo, error)` — a config-file mutation executed off the event loop, mirroring
`AddBucket` (010) exactly: optimistic UI update on success message, error notice on
failure.

**Rationale**: `AddBucket` is the reviewed precedent for "UI mutates config"; same
shape, same testing approach (fake Connector).

**Alternatives considered**: session-only toggle (rejected: FR-018 says without deleting
the declaration — a toggle that silently resets next launch surprises users).

## D10 — Sanitization and validation: pure functions in `internal/plugin`

**Decision**: Every plugin-supplied string passes `Sanitize` before storage in the model:
strips C0/C1 control characters (except keeping nothing — newlines become spaces in
single-line surfaces), CSI/OSC/escape sequences, and enforces the relevant length cap
with explicit truncation marker. Discovered names additionally pass an S3 bucket-name
validity check (3–63 chars, lowercase letters/digits/dot/hyphen, alnum at both ends).
Both are pure, table-driven-tested functions.

**Rationale**: FR-009 — a plugin must never be able to inject terminal control flow into
the TUI; Bubble Tea renders cells, but raw ESC sequences in strings would pass through
to the terminal. The existing preview pipeline already strips for object content; plugin
strings need the same discipline at the trust boundary.

**Alternatives considered**: trusting lipgloss to neutralize (rejected: it doesn't strip
embedded ESC from arbitrary strings); allowlisting printable ASCII only (rejected:
legitimate UTF-8 metadata values — image titles etc. — must survive).

## D11 — Caching: session map in the model, refresh-invalidated

**Decision**: Plugin results cache in App-owned maps keyed `(contextName, pluginName)`
for discovery and `(contextName, pluginName, bucket, key)` for enrichment — same
lifetime/invalidations as `internal/cache` levels: live until manual refresh (`r`)
or context switch; no TTL. Not added to `internal/cache` itself (that package is
level-list-specific); a small dedicated map keeps it obvious.

**Rationale**: FR-014 mandates existing cache semantics; piggybacking the level cache
would force its key shape to grow unrelated dimensions.

## D12 — Logging: invocation facts only

**Decision**: One `slog` record per invocation: `plugin`, `capability`, `target`
(connection name; bucket/key for enrichment), `duration_ms`, `outcome`
(`ok|timeout|exec_error|contract_error|invalid_output|disabled`), plus discarded/invalid
counts for discovery. Never the request context, never response payload, never argv
(may embed user-specific tokens in flags).

**Rationale**: FR-011 + constitution V; argv exclusion closes an easy secret-leak hole
(users will inevitably put tokens in plugin flags despite advice).

## D13 — Failure UX: transient notice + status surface as the detail sink

**Decision**: Plugin failures surface in two layers: (1) a transient, non-modal footer
notice via the existing `m.notice` line — e.g. `discovery failed: avito-bucket-discovery
(timeout) — P for details` — cleared on next keypress like every notice today; (2) the
full sanitized reason retained in plugin status (D8). Repeat failures of the same plugin
in one session do not re-post the notice (first-failure-only) to avoid nagging.

**Rationale**: US1 acceptance demands succinct non-blocking failure visibility; the
status surface satisfies the "why" without modal interruptions. First-failure-only is
the convenience answer to notice fatigue (user's UX directive).

## D14 — Discoverability & convenience (user UX directive)

**Decision**: The convenience budget goes to: (a) conditional `P plugins` hint in the
footer hints line when plugins are declared; (b) pending indicator reusing the existing
spinner segment while discovery is in flight (list already usable with pinned/listed
entries); (c) `:plugins` command-bar alias; (d) help screen (`?`) gains a Plugins
section listing keys and current declarations count; (e) failure notices name the plugin
and point at `P`; (f) enrichment group header carries the plugin name so provenance is
always on-screen; (g) example plugin scripts shipped in `docs/plugins/` (discovery stub +
image-storage metadata stub) so a working setup is copy-paste away.

**Rationale**: Spec's US3 + the explicit planning input "удобный ui/ux". Everything
reuses existing surfaces — no new idioms to learn.

## D15 — Test strategy

**Decision**:
- **`internal/plugin` units**: runner against real subprocess fixtures written to
  `t.TempDir()` as `#!/bin/sh` scripts — success, soft `error` response, nonzero exit,
  sleep-past-timeout, garbage stdout, >1 MiB stdout, missing executable, version
  mismatch. Sanitizer and name-validation table tests.
- **`internal/config` units**: parse/validate the `plugins:` section — defaults, scope
  rules, duplicate names, bad regex, unknown capability.
- **`internal/ui` white-box units**: fake `plugin.Runner` injected; merge math
  (pinned ∪ listed ∪ discovered, dedup, invalid-name discard + notice), enrichment group
  states (pending → populated / failed; non-matching object never invokes), status
  surface rendering and toggle round-trip (fake Connector), gen-guard staleness drops,
  NO_COLOR text-distinct states.
- **Integration**: existing MinIO suite untouched and must stay green; no new
  Docker-dependent tests (the process boundary is already real in unit fixtures).
- **Guards**: `make check-readonly` unaffected — `internal/plugin` imports no S3 SDK
  symbols; lint/vet/fmt as usual.

**Rationale**: Constitution III/IV with the cheapest honest boundaries: real subprocesses
(no mock drift) without Docker; UI logic tested through the interface seam like
`storage.Fake` today.
