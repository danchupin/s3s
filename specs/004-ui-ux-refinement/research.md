# Phase 0 Research: UI/UX Refinement

All unknowns are design decisions internal to `internal/ui` (no external tech to
evaluate). Each is resolved below with rationale and rejected alternatives.

## D1 — Footer composition & 3-row budget

**Decision**: `footerBlock` renders, in order: (1) one compact identity row, (2) one
contextual hints row, (3) the existing transient status row *only when non-empty*. The
standalone separator rule (`strings.Repeat("─", w)`, `app.go:542`) and the entire
`footerEndpointLine` row are removed from the footer.

**Rationale**: The clarified budget is 3 rows (Session 2026-06-05). The box's rounded
bottom border already provides a visual divider, so a dedicated rule row is redundant.
Endpoint/region/version are reference data a user consults rarely → they belong in help,
not in permanent footer real estate.

**Alternatives considered**:
- *Keep the separator rule* — costs a row for pure decoration; rejected against the
  3-row cap.
- *Keep endpoint line, drop only the rule* — still 4 rows on a busy screen; fails
  FR-006. Rejected.

## D2 — Contextual hint catalog, priorities & visibility predicates

**Decision**: Define a static catalog of hints, each `{key, label, prio, visible(ctx)}`.
At render time, filter by a small context struct (`mode`, `writable`, selection `isDir`
/ none, `searching`, `searchActive`, `len(contexts)`, op active), sort by descending
priority, **cap to the top `maxHints = 6`** (highest priority first), then greedily pack
those into ONE row within `width`. The count cap is independent of width: even a 200-col
terminal shows at most 6 hints (FR-001/SC-003). Priority tiers:

| Tier | Hints | Visible when |
|------|-------|--------------|
| P0 (never dropped) | `? help`, `q quit` | always |
| P1 | `enter open`, `esc back` | always (mode-appropriate label) |
| P2 | `/ filter`\|`/ search`, `esc clear` (clear only when search active) | list modes |
| P3 | `d del`, `u upload`, `+ folder`, `y copy`, `m move`, `D rmdir` | `writable && mode==tree`, gated by selection (object vs folder) |
| P4 | `r refresh`, `c context`, `1-9 switch` (switch only when `len(contexts)>1`) | list modes / multi-context |

Selection gating (FR-003): object-only actions (`d`, `y`, `m`) appear only when an object
is selected; `D rmdir` only when a folder is selected; `+ folder`/`u upload` apply to the
level so need no selection.

**Rationale**: A declarative catalog with predicates keeps `footerBlock` a pure function
of state — trivially testable per FR-001/002/003 and matching the project's "stateless
render from selection indices" style. Priority tiers give a deterministic degrade order
(FR-004) and guarantee the P0 escape hatches survive at any width (FR-005).

**Alternatives considered**:
- *Per-mode hardcoded strings* (today's `footerHintsLine`) — duplicates logic, can't
  degrade by priority, already the source of the clutter. Rejected.
- *Wrap to multiple rows* (today's `wrapSegs`) — violates the single-row hints rule and
  the 3-row cap. Rejected; `wrapSegs` stays available for help only / retired from footer.

## D3 — Overflow signalling ("? more")

**Decision**: When the packer drops ≥1 hint due to width, append a trailing
`dimCellStyle`-rendered `… ?` / `? more` segment as the final element (it is reserved
space in the width budget so it always fits). When nothing is dropped, no cue is shown.

**Rationale**: Clarified answer (Session 2026-06-05). Tells the user the keymap continues
in help without the noise of a numeric count.

**Alternatives considered**: silent drop (no discoverability signal); `+N` count
(noisy, count not actionable). Both rejected per clarification.

## D4 — Compact identity row content

**Decision**: `● <context> [RW|RO]` always; append `· <cluster>` only if it fits the row
after the context+tag (reusing `labeledSeg`/`truncate` width-capping). User, endpoint,
region, version are NOT in the footer.

**Rationale**: FR-007/FR-008 — context + read/write status must stay glanceable; cluster
is cheap orientation when space allows; everything else is reference → help.

**Alternatives considered**: keep user on the identity row (frequently long emails push
the row to wrap/truncate; low orientation value). Rejected.

## D5 — Redesigned help surface

**Decision**: `helpLines` becomes a method `m.helpLines()` returning categorized groups —
**Navigation**, **Search & View**, **Context**, **Write**, **Global** — each action
listing all key aliases (from `defaultKeys()` so help can never drift from bindings), plus
a **Connection** section (context, cluster, endpoint, region, user, s3s version) sourced
from `m.info`/`m.ctxName`/`Version`. Write actions are shown only when `m.writable` (or
shown dimmed/marked "(write mode)"); the surface ends with an explicit
"press any key to close" line.

**Rationale**: FR-010..014a. Deriving the key column from `defaultKeys()` keeps help and
bindings in lock-step (prevents the classic stale-help bug). The Connection section is the
new home for footer-evicted metadata (D1/D4).

**Alternatives considered**:
- *Command palette / fuzzy action search* — explicitly out of scope (spec Assumptions);
  larger surface, new input mode. Deferred.
- *Keep flat help list* — fails FR-011 categorization and doesn't house connection
  metadata cleanly. Rejected.

## D6 — Named loading & search-pending status

**Decision**: `statusLine`'s loading branch names the in-flight target derived from
current state: object load when `mode==modeObject` (or op browse), level/listing load in
`modeTree`, bucket load in `modeBuckets` (e.g. `loading object…`, `loading contents…`,
`loading buckets…`). Add a search-pending indicator: while `m.searching` and a debounced
search is scheduled but unfired, show `searching…` rather than a blank/echoing input.

**Rationale**: FR-015/FR-016. The model already knows mode and search state; wording is a
pure function of it. No new async machinery.

**Alternatives considered**: a generic "loading…" (status quo — ambiguous, fails FR-015).
Rejected.

## D7 — Success-notice vs error distinction

**Decision**: Render transient success notices in a success hue (reuse `colOK` green via a
dedicated `noticeStyle`) distinct from `errStyle` red; keep clear-on-next-interaction
behavior (already implemented). Typed-confirmation prompt continues to show the exact
required target alongside input (already in `opPromptLine`); add a regression test.

**Rationale**: FR-017/FR-018 — make the categories visually unambiguous and lock current
safe behavior with tests.

**Alternatives considered**: leave notice on accent/coral (too close to the keybinding
accent and to warnings). Rejected for clarity.

---

**All NEEDS CLARIFICATION resolved.** No open unknowns blocking Phase 1.
