---
description: "Task list for 006-ui-redesign implementation"
---

# Tasks: UI Redesign (k9s-style, menu-less actions, in-app connections)

**Input**: Design documents from `/specs/006-ui-redesign/`
**Prerequisites**: plan.md, spec.md, research.md (R1–R9), data-model.md,
contracts/ (layout, actions-keybindings, command-bar, connection-manager)

**Tests**: INCLUDED — TDD is non-negotiable for this project (Constitution III).
UI tests are white-box (`package ui`, `deliver`/`press` helpers, assert on
`App.View().Content`); units use `storage.Fake` and a mock keyring; integration is
`//go:build integration` against MinIO.

**Organization**: by user story (priority order). US1 + US2 are P1 (MVP); US3 + US4
are P2. Visual-design FRs (FR-031..FR-046) are foundational backbone + final polish.

## Format: `[ID] [P?] [Story] Description with file path`

- **[P]**: parallelizable (different file, no dependency on an incomplete task)
- **[Story]**: US1/US2/US3/US4 (setup/foundational/polish carry no story label)

## Path Conventions

Single Go project: `cmd/s3s/`, `internal/ui/`, `internal/{config,secret,storage}/`.

---

## Phase 1: Setup

**Purpose**: confirm a green baseline before refactoring the UI.

- [x] T001 Confirm baseline green on branch `006-ui-redesign`: run `make test fmt vet build` and record the current `internal/ui` test set that the rework will touch (no code change).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: shared mode/state/message/style scaffolding every story builds on. No
story can start until these land. ⚠️ MUST complete before Phase 3.

- [x] T002 Update the `mode` enum in `internal/ui/app.go`: remove `modeActionMenu`; add `modeCommand`, `modeConnections`, `modeConnForm` (per data-model.md "Mode enum changes").
- [x] T003 Add the new `App` state fields in `internal/ui/app.go`: pane (`paneGen int`, `paneSelKey string`, `paneMeta *storage.ObjectMetadata`, `panePrev *preview.Payload`, `paneVisible bool`), command (`cmdInput string`), connection form/list (`conn` draft + field cursor + `connErr`, `connSel int`), per data-model.md.
- [x] T004 [P] Add message types in `internal/ui/messages.go`: `paneMetaMsg{gen int; key string; md storage.ObjectMetadata}`, `panePreviewMsg{gen int; key string; payload preview.Payload}`, `connTestedMsg{err error}`, `connSavedMsg{names []string; err error}` (do NOT set `modeObject`).
- [x] T005 [P] Add visual backbone in `internal/ui/styles.go` (FR-031/FR-032/FR-034/FR-041/FR-042): new style vars for pane / hint-bar / command-bar / connection-form that REUSE existing tokens only (no new `lipgloss.Color(...)` outside the FR-031 role table); a `colorEnabled()` gate honoring `NO_COLOR`; helpers for the non-color emphasis cues (`▶` gutter, `✓` mark, bold, `error:`/`[RW]` text).

**Checkpoint**: enum/state/messages/styles compile; existing behavior unchanged
(menu still referenced only where it will be deleted in US1).

---

## Phase 3: User Story 1 — Menu-less direct actions + hint bar (Priority: P1) 🎯 MVP

**Goal**: remove the modal action menu; every item action is a single discoverable
keypress; an always-visible contextual hint bar lists the valid actions.

**Independent test**: highlight an object → `d` downloads immediately (no menu);
`a` analyzes; in a writable session `x`/`X`/`y`/`m`/`u`/`+` enter their existing
flows; `a` never opens a menu; the hint bar lists exactly the valid keys; read-only
omits write keys; with marks set, `d`/`x`/`y` act on the marked set.

### Tests (write first, must fail)

- [x] T006 [P] [US1] White-box tests in `internal/ui/hintbar_test.go`: hint-bar contents by mode/selKind/selCount/writable; write actions omitted when `!writable()`; bulk variant + count shown when `selCount()>0`.
- [x] T007 [P] [US1] Tests in `internal/ui/keys_test.go`: each direct key (`a`/`d`/`x`/`X`/`y`/`m`/`u`/`+`/`r`) routes into the correct existing `start*`/`refresh`/`startAnalyze`/`startDownload` flow; pressing `a` does NOT open a menu; destructive keys still reach the two-tier confirmation (FR-005); a write key in read-only is a safe no-op + notice.

### Implementation

- [x] T008 [US1] Rebind in `internal/ui/keys.go`: `Analyze=["a"]`, `Download=["d"]`, `Delete=["x"]`, `DeleteAll=["X"]`, keep `Copy=["y"]`/`Move=["m"]`/`Upload=["u"]`/`NewFolder=["+"]`; bind `Refresh=["r"]` for direct use; remove the `Menu` field (per contracts/actions-keybindings-contract.md A1). The `Command=[":"]` binding is added in US3 (T020), NOT here — so US1 alone never leaves a bound-but-dead `:` key; the US1 hint bar (T010) MUST NOT advertise `:` unless the command handler is present.
- [x] T009 [US1] Delete `internal/ui/actionmenu.go` and `internal/ui/actionmenu_test.go`; remove `modeActionMenu` branches from `View()` and `onKey()`/`onMenuKey`/`openActionMenu` in `internal/ui/app.go`. Preserve the selection/capability gating by moving it into the hint bar (T010).
- [x] T010 [US1] New `internal/ui/hintbar.go`: build the `Action` catalog (`{key,label,writeOnly,available,invoke}`) from `mode`/`selKind()`/`selCount()`/`writable()` (reuse the old `menuItemsFor` predicates); render an always-visible, priority-capped, width-fit hint line (FR-003/FR-004/FR-040); write actions omitted/greyed in read-only.
- [x] T011 [US1] Wire direct-key dispatch in `internal/ui/app.go` (`onBucketsKey`) and `internal/ui/tree.go` (`onTreeKey`): each action key calls the same `start*`/`refresh` entry the menu used; route `d`/`x`/`y` to the bulk variant when `selCount()>0` (FR-006).
- [x] T012 [US1] Replace `footerHints` with the hint bar in `internal/ui/app.go` `footerBlock`/`View`; drop the `a actions` hint from `internal/ui/styles.go` `hintCatalog` (or repoint it).
- [x] T013 [US1] Update `helpView`/`helpLines` in `internal/ui/keys.go`: drop the "Actions (a menu)" framing; advertise the new direct keys (`a` analyze, `d` download, `x`/`X` delete, `:` command) so help never drifts from bindings.

**Checkpoint**: US1 independently testable — direct keys + hint bar work, no menu;
build + `go test ./internal/ui/` green.

---

## Phase 4: User Story 2 — List + persistent details pane (Priority: P1) 🎯 MVP

**Goal**: show the list and a persistent details/preview pane at once; the pane
updates (debounced) as the selection moves; the list no longer fills the screen.

**Independent test**: move the selection → the pane shows the highlighted item's
metadata + bounded preview after a short pause; fast scrolling causes ≤1 fetch for
the settled row; a folder shows a summary; narrow terminals stack/collapse the pane
without clipping the hint bar/footer/badge.

### Tests (write first, must fail)

- [x] T014 [P] [US2] Tests in `internal/ui/pane_test.go`: split renders list+pane at ≥100 cols and stacks/collapses below; object selection shows metadata + preview, folder shows summary; driving the debounce tick fires exactly one fetch for the settled selection; a stale `paneMetaMsg`/`panePreviewMsg` (gen ≠ `paneGen`) is dropped; 80×24 renders hint bar + footer + badge uncliped (per contracts/layout-contract.md).

### Implementation

- [x] T015 [US2] New `internal/ui/pane.go`: `paneView(width,height)` rendering object metadata (size, content-type, last-modified, ETag, storage class) + bounded preview, folder/level summary, and the loading/empty/error states (FR-010/FR-011/FR-046); reuse `metaKeyStyle`/`metaValStyle`/`truncate`.
- [x] T016 [US2] Debounced pane loader in `internal/ui/commands.go`: a `paneTick` (~150–250 ms) + `paneMetaMsg`/`panePreviewMsg` emitters that reuse `loadMetadata`/`loadPreview` under `paneGen` (do NOT flip `modeObject`); add the `Update` handlers in `internal/ui/app.go` with the gen/key drop check (FR-009/FR-012).
- [x] T017 [US2] Split the body in `internal/ui/app.go` `View()`: `JoinHorizontal(list, paneView)` when `width >= 100` (pane = `min(40, width/3)`), else stack/collapse the pane and pass full width; pass the reduced list width to `treeView`/`bucketsView`; keep the box title, footer, hint bar, and write badge (FR-008/FR-013/FR-014/FR-045).
- [x] T018 [US2] On selection move in `onBucketsKey`/`onTreeKey` (`internal/ui/app.go`, `tree.go`): set `paneSelKey`, bump `paneGen`, and schedule the `paneTick`; render instantly-known list fields immediately (FR-009).

**Checkpoint**: US2 independently testable; MVP (US1+US2) complete and shippable.

---

## Phase 5: User Story 3 — Command bar `:` (Priority: P2)

**Goal**: `:` opens a command bar to jump to views / run actions with discovery.

**Independent test**: `:` opens; a known command + Enter jumps/acts; typing shows
candidates; Esc is a no-op; unknown → notice; `:` is inert during search/op prompts.

### Tests (write first, must fail)

- [x] T019 [P] [US3] Tests in `internal/ui/command_test.go`: `:` enters `modeCommand`; `buckets`/`contexts`/`conn`/`analyze`/`refresh`/`help`/`quit` (+ aliases `ctx`/`du`/`q`) dispatch; prefix typing filters candidates; Esc closes with no effect; unknown name → `notice`, no action; `:` inert while searching or an op prompt is active (per contracts/command-bar-contract.md).

### Implementation

- [x] T020 [US3] Add the `Command=[":"]` binding in `internal/ui/keys.go`; new `internal/ui/command.go`: `modeCommand` input handling + a `Command` registry (`{name,aliases,invoke}`) covering buckets/contexts/conn/analyze/refresh/help/quit; prefix-match + dispatch + unknown→notice.
- [x] T021 [US3] Wire `:` open in `internal/ui/app.go` `onKey` (after op/search precedence) and render the command input via `statusLine` (reuse the search-input path); ensure `:`/`/` are mutually exclusive (FR-019).

**Checkpoint**: US3 independently testable on top of US1/US2.

---

## Phase 6: User Story 4 — In-app connection manager (Priority: P2)

**Goal**: add a cluster connection from the UI; persist to config (triple, schema
unchanged); secret to the OS keychain; reachability-tested with save-anyway.

**Independent test**: open the manager, add a connection to MinIO, save → new context
appears and is switchable in-session; config gains the context with NO plaintext
secret; the secret resolves from the keychain on next launch.

### Tests (write first, must fail)

- [x] T022 [P] [US4] UI tests in `internal/ui/connections_test.go`: form validation blocks empty name/endpoint and a non-URL endpoint; duplicate name rejected; a failed `Test` surfaces "save anyway" and choosing it persists; on `connSavedMsg` the new name appears in `m.contexts` and is switchable.
- [x] T023 [P] [US4] Unit tests in `internal/config/connection_test.go` (+ `internal/secret` mock keyring): `AddConnection` maps one draft → cluster+user+context triple with `keychain:true` and NO plaintext secret in the marshaled config; existing entries preserved; duplicate derived names rejected; `StoreKeychain` called with the right account.

### Implementation

- [x] T024 [US4] New `internal/config/connection.go`: `AddConnection(draft)`-style writer (UI-agnostic) mapping `ConnDraft`→`Cluster{Name,Endpoint,Region}`+`User{Name,AccessKeyID,Keychain:true}`+`Context{Name,Cluster,User,ReadOnly}` via existing `Upsert` + `Marshal` + `Save`; validate required/URL/uniqueness (reuse `Validate` rules); return updated context names (FR-022/FR-022a/FR-024/FR-027). Emit a structured `connection.add` log line with the outcome and NON-secret fields only (name/endpoint/region/readonly) — never the secret — per Constitution V (Observability); reuse the `log/slog` JSON-to-file path.
- [x] T025 [US4] In `cmd/s3s/main.go`: build and inject a `Connector{Test,Save}` closure into `ui.New` — `Test` = `storage.New(cc)` + `ListBuckets` (off-loop reachability, FR-025a); `Save` = `secret.StoreKeychain(account, secret)` FIRST then `config.AddConnection` (abort config on keychain failure, FR-026); add the `Connector` field + param to `internal/ui/app.go` `App`/`New` (nil disables, like `Resolver`).
- [x] T026 [US4] New `internal/ui/connections.go`: `modeConnections` list (existing contexts + "add") and `modeConnForm` form (name/endpoint/region/accessKeyId/secret/readonly) with per-field validation; run `Test`/`Save` via `tea.Cmd`; handle `connTestedMsg`/`connSavedMsg`; on save-success replace `m.contexts` (FR-020/FR-021/FR-025). ⚠️ The secret field is masked **by the form itself** inside Bubble Tea (render `•`/`*`, hold the raw value in a `logging.Secret`) — do NOT call `secret.Prompt` (x/term no-echo), which only works BEFORE the TUI starts (005 R12) and would corrupt the alt-screen.
- [x] T027 [US4] Reach the manager: `+` in the context switcher (`onContextKey`, `internal/ui/app.go`) and the `:conn` command (registry, T020); add a connection entry to the hint bar where appropriate.
- [x] T028 [P] [US4] Integration test `internal/ui/connections_integration_test.go` (`//go:build integration`): add a connection against MinIO → `Test` passes → `Save` (mock or real keyring) → switch → bucket list loads.

**Checkpoint**: all four stories independently functional.

---

## Phase 7: Polish & Cross-Cutting (visual design FR-031..FR-046 + regression)

**Purpose**: enforce the visual contract and guard the preserved feature set.

- [x] T029 [P] Enforce the palette/emphasis/restraint contract across all new surfaces (FR-031..FR-041): audit `internal/ui/{pane,hintbar,command,connections,styles}.go` — single accent + bounded param hues (≤4/screen), neutral baseline majority, `[RW]` badge the sole loud element, every color meaning has a non-color cue.
- [x] T030 [P] Per-screen data inventory completeness (FR-043) + truncation (FR-044) + narrow reflow priority (FR-045): audit each surface renders its full inventory; long keys/ETags/endpoints ellipsis-truncate; nothing required dropped at 80×24.
- [x] T031 [P] Tests in `internal/ui/styles_test.go` / `view_test.go` for the visual Success Criteria: `NO_COLOR` legibility via glyph/weight cues (SC-012), accent-hue budget ≤4 (SC-009), badge sole-loud (SC-011), full data inventory present (SC-013).
- [x] T032 Repair tests broken by the rework in `internal/ui`: update `footer_test.go`, `keys_test.go`, remove obsolete `actionmenu_test.go` assertions; ensure `confirm_test.go`/operation flows still pass (confirmations unchanged, FR-005).
- [x] T033 [P] Quality gates: `make fmt vet lint`, `make check-readonly` (no new S3 write symbol leaves `internal/storage`), full `go test ./...` + `make test-integration`; update `README.md`, `ROADMAP.md`, and confirm `quickstart.md` parity.
- [x] T034 [P] Regression smoke per `quickstart.md` (FR-028): context quick-switch `1`–`9`, object Enter view, `du` drill-down, bulk download/delete/copy, sort `s`/`S`, write toggle `w` + loud badge, structured logs of destructive ops.

---

## Dependencies & Execution Order

- **Setup (P1)** → **Foundational (P2)** block everything.
- **US1 (Ph3)** and **US2 (Ph4)** are the P1 MVP. Both depend only on Phase 2; they
  touch overlapping files (`app.go`, `tree.go`) so run **US1 then US2** sequentially
  to avoid conflicts (not in parallel).
- **US3 (Ph5)** depends only on Phase 2 and is self-contained (it adds its own `:`
  binding in T020); US1 deliberately does NOT bind `:`, so US1 alone leaves no dead key.
- **US4 (Ph6)** depends on Phase 2; the `:conn` reach (T027) depends on US3's
  registry (T020) — if US3 is deferred, US4 is still reachable via the context-
  switcher `+`.
- **Polish (Ph7)** runs after the stories it audits; T031/T029/T030 can run in
  parallel; T032/T033/T034 gate the merge.

## Parallel Opportunities

- Phase 2: T004 + T005 in parallel (different files) after T002/T003.
- US1: T006 + T007 (tests, different files) in parallel before impl.
- US2: T014 authored in parallel with US1 polish (different file).
- US4: T022 + T023 + T028 in parallel (UI test / config-secret unit / integration).
- Phase 7: T029 + T030 + T031 in parallel.

## Independent Test Criteria (per story)

- **US1**: actions reachable in one keypress, no menu, hint bar correct by
  capability — `go test ./internal/ui/ -run 'Hint|Keys|Direct'`.
- **US2**: list+pane coexist, debounced/superseded pane, graceful narrow reflow —
  `-run 'Pane|Layout'`.
- **US3**: `:` parse/dispatch/unknown/precedence — `-run 'Command'`.
- **US4**: form validation, keychain-no-plaintext mapping, in-session switch,
  integration add→test→switch — `-run 'Conn'` + integration tag.

## Implementation Strategy

- **MVP = US1 + US2** (both P1): the menu-less interaction + the rebalanced layout
  are the user's core complaint; ship these first as a coherent increment.
- **Increment 2 = US3** (`:` command bar) — discovery polish.
- **Increment 3 = US4** (in-app connections) — onboarding feature; largest new
  surface, isolated behind the `Connector` seam.
- Visual-design FRs are enforced continuously (Phase 2 backbone) and audited in
  Phase 7; do not defer them past each story's checkpoint.
- TDD throughout: the `Tests (write first)` block in each phase MUST be red before
  its implementation tasks (Constitution III).
