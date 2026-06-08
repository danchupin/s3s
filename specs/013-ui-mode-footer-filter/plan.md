# Implementation Plan: UI mode chip dedup, footer breathing room, applied-filter state

**Branch**: `013-ui-mode-footer-filter` | **Date**: 2026-06-08 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/013-ui-mode-footer-filter/spec.md`

## Summary

A small presentation/UX iteration on the two-pane browser, continuing 012. Three changes, all in
`internal/ui`, no storage-contract change:

1. **One read/write mode indicator (US1, P1).** Today the mode shows twice on every browse screen — the
   newer border-mounted chip (`modeChip`, app.go:1294) AND the older `[RW]/[RO]` tag inside
   `footerIdentityCompact` (styles.go:512). Keep the chip, **remove the footer tag everywhere**. Make the
   chip **universal**: it already rides the buckets/tree boxes; the only browse box missing it is the opened
   object (`modeObject`, app.go:1178, plain `boxView`) — route that through `boxViewChip` so write-state is
   never lost when the tag goes away. Modal write badges (`writeBadge` in confirm/arm popups, the help-overlay
   prefix) STAY — they are the deliberate safety redundancy, not the duplicate.

2. **Applied-filter state visible (US2, P1).** When a filter is committed and the input is closed, show the
   active term as a **persistent border chip on the filtered pane's box** (clarify: NOT in the footer, NOT
   appended to the breadcrumb title). Buckets-list filter → chip on the buckets box; objects-level filter →
   chip on the objects box. Gate on `!m.searching` so it never double-renders with the transient typing input
   in `statusLine`. Clearing the filter removes it automatically (existing clear paths empty the term). The
   breadcrumb-embedded filter markers that 012 left behind (`objectsZoneTitle` ` (term*)` app.go:1354;
   `resourceTitle` `/term*` app.go:1478) move to the chip so the term is shown once.

3. **Footer / command-bar breathing room (US3, P2).** Widen the separators (` · ` → `  ·  `), the key↔label
   gap (1→2 spaces), and the inter-column gap (2→3) so elements stop merging — consistently across the wide
   (3-column) and collapsed (drop-trailing) command-bar paths. Almost every fitter already re-measures its
   separator (`lipgloss.Width`), so widening self-accounts; the ONE manual-math site (the `+4` natural-width
   constant coupled to the `"  "` gap, commandbar.go:175/179) is **derived from the gap literal** to kill the
   double-count hazard. Horizontal-only: no new newline, so footer height and the body budget are unchanged;
   the no-wrap/no-scroll invariant (constitution VI / FR-016) holds.

Approach grounded in `research.md` (R1–R7) by reading the real render/test paths; reuse existing components
(`boxViewWith` chip slot, the palette roles, the keymap, the white-box test helpers). No new package, no new
hue, no storage method — `make check-readonly` stays green.

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`).

**Primary Dependencies**: Bubble Tea v2 (`charm.land/bubbletea/v2`), Lipgloss v2 (`charm.land/lipgloss/v2`).
`internal/storage` wraps `aws-sdk-go-v2/service/s3` — **unchanged this feature**.

**Storage**: S3-compatible (Ceph RGW / MinIO) via the `storage.Storage` interface — **no change**. The
applied-filter chip is pure render over the already-committed `m.search` / `m.bucketFilter`; it issues **zero**
new backend calls (asserted via `Fake.ListLevelCalls`).

**Testing**: `go test ./...` white-box `package ui` (`deliver`/`press`/`viewOf`/`stripANSI` helpers; builders
`dualApp`/`treeApp`/`buildApp`/`withBuckets`/`crossToObjects`) + in-memory `storage.Fake` with the existing
`ListLevelCalls`/`ListBucketsCalls` counters. No integration tests added (no storage-contract change —
constitution IV N/A, justified). `make fmt vet lint check-readonly`.

**Target Platform**: terminal TUI (macOS/Linux), truecolor + `NO_COLOR`; Bubble Tea v2 cell renderer.

**Project Type**: single Go project (CLI/TUI). All changes in `internal/ui`;
`internal/{storage,cache,preview,config,logging}` untouched.

**Performance Goals**: non-blocking TUI (constitution II) — every backend call already runs in a `tea.Cmd`;
this feature adds **no** I/O. Mode chip, filter chip, and spacing are pure render.

**Constraints**: the footer + command/hint bar MUST stay fully visible at every width tier (constitution VI /
FR-016 / `boxViewWith` `minRows` hard-cap, styles.go:347); every cue NO_COLOR-safe; one separator token; no
new palette hue (VII).

**Scale/Scope**: ~8–10 files in `internal/ui`; 3 user stories; one signature extension (a second, inboard
chip slot in `boxViewWith` + the `*Chip` wrappers); one derived layout constant; existing `[RW]/[RO]` and
separator tests migrate to assert the chip / new spacing. No new package, no new keymap field.

## Constitution Check

*GATE: must pass before Phase 0 and re-checked after Phase 1. Constitution v1.1.0.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Core/UI Separation | ✅ PASS | All work in `internal/ui`; no S3/SDK use outside `internal/storage`; UI stays render+intent. |
| II. Non-Blocking TUI | ✅ PASS | No synchronous I/O. Chip/filter-chip/spacing are pure render; filter chip reads already-committed state — zero new `ListLevel` calls (asserted). |
| III. Test-First (NON-NEGOTIABLE) | ✅ PASS (gate) | TDD per `research.md` R7: failing-first white-box test per user story (chip-on-object-box present; footer `[RW]/[RO]` absent; applied-filter chip present+cleared; widened-gap present + width-sweep still green). |
| IV. Integration Testing | ✅ N/A (justified) | No storage-client contract change. No new credential/pagination/transfer path. Only a test-only counter is read on `Fake`. |
| V. Observability & Safe Operations | ✅ PASS | Write-arm keeps its `slog` event + deliberate confirm popup; modal write badges retained; the help-overlay badge stays; the universal chip means write-state is **never lost** by removing the footer tag. No new destructive op. |
| VI. UI Legibility (v1.1.0) | ✅ PASS (drives feature) | Chip rides the top border → **zero** body rows, footer never scrolls (FR-016 / `minRows` cap). Filter term capped with an explicit ellipsis; the full committed term recoverable by re-opening the filter input (`/`, pre-filled); cues NO_COLOR-safe (text `WRITE`/`RO`/`filter:`). |
| VII. UI Consistency / Design System (v1.1.0) | ✅ PASS (drives feature) | Dedup = ONE mode indicator. Filter chip reuses `boxViewWith`'s chip slot + an existing palette role (`warnStyle`, no new hue). Spacing single-sourced through one separator token + one derived gap constant. |

**Read-only structural guard**: STAYS green — no new write-capable S3 symbol outside `internal/storage`, no
new storage method. **Initial gate: PASS. No violations → Complexity Tracking empty.**

Post-Phase-1 re-check: design adds no new package and no new hue; the only structural change is a **minimal
extension of an existing shared component** (a second, inboard chip slot on `boxViewWith`, dropped before the
safety-critical mode chip) — this is exactly the VII-preferred "reuse/extend the shared component" over
parallel styling. **PASS, no new violations.**

## Project Structure

### Documentation (this feature)

```text
specs/013-ui-mode-footer-filter/
├── plan.md              # This file
├── spec.md              # 3 user stories, FR-001..018, SC-001..008, 2 clarifications
├── research.md          # Phase 0 (R1–R7, grounded in real file:line)
├── data-model.md        # Phase 1 (UI state entities touched/added)
├── quickstart.md        # Phase 1 (manual verification mapped to acceptance scenarios)
├── contracts/           # Phase 1 (border-chip, mode-indicator, applied-filter, footer-spacing, layout-visibility)
├── checklists/
│   └── requirements.md  # Spec quality checklist (16/16)
└── tasks.md             # Phase 2 (/speckit-tasks — NOT created here)
```

### Source Code (repository root)

```text
internal/ui/                     # ALL changes live here
├── app.go                       # modeObject render → boxViewChip (+ m.modeChip()); listWithPane threads a
│                                #   per-pane filter chip; objects pane → chip-bearing variant; objectsZoneTitle /
│                                #   resourceTitle drop the breadcrumb-embedded filter marker (moves to chip);
│                                #   footerIdentityCompact callers updated for the dropped [RW]/[RO] tag
├── styles.go                    # boxViewWith gains a 2nd (inboard) chip slot + degrade order (filter chip
│                                #   drops before the mode chip); boxViewChip/boxViewFocusChip updated;
│                                #   footerIdentityCompact: [RW]/[RO] tag removed (identity = ● ctx · cluster);
│                                #   one separator token; renderHintRow separator widened
├── commandbar.go                # inter-column gap derived from a colGap const (kills the +4 double-count);
│                                #   entryStyled key↔label gap widened; barGlobals / fitEntries / collapsed
│                                #   separators use the shared token; infoColumn/collapsedBarView identity updated
├── pane.go                      # details-pane hint separators → shared token (consistency)
├── search.go                    # (no behavior change) per-pane committed-term predicate used by the chip
└── *_test.go                    # failing-first white-box tests per US; migrate existing [RW]/[RO] +
                                 #   separator/width assertions to the chip / new spacing
# internal/{storage,cache,preview,config,logging} — UNCHANGED
```

**Structure Decision**: Single project; the feature is entirely within `internal/ui`, editing existing files
only (no new file). The lone structural change is a backward-compatible extension of the shared
`boxViewWith` border composer (a second inboard chip slot with an explicit degrade order) — a refactor of an
existing component, not a new subsystem — honouring Core/UI Separation (I) and the design-system principle
(VII).

## Complexity Tracking

> No constitution violations — section intentionally empty. The `boxViewWith` second-chip extension is a
> reuse/extension of the existing shared component (VII-aligned), not added complexity requiring justification.

## Phase 1 Artifacts

- [research.md](./research.md) — R1 universal mode chip (the single missing box is `modeObject`); R2 remove
  the footer `[RW]/[RO]` tag (chip owns mode state; modal/help badges exempt); R3 applied-filter chip
  (per-pane state predicate, `warnStyle`, capped term, `!m.searching` gate, move breadcrumb markers);
  R4 `boxViewWith` second inboard chip slot + degrade order; R5 footer spacing (single token + derived
  inter-column constant); R6 automatic clear lifecycle; R7 TDD test plan + existing-test churn.
- [data-model.md](./data-model.md) — UI render-state entities: Mode chip (now universal), Applied-filter chip
  (new render-only derivation), Footer separator token, Border chip slots, Filter state fields (existing).
- [contracts/](./contracts/) — `border-chip-contract.md` (two-slot `boxViewWith` + degrade), 
  `mode-indicator-contract.md` (universal chip; footer tag removed; exemptions), 
  `applied-filter-contract.md` (per-pane chip predicate, format, lifecycle), 
  `footer-spacing-contract.md` (token + derived gap + invariant), 
  `layout-visibility-contract.md` (no-wrap/no-scroll at every tier; reveal fallback).
- [quickstart.md](./quickstart.md) — manual verification walkthrough mapped to the acceptance scenarios.
- CLAUDE.md `<!-- SPECKIT … -->` block updated to point at this plan.
