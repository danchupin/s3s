# Implementation Plan: Plugin System for External Capability Providers

**Branch**: `018-plugin-system` | **Date**: 2026-06-11 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/018-plugin-system/spec.md`

## Summary

s3s gains a plugin layer: user-declared external programs that provide data the storage
protocol cannot — (a) **bucket discovery** for connections where listing is denied or
physically impossible (domain-style-only endpoints), and (b) **object metadata
enrichment** from external systems (e.g., image-storage info keyed by an image id encoded
in the object key). Plugins are explicit opt-in config entries, invoked as argv-exec'd
subprocesses speaking a versioned JSON request/response contract over stdin/stdout —
the same trust and execution model as the existing credential `cmd` source. The contract
is channel-agnostic so a future MCP bridge can implement the same capability shapes
without redesign. All invocations run as `tea.Cmd`s under the existing generation guard:
non-blocking, timeout-bounded, failure-isolated (core browsing never degrades). A new
full-screen plugin status surface (`P` / `:plugins`) gives visibility and per-plugin
enable/disable; discovery results merge additively (pinned ∪ listed ∪ discovered);
enrichment appears as an attributed, copyable group in the details pane reusing the
017 grouping/tri-state machinery.

## Technical Context

**Language/Version**: Go 1.25 (toolchain pinned in go.mod)

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lip Gloss v2
(`charm.land/lipgloss/v2`), `github.com/google/shlex` (already a dependency — command-line
splitting), `go.yaml.in/yaml/v3` (config). aws-sdk-go-v2 untouched by this feature.

**Storage**: YAML config file (`plugins:` section + per-connection assignment); no new
persistent storage. Plugin results cached in-memory per session (existing cache semantics:
manual refresh invalidates).

**Testing**: `go test` unit suites (white-box UI tests via `deliver`/`press`; plugin runner
tested against real tiny subprocess fixtures — `/bin/sh` scripts in `t.TempDir()`, no
Docker). Existing MinIO integration suite must stay green (S3 surface unchanged).
`make check-readonly` unaffected (no SDK symbols outside `internal/storage`).

**Target Platform**: macOS / Linux terminals (same as today). Subprocess fixtures in tests
use POSIX `sh` (darwin/linux CI parity with existing suite).

**Project Type**: Single Go module, TUI application; new UI-agnostic core package
`internal/plugin`.

**Performance Goals**: Discovery results visible ≤ 5 s from connection open (SC-001);
enrichment fields ≤ 3 s typical (SC-004); UI input latency unaffected during invocations
(SC-002) — all plugin I/O off the event loop.

**Constraints**: Default per-invocation timeout 5 s (per-plugin configurable). Result caps:
5 000 discovered names / 64 metadata fields / 4 096 bytes per field value, truncation
indicated (FR-010). Secrets never enter a plugin request or a log line (FR-008, SC-006).
All plugin-supplied text sanitized (control/CSI/OSC stripped) before rendering (FR-009).

**Scale/Scope**: O(10) declared plugins per config; one discovery invocation per
connection open/refresh; one enrichment invocation per matching object selection
(debounced, gen-guarded, session-cached).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Verdict | How the design complies |
|-----------|---------|-------------------------|
| I. Core/UI Separation | PASS | New `internal/plugin` package is UI-agnostic (types, exec runner, sanitizer, validation); compiles and tests without Bubble Tea. UI depends on a `plugin.Runner` interface and injects a fake in tests, mirroring `storage.Storage`. Config parsing stays in `internal/config`. |
| II. Non-Blocking TUI | PASS | Every invocation is a `tea.Cmd` carrying the generation it was issued under; results arrive as messages and stale ones are dropped. Timeouts enforced via `context.WithTimeout`; navigation/refresh cancels via the existing `beginLoad` cancel chain. |
| III. Test-First | PASS | RED sets defined in quickstart.md before implementation: contract/envelope tests, runner subprocess tests (success/timeout/garbage/huge/missing), sanitizer tests, config validation tests, UI merge/enrichment/status-surface tests. |
| IV. Integration Testing | PASS | The storage-client contract is untouched — no new S3 calls, existing MinIO suite remains the gate. The plugin runner is exercised against real subprocesses in unit tests (true process boundary, no Docker needed). |
| V. Observability & Safe Ops | PASS | Every invocation logged via `slog` (plugin, capability, target, duration, outcome) — never payload, never identity context (FR-011). No destructive actions added; read-only posture preserved (FR-017). Secrets never passed (FR-008). |
| VI. UI Legibility | PASS | Enrichment fields fully revealable/copyable via the existing per-field reveal/copy paths; truncation always indicated; status states (`ok/failed/timeout/disabled/unavailable`) are text-distinct and NO_COLOR-safe; footer/hints never scrolled off (status surface reuses the modeHealth full-screen pattern with its height budget). |
| VII. UI Consistency | PASS | Status surface reuses the full-screen card pattern, shared label vocabulary (key glyph + verb), palette roles, and the established tri-state text conventions; failure notices use the existing transient `notice` line; no new ad-hoc styling. |

**Technology & Security Constraints check**: "No telemetry or network calls beyond the
configured S3 endpoint" — s3s itself makes **no** new network calls. Plugins are
user-declared local subprocesses; any network activity is the plugin's own, behind an
explicit per-entry opt-in declaration. Execution mirrors the blessed credential `cmd`
source precedent: shlex-split argv, **never** `sh -c`, refused unless the config file is
owner-only-writable (same "attacker edits YAML → command runs at launch" defense as
`internal/secret/command.go`). Verdict: PASS, no amendment needed.

**Post-design re-check (after Phase 1)**: PASS — contracts introduce no UI-embedded logic,
no blocking paths, no secret flow into plugin requests; no Complexity Tracking entries
required.

## Project Structure

### Documentation (this feature)

```text
specs/018-plugin-system/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions D1–D15
├── data-model.md        # Phase 1 output — entities, validation, state machine
├── quickstart.md        # Phase 1 output — RED test sets per user story
├── contracts/
│   ├── plugin-exec-contract.md   # subprocess JSON contract (versioned envelope)
│   ├── config-plugins.md         # YAML schema extension + Connector persistence
│   └── ui-plugin-surfaces.md     # keys, status surface, details group, notices
└── tasks.md             # Phase 2 output (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/
├── plugin/                  # NEW — UI-agnostic core (constitution I)
│   ├── plugin.go            # Capability enum, Declaration, Request/Response envelopes
│   ├── runner.go            # exec runner: shlex argv, stdin/stdout JSON, timeout, caps
│   ├── sanitize.go          # control/CSI/OSC stripping, length caps, bucket-name check
│   ├── plugin_test.go       # envelope encode/decode + version mismatch
│   ├── runner_test.go       # real subprocess fixtures: ok/timeout/garbage/huge/missing
│   └── sanitize_test.go
├── config/
│   ├── config.go            # + Plugins []PluginDecl, validation, defaults
│   └── plugins_test.go      # NEW — parse/validate/scope-match tests
├── ui/
│   ├── app.go               # discovery merge into bucket load; enrichment in details load
│   ├── commands.go          # discoverCmd / enrichCmd (tea.Cmd wrappers, gen-carrying)
│   ├── messages.go          # discoveryDoneMsg / enrichDoneMsg / pluginToggledMsg
│   ├── plugins.go           # NEW — modePlugins status surface (list, toggle, detail)
│   ├── plugins_test.go      # NEW — white-box surface + merge + enrichment tests
│   ├── connections.go       # Connector + SetPluginEnabled persistence
│   ├── keys.go              # + Plugins key ("P"), hints
│   ├── metadata.go          # + attributed enrichment group (017 grouping reuse)
│   └── pane.go              # details-pane variant of the enrichment group
└── secret/                  # unchanged; owner-only check pattern replicated in plugin
cmd/s3s/main.go              # wire config.Plugins → plugin.Runner → ui.App
```

**Structure Decision**: Single-project layout, consistent with every prior feature. The
one new package is `internal/plugin` (core side of the I-boundary); UI changes ride in
`internal/ui` behind the `plugin.Runner` interface. No changes to `internal/storage` —
the read-only guard and the S3 integration suite are untouched.

## Complexity Tracking

No constitution violations — table intentionally empty.
