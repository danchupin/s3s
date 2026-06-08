# Contract: Count-Bearing, Term-Gated Indicator Chip

## API (internal)

```go
// app.go — filterChipText gains count params.
func (m App) filterChipText(term string, matched, total int, hasTotal bool) string
// "filter: <term> · M/T"  (hasTotal == true,  bucket scope)
// "filter: <term> · N"    (hasTotal == false, object scope)

func (m App) bucketFilterChip() string  // → filterChipText(m.bucketFilter, len(filteredBuckets()), len(m.buckets), true)
func (m App) objectsFilterChip() string // → filterChipText(m.search, m.level.count(), 0, false)
```

## Rules

- **Term-gated**: a scope's chip renders whenever that scope has a committed term, INDEPENDENT of
  which pane is focused. In the two-pane layout both chips show at once (bucket chip on the bucket
  box, object chip on the objects box).
- **Hidden while editing that scope**: a scope's chip hides only while it is being actively edited
  (`searching` on that scope) — its live term shows in the strip instead; the OTHER scope's chip
  stays.
- **Count**: bucket = matched/total (local, instant); object = matched only (no level total
  fetched).
- **Elision/degradation**: term elides first to fit `filterChipTermMax` + the ` · M/T` suffix;
  under width pressure the whole filter chip drops (mode chip survives); the strip still shows the
  active filter.
- **NO_COLOR**: the chip is identifiable by text (`filter:` + term + count), not color alone.

## Acceptance

1. A committed bucket filter `dev` over 12 buckets, 3 matching → chip `filter: dev · 3/12`.
2. A committed object filter `log` with 8 loaded matches → chip `filter: log · 8`.
3. Both filters committed → both chips visible simultaneously, regardless of focused pane.
4. Focus moves to the other pane → both chips remain visible.
5. Editing the bucket filter hides the bucket chip (term is live in the strip) but keeps the
   object chip.
