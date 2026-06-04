---

description: "Task list for 001-s3-readonly-browser"
---

# Tasks: Read-Only S3 Browser (TUI)

**Input**: Design documents from `specs/001-s3-readonly-browser/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: REQUIRED — Constitution v1.0.0 Principle III (Test-First, NON-NEGOTIABLE). Every story
writes failing tests before implementation (Red → Green → Refactor). Integration tests run against a
real MinIO (Principle IV).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1–US5 maps to spec.md user stories
- Module path assumed `github.com/dochupin/s3s` (set at T001)

## Path Conventions

Single Go project. Source under `cmd/` and `internal/`; tests co-located as `*_test.go`;
integration tests build-tagged `//go:build integration`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and tooling.

- [ ] T001 Initialize module and directory tree: `go mod init github.com/dochupin/s3s`; create `cmd/s3s/`, `internal/{config,storage,preview,cache,ui,logging}/` per plan.md
- [ ] T002 Add dependencies in go.mod and run `go mod tidy`: aws-sdk-go-v2 (+config,+credentials,+service/s3), charmbracelet/bubbletea/v2 (+bubbles/v2,+lipgloss/v2), eliukblau/pixterm, blacktop/go-termimg, go.yaml.in/yaml/v3, testcontainers-go/modules/minio
- [ ] T003 [P] Add `.golangci.yml`, `Makefile` (fmt/vet/lint/test/test-integration targets), and `.gitignore` (binaries, `.env`, local config) at repo root
- [ ] T004 [P] Add CI read-only guard script `scripts/check-readonly.sh` that fails if `Put|Delete|Create|Copy` S3 symbols appear outside `internal/storage/` (Constitution V, FR-019)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core contracts/infra all stories depend on.

**⚠️ CRITICAL**: No user story work begins until this phase is complete.

- [ ] T005 Define read-only storage interface, DTOs, and error sentinels in `internal/storage/storage.go` (`Storage`, `Bucket`, `LevelQuery`, `Page`, `ObjectRef`, `ObjectMetadata`, `ErrNotFound/ErrAccessDenied/ErrUnreachable/ErrInvalidConfig`) per contracts/storage-interface.md — NO mutating methods
- [ ] T006 [P] Write failing tests for in-memory fake in `internal/storage/fake_test.go` (seed buckets/keys; assert delimiter tree shaping, pagination boundary, search narrowing, error mapping)
- [ ] T007 [P] Write failing tests for secret redaction in `internal/logging/secret_test.go` (a `Secret` string type redacts in `String()`/`%v`/`%s`)
- [ ] T008 [P] Write failing tests for slog file logger in `internal/logging/log_test.go` (JSON handler writes to file, never stdout/stderr)
- [ ] T009 Implement in-memory fake `internal/storage/fake.go` to pass T006
- [ ] T010 [P] Implement `internal/logging/secret.go` (redacting Secret type) to pass T007
- [ ] T011 [P] Implement `internal/logging/log.go` (slog JSON → file, level config) to pass T008

**Checkpoint**: Storage interface + fake + logging/redaction ready — stories can begin.

---

## Phase 3: User Story 1 - Connect to a cluster and list buckets (Priority: P1) 🎯 MVP

**Goal**: Configure clusters as kubectl-style contexts, select one, see its buckets in the TUI.

**Independent Test**: With one valid context, launch → bucket list renders; switch context → list
changes (spec US1, SC-001/SC-005).

### Tests for User Story 1 (write first, ensure FAIL) ⚠️

- [ ] T012 [P] [US1] Failing tests for config load/resolve in `internal/config/config_test.go` (valid config; dangling cluster/user ref rejected; anonymous user accepted, non-anonymous missing keys rejected; `${ENV}` resolution + env-over-inline precedence; secret redacted) per contracts/config-schema.md
- [ ] T013 [P] [US1] Failing tests for active-context precedence in `internal/config/resolve_test.go` (flag > `S3S_CONTEXT` env > `current-context`) (FR-002)
- [ ] T014 [P] [US1] Failing UI model test in `internal/ui/app_test.go` (with fake Storage: buckets render; context switcher changes active context; loading + error states) (FR-006, SC-005)
- [ ] T015 [P] [US1] Failing integration test in `internal/storage/s3client_integration_test.go` (`//go:build integration`, testcontainers MinIO): client construction for path-style + static creds + anonymous; `ListBuckets` returns seeded bucket (Constitution IV)

### Implementation for User Story 1

- [ ] T016 [P] [US1] Implement config structs + YAML loader in `internal/config/config.go` (clusters/users/contexts/current-context, `${ENV}` + precedence, validation) to pass T012
- [ ] T017 [US1] Implement active-context resolver + CLI flag/env precedence in `internal/config/resolve.go` to pass T013 (depends on T016)
- [ ] T018 [US1] Implement aws-sdk-go-v2 client construction + `ListBuckets` in `internal/storage/s3client.go` (`BaseEndpoint`, `UsePathStyle`, `AnonymousCredentials`/`NewStaticCredentialsProvider`, TLS skip opt-in, error classification) to pass T015
- [ ] T019 [P] [US1] Implement tea.Cmd wrappers + message types in `internal/ui/commands.go` and `internal/ui/messages.go` (async `ListBuckets`, errMsg)
- [ ] T020 [US1] Implement root model + bucket-list view + context switcher + keymap in `internal/ui/app.go`, `internal/ui/keys.go` to pass T014 (depends on T019)
- [ ] T021 [US1] Implement entrypoint wiring in `cmd/s3s/main.go` (parse `--context`/`--config`, load config, build Storage, start `tea.Program`, init logging)

**Checkpoint**: Launch → buckets listed; context switch works. MVP demoable.

---

## Phase 4: User Story 2 - Navigate the object tree by delimiter (Priority: P1)

**Goal**: Walk the key namespace as a tree (common prefixes = dirs), drill in/out by keyboard, with
on-demand paging and session cache + manual refresh.

**Independent Test**: Open a bucket with nested keys → only first level shown; drill in → children
load; back → parent restored; scroll to end → next page fetched once (spec US2, SC-002/SC-003).

### Tests for User Story 2 (write first, ensure FAIL) ⚠️

- [ ] T022 [P] [US2] Failing unit tests for level cache in `internal/cache/cache_test.go` (session cache by `(context,bucket,prefix,search)`; no TTL; manual invalidate forces miss) (FR-011/FR-011a)
- [ ] T023 [P] [US2] Failing UI tests in `internal/ui/tree_test.go` (drill-down/back transitions; breadcrumb updates incl. root; paging-on-scroll triggers exactly one load; cache hit on return; refresh invalidates; cancellation drops stale generation) (FR-007..012)
- [ ] T024 [P] [US2] Failing integration test in `internal/storage/s3client_integration_test.go` (`ListLevel` with delimiter `/`: common-prefixes vs objects; pagination across >1000 seeded keys; empty prefix) (Constitution IV, FR-010)

### Implementation for User Story 2

- [ ] T025 [P] [US2] Implement session level cache in `internal/cache/cache.go` to pass T022
- [ ] T026 [US2] Implement `ListLevel` (ListObjectsV2 with Delimiter/Prefix/ContinuationToken, CommonPrefixes mapping) in `internal/storage/s3client.go` to pass T024
- [ ] T027 [US2] Add async `loadLevel` tea.Cmd with `context.CancelFunc` + generation-id, and refresh command, in `internal/ui/commands.go` (depends on T026)
- [ ] T028 [US2] Implement tree view: navigation, breadcrumb, paging-on-scroll, cache integration, refresh (`r`), cancel in `internal/ui/tree.go` to pass T023 (depends on T025, T027)

**Checkpoint**: Bucket + tree navigation form the full read-only browser MVP.

---

## Phase 5: User Story 3 - Inspect object metadata (Priority: P2)

**Goal**: Show size, last-modified, content type, storage class, ETag, user metadata for an object.

**Independent Test**: Highlight an object → details match backend (spec US3, FR-013).

### Tests for User Story 3 (write first, ensure FAIL) ⚠️

- [ ] T029 [P] [US3] Failing integration test in `internal/storage/s3client_integration_test.go` (`HeadObject` returns size/type/last-modified/etag/storage-class/user-metadata; access-denied path)
- [ ] T030 [P] [US3] Failing UI test in `internal/ui/metadata_test.go` (metadata pane renders fields; access-denied state)

### Implementation for User Story 3

- [ ] T031 [US3] Implement `HeadObject` mapping in `internal/storage/s3client.go` to pass T029
- [ ] T032 [US3] Add `loadMetadata` tea.Cmd in `internal/ui/commands.go` and metadata pane in `internal/ui/metadata.go` to pass T030 (depends on T031)

**Checkpoint**: Metadata view works on top of navigation.

---

## Phase 6: User Story 4 - Search within the current level by prefix (Priority: P2)

**Goal**: Server-side prefix narrowing of the current level; clearable; explicit no-match state.

**Independent Test**: In a large level, type a prefix → only matching entries (incl. not-yet-loaded)
appear; clear → full level restored (spec US4, FR-017/018).

### Tests for User Story 4 (write first, ensure FAIL) ⚠️

- [ ] T033 [P] [US4] Failing integration test in `internal/storage/s3client_integration_test.go` (`ListLevel` with `Search` set: effective prefix = level prefix + term, complete server-side results, no-match returns empty)
- [ ] T034 [P] [US4] Failing UI test in `internal/ui/search_test.go` (search input narrows level; ~300 ms debounce coalesces keystrokes into ≤1 in-flight request; clear restores; no-match state; search re-uses the cancellation/generation path) (FR-017a)

### Implementation for User Story 4

- [ ] T035 [US4] Wire `Search` into the level query path in `internal/storage/s3client.go` and cache key to pass T033
- [ ] T036 [US4] Implement search input + ~300 ms debounce + narrow/clear/no-match + cancellation dispatch in `internal/ui/search.go` (and integrate into tree view) to pass T034 (depends on T035) (FR-017a)

**Checkpoint**: Large levels are searchable.

---

## Phase 7: User Story 5 - Preview object content (Priority: P3)

**Goal**: Inline preview — text scrollable; images visual (half-block default, protocol-enhanced);
bounded to first 5 MiB with truncation notice; safe fallback for binary/non-capable terminals.

**Independent Test**: Preview small text → readable; small image → visual or clean fallback;
>5 MiB → truncated notice (spec US5, FR-014/015/016).

### Tests for User Story 5 (write first, ensure FAIL) ⚠️

- [ ] T037 [P] [US5] Failing unit tests in `internal/preview/preview_test.go` (content classification text/image/binary; 5 MiB truncation flag; binary safe-summary)
- [ ] T038 [P] [US5] Failing unit tests in `internal/preview/image_test.go` (half-block render of a small image; terminal-capability detection from env: kitty/iTerm2/sixel → protocol, else half-block fallback)
- [ ] T039 [P] [US5] Failing integration test in `internal/storage/s3client_integration_test.go` (`GetObjectRange` returns at most first 5 MiB; small object returns full bytes)
- [ ] T040 [P] [US5] Failing UI test in `internal/ui/preview_view_test.go` (text pane scrollable; truncated notice; image fallback summary)

### Implementation for User Story 5

- [ ] T041 [US5] Implement `GetObjectRange` (ranged GET, bounded reader) in `internal/storage/s3client.go` to pass T039
- [ ] T042 [P] [US5] Implement text + binary preview classification in `internal/preview/text.go` to pass T037
- [ ] T043 [P] [US5] Implement image preview (pixterm half-block default; go-termimg protocol path + env detection) in `internal/preview/image.go` to pass T038
- [ ] T044 [US5] Add `loadPreview` tea.Cmd and preview pane in `internal/ui/commands.go`, `internal/ui/preview_view.go` to pass T040 (depends on T041, T042, T043)

**Checkpoint**: All five stories independently functional.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Hardening and validation across stories.

- [ ] T045 [P] Add terminal-resize reflow + selection-preservation handling in `internal/ui/app.go` (Edge Case)
- [ ] T046 [P] Add non-UTF-8 / unusual key-name safe rendering in `internal/ui/tree.go` (Edge Case)
- [ ] T047 [P] Add help overlay (`?`) listing keybindings in `internal/ui/keys.go` (contracts/tui-contract.md)
- [ ] T048 Verify structured logging covers all operations/errors with secrets excluded across `internal/storage` and `internal/ui` (FR-021)
- [ ] T049 [P] Write `README.md` from quickstart.md (config, run, keybindings, tests)
- [ ] T050 Run `make fmt vet lint` and `go test ./...` + `go test -tags=integration ./...`; fix gaps
- [ ] T051 Run `scripts/check-readonly.sh` and capture a backend access log of a full session to confirm zero write requests (SC-009)
- [ ] T052 Validate quickstart.md end-to-end against a live MinIO; confirm SC-001..SC-007 latency targets

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (P1)**: no deps.
- **Foundational (P2)**: depends on Setup — BLOCKS all stories.
- **US1 (P3)**: depends on Foundational. MVP start.
- **US2 (P4)**: depends on Foundational; independently testable. (Pairs with US1 for full MVP.)
- **US3 (P5)**, **US4 (P6)**: depend on Foundational; build naturally on US2 navigation but are
  independently testable with the fake.
- **US5 (P7)**: depends on Foundational; independently testable.
- **Polish (P8)**: after desired stories complete.

### User Story Dependencies

- US1, US2 — both P1, independent (each testable via fake Storage). US1 is the single-slice MVP.
- US3, US4, US5 — independent of each other; reuse the storage interface and UI shell.

### Within Each User Story

- Tests written and FAILING before implementation (Constitution III).
- Storage method → tea.Cmd → UI view. Cache/config before the views that use them.

### Parallel Opportunities

- Setup: T003, T004 parallel.
- Foundational: T006/T007/T008 (tests) parallel; then T009/T010/T011 (T010/T011 parallel).
- Each story's test tasks `[P]` run together first; `[P]` impl tasks in different files run together.
- Across stories: after Foundational, US1–US5 can be staffed in parallel (independent files).

---

## Parallel Example: User Story 1

```bash
# Tests first (write together, ensure they FAIL):
Task: "T012 config load/resolve tests in internal/config/config_test.go"
Task: "T013 context-precedence tests in internal/config/resolve_test.go"
Task: "T014 UI bucket-list model test in internal/ui/app_test.go"
Task: "T015 integration ListBuckets test in internal/storage/s3client_integration_test.go"

# Then parallel implementation in distinct files:
Task: "T016 config loader in internal/config/config.go"
Task: "T019 tea.Cmd/messages in internal/ui/commands.go + messages.go"
```

---

## Implementation Strategy

### MVP First

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US1 → STOP, validate buckets + context switch independently.
3. Phase 4 US2 → full navigable read-only browser. Demo.

### Incremental Delivery

US1 → US2 (MVP) → US3 (metadata) → US4 (search) → US5 (preview). Each adds value without breaking
prior stories; validate independently at each checkpoint.

### Notes

- `[P]` = different files, no incomplete-task deps.
- Verify each test FAILS before implementing (TDD, non-negotiable).
- `internal/storage` is the ONLY package importing `service/s3`; T004/T051 guard read-only.
- Commit after each task or logical group.
