# Feature Specification: Metadata Enrichment & Inline Usage

**Feature Branch**: `016-metadata-enrichment`

**Created**: 2026-06-09

**Status**: Draft

**Input**: User description: "Я бы хотел исследовать какой мета информации о бакетах и объектах сейчас недостаточно, хочется добавить это в интерфейс. Плюс мне не нравится сейчас отдельный интерфейс analyze, не понимаю его надобность, хочется иметь возможность получать эту инфу на основном экране. Давай поисследуем aws и minio интерфейсы и поймем какую мета информацию стоит добавить и спроектируем решение"

## Overview

Today `s3s` shows a thin slice of the metadata that S3-compatible backends expose:
buckets show only name + creation date; objects show only key, size, modified,
type, storage class, ETag, and user metadata; and aggregate usage (size + object
count + size breakdown) lives behind a *separate full-screen* `analyze` view that
the user must explicitly enter and leave.

Research against the AWS S3 console / API and the MinIO console (see
`research-notes` below) shows two concrete gaps:

1. **Free metadata is fetched and discarded.** The single `HeadObject` call the
   browser already makes on every object open returns version id, encryption
   state, replication status, archival/restore state, retention/legal-hold,
   lifecycle expiration, and content-handling headers — none of which reach the
   screen.
2. **Aggregate usage is exiled to a separate screen.** Size/count totals and the
   "where did the space go" breakdown require entering a dedicated mode, breaking
   the main browsing flow.

This feature closes both gaps: it surfaces richer per-object and per-bucket
metadata **on the main browse screen**, and it folds the separate `analyze`
interface into the main screen's details area so usage information is available
in-context without a mode switch.

### Research notes (grounding)

- A list call (`ListObjectsV2`) returns only key, size, last-modified, ETag, and
  storage class per object; everything richer needs a per-object call.
- `HeadObject` returns (beyond what is shown today) version id + delete-marker
  flag, server-side-encryption type and KMS key, replication status, Glacier
  restore/archive status, object-lock mode + retain-until + legal-hold, lifecycle
  expiration date, and content-encoding / cache-control / content-disposition.
  **These cost no extra round-trip — the browser already issues this `HeadObject`.**
- Object **tag values** require a separate call (a head only returns the tag
  *count*). Bucket configuration (versioning, encryption, lifecycle, replication,
  policy/public, tags, location) has no single "describe bucket" call — each is a
  separate read.
- There is **no cheap native bucket size/count** on Ceph RGW or MinIO over the S3
  API; totals require paginating and summing object listings (a full scan,
  O(objects)). This is exactly what the existing usage backend already does.

## Clarifications

### Session 2026-06-09

- Q: How much object metadata depth should this feature add? → A: Surface the
  fields the existing `HeadObject` already returns (no new round-trips) **plus**
  new read-only calls for object **tag values** and **bucket configuration**
  (versioning, encryption, lifecycle, replication, policy/public). Object
  **version history** listing is out of scope for this feature.
- Q: What happens to the separate `analyze` (usage) screen? → A: Remove the
  separate full-screen mode. Show bucket/prefix **totals (size + object count)**
  inline in the details area, and keep the ranked largest-first child breakdown as
  an **expandable section in the same area** (no full-screen transition).
- Q: When are bucket/prefix totals computed (each scan is O(objects))? → A:
  **Lazily, on focus** of a bucket or prefix, as a background cancelable scan whose
  result is cached for the session; navigating away cancels an in-flight scan.
- Q: How are the on-demand affordances (expand usage breakdown; load object tags /
  bucket configuration) triggered? → A: A **single, context-aware "more detail"
  key** — the key freed by removing `analyze` (the former `a`). On bucket/prefix
  focus it expands the usage breakdown and loads bucket configuration; on object
  focus it loads tags and governance detail. One key, one mental model (no new
  separate keybindings).
- Q: How should the object detail pane handle absent metadata values, given ~10
  added fields in a height-bounded pane? → A: **Omit absent optional fields**
  entirely (console-like, compact). Always render the existing core fields, and
  always render permission-gated fields (object-lock, legal-hold) as
  "unknown/denied" because there absence is itself meaningful information.
- Q: How is the auto usage-scan gated so navigating a list does not fire a scan per
  transited entry? → A: **Debounce / dwell-gate** — the auto-scan starts only after
  focus has rested on a bucket/prefix for a brief idle interval; rapid transit
  starts no scan. Already-cached totals still appear immediately.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Complete object metadata in-context (Priority: P1)

As someone inspecting an object, when I open or focus it I want to see its full
technical metadata — version, encryption, replication, archival/restore state,
retention/legal-hold, lifecycle expiration, and content-handling headers — in the
details area, without entering any separate tool, so I can answer "is this
encrypted / archived / locked / versioned?" at a glance.

**Why this priority**: Highest value for lowest cost. Every field here is already
returned by the single `HeadObject` the browser issues on object open — this is
pure presentation of data currently discarded. It needs no new backend method and
adds no round-trips, so it is the natural MVP slice.

**Independent Test**: Open an object whose backend metadata includes a version id,
an encryption type, and a non-empty user-metadata map; assert the details area
renders each of the new fields with correct values, and that fields with no value
render a clear placeholder rather than a blank line. No separate screen is entered.

**Acceptance Scenarios**:

1. **Given** an object encrypted with SSE-KMS that has a version id, **When** I
   open it, **Then** the details area shows the encryption type, the KMS key
   reference, and the version id alongside the existing key/size/modified/type/
   class/ETag fields.
2. **Given** an archived (Glacier-class) object with an in-progress restore,
   **When** I open it, **Then** the details area shows its archival/restore state.
3. **Given** an object where the caller lacks permission to read object-lock /
   legal-hold, **When** I open it, **Then** those fields render as "unknown" (not
   "none"), so absence-of-permission is not misread as absence-of-lock.
4. **Given** an object with no value for an optional field (e.g. no replication),
   **When** I open it, **Then** that field is omitted from the pane (no placeholder
   line), keeping the pane compact and the footer/command bar fully visible.

---

### User Story 2 - Bucket/prefix usage on the main screen (Priority: P1)

As someone browsing, I want a bucket's or prefix's **total size and object count**
to appear in the details area of the main screen when I focus it, computed in the
background without freezing the UI, so I no longer have to enter a separate
`analyze` mode to learn how big something is.

**Why this priority**: Directly resolves the user's stated objection ("I don't
understand the need for a separate analyze interface; I want this info on the main
screen"). Removing the mode switch is the core experience change.

**Independent Test**: Focus a non-empty bucket; assert a running progress
indicator appears, then a final total size + object count, all within the main
screen's details area with no full-screen transition; assert navigating away mid-
scan cancels it and the UI stays responsive; assert the previously-separate usage
mode no longer exists as a destination.

**Acceptance Scenarios**:

1. **Given** the bucket list, **When** I focus a non-empty bucket, **Then** the
   details area shows a running "scanning…" total that resolves to a final size +
   object count, and the input stays responsive throughout.
2. **Given** an in-flight usage scan, **When** I navigate to a different bucket or
   prefix, **Then** the previous scan is cancelled and a new one begins for the
   new target.
3. **Given** a bucket whose totals were already computed this session, **When** I
   focus it again, **Then** the cached totals appear immediately without rescanning
   (until a manual refresh).
4. **Given** the running application, **When** I attempt to enter the former
   `analyze` screen by its old trigger, **Then** no separate full-screen usage view
   appears — the information is presented inline instead.

---

### User Story 3 - Drill into where the space went, inline (Priority: P2)

As someone who has seen a bucket/prefix total, I want to expand a ranked,
largest-first breakdown of its immediate children (sub-prefixes and direct
objects) with each child's size and share, in the same details area, so I can find
what is consuming space and step into it — without a separate screen.

**Why this priority**: Preserves the genuinely useful capability of the old
`analyze` view (the breakdown + drill-down) while keeping it on the main screen.
Depends on US2's totals existing, so it follows at P2.

**Independent Test**: After totals are shown for a prefix with several children,
expand the breakdown; assert children are listed largest-first with size and a
share indicator; assert collapsing returns to the compact totals; assert stepping
into a child sub-prefix re-targets navigation to that prefix.

**Acceptance Scenarios**:

1. **Given** computed totals for a prefix with multiple children, **When** I press
   the context-aware "more detail" key (the repurposed former `analyze` key) to
   expand the breakdown, **Then** immediate children are listed largest-first with
   each child's size and relative share.
2. **Given** an expanded breakdown, **When** I collapse it, **Then** the details
   area returns to the compact totals and the footer remains fully visible.
3. **Given** an expanded breakdown, **When** I select a child sub-prefix to step
   into, **Then** navigation moves into that sub-prefix (and its usage is shown by
   the US2 mechanism).

---

### User Story 4 - Object tags & bucket configuration on demand (Priority: P2)

As someone auditing storage, I want to see an object's **tag values** and a
bucket's **configuration** (versioning, default encryption, lifecycle rules,
replication, public-access-block status, and location), loaded on demand without
blocking, so I can verify governance state from within the browser.

**Why this priority**: High informational value but each item costs an additional
read call (object tag values, and one call per bucket-config sub-resource), so it
is loaded lazily on an explicit details/info request rather than on every focus.
Independent of US1–US3.

**Independent Test**: Open the details/info request for an object that has tags;
assert tag key/value pairs appear. Open it for a bucket; assert versioning,
encryption, lifecycle, replication, and public-access status appear, each
degrading to a clear "none"/"disabled"/"unknown" label distinguishing
"not-configured" from "access-denied" from "unsupported-by-backend".

**Acceptance Scenarios**:

1. **Given** an object with tags, **When** I press the context-aware "more detail"
   key (the repurposed former `analyze` key), **Then** the tag key/value pairs are
   shown (loaded without freezing the UI).
2. **Given** a bucket with versioning enabled and a lifecycle rule, **When** I
   request its info, **Then** versioning state and the lifecycle rule(s) are shown.
3. **Given** a bucket on a backend that does not support a given configuration
   call, **When** I request its info, **Then** that item shows an "unsupported"
   (or equivalent) label and the rest of the info still loads.
4. **Given** a configuration sub-resource the caller cannot read, **When** I
   request bucket info, **Then** that item shows "unknown/denied" — distinct from a
   configured-but-empty "none".

---

### User Story 5 - Storage class visible while listing (Priority: P3)

As someone scanning a listing, I want non-standard storage classes (e.g. Glacier)
to be visible for objects in the list, so I can spot archived/cold objects without
opening each one.

**Why this priority**: Cheap (the value is already in the list response) and
useful, but a refinement on top of the core metadata/usage work, so lowest
priority.

**Independent Test**: List a level containing one STANDARD object and one
GLACIER-class object; assert the non-standard class is visibly indicated for the
archived object while a standard class does not add visual noise.

**Acceptance Scenarios**:

1. **Given** a listing containing an object with a non-standard storage class,
   **When** the list renders, **Then** that object's storage class is visible.
2. **Given** a listing of only standard-class objects, **When** the list renders,
   **Then** no redundant per-row class noise is added and column widths stay within
   the legibility budget.

---

### Edge Cases

- **Object with no optional metadata** (no replication, no lock, no user metadata,
  no tags): absent optional fields are omitted entirely; permission-gated fields
  still render as "unknown/denied"; no blank gap or misleading value appears.
- **Permission-gated fields** (object-lock mode, legal-hold): when the caller lacks
  read permission, render "unknown", never "none".
- **Encryption / config not configured vs denied vs unsupported**: a bucket with
  no encryption rule, a caller without permission to read it, and a backend that
  does not implement the call must be visibly distinguishable.
- **Multipart ETag** (`"<hash>-<n>"`): presented as-is; not labeled as an MD5
  checksum of the content.
- **Empty bucket/prefix**: totals resolve to 0 bytes / 0 objects quickly and
  clearly, not an infinite "scanning…".
- **Very large bucket** (millions of objects): the usage scan must remain
  cancelable, must not block input, and must surface a running partial total;
  navigating away cancels it.
- **Cancelled scan**: a partial total is clearly marked as partial, not presented
  as final.
- **Rapid list navigation**: scrolling quickly through a bucket/prefix list MUST
  NOT spawn a scan per transited entry — the dwell gate suppresses scans until focus
  settles; only cached totals (if any) appear during transit.
- **Narrow terminal**: all added metadata values are fully visible or revealable
  (no permanently truncated identifier) and the footer/command bar is never
  scrolled off (constitution principle VI).
- **Backend without a config sub-resource** (e.g. RGW lacking public-access-block):
  that item degrades gracefully while the rest of the info loads.
- **Switching context/connection**: cached totals and metadata are scoped so a
  context switch does not show another context's figures.

## Requirements *(mandatory)*

### Functional Requirements

#### Object metadata (US1)

- **FR-001**: The object details area MUST display, in addition to the fields shown
  today, the following whenever the object's metadata provides them: version id and
  delete-marker state; server-side-encryption type and (when applicable) the key
  reference; replication status; archival/restore state; object-lock mode and
  retain-until; legal-hold state; lifecycle expiration; and the content-handling
  headers (content-encoding, cache-control, content-disposition).
- **FR-002**: The fields in FR-001 MUST be populated from the metadata the browser
  already retrieves on object open, without issuing additional per-object requests
  for them.
- **FR-003**: The object detail pane MUST omit optional fields that have no value
  (no placeholder line for them), so the pane stays compact and the footer/command
  bar is never pushed off-screen. The pre-existing core fields (key, size, modified,
  type, class, ETag) MUST always render. Permission-gated fields (object-lock mode,
  legal-hold) MUST always render with an "unknown/denied" indicator — distinct from
  a configured-but-empty "none" — because at those fields the absence of a value is
  itself information.

#### Inline usage totals (US2)

- **FR-004**: When a bucket or prefix is focused, the system MUST present its total
  size and object count in the main screen's details area, without entering a
  separate full-screen view.
- **FR-005**: Usage totals MUST be computed in the background so that input remains
  responsive while a scan is in progress, with a visible running/partial indication.
  The auto-scan MUST be dwell-gated: it starts only after focus has rested on a
  bucket/prefix for a short, fixed debounce interval (the same brief delay already
  used for the pane metadata load), so transiting past entries during rapid
  navigation starts no scan. A target whose totals are already cached MUST show them
  immediately, without waiting on the dwell interval.
- **FR-006**: An in-flight usage computation MUST be cancelled when the user
  navigates to a different target, and a cancelled computation MUST surface its
  result as explicitly partial.
- **FR-007**: Usage totals MUST be cached for the session keyed by the focused
  target, so re-focusing a previously computed target shows results immediately;
  the existing manual refresh MUST invalidate this cache consistently with the rest
  of the browser's caching.
- **FR-008**: The separate full-screen usage/`analyze` destination MUST be removed;
  the key it formerly used MUST NOT open any separate screen, and is instead
  repurposed as the context-aware "more detail" trigger defined in FR-019.

#### Inline breakdown (US3)

- **FR-009**: From a focused target's totals, the user MUST be able to expand a
  ranked, largest-first breakdown of immediate children (sub-prefixes and direct
  objects), each showing its size and relative share, within the same details area.
- **FR-010**: The user MUST be able to collapse the breakdown back to compact
  totals, and to step navigation into a child sub-prefix from the breakdown.

#### Tags & bucket configuration (US4)

- **FR-011**: The system MUST provide, on an explicit details/info request, an
  object's tag key/value pairs, loaded without blocking input.
- **FR-012**: The system MUST provide, on an explicit bucket info request, the
  bucket's versioning state, default encryption, lifecycle rule(s), replication
  configuration, public-access-block status, and bucket location, each loaded
  without blocking input. (Bucket *policy* / policy-public status is out of scope
  for this feature — see Assumptions.)
- **FR-013**: Each configuration item in FR-011/FR-012 MUST visibly distinguish
  "not configured / none", "access denied / unknown", and "unsupported by this
  backend".
- **FR-014**: All data access added by this feature MUST be read-only — no
  mutating storage operation is introduced (the read-only structural posture and
  its build guard remain satisfied).

#### Listing (US5)

- **FR-015**: A non-standard object storage class MUST be visible in the object
  listing, while standard-class objects MUST NOT add redundant per-row noise, and
  the listing MUST stay within the column legibility budget. The marker uses a
  CLOSED, documented token set (no ad-hoc abbreviation) and the full class MUST be
  recoverable via the reveal affordance.

#### Cross-cutting (constitution)

- **FR-016**: Every metadata/usage/config retrieval added by this feature MUST run
  off the UI loop (non-blocking) and MUST be generation/cancellation guarded so
  that results from a superseded target never corrupt the current view.
- **FR-017**: All added identifiers and values MUST be fully visible or revealable
  to read/copy, and the footer/command bar MUST remain on-screen at all supported
  terminal sizes (constitution principle VI).
- **FR-018**: All added presentation MUST reuse the established label/pane/prompt
  patterns and palette roles rather than introducing ad-hoc layouts or colors
  (constitution principle VII).
- **FR-019**: A single context-aware "more detail" trigger (the key freed by
  removing the `analyze` screen) MUST drive the on-demand affordances: on
  bucket/prefix focus it expands/collapses the usage breakdown (US3) and loads
  bucket configuration (US4); on object focus it loads object tags and governance
  detail (US4). No additional separate keybindings are introduced for these
  affordances.

### Key Entities *(include if feature involves data)*

- **Enriched object metadata**: the per-object attributes surfaced on open — adds
  version/delete-marker, encryption (type + key reference), replication status,
  archival/restore state, object-lock mode + retain-until, legal-hold, lifecycle
  expiration, and content-handling headers to the existing key/size/modified/type/
  class/ETag/user-metadata.
- **Object tags**: zero or more key/value pairs associated with an object, fetched
  on demand (values, not just count).
- **Bucket configuration**: a bucket's governance state — versioning, default
  encryption, lifecycle rule(s), replication, public-access-block status, and
  location — each an independently retrievable item that may be configured, empty,
  denied, or unsupported.
- **Usage report**: total size and object count for a bucket/prefix subtree plus a
  ranked immediate-child breakdown — already produced by the existing usage backend,
  now presented inline instead of in a separate screen.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can determine an object's encryption state, version id, and
  storage class from a single object-open action, with zero additional keypresses
  and zero separate screens, whenever the backend provides those values.
- **SC-002**: A bucket's or prefix's total size and object count are obtainable on
  the main browse screen with **zero mode switches** (down from the current path
  that requires entering and leaving a dedicated screen).
- **SC-003**: During any metadata, tag, configuration, or usage retrieval, user
  input continues to be handled (the UI never blocks), and an in-progress usage
  scan can be cancelled at any time with the UI returning to responsiveness
  immediately.
- **SC-004**: For every added field, an unavailable value is presented as a clear
  "none", "unknown/denied", or "unsupported" label — a user can always tell *why* a
  value is absent, and never sees a misleading blank or a false "none".
- **SC-005**: At terminal widths down to the project's supported minimum (80
  columns), every added metadata value is fully visible or revealable and the
  footer/command bar is never scrolled off screen.
- **SC-006**: The separate `analyze` screen no longer exists as a destination, and
  100% of the information it previously provided (totals + ranked breakdown +
  drill-down) is reachable from the main screen.
- **SC-007**: Re-focusing a bucket/prefix whose totals were computed earlier in the
  session shows the totals immediately (no rescan) until a manual refresh.

## Assumptions

- The read-only posture is preserved: every new data access is a read; no S3 write
  symbol is introduced, so the structural read-only build guard stays green. Adding
  read methods to the storage interface (object tagging, bucket configuration) is
  permitted within this posture.
- Constitution v1.2.0 governs; principles VI (UI Legibility) and VII (UI
  Consistency) apply directly. No constitution amendment is required (this is
  additive metadata presentation and a UI consolidation, not a new principle).
- The existing usage-computation backend is reused for totals and the breakdown;
  this feature changes where/how its results are presented, not how they are
  computed.
- New read-only storage methods (object tag values, bucket configuration
  sub-resources) extend the storage contract and therefore require real-backend
  integration coverage (constitution IV) in addition to unit coverage; the
  presentation-only and usage-fold portions do not change the storage contract.
- Permission-gated fields (object-lock, legal-hold) and backend-specific
  configuration calls (e.g. public-access-block) may be denied or unsupported on
  Ceph RGW / MinIO; the feature degrades these gracefully rather than failing the
  whole view.
- Object **version history** listing, multipart **parts** breakdown, and any
  admin-API / out-of-band metrics (CloudWatch, Storage Lens, `radosgw-admin`,
  `mc admin`) sources are **out of scope** for this feature.
- Bucket **policy** and **policy-derived public status** (`GetBucketPolicy` /
  `GetBucketPolicyStatus`) are **out of scope** — the bucket-config panel surfaces
  the public-access-**block** configuration only. These calls are AWS-centric and
  frequently unsupported on Ceph RGW; revisit in a later feature if needed.
- Bucket/prefix totals are computed by scanning object listings (no cheap native
  count/size exists on the target backends over the S3 API); cost scales with
  object count and is mitigated by background execution, cancellation, and caching.
