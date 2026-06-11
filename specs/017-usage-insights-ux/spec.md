# Feature Specification: Budgeted Usage, Insights & Details UX

**Feature Branch**: `017-usage-insights-ux`

**Created**: 2026-06-11

**Status**: Draft

**Input**: User description: "Оформить вариант 2 (бюджетный ambient-скан объёма + полный скан по явному действию) как спеку. Дополнительно пересмотреть удобность, читабельность и UI/UX панели метаданных и фичей анализа: что поправить, что убрать, что добавить. Дать максимально подробную и удобную информацию о бакетах и объектах для разработчиков и операторов кластеров. Киллер-фичи, выгодно выделяющие продукт среди s3cmd и MinIO CLI."

**Scope decision (user-confirmed)**: base (budgeted scan + details-pane UX overhaul) + copy/share
affordances + operator health card + preview upgrades. Object **version browsing stays out of
scope** (carried over from 016).

## The problem being solved

Today the inline usage feature starts an **unbounded recursive enumeration** of the focused
bucket/prefix after a 180 ms hover. For a small bucket that is a pleasant ambient detail; for a
large one (millions of objects) it is a multi-minute stream of listing requests against the
storage cluster that the user almost never waits out — the partial progress is then discarded,
so the cluster pays repeatedly and the user receives nothing. Separately, the details pane has
grown into a flat, hard-to-scan list of label/value rows, and the product lacks the
"finish-the-job" affordances (copy a usable link, export a report, see where a bucket is
wasting space) that would make a developer or a cluster operator choose it over `s3cmd` /
`mc`.

## Clarifications

### Session 2026-06-11

- Q: Где рендерится операторская health-карта (US4)? → A: Отдельный полноэкранный вид по
  явному действию; inline details-панель сохраняет только totals + breakdown.
- Q: Поведение раскрытия breakdown ('a'/:detail) при partial/отсутствующих данных? → A:
  Раскрытие показывает уже собранные данные (бюджетный скан, если данных нет) и отдельный
  явный аффорданс «full scan»; безлимитная работа никогда не стартует неявно.
- Q: Как вызываются copy/share-действия? → A: Одно copy-меню на одной клавише; состав пунктов
  зависит от фокуса (объект / бакет / префикс / открытый отчёт).
- Q: Пресеты срока действия presigned GET-линка? → A: 15 минут / 1 час / 24 часа / 7 дней;
  default 1 час; 7 дней — потолок схемы подписи, свободного ввода нет.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Budgeted ambient usage with explicit full scan (Priority: P1)

As a user browsing buckets, when my cursor rests on a bucket or folder I see its size and
object count appear automatically **only up to a fixed, small work budget**. If the target is
bigger than the budget, the scan stops at the bound and shows what it learned as an explicit
lower bound ("≥ 20 000 objects, ≥ 4.2 GiB — partial"), together with an affordance to run the
full scan deliberately. The full scan runs only when I ask for it, shows live progress, can be
cancelled, and — crucially — whatever it managed to count is kept as a lower bound instead of
being thrown away.

**Why this priority**: This is the cluster-safety fix. Without it the feature actively harms
large shared clusters (continuous index-pool load) while delivering no value for exactly the
buckets where size information matters most.

**Independent Test**: Seed a fake backend with more objects than the budget; rest the cursor on
the bucket; verify enumeration stops at the budget and the pane shows a lower bound plus the
full-scan affordance. Trigger the full scan; verify progress, cancellation, and that a
cancelled full scan leaves a cached lower bound.

**Acceptance Scenarios**:

1. **Given** a bucket whose object count exceeds the scan budget, **When** the cursor dwells on
   it, **Then** background enumeration stops after the budgeted amount and the details pane
   shows totals marked as a lower bound ("≥") with a visible hint for running the full scan.
2. **Given** a bucket smaller than the budget, **When** the cursor dwells on it, **Then** exact
   totals appear (no lower-bound marker), identical to today's behaviour.
3. **Given** a partial (budget-bounded) result is shown, **When** the user triggers the full
   scan, **Then** a progress indicator shows running totals, the scan is cancellable, and on
   completion the exact totals replace the lower bound in the session cache.
4. **Given** a running full scan, **When** the user navigates away or cancels, **Then** the
   counted progress is retained and shown as a lower bound on return — never discarded.
5. **Given** the scan budget is configured to zero, **When** the cursor dwells anywhere,
   **Then** no ambient enumeration happens at all; usage appears only via the explicit action.
6. **Given** a previously inspected target (exact or partial), **When** the user returns to it
   in the same session, **Then** the cached result appears instantly with no new requests.

---

### User Story 2 - A details pane people can actually read (Priority: P1)

As a user inspecting an object or bucket, the details pane presents information in named,
visually separated groups — identity & content, security & governance, delivery/HTTP, user
metadata — instead of one flat label column. Dates read as both relative ("3 days ago") and
exact. Field states are visually distinct: a denied value looks different from "not set" and
from "unknown". A multipart upload's checksum is explained ("multipart, 7 parts — not a content
hash") instead of silently puzzling the user. Any single field value can be copied without
copying the whole pane.

**Why this priority**: Readability is the product's core promise (constitution VI/VII). The 016
enrichment added up to a dozen optional rows; without grouping and state colour the pane is a
wall of text and the new data works against the user.

**Independent Test**: Render the details pane for a fully-enriched object at a constrained
terminal size; verify group headers, relative+exact dates, distinct denied/none/unknown
styling, the multipart explanation, and per-field copy.

**Acceptance Scenarios**:

1. **Given** an object with core, security and delivery fields populated, **When** the details
   pane renders, **Then** fields appear under named group headers in a stable order, and empty
   optional groups are omitted entirely.
2. **Given** a field whose read was denied and a field that is genuinely unset, **When** the
   pane renders, **Then** the two are visually and textually distinct (e.g. "denied" vs "—"),
   using palette roles rather than ad-hoc colours.
3. **Given** an object uploaded in multiple parts, **When** its checksum row renders, **Then**
   the part count is shown with a note that the value is not a content hash.
4. **Given** any visible field, **When** the user invokes per-field copy on it, **Then** the
   full untruncated value lands in the clipboard and the footer confirms what was copied.
5. **Given** a 130×24 terminal, **When** any enriched object or bucket is inspected, **Then**
   every value is either fully visible or explicitly revealable — nothing silently truncated
   off-screen (height-budget rule carried over from 016).
6. **Given** an object modified recently, **When** the date renders, **Then** both a relative
   form and the exact timestamp are available to read.

---

### User Story 3 - Copy & share affordances (Priority: P2)

As a developer who found the object I was hunting for, I can — in a couple of keystrokes —
copy: its canonical storage URI, a direct HTTPS URL (matching the endpoint's addressing
style), a ready-to-run download command for common CLI tools, or a **presigned time-limited
GET link** I can hand to a teammate who has no credentials. For a bucket or prefix I can copy
its URI, and I can export the current usage/health report to a local CSV or JSON file.

**Why this priority**: This is the "finish the job" moment. Competing CLIs make link/command
construction a manual chore; one-keystroke share converts every successful browse into a
shareable artifact. Generating a presigned link is a pure client-side computation — no storage
request, no mutation — so the read-only posture holds.

**Independent Test**: On a fake backend, invoke each copy action on an object, a prefix and a
bucket; verify clipboard payloads (URI shape, URL addressing style, command correctness,
presigned link validity window) and the export file contents.

**Acceptance Scenarios**:

1. **Given** an object is focused, **When** the user opens the copy menu, **Then** the choices
   include storage URI, direct HTTPS URL, download command snippet, and presigned GET link.
2. **Given** the active endpoint uses path-style addressing, **When** a direct URL is copied,
   **Then** the URL is path-style; with virtual-host-style addressing it is virtual-host-style.
3. **Given** the user requests a presigned link, **When** they confirm the validity duration
   (sensible default offered), **Then** a working time-limited GET link is produced without any
   request to the backend, and the link itself is never written to logs.
4. **Given** temporary credentials that expire before the chosen link duration, **When** the
   link is generated, **Then** the user is warned the link dies with the credentials.
5. **Given** a usage/health report is on screen, **When** the user exports it, **Then** a local
   CSV or JSON file is written and the footer names the path; a write failure is reported
   without leaving a misleading partial file.
6. **Given** the clipboard is unavailable (e.g. remote session), **When** a copy action runs,
   **Then** the value is displayed for manual copying instead of failing silently.
7. **Given** any copy action succeeds, **Then** the footer states what was copied (which kind
   of value, for which key) — never just "copied".

---

### User Story 4 - Operator health card for a bucket or prefix (Priority: P2)

As a cluster operator, I can open — by an explicit action — a **dedicated full-screen "health
card" view** for a bucket or prefix that answers, on one screen: how the data ages
(last-modified distribution), how it is sized (size distribution, with an explicit warning when
a large share of objects is small enough to pressure the index), how it spreads across storage
classes, and how much is wasted in **incomplete multipart uploads** (count, accumulated size
where available, oldest age). The inline details pane keeps only totals and the ranked
breakdown; the card is its own view, dismissed back to browsing with the standard back action.
The distributions are computed from the same enumeration the usage scan already performs —
asking for more insight does not cost the cluster more requests.

**Why this priority**: This is the operator killer feature. Today answering "what is stale,
what is fragmented, what is silently costing money in this bucket" requires scripting against
raw CLI output. No mainstream S3 CLI shows it interactively.

**Independent Test**: Seed a fake backend with a known age/size/class mix and incomplete
uploads; request the health card; verify each distribution, the small-object warning trigger,
and the incomplete-upload figures; verify denied/unsupported sub-results render as such, never
as zero.

**Acceptance Scenarios**:

1. **Given** a completed full scan of a target, **When** the user requests the health card,
   **Then** age, size and storage-class distributions render from the already-collected data
   with no additional per-object requests.
2. **Given** only a budget-bounded (partial) scan exists, **When** the health card is shown,
   **Then** every figure is explicitly marked as a lower bound / sample, never presented as
   exact.
3. **Given** a bucket where more than the configured share of objects falls below the
   small-object threshold, **When** the card renders, **Then** an explicit index-pressure
   warning appears naming the share and threshold.
4. **Given** incomplete multipart uploads exist, **When** the card renders, **Then** their
   count, total size where the backend reports it, and the oldest upload's age are shown.
5. **Given** the incomplete-uploads listing is denied or unsupported by the backend, **When**
   the card renders, **Then** that line reads "denied"/"unsupported" — never "0" or "clean".
6. **Given** a 130×24 terminal, **When** the full-screen health card is open, **Then** the
   footer/hint line stays visible and every figure is readable or revealable — card content
   adapts (collapses/scrolls) rather than pushing chrome off-screen.
7. **Given** the health card is open, **When** the user presses the standard back action,
   **Then** they return to the exact browsing position they came from.

---

### User Story 5 - Previews that understand the payload (Priority: P3)

As a developer previewing objects, structured text is readable: JSON and NDJSON are
pretty-printed (with a toggle back to raw), gzip-compressed objects are transparently
decompressed for preview (showing both compressed and decompressed sizes), and binary objects
get a proper hex dump (offset + hex + printable column) instead of a one-line summary.

**Why this priority**: Valuable polish, but the product is already usable for plain text and
images; this story rides on the existing preview pipeline without new backend reads.

**Independent Test**: Preview a JSON object, a gzip text object, an NDJSON object and a binary
object on a fake backend; verify pretty/raw toggle, decompression with size indicators and
caps, and the hex dump format.

**Acceptance Scenarios**:

1. **Given** an object classified as JSON, **When** previewed, **Then** it renders
   pretty-printed; a toggle returns the raw bytes; malformed JSON falls back to raw without an
   error banner.
2. **Given** a gzip-compressed text object, **When** previewed, **Then** decompressed content
   is shown within the preview cap, with both compressed and decompressed sizes indicated and
   truncation flagged when the cap is hit.
3. **Given** a binary object, **When** previewed, **Then** a hex dump renders within the
   preview cap (offset, hex bytes, printable characters).
4. **Given** NDJSON content, **When** previewed, **Then** each record renders pretty-printed in
   order within the cap.

---

### Edge Cases

- Target's object count exactly equals the budget → result is exact, no lower-bound marker.
- Listing becomes denied partway through a scan → accumulated totals shown as lower bound with
  the denial indicated; never rendered as a silent exact result.
- The user spams focus moves across many large buckets → at most one enumeration in flight;
  each abandoned ambient scan cost at most the budget.
- Budget changed in the config file → takes effect on next start (no runtime reload); session
  caches do not persist across restarts, so no stale-budget interaction exists.
- Presigned-link duration exceeding what the signing scheme permits → duration choices are
  capped at the maximum the scheme allows.
- Export target path unwritable / disk full → error reported in the footer; no half-written
  file presented as success.
- Decompressed preview exceeding the preview cap (compression bomb) → hard cap on decompressed
  bytes, truncation indicator, both sizes shown.
- Health card on an empty bucket → renders zeros honestly (zero is exact here), no warnings.
- Incomplete-upload listing slow on a huge bucket → loaded lazily as its own cancellable
  background read; the rest of the card renders without waiting for it.
- Clipboard integration absent (no local clipboard utility, restricted terminal) → value shown
  in a revealable form for manual copy.
- Narrow terminal (single-pane tier) → ambient scanning stays disabled exactly as today (no
  pane to show the result in); explicit actions still work from the full-screen views.

## Requirements *(mandatory)*

### Functional Requirements

**Budgeted usage scan**

- **FR-001**: The ambient (hover-triggered) usage scan MUST stop after enumerating the
  configured budget of objects (default 20 000) and present accumulated totals as an explicit
  lower bound, visually distinct from an exact result.
- **FR-002**: Budget-bounded (partial) results MUST be cached for the session like exact
  results, marked partial, and shown instantly on revisit without rescanning.
- **FR-003**: A full (unbudgeted) scan MUST start only via its own dedicated, explicit user
  action, MUST show live running totals, and MUST be cancellable at any time. Expanding the
  breakdown or opening the health card MUST NOT itself start a full scan: with no or partial
  data those views render what was collected (triggering at most a budgeted scan when nothing
  exists yet) alongside the visible full-scan affordance.
- **FR-004**: Progress of a cancelled or interrupted full scan MUST be retained and cached as a
  lower bound — partial work is never discarded.
- **FR-005**: A completed full scan MUST replace the partial entry for its target in the
  session cache.
- **FR-006**: The scan budget MUST be user-configurable; the value zero MUST disable ambient
  scanning entirely (explicit-only mode).
- **FR-007**: Existing 016 safeguards MUST be preserved: dwell gating before any ambient scan,
  cancel-on-navigate, at most one enumeration in flight, per-context session cache invalidated
  by manual refresh.

**Details-pane readability**

- **FR-008**: Object and bucket details MUST render in named groups (identity & content;
  security & governance; delivery/HTTP; user metadata) with visible group headers; groups with
  no populated fields are omitted.
- **FR-009**: Field states MUST be visually distinct via palette roles: populated, not set,
  permission-denied, unknown, and unsupported are each distinguishable without reading
  carefully.
- **FR-010**: Timestamps MUST offer both a relative form and the exact timestamp.
- **FR-011**: A multipart-uploaded object's checksum row MUST state the part count and that the
  value is not a content hash.
- **FR-012**: The user MUST be able to copy any single field's full (untruncated) value; the
  footer MUST confirm which value was copied.
- **FR-013**: The pane MUST keep the 016 height-budget guarantees: one expandable section at a
  time, "+N more" reveal, footer never scrolled off, every value visible or revealable at
  130×24.

**Copy & share**

- **FR-014**: All copy/share actions MUST be reachable from a single copy menu opened by one
  key; the menu's items adapt to the focus. For a focused object the user MUST be able to
  copy: the canonical storage URI, a direct HTTPS URL that matches the endpoint's addressing
  style, and a ready-to-run download command snippet; for a focused bucket or prefix, the
  corresponding URI; with a usage/health report visible, its export actions.
- **FR-015**: The user MUST be able to generate a presigned, time-limited GET link for an
  object entirely client-side (no storage request), choosing a validity duration from exactly
  four presets — 15 minutes, 1 hour (default), 24 hours, 7 days — with no free-form input;
  7 days is the signing scheme's hard maximum.
- **FR-016**: The presigned link MUST never be written to logs (it is a bearer capability);
  the log MAY record that a link was generated, for which key, and its duration.
- **FR-017**: When the active credentials are temporary and expire before the chosen link
  duration, the user MUST be warned at generation time.
- **FR-018**: The user MUST be able to export the current usage/health report for a target to
  a local CSV or JSON file; failures are reported and never presented as success.
- **FR-019**: When no clipboard is available, copy actions MUST fall back to displaying the
  value for manual copy.

**Operator health card**

- **FR-020**: On explicit request for a bucket or prefix, the product MUST open a dedicated
  full-screen health-card view presenting: an age (last-modified) distribution, a size
  distribution, and a storage-class distribution, all computed from data already collected by
  the usage enumeration — no additional per-object requests. The standard back action returns
  to the prior browsing position; the inline details pane is unchanged by this feature beyond
  US1/US2.
- **FR-021**: The health card MUST surface incomplete multipart uploads for the target: count,
  accumulated size where the backend reports it, and the age of the oldest; the listing is a
  lazily-loaded, cancellable background read.
- **FR-022**: Incomplete-upload and any other card sub-result MUST reuse the established
  tri-state semantics — configured/none/denied/unsupported — and MUST never render a denied or
  unsupported probe as zero/clean.
- **FR-023**: When more than a configured share (default 50%) of enumerated objects is smaller
  than a configured threshold (default 128 KiB), the card MUST show an explicit small-object
  index-pressure warning naming both numbers.
- **FR-024**: A health card built from partial (budget-bounded) data MUST mark every figure as
  a lower bound / sample; exact presentation requires a completed full scan.

**Preview upgrades**

- **FR-025**: JSON and NDJSON previews MUST render pretty-printed with a raw/pretty toggle;
  content that fails to parse falls back to raw silently.
- **FR-026**: Gzip-compressed objects MUST be transparently decompressed for preview, with a
  hard cap on decompressed bytes, a truncation indicator when capped, and both sizes shown.
- **FR-027**: Binary objects MUST offer a hex-dump preview (offset, hex bytes, printable
  column) within the existing preview cap.

**Cross-cutting**

- **FR-028**: Every new backend interaction MUST be a read; the structural read-only guard
  MUST stay green; new failure modes map to the existing error sentinels.
- **FR-029**: All new keys/affordances MUST appear in the hint system and follow the shared
  prompt/label patterns (constitution VII); no new colour outside palette roles.

### Key Entities

- **Scan Budget**: per-user configurable cap (object count) on ambient enumeration; zero
  disables ambient scanning.
- **Usage Report (extended)**: totals + ranked children, now carrying completeness (exact vs
  lower bound), and the age/size/storage-class distributions gathered during enumeration.
- **Health Card**: the on-demand, full-screen operator view assembled from a Usage Report plus
  the incomplete-uploads probe and derived warnings; never the trigger of an implicit full scan.
- **Incomplete Upload Entry**: one in-progress multipart upload — key, initiated time,
  accumulated size where available.
- **Share Artifact**: a copyable value produced from a focused entity — URI, direct URL,
  command snippet, or presigned link (with validity window).
- **Export Report**: local CSV/JSON serialization of a Usage Report / Health Card.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Resting the cursor on a bucket of any size triggers no more background
  enumeration than the configured budget — for a 10-million-object bucket that is ≤0.2% of the
  work the current implementation performs.
- **SC-002**: For any target, the user sees a usage answer (exact or explicit lower bound)
  within 3 seconds of dwelling, in 95% of cases on a healthy backend.
- **SC-003**: Returning to any previously inspected target shows its usage instantly with zero
  new backend requests.
- **SC-004**: A user can produce a working shareable link or download command for a focused
  object in at most 3 keystrokes, without leaving the terminal.
- **SC-005**: An operator can answer "which prefix holds the bulk, what share is stale, what is
  wasted in incomplete uploads" for a scanned bucket from one screen, without external tools.
- **SC-006**: At 130×24, 100% of details-pane values for a fully-enriched object are readable
  or explicitly revealable (verified by a height-sweep over seeded data).
- **SC-007**: A denied or unsupported probe is never displayed as a zero/clean result anywhere
  in the details pane or health card (verified over all probe failure modes).
- **SC-008**: Developers can read JSON, gzipped-text and binary payloads in-product, with no
  external CLI round-trip, for objects within the preview cap.

## Assumptions

- Default scan budget: 20 000 enumerated objects (≈ a small number of listing pages);
  configurable in the existing config file alongside other per-user settings.
- Small-object warning defaults: threshold 128 KiB, share 50%; both configurable.
- Presigned links: GET only (read-only posture preserved — the product still cannot mutate the
  backend); validity presets 15 m / 1 h (default) / 24 h / 7 d, no free-form input; 7 days is
  the signing scheme's hard limit. Generating a link makes no backend request.
- Distributions (age/size/class) are byproducts of the enumeration the usage scan already
  performs; they add no listing requests. Incomplete-uploads listing is the only new request
  type for the health card and is itself a read.
- Bucket inventory/analytics facilities of specific backends (e.g. admin APIs) are out of
  scope — only the standard S3 protocol surface is used, since the primary targets are Ceph
  RGW and MinIO.
- The clipboard mechanism is environment-dependent; a display-for-manual-copy fallback always
  exists.
- Relative dates use the system clock; the exact timestamp remains authoritative.

### Out of Scope

- Object **version browsing** (ListObjectVersions UI) — explicitly deferred again, per user
  decision for 017.
- Any mutating operation, including aborting the discovered incomplete multipart uploads
  (surfacing them is in scope; cleaning them up belongs to the future write iteration).
- Cost estimation in currency (size × price tables).
- Cluster-admin telemetry (RGW admin API, MinIO admin API) and any non-S3 protocol.
- Cross-bucket / cross-context aggregate dashboards.
