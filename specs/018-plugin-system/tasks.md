# Tasks: Plugin System for External Capability Providers

**Input**: Design documents from `/specs/018-plugin-system/`

**Prerequisites**: plan.md, spec.md, research.md (D1–D15), data-model.md, contracts/, quickstart.md

**Tests**: INCLUDED — constitution III makes TDD non-negotiable. Every story phase starts
with its RED set (quickstart.md); tests must fail before implementation.

**Organization**: Grouped by user story; each story is an independently testable increment.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1 (discovery), US2 (enrichment), US3 (status surface)

## Path Conventions

Single Go module at repo root: `internal/…`, `cmd/s3s/…`, `docs/…` (per plan.md).

---

## Phase 1: Setup

**Purpose**: Package scaffolding — minimal; no new dependencies (shlex already in go.mod).

- [X] T001 Scaffold `internal/plugin/` package (`doc.go` with package comment describing the capability-provider boundary) and empty `docs/plugins/` directory

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The plugin core (envelope, runner, sanitizer), config section, and the
UI seam every story builds on. RED set 0 first, then implement to green.

**⚠️ CRITICAL**: No user story work until this phase completes.

### Tests (RED set 0 — write first, confirm failing)

- [X] T002 [P] Envelope tests in `internal/plugin/plugin_test.go`: request marshals contractVersion=1 + capability + connection (name/endpoint/userLabel/accessKeyId) + target for object-metadata; marshaled JSON contains NO secret-key field (assert on key set); response decode for buckets/fields/error variants; exactly-one-of payload/error rule
- [X] T003 [P] Runner tests in `internal/plugin/runner_test.go` against real `#!/bin/sh` fixtures in `t.TempDir()`: valid discovery response ⇒ `ok`; sleep past 100ms test timeout ⇒ `timeout` + process killed; nonzero exit ⇒ `exec_error`; garbage stdout ⇒ `invalid_output`; stdout >1MiB ⇒ `invalid_output`; `{"error":…}` ⇒ `contract_error` with reason ≤200 chars; `contractVersion:2` ⇒ `incompatible`; missing executable ⇒ `exec_error` classified for `unavailable`; group/world-writable config path ⇒ refusal, no process spawned; slog capture: every invocation emits exactly one record with plugin/capability/target/duration_ms/outcome and the record contains neither response payload nor argv
- [X] T004 [P] Sanitizer + name-validation tables in `internal/plugin/sanitize_test.go`: CSI/OSC/C0/C1 stripped, UTF-8 survives, newline collapse on single-line surfaces, length cap adds truncation marker; S3 bucket-name rules (3–63, lowercase/digits/dot/hyphen, alnum ends) — invalid rejected and counted
- [X] T005 [P] Config tests in `internal/config/plugins_test.go`: `plugins:` section parses with defaults (timeout 5s, enabled true); validation matrix — duplicate name / unknown capability / empty cmd / timeout ≤ 0 / discovery without connections / metadata without match.connections / non-compiling keyPattern ⇒ load errors; unknown connection name ⇒ warning + `unavailable`, config still loads

### Implementation

- [X] T006 [P] Envelope types + Capability enum + Invocation/outcome types in `internal/plugin/plugin.go` (greens T002)
- [X] T007 [P] `Sanitize`/`ValidBucketName` pure functions in `internal/plugin/sanitize.go` (greens T004)
- [X] T008 Exec runner in `internal/plugin/runner.go`: shlex argv (never `sh -c`), `exec.CommandContext` deadline, one JSON request → stdin, stdout read capped 1MiB, outcome classification per contracts/plugin-exec-contract.md, owner-only config gate (mirror `internal/secret/command.go` defense), one `slog` record per invocation (plugin/capability/target/duration_ms/outcome — never payload, never argv) (greens T003; depends on T006, T007)
- [X] T009 `PluginDecl`/`MatchRule` config parsing, defaults, validation matrix in `internal/config/config.go` (+ `internal/config/plugins.go` if cleaner) (greens T005)
- [X] T010 `plugin.Runner` interface (in `internal/plugin`) + App seam in `internal/ui/app.go`: runner/declarations/status fields, constructor wiring, `fakeRunner` test helper with call counter in `internal/ui/plugins_test.go` (no behavior yet)
- [X] T011 Wire `cmd/s3s/main.go`: parsed `config.Plugins` → real runner (config path for the owner-only gate) → `ui.App`

**Checkpoint**: plugin core green in isolation; app compiles with dormant plugin seam;
zero plugins declared ⇒ zero behavior change.

---

## Phase 3: User Story 1 — Bucket discovery through an external provider (Priority: P1) 🎯 MVP

**Goal**: Connections with assigned discovery plugins show the deduplicated, sorted union
pinned ∪ listed (when available) ∪ discovered — no storage-side listing required; failures
fall back with a succinct notice.

**Independent Test**: Stub discovery provider + listing-denied connection ⇒ names appear
as browsable buckets ≤ 5s; kill the provider ⇒ pinned-only + notice (quickstart RED set 1).

### Tests (write first, confirm failing)

- [X] T012 [P] [US1] Merge tests in `internal/ui/plugins_test.go` (fake Runner): listing denied + discovery ok ⇒ pinned ∪ discovered, dedup, sorted; listing available ⇒ pinned ∪ listed ∪ discovered (additive); disabled plugin never invoked; connection without assigned plugin never invoked
- [X] T013 [P] [US1] Failure-path tests in `internal/ui/plugins_test.go`: provider failure ⇒ pinned/listed intact + transient notice naming plugin and reason with "P for details"; second failure same session ⇒ no repeat notice; invalid names discarded with count in notice; >5000 names truncated with indication
- [X] T014 [P] [US1] Staleness/cache tests in `internal/ui/plugins_test.go`: discovery result carrying gen N dropped at gen N+1; reopening connection hits cache (fake call counter static); refresh `r` re-invokes; context switch clears that context's entries

### Implementation

- [X] T015 [US1] `discoverCmd` (gen-carrying `tea.Cmd`) in `internal/ui/commands.go` + `discoveryDoneMsg` in `internal/ui/messages.go`
- [X] T016 [US1] Discovery integration in `internal/ui/app.go` + `internal/ui/plugins.go`: dispatch discovery legs alongside the bucket load (open/refresh), pure merge+filter function (union, dedup, sort, invalid-name discard count), session cache map keyed (context, plugin), gen guard before cache write (greens T012, T014)
- [X] T017 [US1] Notices + pending UX in `internal/ui/app.go`: `discovering…` spinner segment while in flight (list stays interactive), first-failure-only notice per plugin per session, partial/discarded-count notice (greens T013)
- [X] T018 [P] [US1] Example stub `docs/plugins/discovery-static.sh` (names from a flat file; provisioning-API template comments) + `docs/plugins/README.md` pointing at the exec contract

**Checkpoint**: US1 fully functional — MVP. Validate independently before US2.

---

## Phase 4: User Story 2 — External metadata for objects (Priority: P2)

**Goal**: Objects matching a metadata plugin's scope grow an attributed `From <plugin>`
details group: pending → fields (copyable) / failed / empty — async, gen-guarded, cached.

**Independent Test**: Stub metadata provider + key pattern ⇒ matching object's details
show the group with fields after pending; non-matching object ⇒ no group, no invocation
(quickstart RED set 2).

### Tests (write first, confirm failing)

- [X] T019 [P] [US2] `MatchRule.Matches` tests in `internal/config/plugins_test.go`: connection + bucket-glob + keyPattern combinations; empty buckets/keyPattern ⇒ wildcard; non-match ⇒ false
- [X] T020 [P] [US2] Group lifecycle tests in `internal/ui/plugins_test.go`: `pending` state on selection, populated fields in plugin order under `From <plugin>` header; `failed: <reason>` text-distinct from empty-result state; NO_COLOR pass; non-matching object renders no group and triggers zero invocations (fake counter)
- [X] T021 [P] [US2] Copy/reveal/caps tests in `internal/ui/plugins_test.go`: plugin fields flow through the existing per-field copy path; >4096B value truncated with marker and fully revealable; >64 fields truncated with indication
- [X] T022 [P] [US2] Staleness/cache/multi tests in `internal/ui/plugins_test.go`: selection change before result ⇒ late result dropped; reselect hits cache; two matching plugins ⇒ two groups in declaration order; rapid repeated selection of the same object yields at most one in-flight invocation (fake counter = 1 until the result lands)

### Implementation

- [X] T023 [US2] `MatchRule.Matches(connection, bucket, key)` (globs + compiled RE2) in `internal/config/plugins.go` (greens T019)
- [X] T024 [US2] `enrichCmd` (gen-carrying) in `internal/ui/commands.go` + `enrichDoneMsg` in `internal/ui/messages.go`
- [X] T025 [US2] Details group rendering in `internal/ui/metadata.go` (existing metadata-group renderer) + `internal/ui/pane.go` (details-pane variant) + `internal/ui/plugins.go`: `From <plugin>` group appended after existing groups, state texts (pending / populated / empty / failed), per-field reveal/copy wiring, truncation markers (greens T020, T021)
- [X] T026 [US2] Enrichment dispatch in `internal/ui/app.go`: join the debounced selection load and the full-screen object open, scope matching before invocation, session cache keyed (context, plugin, bucket, key), gen guard, single in-flight invocation per (plugin, target) — repeat triggers coalesce (greens T022)
- [X] T027 [P] [US2] Example stub `docs/plugins/image-storage-meta.sh` (image id from key → image-storage query → fields mapping)

**Checkpoint**: US1 + US2 work independently; browsing without plugins unchanged.

---

## Phase 5: User Story 3 — Plugin visibility and control (Priority: P3)

**Goal**: Full-screen `modePlugins` (`P` / `:plugins`): per-plugin name, capability,
scope, enabled state, last outcome; `space` toggles with persistence; `Enter` reveals
error detail; `r` retries.

**Independent Test**: Two declared plugins (one healthy, one missing executable) ⇒
surface lists both with correct states; toggle stops invocations immediately
(quickstart RED set 3).

### Tests (write first, confirm failing)

- [ ] T028 [P] [US3] Surface lifecycle tests in `internal/ui/plugins_test.go`: `P` and `:plugins` open modePlugins, `Esc` returns to previous mode; zero declared plugins ⇒ `P` no-op and hints line carries no plugin hint; ≥1 declared ⇒ hint present
- [ ] T029 [P] [US3] Rendering tests in `internal/ui/plugins_test.go`: rows show name/capability/scope/state; `ok <dur> · <age>` / `failed: <reason> · <age>` / `running` / `disabled` / `unavailable: <reason>` / `incompatible: contract v<N>` text-distinct under NO_COLOR; `Enter` reveals full sanitized error; footer + hints visible at 130×24 and the height floor
- [ ] T030 [P] [US3] Toggle/retry tests in `internal/ui/plugins_test.go`: `space` calls fake Connector `SetPluginEnabled`, optimistic flip, Connector error ⇒ revert + notice; disabled plugin skipped by the very next load; `r` re-invokes the selected plugin's last failed target; a plugin that returned `incompatible` is skipped by all subsequent loads in the session (fake counter static after the mismatch)
- [ ] T031 [P] [US3] Config mutation tests in `internal/config/plugins_test.go`: `SetPluginEnabled` flips the flag with atomic write (temp+rename), idempotent, unknown plugin name ⇒ error, other declarations untouched

### Implementation

- [ ] T032 [US3] `Config.SetPluginEnabled(name, enabled)` in `internal/config/plugins.go` following `AppendBucket`'s atomic-write pattern in `internal/config/connection.go` (greens T031)
- [ ] T033 [US3] Extend `Connector` interface in `internal/ui/connections.go` + implement `connSeam.SetPluginEnabled` in `cmd/s3s/connection.go`
- [ ] T034 [US3] Keys & entry points: `Plugins` binding `"P"` in `internal/ui/keys.go`, conditional `P plugins` hint in `internal/ui/hintbar.go`, `:plugins` case in `dispatchCommand` in `internal/ui/command.go`, help (`?`) Plugins section
- [ ] T035 [US3] `modePlugins` surface in `internal/ui/plugins.go` + mode wiring in `internal/ui/app.go`: row list with selection, state vocabulary, `Enter` detail reveal, `space` toggle via `tea.Cmd` persistence + `pluginToggledMsg` in `internal/ui/messages.go`, `r` retry, height budget keeping footer/hints visible (greens T028–T030)

**Checkpoint**: All three stories independently functional.

---

## Phase 6: Polish & Cross-Cutting

- [ ] T036 [P] README.md: Plugins section (declaring, contract pointer, example stubs, security model: argv exec / owner-only config / no secrets)
- [ ] T037 Gates: `make fmt vet lint check-readonly` + `make test` fully green; confirm `internal/plugin` imports no S3 SDK symbols
- [ ] T038 `make test-integration` (MinIO suite) — must stay green, unchanged by this feature
- [ ] T039 Manual validation per quickstart.md: stub discovery on a `pathStyle: false` listing-denied connection (≤5s), enrichment flow with copy, NO_COLOR pass, 130×24 + narrow footer check, `chmod 666` config refusal

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: after Setup. Blocks all stories. Within: T002–T005 [P] first (RED), then T006/T007 [P] → T008 → T009 (independent of T008) → T010 → T011.
- **US1 (Phase 3)**: after Phase 2. T012–T014 [P] (RED) → T015 → T016 → T017; T018 [P] anytime.
- **US2 (Phase 4)**: after Phase 2; independent of US1 (shares only foundation). T019–T022 [P] (RED) → T023 → T024 → T025 → T026; T027 [P] anytime.
- **US3 (Phase 5)**: after Phase 2; status content is richer once US1/US2 produce invocations, but the surface itself is independently testable with fake statuses. T028–T031 [P] (RED) → T032 → T033 → T034 → T035.
- **Polish (Phase 6)**: after desired stories. T036 [P] anytime post-foundation.

### Story Dependencies

- US1, US2, US3 all depend only on Phase 2 — mutually independent, parallelizable.
- Single-developer order: priority order US1 → US2 → US3.

### Within Each Story

RED tests first and failing → implementation to green → checkpoint validation. Tasks
touching `internal/ui/app.go` (T016/T017, T026, T035) serialize within and across
stories — same file.

## Parallel Example: Foundational RED set

```bash
# Four independent test files, one task each:
go test ./internal/plugin/ -run TestEnvelope    # T002
go test ./internal/plugin/ -run TestRunner      # T003
go test ./internal/plugin/ -run TestSanitize    # T004
go test ./internal/config/ -run TestPlugins     # T005
```

## Parallel Example: User Story 1

```bash
# T012, T013, T014 — same file, different test funcs: write together in one pass,
# or split by function; T018 (docs stub) fully parallel with everything.
```

## Implementation Strategy

**MVP = Phase 1 + Phase 2 + Phase 3 (US1)** — solves the original domain-style
discovery problem end-to-end. STOP, validate independently (quickstart manual set 1),
demo. Then US2 (enrichment) → validate → US3 (surface) → validate → Polish. Each story
lands without breaking the previous; zero plugins declared stays a zero-change
experience at every checkpoint.

## Notes

- Constitution III: confirm each RED set fails before its implementation tasks.
- Comment hygiene (constitution, Development Workflow): no spec-kit identifiers in code
  comments — state constraints in plain language.
- Commit after each task or logical group; checkpoints are demo-able states.
