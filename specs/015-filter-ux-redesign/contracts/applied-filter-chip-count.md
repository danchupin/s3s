# Contract: Filter term in the form, match count in the title

> Design revision: the committed filter term is shown in the per-scope **form** (not a border
> chip); the **match count** rides the list box **title** above the form. This keeps the boxed
> form clean and avoids duplicating the count, which the list title already carries.

## Where each piece lives

```go
// app.go — the form renders the term; the title renders the count.
func (m App) bucketFilterField(w int) string   // label "filter buckets" + m.bucketFilter (term)
func (m App) objectFilterField(w int) string   // label "filter objects" + m.search (term)

func (m App) resourceTitle() string             // buckets: "buckets[M/T]" (filtered/total, local)
func (m App) objectsZoneTitle(w int) string     // objects: "…[N]"        (N matched; no total fetched)
```

## Rules

- **Term-in-form, focus-agnostic**: each scope's form shows its committed term whenever set,
  INDEPENDENT of which pane is focused. In the two-pane layout both forms show their terms at once.
- **Live while editing**: while a scope is being edited the form shows the live input + caret; the
  list title's count narrows live per keystroke (bucket `M/T` is recomputed instantly).
- **Count**: bucket title = `M/T` (matched/total, local, instant); object title = `N` matched only
  (no level total fetched — paginated server-side, FR-013).
- **Elision**: a long term elides with `…` inside the form; re-open `/` to see/edit the full term.
- **NO_COLOR**: the form is identifiable by text (`filter buckets`/`filter objects` + the term) and
  the count by the title text, not color alone.

## Acceptance

1. A committed bucket filter `dev` over 12 buckets, 3 matching → bucket form shows `dev`, the box
   title shows `buckets[3/12]`.
2. A committed object filter `log` with 8 loaded matches → object form shows `log`, the title `…[8]`.
3. Both filters committed → both forms show their terms simultaneously, regardless of focused pane.
4. Editing one scope shows its live input in that form; the other scope's form keeps its term.
