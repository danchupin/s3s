# Implementation Plan: Connection Management UX Fixes

**Branch**: `008-connection-form-ux` | **Date**: 2026-06-07 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/008-connection-form-ux/spec.md`

## Summary

Nine usability fixes, all in `internal/ui` (no storage / config behaviour change):

1. **Discoverable connection delete** — inline hint in the connections view (not the command-bar catalog) showing the delete keystroke for a deletable selection.
2. **Spelled-out chord labels** — `^x`/`^o` → `Ctrl+X`/`Ctrl+O` at the single source (`keyGlyph`), so every surface follows.
3. **Usable text entry** — replace the append-only string fields with a small rune-aware single-line editor (`textField`) supporting caret movement (←/→/Home/End), insert/delete at the caret, and clipboard paste (`tea.PasteMsg`, bracketed-paste, on by default). Shared by the add-connection form AND the typed-confirm input.
4. **Secret guidance** — per-field expectation text in the form; the secret field names what to enter (stored in OS keychain) and that other sources are config-file-only. No source selector, no config-writer change (clarified).
5. **Quieter command bar** — drop ALL THREE block headings (`INFO`/`READ`/`WRITE`); keep the column grouping (≥2-space inter-column gap is the separator). Preserve the read-only `(w to arm)` cue as literal amber text in the write group.
6. **Post-mutation visibility (US6)** — every action (incl. same-bucket cross-prefix copy/move/bulk-copy) shows immediately; invalidate PRECISELY the SOURCE and DESTINATION prefix keys (same bucket; not a whole-cache clear), not just the current view. No cross-bucket case (copy/move are single-bucket).
7. **Connection affordance (US7)** — relabel the existing command-bar connection entry from "new conn" to "connections" at BOTH render sites (infoColumn + collapsed read row); on the collapsed bar, reorder it ahead of droppable read entries so width-trimming doesn't drop it first (FR-020). No separate switch entry.
8. **Filter-reset affordance (US8)** — when a filter/search is applied (and not actively typing), show an `Esc clear` entry in the command bar read group (list modes render the bar, not the legacy `footerHints` that already had this cue).
9. **No duplicate delete labels (US9)** — show only the selection-applicable delete in the write group (suppress the inapplicable one) so two identical "delete" never appear; targeted exception to 007's "all write always shown".

Technical approach: extract one reusable single-line text editor and route a new `tea.PasteMsg` case to whichever text surface is active; broaden post-mutation cache invalidation to all affected levels; the rest are render/label tweaks against the existing palette and keymap.

## Technical Context

**Language/Version**: Go 1.25 (per go.mod)

**Primary Dependencies**: `charm.land/bubbletea/v2` v2.0.6, `charm.land/lipgloss/v2` v2.0.3. No new dependency (bubbles `textinput` is NOT in go.mod; the editor is hand-rolled, consistent with the existing append-tail fields).

**Storage**: N/A — no storage/config code changes. Secret remains keychain-only (`logging.Secret`, never plaintext).

**Testing**: `go test ./internal/ui/` white-box (`package ui`); `deliver`/`press` helpers; assert on `App.View().Content`. New: a paste-delivery helper (`tea.PasteMsg{Content:…}`) and `textField` unit tests.

**Target Platform**: terminal (TUI), darwin/linux.

**Project Type**: single project (CLI/TUI).

**Performance Goals**: render stays instant; no I/O added (paste is synchronous input). Non-blocking II unaffected.

**Constraints**: rune-aware caret (never split a multi-byte rune); single-line only; horizontal scroll keeps the caret on screen; masking preserved for the secret; NO_COLOR-safe text cues retained.

**Scale/Scope**: ~5 fields in one form + one confirm input; plus command-bar/cache tweaks. Small, bounded.

**Cache**: per-session `cache.Cache[*levelState]` keyed by `(context,bucket,prefix,search)`. Today only the current `levelKey()` is invalidated on `refresh()`. US6/FR-016 broadens this: after copy/move/bulk-copy invalidate PRECISELY the source + destination prefix keys (same bucket — `CopyKey`/`MoveObject` are single-bucket), NOT a whole-cache `cache.Clear()` (clarified). No cache API change required (`Invalidate(key)` already exists).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. Core/UI Separation** — PASS. All edits in `internal/ui`. `textField` is a UI helper; no SDK, no config marshalling. No storage interface change.
- **II. Non-Blocking TUI** — PASS. No network/disk I/O introduced. Paste is in-process input handled on the event loop like a keypress.
- **III. Test-First** — PASS (enforced in tasks). Failing white-box tests first: caret/paste/mask on `textField`; delete-hint visibility; chord-label rendering; absence of block titles; secret guidance text.
- **IV. Integration Testing** — PASS (N/A) under the confirmed SAME-BUCKET scope: US6 adds no storage-client contract change (UI cache only). NOTE: this N/A is valid only because copy/move stay same-bucket; if cross-bucket were ever adopted it WOULD change the storage contract and Constitution IV would mandate a real-backend (MinIO) integration test.
- **V. Observability & Safe Operations** — PASS. No new mutation. Connection delete (already logged + typed-confirm) unchanged. Secret stays masked + keychain-only; guidance must not mislead into plaintext env refs (FR-010).
- **Read-only structural guard** — PASS. No new write-capable S3 symbol leaves `internal/storage` (no storage edits at all). US6 post-mutation visibility is pure UI cache invalidation (FR-018) — no storage/config touch.

**Result**: No violations. No Complexity Tracking entries needed. Constitution v1.0.0; no amendment.

## Project Structure

### Documentation (this feature)

```text
specs/008-connection-form-ux/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── text-input-contract.md
│   ├── connection-ui-contract.md
│   ├── command-bar-contract.md
│   ├── key-label-contract.md
│   └── post-mutation-visibility-contract.md
└── checklists/
    ├── requirements.md  # from /speckit-specify
    └── ux.md            # from /speckit-checklist (follow-up defects)
```

### Source Code (repository root)

```text
internal/ui/
├── textfield.go         # NEW — rune-aware single-line editor (value + caret); insert/del/left/right/home/end/paste/render(masked,width)
├── textfield_test.go    # NEW — unit tests for the editor
├── app.go               # EDIT — add `case tea.PasteMsg` in Update; route paste to active text surface (search/command/connForm/op.input)
├── keys.go              # EDIT — keyGlyph: "ctrl+x"→"Ctrl+X", "ctrl+o"→"Ctrl+O"
├── commandbar.go        # EDIT — (US5) drop ALL three titles INFO/READ/WRITE (lines 162/148/191), keep columns+gap, relocate "(w to arm)" to write-group literal text; (US7) relabel "new conn"→"connections" at infoColumn:172 AND collapsedBarView:220, and reorder it ahead of droppable read entries on collapse (FR-020); (US8) add "Esc clear" read entry when searchActive()&&!searching; (US9) writeEntries shows only the selection-applicable delete
├── hintbar.go           # EDIT — chord nudge text reads naturally with the new glyph (trim "(Ctrl chord required)" tail)
├── connections.go       # EDIT — connForm uses textField; caret/paste editing; secret + per-field guidance; delete hint in list view
├── confirm.go           # EDIT — op.input uses textField (caret + paste) for the typed-confirm tier
├── confirmview.go       # EDIT — render op.input via textField window (caret position, not just tail)
├── operation.go         # EDIT (US6) — onOperationDone: precisely invalidate source + destination prefix keys (same bucket) for copy/move/bulk_copy so the result shows on later navigation (NOT cache.Clear)
├── tree.go              # EDIT (US6) — invalidateLevel(key) helper (beyond the current one), reuses parentPrefix; called from operation.go
└── *_test.go            # EDIT/NEW — textfield_test (new), connections_test, confirm/confirmview, hintbar_test (existing command-bar/title/^x asserts live here), operation (cross-prefix visibility) tests
```

**Structure Decision**: Single project; all changes under `internal/ui` (plus its tests). One new file pair (`textfield.go` / `textfield_test.go`) holds the shared editor; everything else edits existing files. No `internal/storage` or `internal/config` change — including US6 post-mutation visibility, which is purely UI cache invalidation (FR-018), so `make check-readonly` stays green.

## Complexity Tracking

No constitution violations — section intentionally empty.
