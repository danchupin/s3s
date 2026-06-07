# Quickstart: Feature 011 — Three-zone master-detail browse + hotkey mnemonic review

This guide is for a developer **validating** feature 011. It covers building/running `s3s`, the test + guard gates, a User-Story-to-validation mapping, manual smoke steps at three terminal widths, the white-box test patterns to reuse, and the TDD order to follow.

Feature 011 has two threads:
- **(A) Three-zone master-detail browse** — `buckets | objects | details`, with per-zone focus, live objects listing on the *settled* bucket selection, and adaptive collapse across width tiers (US1, US2, US3).
- **(B) Hotkey mnemonic review + bold glyphs** — remove the now-redundant `n` AddConn key (the `+ add connection` row already exists at `connections.go:103`), keep `y`/`ctrl+o`, render every advertised key **bold**, and keep a non-color cue under `NO_COLOR` (US4).

Already-decided technical direction (validate *against* it, do not re-litigate): the objects zone reuses `storage.Storage.ListLevel(...)` (no new storage method — read-only guard stays green); the existing per-session level `cache` is shared with the full-screen level view; bucket-scroll debounce reuses `paneDebounce` (`internal/ui/commands.go:302`, `180 ms`, ceiling ≤ 200 ms) plus the `m.gen` generation drop; layout tiers reuse `boxView`/`windowBounds`/`lipgloss.JoinHorizontal` and the existing `paneSplitMin = 100` split (`internal/ui/app.go:929`); the keymap stays single-source in `keys.go` `defaultKeys()`.

---

## 1. Prerequisites

- Go toolchain matching `go.mod` (`go 1.25`). `golangci-lint` **must be built with this toolchain** or it refuses to run ("targeted Go version" error).
- No Docker is needed for any 011 validation — every 011 test is a white-box `package ui` unit test against `storage.Fake`. (Docker is only for `make test-integration`, which is unaffected here.)
- A terminal you can resize to **≥ 130**, **100–129**, and **≤ 99** columns for the manual smoke (US3 tier checks).

---

## 2. Build, run, and gate commands

```bash
make build              # -> bin/s3s
./bin/s3s               # run (uses your ~/.config/s3s config / active context)

make test               # unit tests (in-memory storage.Fake; no Docker) — ALL 011 tests live here
make fmt vet lint       # gofmt, go vet, golangci-lint (lint must be 0)
make check-readonly     # structural read-only guard — MUST stay green for 011

# single test / package while iterating:
go test ./internal/ui/ -run TestThreeZone
go test -cover ./internal/ui/
```

011 acceptance bar (mirror of the 010 bar): `go test` + `fmt` + `vet` + `lint(0)` + `check-readonly` all green; `internal/ui` coverage does not regress.

**Why `check-readonly` stays green:** 011 adds no S3 write symbol and no new `storage` method. The objects zone calls the existing read method `ListLevel` (`internal/storage/storage.go:109`); the details zone reuses the existing `HeadObject` + ranged `GetObjectRange` pane loads (`loadPaneMeta`/`loadPanePreview` in `commands.go`). The guard (`scripts/check-readonly.sh`) scans for `Put*/Delete*/Create*/Copy*/Upload*/...` outside `internal/storage/` — 011 introduces none.

---

## 3. User Story / Success Criterion → how to validate

White-box UI tests live in `internal/ui/*_test.go` (`package ui`). Drive the model with `deliver` / `press` / `pressCmd` and assert on `viewOf(m)` (= `m.View().Content`) or on model fields (`m.focus`, `m.treeSel`, `m.bucketSel`). Seed backend state with `storage.Fake`; assert listing counts via the **already-present** counters `f.ListBucketsCalls` and `f.ListLevelCalls` (`internal/storage/fake.go:30-31`). Failure/denied knobs: `f.FailListBuckets` and `f.AccessDeniedBuckets[name]=true` (`fake.go:25,28`). **No new Fake knob is required** — the list-call counters and both error knobs already exist.

| Story / SC | What it asserts | How to validate (white-box test or manual step) |
|---|---|---|
| **US1 — live objects on settled bucket; 0→Enter** (FR-002a, FR-026/027) | Highlighting a bucket loads its first object level into the objects zone *without* pressing Enter; a 0-keystroke flow shows contents. | Test: `withBuckets`, then fire the settle path (deliver `paneTickMsg`-equivalent settle / the bucket-settle tick) and assert `viewOf(m)` shows the objects-zone rows. Assert `f.ListLevelCalls == 1` for one settled selection and the first page was fetched. |
| **US1 — startup lists only NAMES** (FR-002a / SC-010) | Entering browse with K buckets issues exactly one bucket-NAME listing and **zero** object-level listings until a selection settles. | Test: build app + deliver `bucketsMsg`; assert `f.ListBucketsCalls == 1` and `f.ListLevelCalls == 0` *before* any settle; then settle and assert `ListLevelCalls == 1`. |
| **US2 — focus cross / Tab / drill** (FR-007/008/009) | `Tab` toggles focus `buckets↔objects` symmetrically; `Right`/`l`/`Enter`-on-bucket crosses into objects; `Left`/`h`/`Esc` ascends-or-returns. Active zone shows accent border/title, others dim. | Test: `press(m,"tab")` and assert focus field flips both directions; `press(m,"l")` (or `"right"`/`"enter"`) on a bucket and assert focus moved to objects + objects cursor armed; `press(m,"h")` and assert ascend/return. Assert the active title uses the accent style and the inactive box is dim in `viewOf`. |
| **US3 — adaptive details; collapse in Dual** (FR-013/015) | Full (≥130) shows `buckets\|objects\|details` with the *preserved* 006 details pane as a non-focusable third zone; Dual (100–129) collapses details; Single (≤99) is the current single-column mode-stack. Reflow across tiers keeps highlighted bucket, objects cursor, and focused zone. | Test: deliver `tea.WindowSizeMsg{Width:140}` and assert three boxes (count `lipgloss.JoinHorizontal` segments / box borders) incl. the `details` title; `Width:115` → two boxes, no `details`; `Width:90` → single box. Resize 140→115→140 and assert `m.bucketSel`, objects cursor, and focus survive. |
| **US4 — bold keys; `n` removed** (FR-019..025) | Every advertised key is rendered **bold** (today they use `accentStyle` foreground only); `n` (AddConn) is gone from the keymap and from hint/help; `y` (Copy) and `ctrl+o` (MoveChord) remain. The `+ add connection` row (`connections.go:103`) is the sole add-connection affordance. | Test: assert `defaultKeys().AddConn` is empty/removed and pressing `n` in `modeBuckets`/`modeTree` does **not** open connections (`m.mode` unchanged). Assert `defaultKeys().Copy == ["y"]` and `defaultKeys().MoveChord == ["ctrl+o"]` still hold. Assert the rendered hint/help key cells carry the bold SGR (or, under `NO_COLOR`, a non-color cue). |
| **SC-003 — debounce on fast scroll** | N intermediate bucket selections produce ≤ 1 listing; the settled selection fetches within ≤ 200 ms; input is never blocked. | Test: drive several `press(m,"down")` rapidly *without* delivering the settle tick for the intermediate rows (bump the settle generation each move), then deliver the settle tick only for the final row; assert `f.ListLevelCalls == 1`. (Reuses `paneDebounce = 180 ms`, `commands.go:302`.) |
| **SC-004 — cache on revisit** | Revisiting a bucket viewed earlier this session shows contents with **no** extra listing (served from `internal/cache`). | Test: settle bucket A (`ListLevelCalls == 1`), move to B and settle (`== 2`), return to A and settle; assert `ListLevelCalls` did **not** increment for A's revisit (still `2`). |
| **SC-010 — lazy + denied not cached** | Exactly one bucket-name listing and zero object listings on entry; an object listing happens only after a settle; a **denied** object listing is re-attempted (not cached) on the next revisit. | Test: set `f.AccessDeniedBuckets["denied"]=true`; settle on it → error shown, level **not** cached; revisit and settle again → `ListLevelCalls` increments again (re-attempt), confirming errors are not cached (FR-006b/c). |
| **Read-only guard** (Constitution I/V) | No write symbol or new storage method introduced. | `make check-readonly` PASS; `grep` confirms 011 code touches only `ListLevel`/`HeadObject`/`GetObjectRange` reads. |

---

## 4. Manual smoke (three width tiers)

Run `./bin/s3s` against a context with several buckets (or a scoped/pinned connection per 010). Then:

**Wide — ≥ 130 cols (Full tier: `buckets | objects | details`)**
1. Confirm three side-by-side bordered zones: bucket list (left), objects (middle), details (right).
2. Move the bucket cursor with `↑/↓` or `j/k`. Pause on a bucket: within ~180 ms (≤ 200 ms) the **objects** zone fills with that bucket's first level — **no Enter pressed** (US1).
3. Scroll the bucket list fast through several buckets, then stop. Only the **settled** bucket's contents load; intermediate buckets are not fetched (SC-003).
4. Press `Tab`: focus moves `buckets → objects`; the active zone shows the accent border/title, the others dim. `Tab` again returns to buckets (symmetric, US2).
5. From a focused bucket, press `Right`/`l`/`Enter` to cross into objects; press `Left`/`h`/`Esc` to return/ascend (US2 / FR-009).
6. Highlight a bucket → details shows bucket metadata; cross into objects and highlight an object → details shows object metadata + the bounded preview (US3, preserved 006 pane).
7. Revisit a previously viewed bucket → contents appear instantly with no spinner reload (cache, SC-004).

**Medium — 100–129 cols (Dual tier: `buckets | objects`)**
1. Resize down across 130. The **details** zone collapses first; `buckets | objects` remain side by side (FR-015).
2. Verify the highlighted bucket, the objects cursor, and the focused zone all survive the resize (no reset).
3. `Tab` / cross-focus still works between the two visible zones.

**Narrow — ≤ 99 cols (Single tier: current mode-stack)**
1. Resize below 100 (the existing `paneSplitMin`). The objects zone collapses; the UI returns to the current single-column mode-stack (buckets → tree on Enter), exactly as before 011.
2. Confirm the footer/hints line is still visible (height budgeting — the box body must not push the footer off; see `boxView` `minRows` cap).

**Hotkeys (any width — US4)**
1. Open help (`?`). Confirm advertised keys render **bold**. Confirm `n` is **absent** as an add-connection key; the only add-connection affordance is the `+ add connection` row in the connection manager.
2. Confirm `y` (copy) and `ctrl+o` (move chord) still work.
3. Re-run with `NO_COLOR=1 ./bin/s3s`: confirm key glyphs still carry a non-color cue (bold/text marker) so advertised keys stay legible (FR-025).

---

## 5. White-box test patterns to reuse

All helpers are in `internal/ui/app_test.go`:
- `newApp(f, contexts, resolve)` / `withBuckets(f, contexts, resolve)` — build an App on a `storage.Fake`; `withBuckets` also delivers the initial `bucketsMsg`.
- `deliver(m, msg)` — run one `Update` and return the new `App`.
- `press(m, "down")`, `press(m, "tab")`, `press(m, "l")`, `press(m, "ctrl+o")` — synthesize a `tea.KeyPressMsg` (see `keyMsgFor`); `pressCmd` also returns the `tea.Cmd` when you need to inspect/run the resulting command.
- `viewOf(m)` = `m.View().Content` — assert on rendered text.
- For a connection seam, use `fakeConnector` from `internal/ui/connections_test.go` (records `Test`/`Save`/`Delete`/`AddBucket`); `connApp` / `bucketsConnApp` (see `pinned_test.go`) wire it in.

`storage.Fake` knobs (all already exist — **no new counter knob needed**):
- `f.ListBucketsCalls` / `f.ListLevelCalls` (`fake.go:30-31`) — assert exact listing counts for lazy/debounce/cache SCs.
- `f.FailListBuckets = true` (`fake.go:25`) — list-all returns `ErrAccessDenied`.
- `f.AccessDeniedBuckets["bucket"] = true` (`fake.go:28`) — `ListLevel` on that bucket returns `ErrAccessDenied` (used for the SC-010 "denied not cached" re-attempt).
- `f.Seed(bucket, keys...)` / `f.SeedObject(...)` — shape the tree.

Counting + settle pattern (SC-003 / SC-010), since the bucket-settle path reuses the pane debounce (`paneDebounce`, gen-suppressed):
1. `m := withBuckets(f, ...)` → assert `f.ListBucketsCalls == 1`, `f.ListLevelCalls == 0`.
2. `press(m,"down")` several times (each bumps the settle generation; intermediate ticks are dropped).
3. Deliver the settle tick **only** for the final selection (run the `tea.Cmd` returned by the last move, or `deliver` the settle message with the current generation), then `deliver` the resulting `levelMsg`.
4. Assert `f.ListLevelCalls == 1`. For the cache check, revisit and assert it stays `1` for the already-loaded level; for the denied check, assert it **increments** on re-attempt.

Stale-drop pattern (Constitution II): every result message carries the `gen` it was issued under; delivering a `levelMsg`/`paneMetaMsg` with `gen != m.gen` (or `paneGen`) must be a no-op (`app.go` Update already drops these). Use a wrong gen to assert supersession.

---

## 6. TDD order (Constitution III — write the failing test first)

Per story, write the **failing** test, then implement to green. Run `make test` after each.

**US4 — hotkey review (cheapest, fully decoupled; ship first):**
1. Failing: `n` no longer opens connections in `modeBuckets`/`modeTree` (`m.mode` unchanged after `press(m,"n")`); `defaultKeys().AddConn` is removed/empty.
2. Failing: `y == Copy`, `ctrl+o == MoveChord` still bound (regression guard).
3. Failing: rendered hint/help key cells carry **bold** (and a `NO_COLOR` non-color cue). Implement by adding `Bold(true)` to the key style and updating `keys.go`/`hintbar.go`/help — keymap stays single-source so dispatch, hint bar, and help cannot drift.

**US1 — live objects on settled selection + lazy/cache:**
4. Failing: startup issues exactly one bucket-name listing, zero object listings (`ListBucketsCalls == 1`, `ListLevelCalls == 0`).
5. Failing: settling on a bucket triggers exactly one `ListLevel` and renders the objects zone (0-Enter).
6. Failing: fast-scroll N selections → ≤ 1 `ListLevel` (SC-003, debounce + gen drop).
7. Failing: revisit served from cache, no extra listing (SC-004); denied listing re-attempted, not cached (SC-010 / FR-006b/c).

**US2 — focus + crossing:**
8. Failing: per-zone cursor state exists; `Tab` toggles focus symmetrically.
9. Failing: `Right`/`l`/`Enter` crosses buckets→objects; `Left`/`h`/`Esc` ascends-or-returns (FR-009).
10. Failing: active zone renders accent border/title, inactive zones dim.

**US3 — adaptive layout (build last; depends on US1/US2 zone state):**
11. Failing: `WindowSizeMsg{Width:140}` → three zones incl. `details`; `115` → two zones, no details; `90` → single column.
12. Failing: resize across tier boundaries preserves highlighted bucket, objects cursor, and focused zone (FR-015).

Finally, run the full gate: `make test fmt vet lint check-readonly` — all green, coverage non-regressed.

---

**Key files to touch (all under `internal/ui`, storage read-only):** `app.go` (`View`, `listWithPane`, `paneSplitMin`, `Update` dispatch, `beginLoad`, `gen`), `pane.go` (details zone), `styles.go` (`boxView`, `windowBounds`, `renderTable` + bold key style), `keys.go` (`defaultKeys`, `formatKeys`, `helpLines`, `keyGlyph` — remove `AddConn`, add `Bold`), `commands.go` (`loadBuckets`, `paneDebounce`/settle tick, `ListLevel` load), `hintbar.go` (`actionCatalog`/hint rendering), `commandbar.go` (`commandBarView`), `connections.go` (the surviving `+ add connection` row at line 103), `messages.go`, `tree.go`. Reused unchanged: `internal/cache`, `internal/storage` (interface + `Fake`).
