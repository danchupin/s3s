# Feature Specification: Read-Only S3 Browser (TUI)

**Feature Branch**: `001-s3-readonly-browser`

**Created**: 2026-06-04

**Status**: Draft

**Input**: User description: "Сначала поддержка Ceph RGW (domain и path style access) и MinIO. Удобно конфигурировать кластера через context (по аналогии с kubectl). Сначала read-only: просмотр бакетов в древовидной структуре с погружением во вложенные директории по разделителю, просмотр метаданных объектов, превью (в т.ч. изображений). Современный интерактивный TUI, навигация стрелками. Поиск по префиксам/ключу. Эффективное взаимодействие с S3, без перегрузки сервера (листинг — тяжёлая операция)."

## Clarifications

### Session 2026-06-04

- Q: Preview fetch size limit ("bounded by max size")? → A: 5 MiB (ranged fetch of the first
  ~5 MiB); anything beyond is shown as a truncated preview.
- Q: How are stale cached levels refreshed? → A: Manual refresh only — the level cache persists
  for the whole session (no auto-expiry/TTL); a key (e.g. `r`) forces a re-fetch of the current
  level.
- Q: Support a credential-less (anonymous) context for public buckets? → A: Yes — a context may
  omit credentials and access public buckets anonymously.
- Q: Which credential types in v1? → A: Static access key + secret, plus an optional session/STS
  token; anonymous (no credentials) is also allowed.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Connect to a cluster and list buckets (Priority: P1)

An operator who works with several S3-compatible storages (Ceph RGW, MinIO) defines their
clusters once as named **contexts**, then launches the tool, picks (or defaults to) a context,
and immediately sees the list of buckets for that cluster in an interactive terminal view.

**Why this priority**: Without selecting a target cluster and seeing its buckets there is no
product. This is the smallest end-to-end slice that delivers value and is independently
demonstrable.

**Independent Test**: Configure one context pointing at a reachable MinIO (or Ceph RGW)
endpoint, launch the tool, and confirm the bucket list renders. Switch the selected context to a
second cluster and confirm the bucket list changes accordingly.

**Acceptance Scenarios**:

1. **Given** a configuration with at least one valid context, **When** the tool launches with no
   explicit context selected, **Then** the current/default context is used and its buckets are
   listed.
2. **Given** multiple configured contexts, **When** the operator selects a different context
   (via launch flag, environment variable, or in-app switcher), **Then** the view switches to
   that cluster's buckets without restarting the tool.
3. **Given** a context whose credentials are invalid, **When** the operator activates it, **Then**
   a clear, non-technical error is shown and no partial/garbage data is displayed.
4. **Given** a context configured for path-style access and one for domain (virtual-host) style,
   **When** each is used against its matching backend, **Then** bucket listing succeeds for both.

---

### User Story 2 - Navigate the object tree by delimiter (Priority: P1)

After choosing a bucket, the operator walks the object key namespace as a tree: keys that share a
prefix up to the `/` separator appear as expandable "directories"; entering one drills down a
level, and going back returns to the parent. Navigation is keyboard-driven (arrows / vim-like
keys) and feels instant even in buckets with very many objects.

**Why this priority**: Browsing the hierarchy is the core daily activity and the main reason to
prefer this tool over a flat CLI listing. Pairs with US1 to form the MVP.

**Independent Test**: Point at a bucket containing nested keys (e.g. `a/b/c/file`), enter the
bucket, and confirm only the first level of prefixes/objects is shown; drill into a prefix and
confirm its children appear; navigate back and confirm the parent level is restored.

**Acceptance Scenarios**:

1. **Given** a bucket with keys sharing common prefixes, **When** the operator opens it, **Then**
   the first level shows common prefixes (as directories) and top-level objects, not the entire
   flattened key list.
2. **Given** a directory (common prefix) is highlighted, **When** the operator enters it, **Then**
   the level below that prefix is loaded and shown.
3. **Given** a nested level is open, **When** the operator goes back, **Then** the parent level is
   shown again without re-fetching if already loaded.
4. **Given** a level contains more entries than one server page returns, **When** the operator
   scrolls to the end, **Then** the next page is fetched on demand and appended (no full upfront
   load).
5. **Given** an empty bucket or empty prefix, **When** it is opened, **Then** an explicit "empty"
   state is shown rather than a blank or error screen.

---

### User Story 3 - Inspect object metadata (Priority: P2)

With an object highlighted, the operator opens a metadata view showing size, last-modified time,
storage class, ETag, content type, and user-defined metadata, without leaving the browser.

**Why this priority**: Metadata is the most common read-only question ("how big / when changed /
what type"). High value, but depends on US1+US2 navigation existing first.

**Independent Test**: Highlight a known object and open its details; confirm the displayed size,
last-modified, and content type match what the backend reports.

**Acceptance Scenarios**:

1. **Given** an object is highlighted, **When** the operator requests details, **Then** size,
   last-modified, content type, storage class, ETag, and any user metadata are displayed.
2. **Given** an object the operator lacks permission to read, **When** details are requested,
   **Then** an access-denied message is shown instead of details.

---

### User Story 4 - Search within the current level by prefix (Priority: P2)

Because a level can hold a very large number of entries, the operator types a search term and the
view narrows to keys/prefixes that start with that term at the current level, fetched directly
from the server so the result is complete (not limited to what was already loaded).

**Why this priority**: Search makes large buckets usable, but is only meaningful once browsing
(US2) exists.

**Independent Test**: In a level with many entries, type a prefix that matches a known subset and
confirm only matching entries appear, including entries that had not yet been loaded by scrolling.

**Acceptance Scenarios**:

1. **Given** a level with many entries, **When** the operator types a prefix, **Then** the listing
   is narrowed server-side to entries under that prefix at the current level.
2. **Given** an active search, **When** the operator clears the term, **Then** the unfiltered
   level view is restored.
3. **Given** a prefix that matches nothing, **When** the search runs, **Then** an explicit
   "no matches" state is shown.

---

### User Story 5 - Preview object content (Priority: P3)

The operator previews an object's content inline: text/structured files render as readable text;
images render as a visual preview in terminals that support it, with a graceful fallback
otherwise. Previews are bounded in size so large objects never stall the interface.

**Why this priority**: Preview is a strong convenience (especially image preview) but is the least
essential read-only capability; navigation and metadata deliver value without it.

**Independent Test**: Preview a small text object and confirm its content is shown; preview a
small image and confirm a visual representation (or a clear unsupported-terminal fallback)
appears.

**Acceptance Scenarios**:

1. **Given** a text/structured object under the preview size limit, **When** preview is requested,
   **Then** its content is displayed in a readable, scrollable view.
2. **Given** an image object, **When** preview is requested in a capable terminal, **Then** a
   visual preview is rendered; **and** in a non-capable terminal a clear fallback (e.g. type/size
   summary) is shown.
3. **Given** an object larger than the preview limit, **When** preview is requested, **Then** only
   a bounded portion is fetched and the user is told the preview is truncated.

---

### Edge Cases

- **Unreachable endpoint / network timeout**: show a clear, dismissible error; the interface stays
  responsive and the operator can retry or switch context.
- **Expired or rejected credentials (403/401)**: surface an access/authentication error without
  exposing secrets; never log credential values.
- **Style mismatch**: a context set to the wrong access style (path vs domain) for its backend
  yields a clear failure rather than a hang.
- **Very large levels**: thousands+ of entries must remain navigable through on-demand paging; the
  tool must never attempt to load an entire bucket's key space at once.
- **Deeply nested prefixes**: drilling many levels deep stays responsive and the path/breadcrumb
  remains clear.
- **Non-printable / non-UTF-8 / unusual key names**: render safely without breaking layout.
- **Keys without the delimiter**: objects with no `/` appear as plain objects at the current
  level.
- **Binary object preview**: a non-text, non-image object shows a safe summary instead of dumping
  raw bytes.
- **Empty configuration / no contexts**: first-run guidance explains how to add a context rather
  than crashing.
- **Anonymous / public buckets**: a context without credentials accesses public buckets; if a
  bucket is not public, the access-denied state is shown clearly.
- **Stale cached data**: data changed on the backend after a level was cached is not reflected
  until the operator forces a refresh of that level.
- **Terminal resize**: the layout reflows without losing the current position.

## Requirements *(mandatory)*

### Functional Requirements

**Configuration & contexts**

- **FR-001**: System MUST let the operator define multiple clusters as named **contexts**, each
  binding an endpoint definition (address, region, access style, TLS settings) and a credential
  definition, with one context marked as current/default.
- **FR-002**: System MUST support selecting the active context via launch flag, environment
  variable, and an in-app switcher, with a defined precedence among them.
- **FR-003**: System MUST support both **path-style** and **virtual-host/domain-style** addressing
  per context, and MUST interoperate with Ceph RGW and MinIO backends.
- **FR-004**: System MUST allow disabling TLS verification only as an explicit, per-context opt-in
  (for local/self-signed backends), defaulting to verification enabled.
- **FR-005**: System MUST obtain credentials following a defined precedence (environment overrides,
  then the context's credential definition), and MUST NOT log or display secret values.
- **FR-005a**: System MUST support these credential types per context: a static access key id +
  secret, optionally accompanied by a session/STS token. Anonymous access (a context with no
  credentials) MUST be supported for public buckets.

**Browsing & navigation**

- **FR-006**: System MUST list the buckets available to the active context.
- **FR-007**: System MUST present object keys as a tree using the `/` delimiter, showing common
  prefixes as navigable directories and showing objects at the current level.
- **FR-008**: Users MUST be able to enter a directory (common prefix) and return to its parent
  using keyboard navigation (arrow keys and/or vim-style keys).
- **FR-009**: System MUST display a breadcrumb/path indicator of the current bucket and prefix.
- **FR-010**: System MUST load each level incrementally (one server page at a time) and fetch the
  next page only on demand (scroll/▼), never loading an entire bucket's keys up front.
- **FR-011**: System MUST avoid redundant re-fetching when returning to an already-loaded level
  during a session; level data is cached for the duration of the session with no automatic
  expiry.
- **FR-011a**: Users MUST be able to force a refresh of the current level (e.g. via a refresh key)
  that discards its cached data and re-fetches from the backend.
- **FR-012**: System MUST keep the interface responsive while data is loading (non-blocking),
  showing a loading indicator and allowing cancellation of an in-flight load.

**Metadata & preview**

- **FR-013**: System MUST show object metadata on demand: size, last-modified, content type,
  storage class, ETag, and user-defined metadata.
- **FR-014**: System MUST preview text/structured object content in a readable, scrollable view,
  bounded by a maximum preview size of 5 MiB.
- **FR-015**: System MUST preview image objects visually in capable terminals and MUST provide a
  graceful fallback in non-capable terminals.
- **FR-016**: System MUST fetch only a bounded portion of an object for preview (at most the first
  5 MiB, via a ranged read) and MUST indicate when a preview is truncated.

**Search**

- **FR-017**: Users MUST be able to search the current level by prefix, with the narrowing applied
  server-side so results are complete (not limited to already-loaded entries).
- **FR-018**: System MUST allow clearing the search to restore the full current-level view, and
  MUST show an explicit no-matches state.

**Read-only & safety**

- **FR-019**: System MUST be strictly read-only in this feature: it MUST NOT create, modify, move,
  or delete buckets or objects.
- **FR-020**: System MUST present clear, non-technical error states for unreachable endpoints,
  authentication/permission failures, and timeouts, while remaining responsive.
- **FR-021**: System MUST record operational diagnostics (operations, targets, outcomes, errors)
  to a side channel (log file/stderr) that does not corrupt the terminal view, excluding secrets.

### Key Entities *(include if feature involves data)*

- **Context**: A named selection binding a Cluster and a Credential, plus the active/default
  marker. Mirrors the kubectl "context" concept.
- **Cluster (endpoint)**: Connection target — address, region, access style (path/domain), TLS
  settings.
- **Credential (user)**: Access identity for a cluster — a static access key id and secret, with
  an optional session/STS token, supplied inline or via an environment reference; may be absent
  entirely for anonymous access. Secrets are sensitive and never displayed/logged.
- **Bucket**: A top-level container listed for the active context.
- **Prefix (directory)**: A common key prefix up to the delimiter, navigable as a tree node.
- **Object**: A stored key with attributes (size, last-modified, content type, storage class,
  ETag, user metadata) and retrievable content for preview.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: From launch with a valid current context, the operator sees the bucket list within
  2 seconds on a reachable backend.
- **SC-002**: Entering a level renders the first page of entries within 1 second for typical
  prefixes, and the operator can begin navigating immediately.
- **SC-003**: Browsing any single level issues at most one listing request per page shown; the
  tool never fetches more than the currently needed page (bounded server load on the heavy listing
  operation).
- **SC-004**: A prefix search at the current level returns its first page of complete results
  within 1 second on a reachable backend.
- **SC-005**: Switching the active context updates the view without restarting the tool, in under
  2 seconds.
- **SC-006**: Text preview of an object under the size limit opens within 1 second; image preview
  opens within 2 seconds in a capable terminal or falls back cleanly otherwise.
- **SC-007**: The interface remains interactive (accepts input, can cancel) during any in-flight
  load — no frozen frames.
- **SC-008**: A new operator can configure a working context and reach a bucket listing in under
  5 minutes using only the documentation.
- **SC-009**: Across a browsing session, zero create/update/delete requests are issued to the
  backend (verifiable read-only guarantee).

## Assumptions

- **Config format**: A single human-readable **YAML** configuration file (kubectl-style), located
  at the platform config path (e.g. `~/.config/s3s/config.yaml`), with separate `clusters`,
  `users` (credentials), and `contexts` sections plus a `current-context`. YAML chosen over TOML
  for readability of nested context/cluster/user structures and parity with kubeconfig. Exact
  schema is finalized in planning.
- **Credential storage**: Mirrors kubeconfig — secrets may be stored inline in the `users` section
  (file expected to be user-protected, e.g. `0600`, with a warning) or referenced from environment
  variables; environment values take precedence. Supported credential types: static access
  key + secret, with an optional session/STS token; a context may also omit credentials for
  anonymous public-bucket access. No secret is ever printed or logged.
- **Search scope (v1)**: Search narrows the **current level** server-side via prefix; recursive
  cross-level/deep search is out of scope for v1.
- **Preview bound**: Previews fetch at most the first 5 MiB of an object (ranged read); larger
  objects are shown truncated.
- **Caching & refresh**: Listed levels are cached for the whole session with no automatic
  expiry; the operator forces a re-fetch of a level explicitly via a refresh key. Auto-refresh
  and background polling are out of scope for v1.
- **Read-only v1 scope**: Includes browsing, metadata, and preview (text + image). **Out of scope
  for v1**: downloading objects to disk, uploading, copying/moving, deleting, multipart transfers,
  bucket/object creation, ACL/policy editing.
- **Backends**: Ceph RGW and MinIO are the validated targets; broad AWS-S3 compatibility is
  expected as a side effect but not separately certified in v1.
- **Image preview**: Visual rendering depends on terminal capabilities; terminals lacking image
  support get a metadata/summary fallback rather than a degraded render.
- **Single active context at a time**: The browser operates against one cluster context per view;
  multi-cluster side-by-side comparison is out of scope for v1.
- **Connectivity**: The operator has network reachability and valid credentials for the configured
  backends; the tool surfaces failures but does not provision access.
