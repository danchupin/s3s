# Implementation Plan: Pinned Buckets for Scoped Connections

**Branch**: `010-pinned-buckets` | **Date**: 2026-06-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/010-pinned-buckets/spec.md`

## Summary

Let a connection carry an ordered list of **pinned bucket names**. When present, s3s renders that
list as the bucket view and never calls `ListBuckets`; the user opens/switches between them with the
existing browse machinery (which works with bucket-scoped credentials). The set is editable at
creation (a new `buckets` field on the add-connection form) and at runtime (a `+ add bucket` row on
the bucket list, shown only for *scoped* lists), with additions persisted to config off the event
loop. The add-connection reachability test probes a pinned bucket via `ListLevel(MaxKeys=1)` instead
of `ListBuckets`, and the test-result handler surfaces the *classified* error (`errorText()`) instead
of the hardcoded `"unreachable"`, treating `AccessDenied` as reachable.

All changes live in `internal/config`, `internal/ui`, and `cmd/s3s`. No new `storage.Storage`
methods; the structural read-only guard stays green. The only `internal/storage` edit is a test-only
error-injection knob on `storage.Fake`.

## Technical Context

**Language/Version**: Go 1.25 (per go.mod)

**Primary Dependencies**: `charm.land/bubbletea/v2` + `charm.land/lipgloss/v2` (TUI), `aws-sdk-go-v2`
(only inside `internal/storage`), `go.yaml.in/yaml/v3` (config marshal), OS keychain (secrets).

**Storage**: kubectl-style YAML config (`~/.config/s3s/config.yaml`) + OS keychain for secrets. No DB.
Pinned bucket names are non-secret config (`Cluster.Buckets`).

**Testing**: `go test` — white-box `package ui` tests via `deliver`/`press`/`viewOf` helpers;
`internal/config` unit tests (temp-file round-trip); `storage.Fake` for storage units. Integration
(testcontainers MinIO) is **unaffected** — no storage-client contract change.

**Target Platform**: terminal (darwin/linux), Bubble Tea v2 cell renderer.

**Project Type**: single Go CLI/TUI.

**Performance Goals**: a pinned bucket list resolves with **zero** network calls (instant render);
no `ListBuckets` round-trip for scoped connections (SC-005).

**Constraints**: read-only structural guard (`make check-readonly`) MUST stay green — no write-S3
symbols outside `internal/storage`; configs without `buckets` MUST stay byte-identical (`omitempty`);
every backend call stays off the UI loop with a generation guard (Constitution II).

**Scale/Scope**: ~9 source files touched + 1 test-only Fake knob. No schema migration (additive,
backward-compatible YAML field).

## Constitution Check

*GATE: re-checked after Phase 1 design — still PASS.*

- **I. Core/UI Separation** — PASS. UI never imports the SDK. Pinned-bucket logic splits cleanly:
  config (`Cluster.Buckets`, `AppendBucket`), UI (`m.pinnedBuckets`, `loadBuckets` branch, add-row,
  add-form), and the `Connector` seam (`Test` probe + new `AddBucket`) implemented in `cmd/s3s`.
  `storage.Storage` is unchanged.
- **II. Non-Blocking TUI** — PASS. `AddBucket` runs off-loop via a new `addBucketCmd`/`addBucketMsg`
  carrying `m.gen`; the synthesized pinned `bucketsMsg` is emitted from a `tea.Cmd` (no inline
  blocking); the `Test` probe already runs off-loop.
- **III. Test-First (NON-NEGOTIABLE)** — PASS by mandate. Every task below writes the failing test
  first (config round-trip, Fake error-injection, UI render/nav/add-row, probe, error text).
- **IV. Integration Testing** — PASS (N/A delta). No change to the storage-client contract; the probe
  reuses already-integration-tested `ListLevel`. No new integration test required; existing suite
  must stay green.
- **V. Observability & Safe Operations** — PASS. `AppendBucket` logs `connection.bucket-add`
  (non-secret: context + bucket + outcome) mirroring `connection.add`. The action is **additive**,
  not destructive, so no typed confirmation is required (it does not delete or overwrite data).

No violations → **Complexity Tracking is empty**.

## Project Structure

### Documentation (this feature)

```text
specs/010-pinned-buckets/
├── plan.md              # This file
├── spec.md              # Feature spec (+ Clarifications session 2026-06-07)
├── research.md          # Phase 0 — decisions R1..R9
├── data-model.md        # Phase 1 — entities, field-index shift, normalization
├── quickstart.md        # Phase 1 — user how-to + test how-to
├── contracts/           # Phase 1 — behavior contracts
│   ├── config-schema.md
│   ├── pinned-bucket-list.md
│   ├── add-bucket.md
│   ├── conn-form-buckets-field.md
│   └── conn-test-and-error.md
├── checklists/
│   └── requirements.md  # 16/16 (from /speckit-specify + /speckit-clarify)
└── tasks.md             # Phase 2 — /speckit-tasks (NOT created here)
```

### Source Code (repository root) — files this feature touches

```text
internal/config/
├── config.go        # Cluster gains Buckets []string `yaml:"buckets,omitempty"` (~line 53)
├── connection.go    # NewConnection.Buckets; AddConnection maps it (~50); NEW AppendBucket(name,bucket)
├── resolve.go       # no signature change — Resolve already returns Cluster (now carries Buckets)
└── *_test.go        # round-trip + AppendBucket + normalization tests (test-first)

internal/ui/
├── app.go           # Backend.PinnedBuckets; m.pinnedBuckets; New() seeds it; bucketsView add-row;
│                    #   onBucketsKey add-row branch; Update dispatch for addBucketMsg; modeAddBucket;
│                    #   onConnTested AccessDenied=save + errorText(); paste routing
├── commands.go      # loadBuckets gains pinned param → synthesize bucketsMsg when pins exist
├── connections.go   # connForm.buckets field + fldBuckets + label + hint + focusField + draft();
│                    #   Connector.AddBucket; addBucketCmd; onAddBucket; bucketAddForm + onAddBucketKey
├── messages.go      # addBucketMsg{gen, buckets, err}
└── *_test.go        # white-box render/nav/add-row/probe/error tests (test-first)

cmd/s3s/
├── connection.go    # connSeam.AddBucket (via cfg.AppendBucket); connSeam.Test probes ListLevel(1)
│                    #   on first pinned bucket when draft has Buckets, else ListBuckets
└── main.go          # backendFrom populates Backend.PinnedBuckets from resolved Cluster.Buckets

internal/storage/
└── fake.go          # TEST-ONLY: FailListBuckets bool + per-bucket access-denied knob for ListLevel
                     #   (read-only knobs; check-readonly only guards write-S3 symbols)
```

**Structure Decision**: single-project Go layout, unchanged. The feature is a thin additive slice
across the existing three layers (config → seam → ui), reusing every established idiom (trial-copy
config persist, `Connector` seam, `tea.Cmd`+gen-guarded msg, `textField`, the `+ add connection`
synthetic-row pattern). No new package, no interface on `storage.Storage`.

## Key Design Decisions (see research.md for rationale)

1. **Storage location of pins**: `config.Cluster.Buckets []string` (`yaml:"buckets,omitempty"`), not
   `Context`. Pins are endpoint-addressing metadata, next to `Endpoint`/`PathStyle`; the form already
   maps a connection name onto a same-named cluster/user/context triple.
2. **Canonical names**: config `Cluster.Buckets`; `NewConnection.Buckets`; `ConnDraft.Buckets`;
   `ui.Backend.PinnedBuckets`; model `App.pinnedBuckets`; form field `connForm.buckets` / `fldBuckets`
   / label `"buckets"`. ("Pinned" only in the UI-facing `Backend`/model names; config key is the
   terse `buckets`.)
3. **Pinned vs list-all branch**: a connection is *pinned* iff `len(pinnedBuckets) > 0`. Pinned →
   `loadBuckets` synthesizes `[]storage.Bucket{{Name: n}}` (zero `CreationDate`) and emits
   `bucketsMsg` with **no** `ListBuckets` call. Empty → unchanged `ListBuckets`.
4. **`+ add bucket` visibility (FR-013a)**: shown when `len(pinnedBuckets) > 0` **OR** the last
   bucket load failed/denied **OR** returned empty — i.e. only for "scoped" lists; hidden when
   list-all succeeds with results, so working connections never gain hidden buckets.
5. **Add-bucket UX**: a synthetic `+ add bucket` row at the end of the bucket list (mirror
   `connRows`/`+ add connection`); Enter opens a one-field input (`modeAddBucket` + `bucketAddForm`
   reusing `textField`); submit → `Connector.AddBucket` → persist + reload.
6. **Reachability probe**: `connSeam.Test` uses `ListLevel(Bucket: d.Buckets[0], MaxKeys: 1)` when
   `d.Buckets` is non-empty, else `ListBuckets`. It returns the classified error verbatim.
7. **AccessDenied tolerance + honest error**: in `onConnTested`, treat `nil` **or**
   `storage.ErrAccessDenied` as success (save); for other errors set `m.err = msg.err` and render
   `m.errorText() + " — press Enter again to save anyway"`. This is UI-testable via `fakeConnector`.
8. **Persistence seam**: new `config.AppendBucket(ctxName, bucket)` mirrors `AddConnection`
   (trial-copy → `Validate` → `Marshal`/`Save` → commit live), normalizes (trim/dedupe/order-stable),
   logs `connection.bucket-add`. Exposed to UI as `Connector.AddBucket`.
9. **Test-only Fake knob**: `Fake.FailListBuckets` (→ `ErrAccessDenied`) and a per-bucket
   access-denied set so a test can model "list-all denied, named bucket reachable".
