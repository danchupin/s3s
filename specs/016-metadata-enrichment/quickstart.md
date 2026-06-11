# Quickstart: Metadata Enrichment & Inline Usage (016)

## Build, test, and gate

```bash
make test              # unit tests: white-box UI (package ui) + storage Fake units. No Docker.
make test-integration  # integration tests vs real MinIO (testcontainers). REQUIRED for US4.
make fmt vet lint      # gofmt, go vet, golangci-lint (built with the module's Go 1.25 toolchain)
make check-readonly    # structural read-only guard (scripts/check-readonly.sh) — MUST stay green
make build             # -> bin/s3s
```

Run a focused test:

```bash
go test ./internal/ui/      -run TestObjectMetadata
go test ./internal/ui/      -run TestInlineUsage
go test ./internal/storage/ -run TestGetBucketConfiguration
go test ./internal/storage/ -run TestClassifyUnsupported   # the unsupported-branch unit
go test -cover ./...
```

### Integration tests (constitution IV — REQUIRED for the US4 read-contract change)

`make test-integration` `t.Skip`s automatically when no Docker provider is found.
testcontainers does not auto-detect a Lima-based Docker; point it at the Lima socket and
disable Ryuk:

```bash
DOCKER_HOST=unix://$HOME/.lima/<vm>/sock/docker.sock \
TESTCONTAINERS_RYUK_DISABLED=true \
make test-integration
```

The new storage methods (`GetObjectTagging`, `GetBucketConfiguration`) extend the
storage-client contract, so they MUST be exercised against real MinIO in
`internal/storage/s3client_integration_test.go` (seed via the raw SDK seed client). What
MinIO CAN and CANNOT verify:

- **CAN (MinIO integration)**: tag KV pairs; a `configured` versioning/encryption state; a
  `none` for an unconfigured sub-resource (MinIO returns the `*NotFound`/`*NotConfiguration`
  family, which maps to `none`); a `denied` for a policy-denied read; and partial success
  when one sub-resource fails while the rest load.
- **CANNOT (so covered elsewhere)**: the `unsupported` state. MinIO IMPLEMENTS every config
  sub-resource — it returns "not-configured" codes, never a "method not implemented" error,
  so it can never produce `ConfigState "unsupported"`. That riskiest branch (it gates
  SC-004's three-way distinction) is covered by TWO non-MinIO tests:
  1. a `Fake` unit test driving `FakeBucket.UnsupportedGetConfigs[<subresource>] = true`
     and asserting `BucketConfig.<Sub>.State == "unsupported"` and `Reason == ErrUnsupported`;
  2. a `classify` unit test (`internal/storage/classify_unit_test.go`) feeding a synthetic
     `smithy.APIError` with code `NotImplemented` (and a `MethodNotAllowed` / HTTP 501 / 405
     variant) and asserting `errors.Is(classify(err), ErrUnsupported)`, AND feeding a
     `NoSuchLifecycleConfiguration` and asserting it does NOT map to `ErrUnsupported` (so the
     `none` vs `unsupported` split is pinned).

US1's enriched `HeadObject` mapping adds fields, not a new call, so it is asserted by
extending the existing `HeadObject` integration assertion.

### Test-first (constitution III) — COMPLETE RED migration set

Write each test RED before the production code, in this order: US1 object-metadata
render/omit-empty/permission-gated/core (asserting on BOTH the Enter view and the focus
pane, since the rows live in the shared `metaFieldRows`) → US2 inline totals +
dwell/cancel/cache/partial/generation + producer-drain-no-leak + `modeUsage`-removed →
US3 breakdown expand/collapse/drill (one-section-at-a-time) → US4 tags/config tri-state
(+ MinIO integration + the two unsupported units) → US5 storage-class marker +
reveal-recovery → FR-017 height/width sweep.

The migration breaks the build intentionally first — the RED set MUST include EVERY
dangling reference the deletion/rename exposes, or it won't compile:
- `modeUsage`: `app.go:30` (const), `219-227` (fields), `881` (onKey case), `1190-1191`
  (View case); `command.go:57` (drop from `canOpenCommand`); `footer_test.go:194,249`
  (`hintCtx{mode: modeUsage}` — TEST-only; `footerHints` does NOT branch on mode, so no
  production footer change is needed); the whole `analyze.go` `runAnalyze`/`onUsageKey`/
  `usageView`/`usageTitle`.
- `keys.Analyze` → `keys.MoreDetail`: `keys.go:21` (field), `keys.go:54` (binding), help
  row; `hintbar.go:52,70`; AND `pane.go:54`, `pane.go:67`, `pane.go:71` (these three call
  `keyHint(m.keys.Analyze, …)` — omitting them is a compile error).
- `command.go:33`: `analyze`/`du` entry → `detail`/`info` with `invoke: App.startMoreDetail`
  (the SAME target as the key).

Reuse the existing harness: `newApp`/`deliver`/`press`/`withBuckets`/`viewOf`,
`treeApp`/`selectObject`, `dualApp`, `stripANSI`, `enterTree`,
`assertWidthSweep`/`assertHeightSweep` (`footer_test.go:92`), and the storage
`NewFake`/`Seed`/`SeedObject`/`HeadObject` builders (`fake_test.go`). Inject the dwell
tick deterministically by `deliver`ing a synthetic `usageTickMsg{gen, bucket, prefix}`,
and the running indicator by `deliver`ing a synthetic `usageProgressMsg`.

Required new RED tests called out by the verifiers:
- **Generation isolation**: start a scan on bucket A; `deliver` a focus move to bucket B
  (bumps `usageGen` + calls `usageCancel`); then `deliver` the stale `usageDoneMsg` for A
  and assert the pane does NOT show A's totals.
- **No producer leak under rapid navigation**: start a scan, navigate away (starting a new
  scan), and assert the first scan's channel is drained to `close` (the pump re-arms
  ungated) — e.g. by exhausting the channel in the test and confirming no send blocks.
- **Rapid transit spawns no scan**: `deliver` N folder-focus moves, then ONE stale tick,
  and assert zero `loadUsage` dispatches for passed-over folders.
- **Both refresh paths rescan**: cache a usage total, press `r` in tree mode AND in bucket
  mode, and assert the next focus is a `usageResults` miss (rescans) in each path.
- **Context switch parity**: switch context, assert `usageResults.Len()==0`, and that a
  same-named bucket in the new context rescans (different `Context` in the key ⇒ miss).
- **Budget**: at 130×24 seed an object with ALL enriched optional fields populated + one
  detail section (breakdown OR tags) and assert every seeded value is present in
  `View().Content` OR represented by the `… +N more (i to reveal)` affordance (NOT merely
  that the footer is present — that always passes because the body is hard-capped at
  `styles.go:348-350`).
- **Storage-class reveal**: a GLACIER row shows the `glac` marker; `i` (reveal) on it shows
  the full `GLACIER` class string.

## Manual validation walkthrough (maps to SC-001..007)

Build and launch against a MinIO/RGW context (a wide terminal, ≥130 cols, gives the Full
three-zone layout):

```bash
make build && ./bin/s3s
```

1. **SC-001 — object metadata, zero extra keypresses.** Open (or focus, in the pane) an
   object that is SSE-KMS encrypted and versioned. The details area shows encryption type,
   KMS key reference, version id, and storage class alongside
   key/size/modified/type/class/ETag — with no separate screen. An object with no optional
   fields shows only the core block (no placeholder lines). An object whose
   object-lock/legal-hold you cannot read shows "unknown" (not "none").

2. **SC-002 — bucket/prefix totals on the main screen, zero mode switches.** Focus a
   non-empty bucket and rest on it. The details area shows a running "scanning… N objects,
   <size> so far" total (driven by usage-progress ticks) that resolves to
   `total <size> · N objects` — all inline, no full-screen transition.

3. **SC-003 — non-blocking + cancelable.** During a scan, move the selection / type a
   filter / navigate — input stays responsive. Navigate away mid-scan: the scan's own
   context is cancelled and the UI returns to responsiveness immediately; any partial total
   is marked partial, never presented as final.

4. **SC-007 — cached totals + refresh.** Re-focus a bucket whose totals were already
   computed this session: the totals appear immediately with no rescan. Press `r` (refresh)
   in the bucket list: the highlighted bucket's usage entry is invalidated, so the next
   focus rescans. Press `r` inside a tree level: that level's usage entry is invalidated.
   Switch context (`c`): figures do not bleed across contexts.

5. **SC-006 — analyze screen gone; ONE detail section inline.** Press `a` (the former
   analyze key, now "more detail") on a bucket/prefix: no full-screen view appears; instead
   the ranked largest-first breakdown expands in the same details area, each child with size
   and share. Press `a` again to collapse. Select a child sub-prefix and Enter to step into
   it (its usage shows by the same mechanism). Confirm `:analyze`/`:du` no longer exist (try
   `:detail`/`:info` instead — they invoke the SAME action as `a`).

6. **US4 — tags & bucket config on demand.** Press `a` ("more detail") on an object with
   tags: tag key/value pairs appear without freezing the UI. Press `a` on a bucket:
   versioning, default encryption, lifecycle, replication, and public-access/policy appear —
   each labeled "none" / "unknown/denied" / "unsupported" so you can tell *why* a value is
   absent (**SC-004**). On a backend lacking a sub-resource (e.g. RGW without
   public-access-block) that item shows "unsupported" while the rest still loads. Only ONE
   detail section is shown at a time (breakdown XOR tags XOR config) so the footer stays on
   screen.

7. **SC-005 — legibility at the minimum.** Shrink the terminal to the supported minimum
   (80×24) with the above on screen. Every added value is fully visible or revealable (`i`
   reveals long KMS keys / lifecycle dates / child names / the full storage class), and the
   footer/command-hint bar is never scrolled off. Where the detail zone can't fit a section
   fully, a `… +N more (i to reveal)` cue marks the clipped rows.

8. **FR-015 — storage class in the listing.** Enter a level holding a STANDARD object and a
   GLACIER-class object: the non-standard class shows the `glac` marker in the `type` cell
   on the archived row while the STANDARD row adds no per-row noise; `i` on the GLACIER row
   reveals the full `GLACIER` class; column widths stay legible at 80/120/160 cols.

## Final gate before PR

```bash
make fmt vet lint && make check-readonly && make test && make test-integration
```

`check-readonly` must print `PASS — no S3 mutations outside internal/storage/` (the new
methods are all `Get*`, not matched by the `(Put|Delete|Create|Copy|Upload|Restore|
Write)(…)` ban at `scripts/check-readonly.sh:43`).

