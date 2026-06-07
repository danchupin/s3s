# Checklist: Lazy-Load & Caching of Bucket Contents (Requirements Quality)

**Purpose**: Unit-test the *requirements* (not the code) governing how the objects zone loads a highlighted bucket's first level — confirm the spec mandates lazy, on-selection loading + caching rather than eager listing of every bucket's contents at startup, and that those requirements are complete, clear, consistent, and measurable.
**Created**: 2026-06-07
**Feature**: [spec.md](../spec.md)
**Focus**: lazy-load on selection + per-session caching · **Depth**: Standard · **Audience**: Reviewer (PR)
**User must-have**: "No eager listing of all buckets' contents at startup; lazy-load a bucket's objects on selection and cache them."

## Requirement Completeness

- [x] CHK001 - Is there an explicit requirement that NO bucket's object level is listed at startup — only bucket names load, and object listings are deferred until selection? [Gap → resolved by §FR-002a]
- [x] CHK002 - Are the startup load contents explicitly bounded to the bucket-name list with zero object-level listings? [Completeness, Spec §FR-002a/§SC-010]
- [x] CHK003 - Is the event that triggers a lazy load precisely specified (selection settling on a bucket initiates that bucket's first-level listing)? [Completeness, Spec §FR-002a/§FR-003]
- [x] CHK004 - Are cache scope and lifetime specified for cached bucket levels (per-session, TTL-free vs. expiring, when entries are dropped)? [Completeness, Spec §FR-006e]
- [x] CHK005 - Is cache invalidation specified for the objects zone (e.g., does manual refresh `r` re-fetch; what else clears a cached level)? [Spec §FR-006e]
- [x] CHK006 - Is the cache key for a bucket's level fully specified (context, bucket, prefix, search) and stated to be the SAME key space as the existing full-screen level view? [Completeness, Spec §FR-006e / Key Entities]
- [x] CHK007 - Is the extent of a lazy load defined (first page only vs. the entire first level) for buckets with very large first levels? [Spec §FR-006a]
- [x] CHK008 - Are requirements defined for an in-flight load whose selection moves away (cancel the request vs. let it complete and drop the result)? [Completeness, Spec §FR-004 + Assumptions (beginLoad cancels prior)]
- [x] CHK009 - Is de-duplication specified when the same bucket is re-selected while its first load is still in flight (no duplicate backend call)? [Spec §FR-006d]

## Requirement Clarity

- [x] CHK010 - Is "settled selection" defined precisely enough to test (what idle duration / event marks a selection as settled)? [Spec §FR-002a/§FR-003]
- [x] CHK011 - Is the debounce ceiling quantified and stated consistently (≤ 200 ms) across §FR-003, §SC-003, and the Assumptions? [Clarity, Spec §FR-003]
- [x] CHK012 - Is "does not block input" expressed as a measurable property rather than a subjective term ("responsive", "no freeze")? [Clarity, Spec §SC-003]
- [x] CHK013 - Is "no additional backend listing call" unambiguous about what counts as a listing call (first-level list vs. metadata HEAD vs. pagination follow-ups)? [Clarity, Spec §FR-006a/§FR-006]
- [x] CHK014 - Does the spec make explicit the distinction between listing bucket NAMES (eager, at start) and listing bucket CONTENTS (lazy, on selection), so the two are never conflated? [Clarity, Spec §FR-002a]

## Requirement Consistency

- [x] CHK015 - Do the lazy-load requirements (§FR-002/§FR-002a) and the "no new backend listing capability" requirement (§FR-027) align without conflict? [Consistency]
- [x] CHK016 - Is the objects-zone cache consistently described as the SAME per-session cache used by the full-screen level view (not a separate or competing cache)? [Consistency, Spec §FR-006e]
- [x] CHK017 - Are the debounce + supersession requirements consistent between the objects zone (§FR-003/§FR-004) and the existing details pane they are said to reuse? [Consistency, Assumptions]
- [x] CHK018 - Do §FR-006 (cache ⇒ no extra call) and §SC-004 (revisit ⇒ no extra call) state the same guarantee without divergence in wording or scope? [Consistency]

## Acceptance Criteria Quality (Measurability)

- [x] CHK019 - Can "lazy load" be objectively verified by a measurable acceptance criterion (e.g., zero object-listing calls issued at startup)? [Measurability, Spec §SC-010]
- [x] CHK020 - Is the cache-hit guarantee measurable (revisiting a viewed bucket issues exactly zero additional backend listings)? [Measurability, Spec §SC-004]
- [x] CHK021 - Is the fast-scroll guarantee measurable (N intermediate selections produce ≤ 1 backend listing)? [Measurability, Spec §SC-003]

## Edge Case & Scenario Coverage

- [x] CHK022 - Are caching requirements defined for an EMPTY bucket result, so a revisit re-fetches nothing? [Edge Case, Spec §FR-006b]
- [x] CHK023 - Are requirements defined for whether a FAILED/denied listing (e.g., 403 on a scoped bucket) is cached or re-attempted on revisit? [Exception Flow, Spec §FR-006c/§FR-018]
- [x] CHK024 - Are requirements specified for cache memory bounds (a cap on cached bucket levels per session) to prevent unbounded growth during a long browsing session? [Edge Case, Spec §FR-006e — explicit decision: per-session TTL-free, uncapped, rationale stated]
- [x] CHK025 - Are requirements defined for the stale-vs-current race (selection changes mid-load) so a late or cached result never renders under the wrong bucket's title? [Coverage, Spec §FR-004]
- [x] CHK026 - Are the loading / empty / error state requirements complete for the lazy path, each with a defined stable marker? [Coverage, Spec §FR-005]

## Dependencies & Assumptions

- [x] CHK027 - Is the assumption that a reusable per-session level cache already exists stated as traceable/validated rather than merely asserted? [Assumption, Spec §FR-006e / Assumptions — cache module confirmed present]
- [x] CHK028 - Is the assumption that the existing generation-id debounce pattern is reusable for bucket-scroll loads documented and validated? [Assumption, Assumptions]

## Notes

- 2026-06-07: All 28 items resolved. Gaps surfaced by this checklist were closed in `spec.md` by adding **FR-002a** (lazy, no eager startup listing), **FR-006a** (lazy extent / paging on demand), **FR-006b** (cache success incl. empty), **FR-006c** (errors not cached, re-attempt on revisit), **FR-006d** (in-flight de-duplication), **FR-006e** (cache scope keyed by context/bucket/prefix/search, `r`-only invalidation, uncapped per-session by explicit decision), and **SC-010** (zero object-level listings at startup; denied listing re-attempted). The decision trail is also recorded in the spec's Clarifications session (2026-06-07).
