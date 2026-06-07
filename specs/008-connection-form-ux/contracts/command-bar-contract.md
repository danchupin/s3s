# Contract: Command Bar without Block Headings (US5)

## Rendering (FR-013, FR-014)

- The command bar MUST NOT render ANY block heading text — `INFO`, `READ`, or `WRITE`
  (all three are `blockTitleStyle` rows: INFO at commandbar.go:162, READ at :148, WRITE at :191).
- Info, read, and write entries MUST remain in **separate columns** (the ≥2-space inter-column
  gap supplies the grouping; on the collapsed bar, read row and write row are separate lines).
- Read-only cue: when the context is not writable, the literal text `w to arm` (amber) MUST
  appear in the write group (lead row, in place of the removed title); when writable, it is absent.
- The collapsed (narrow) bar MUST have no orphaned heading text and keep its grouping.

## Invariants

- B1: no occurrence of `"INFO"`, `"READ"`, or `"WRITE"` heading strings in `App.View().Content`.
- B2: info / read / write entries are still distinguishable by layout (distinct columns + gap).
- B3: under read-only, the literal `w to arm` is present (NO_COLOR-safe text, not color-only).
- B4: all existing entries (keys + labels) still render; only the titles are removed.

## Connection affordance (US7, FR-019, FR-020)

- The command-bar connection entry (bound to AddConn) MUST be labelled "connections" (NOT
  "new conn") at BOTH render sites: `infoColumn` (commandbar.go:172) AND the collapsed read row
  (`collapsedBarView`, commandbar.go:220). It opens the manager (switch/add/delete).
- Shown whenever a connection manager is available (as today); not gated on connection count.
- On the collapsed bar it MUST be ordered ahead of droppable read entries (or in the
  `fitEntries` keep-min set) so trimming does not drop it first — a relabel alone is insufficient
  because it is currently appended last and `fitEntries` drops trailing entries first.

## Filter-reset affordance (US8, FR-021)

- When a filter/search is applied AND not actively being typed (`searchActive() &&
  !searching`), the read group MUST show a reset entry (`glyph(Esc)` + label "clear").
- When no filter is active, the entry is absent.
- Triggering it restores the full unfiltered list (reuses the existing clear path).

## No duplicate delete labels (US9, FR-022)

- The write group MUST show only the delete action applicable to the current selection:
  object/group delete for an object cursor; recursive delete for a folder cursor.
- The inapplicable delete entry is suppressed (not rendered dimmed) — targeted exception to
  007's "all write always shown", for the delete pair only.
- Result: no two write-group entries share an identical label.

## Acceptance (tests, written first)

1. Wide terminal, writable context → View has no `INFO`/`READ`/`WRITE` title; info keys, read
   keys (`open`,`/`) and write keys still present in separate column areas.
2. Read-only context → View has no titles AND contains the literal `w to arm`.
3. Narrow terminal (collapsed) → renders without title text, grouping intact.
4. Command bar contains a "connections" entry (not "new conn") whenever the manager is available.
5. Triggering "connections" opens the manager (switch/add/delete).
6. Collapsed bar at a width that forces dropping read entries → the "connections" entry SURVIVES
   (not dropped first).
7. Filter applied (not typing) → read group contains "clear"; no filter → no "clear".
8. Object cursor → write group has the object delete only (no recursive entry). Folder cursor
   → recursive delete only (no object-delete entry). No two identical labels in either.
