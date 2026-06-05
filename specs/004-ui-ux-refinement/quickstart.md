# Quickstart: UI/UX Refinement

How to build, run, and verify the footer/help/status redesign.

## Build & run

```bash
make build
./bin/s3s            # read-only (default)
./bin/s3s --write    # write mode (write hints + help Write section active)
```

## What changed (user-visible)

- **Footer is ≤ 3 rows**: one compact identity row (`● <context> [RW|RO] · <cluster>`),
  one contextual hint row, and a status row only when something is happening.
- **Hints adapt** to where you are and what's selected; on narrow terminals low-priority
  hints drop and a `? more` cue points you to help. `? help` and `q quit` always stay.
- **Connection details** (endpoint, region, user, version) now live in **help** (`?`),
  which is now organized into sections.
- **Status is clearer**: loading says *what* is loading; debounced search shows
  `searching…`; success notices are green, errors red.

## Manual verification

1. Launch read-only at ~120 cols → footer shows browse hints, **no** `d/u/y/m/D/+`.
2. Resize terminal narrow (~50 cols) → hint row stays one line, ends with `? more`,
   still shows `? help` and `q quit`.
3. `./bin/s3s --write`, enter a bucket, select an **object** → footer shows
   `d del`, `y copy`, `m move`; select a **folder** → shows `D rmdir` instead.
4. Press `?` → help shows Navigation / Search & View / Context / Write / Global /
   Connection sections, with key aliases and the connection metadata; press any key to
   close.
5. Open a large object → status reads `loading object…`; back at a level → `loading
   contents…`.
6. With a single context configured, confirm `1-9`/`context` hint is absent.

## Automated verification (TDD-first)

```bash
go test ./internal/ui/ -run 'TestFooter|TestHints|TestHelp|TestStatus|TestLoading'
make test            # full unit suite
make fmt vet lint    # gates
make check-readonly  # unchanged guard — must still pass (no SDK writes added)
```

Expected: new white-box tests in `footer_test.go` / `keys_test.go` / `app_test.go`
assert footer ≤ 3 rows, single-line hint row, contextual visibility, `? more` cue,
categorized help with aliases + connection section, and named loading. All pass; the
read-only structural guard is unaffected.

## Out of scope (do not expect)

- No command palette / fuzzy action search (deferred).
- No backend, storage, write-semantics, or confirmation-tier changes.
- No new keybindings or remaps (shortcuts only relocate in where they're advertised).
