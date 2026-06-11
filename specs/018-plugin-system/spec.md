# Feature Specification: Plugin System for External Capability Providers

**Feature Branch**: `018-plugin-system`

**Created**: 2026-06-11

**Status**: Draft

**Input**: User description: "Идея плагинизации s3s: подключить внешний плагин и что-то выполнять. Нужно продумать механизм подключения и контракт. За основу — задача дискавери нужных бакетов для domain-style access; в будущем — возможность подключить MCP явно либо взаимодействовать с MCP через общие механизмы. Понятная задача — получать мета-информацию об объектах из image-storage сервиса по image id."

## Clarifications

### Session 2026-06-11

- Q: Does v1 include user-invocable action plugins ("execute something" on an object)? → A: No — v1 capability surface is data providers only (bucket discovery, object metadata enrichment); actions are a future capability on the same contract.
- Q: How much MCP support lands in v1? → A: None — v1 ships only a channel-agnostic capability contract that a future MCP bridge can target (FR-016); no MCP protocol code in v1.
- Q: How does a discovery provider behave on a connection where storage-side bucket listing works? → A: Always additive — the bucket list is the deduplicated union of pinned, listed (when available), and discovered names; discovery never replaces or suppresses other sources.
- Q: Which connection identity data may be passed to plugins in the request context? → A: Connection name, endpoint, user label, and access key ID (a public identifier, not a secret). The secret key and any other credential material are never passed.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Bucket discovery through an external provider (Priority: P1)

A user connects to storage where listing buckets is denied or physically impossible (domain-style-only endpoint with an unresolvable apex), and the set of accessible bucket names is not known upfront. Today their only option is to obtain names out-of-band and pin them by hand. With this feature, the user declares an external discovery provider (plugin) for that connection. The provider knows how to obtain the list of accessible bucket names (e.g., from an internal provisioning service). When the user opens the connection, the bucket list is populated from the provider — merged with any manually pinned buckets — without any storage-side listing call.

**Why this priority**: This is the original motivating problem and the case that is unsolvable inside the storage protocol itself: buckets granted but not owned never appear in a bucket listing, and a domain-style-only endpoint cannot serve a listing at all. A discovery provider is the only path to "see everything I can access". It alone delivers a complete, valuable MVP.

**Independent Test**: Configure a stub discovery provider returning a fixed set of names for a connection where listing is denied. Open the connection: the names appear as browsable buckets. Make the provider fail: pinned buckets still appear, with a visible notice.

**Acceptance Scenarios**:

1. **Given** a connection with an assigned discovery provider and bucket listing unavailable, **When** the user opens the connection, **Then** the bucket list shows the deduplicated union of pinned and discovered names without issuing a listing request to storage.
2. **Given** discovered names are shown, **When** the user enters one of them, **Then** it behaves like any regular bucket (browsable; an actually inaccessible bucket shows the existing honest access-denied state).
3. **Given** the provider fails, times out, or returns invalid data, **When** the user opens the connection, **Then** pinned buckets are still shown and a succinct, non-blocking notice explains that discovery failed and why.
4. **Given** a previously loaded list, **When** the user triggers manual refresh, **Then** the provider is invoked again and the list updates; a slow result from a superseded invocation never overwrites a newer one.

---

### User Story 2 - External metadata for objects (Priority: P2)

A user browses a bucket whose objects are records in an external system — for example, images whose object key encodes an image id known to an image-storage service. The user declares a metadata provider scoped to those objects (by connection, bucket, and/or key pattern). When the user opens such an object's details, an additional, clearly attributed metadata group appears, populated asynchronously by the provider (e.g., dimensions, owner service, moderation status from image-storage). Each provided field can be copied like any native metadata field.

**Why this priority**: High recurring value for daily work (context about objects lives outside storage), but browsing remains fully usable without it, so it builds on top of the P1 plumbing rather than gating it.

**Independent Test**: Configure a stub metadata provider matching a key pattern. Open a matching object's details: an attributed group appears with the provider's fields after a visible pending state. Open a non-matching object: no group, no invocation.

**Acceptance Scenarios**:

1. **Given** a matching object and a configured provider, **When** the user opens object details, **Then** an enrichment group appears with a pending state and fills with fields when the result arrives, without delaying native metadata.
2. **Given** the provider errors or times out, **When** details are shown, **Then** the group shows an error state textually distinct from "no data", and native metadata is unaffected.
3. **Given** an object not matching any provider's scope, **When** details are opened, **Then** no enrichment group is shown and no provider is invoked.
4. **Given** enrichment fields are displayed, **When** the user uses the existing per-field copy mechanism, **Then** provider fields are copyable exactly like native fields.
5. **Given** the user navigates away before the result arrives, **Then** the late result is discarded and never corrupts the current view.

---

### User Story 3 - Plugin visibility and control (Priority: P3)

A user wants to understand and trust what their plugins are doing. They open an in-app plugin status surface and see every declared plugin: its capability, its scope, whether it is enabled, and the outcome of its last invocation (success, failure reason, timeout) with timing. They can disable a misbehaving plugin without deleting its declaration, and re-enable it later.

**Why this priority**: Trust and operability matter once plugins exist, but logs already provide a baseline; an in-app surface is a usability layer on top of US1/US2.

**Independent Test**: Declare two plugins, one healthy and one pointing at a missing executable. The status surface lists both with correct capability, scope, and last outcome; disabling the healthy one stops further invocations immediately.

**Acceptance Scenarios**:

1. **Given** declared plugins, **When** the user opens the plugin status surface, **Then** each plugin shows name, capability, scope, enabled state, and last invocation outcome with reason and timing.
2. **Given** a plugin that keeps failing, **When** the user inspects it, **Then** the last error reason is visible (sanitized and truncated to a readable length).
3. **Given** the user disables a plugin, **When** they continue browsing, **Then** no further invocations of it occur until re-enabled, effective immediately.
4. **Given** a plugin whose last invocation failed, **When** the user triggers retry from the status surface, **Then** the plugin is re-invoked for that failed target immediately and the row reflects the new outcome.

---

### Edge Cases

- Provider hangs: the enforced timeout converts it into a normal failure; the app never waits indefinitely.
- Provider crashes, exits abnormally, or emits malformed/partial output: treated as failure; raw output is never rendered.
- Provider output contains terminal control/escape sequences: sanitized before any rendering — plugin output must never be able to corrupt or manipulate the display.
- Oversized results (e.g., tens of thousands of discovered names, megabytes of metadata): hard caps with a visible truncation indicator.
- Discovered names the user cannot actually access: shown like any bucket; entering one yields the existing honest access-denied state (no speculative pre-validation storm).
- Names duplicated between pinned and discovered sets: deduplicated; manual pins always survive provider failure.
- Declared executable missing or not runnable: plugin marked unavailable with a clear status; the app starts and runs normally.
- Rapid repeated triggers (e.g., spamming open/close of object details): at most one in-flight invocation per target; superseded results are dropped.
- Provider returns data that looks secret-like: the app logs only invocation facts (who, what, how long, outcome), never payload contents.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to declare external plugins in the application configuration. Every plugin is an explicit opt-in declaration; the system MUST NOT auto-discover or auto-execute anything not declared.
- **FR-002**: Each plugin declaration MUST state exactly which capability it provides. The initial capability set is: (a) bucket discovery, (b) object metadata enrichment.
- **FR-003**: Discovery plugins MUST be assignable to specific connections. Metadata plugins MUST be scoped by a matching rule (connection plus bucket and/or key pattern), and only matching objects trigger invocation.
- **FR-004**: For a connection with an assigned discovery plugin, the system MUST present the bucket list as the deduplicated union of all available sources — pinned names, storage-side listing results (when listing is available), and discovered names. Discovery is always additive: it never replaces or suppresses other sources, and the list MUST NOT require a storage-side listing to be populated.
- **FR-005**: All plugin invocations MUST be asynchronous and non-blocking: the user can navigate, filter, and cancel at any time while a plugin runs.
- **FR-006**: Every invocation MUST run under an enforced timeout (per-plugin configurable, default 5 seconds); exceeding it is a failure.
- **FR-007**: Plugin failure of any kind (timeout, crash, invalid output, missing executable) MUST NOT break or block core browsing. The system falls back to its pre-plugin behavior for the affected view and shows a succinct, non-modal notice.
- **FR-008**: The request context passed to a plugin MUST be limited to non-secret data: connection name, endpoint, user label, access key ID (a public identifier), and the target bucket/object key where applicable. The secret key and any other credential material MUST never be passed to plugins nor written to logs.
- **FR-009**: Plugin output MUST be validated against the declared contract before use; invalid output is a failure. All plugin-supplied text MUST be sanitized before rendering so it cannot inject control sequences into the display.
- **FR-010**: Results MUST be capped (maximum discovered names per connection; maximum metadata fields and field length per object). Truncation MUST be visibly indicated.
- **FR-011**: Every invocation MUST be logged with plugin name, capability, target, duration, and outcome — never with payload contents.
- **FR-012**: Users MUST be able to see, inside the app, every declared plugin with its capability, scope, enabled state, and last invocation outcome including a failure reason. Users MUST be able to re-invoke a failed plugin from the status surface.
- **FR-013**: Enriched metadata MUST appear as a distinct, source-attributed group within object details, consistent with existing metadata grouping, and its fields MUST support the existing per-field copy mechanism.
- **FR-014**: Plugin results MUST be cached per session per target, consistent with existing cache semantics: only manual refresh re-invokes; results of superseded invocations are dropped.
- **FR-015**: The plugin contract MUST be versioned. On a contract-version mismatch the plugin is disabled with a clear status and the application continues normally.
- **FR-016**: The capability contract (request and response shapes) MUST be channel-agnostic so that future provider channels — including a bridge to MCP servers — can be added without redesigning the capability model.
- **FR-017**: No plugin capability may cause a storage mutation through the application; the plugin layer MUST respect the application's read-only posture.
- **FR-018**: Users MUST be able to enable/disable each plugin without deleting its declaration.
- **FR-019**: Discovered bucket names MUST be validated as plausible bucket names; invalid entries are discarded and counted in the failure/partial notice.

### Key Entities

- **Plugin declaration**: A named, user-authored configuration entry: capability, scope (connections / matching rule), invocation target, timeout, enabled flag.
- **Capability**: The contract type a plugin fulfills — bucket discovery or object metadata enrichment — defining request context and expected result shape.
- **Invocation**: A single request/response exchange with a plugin: target, start time, duration, outcome (success / failure reason / timeout), result reference.
- **Discovery result**: A set of bucket names produced for one connection, merged with pinned names for display.
- **Enrichment result**: An ordered list of named fields attributed to a provider for one specific object.
- **Plugin status**: The last-known operational state per plugin (enabled, available, last invocation outcome), surfaced in-app.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a connection where bucket listing is unavailable, a user with a configured discovery provider sees their accessible buckets within 5 seconds of opening the connection, without typing a single bucket name.
- **SC-002**: The interface stays responsive during all plugin activity: the user can navigate or cancel at any moment, and no plugin call ever blocks input.
- **SC-003**: 100% of plugin failure modes (hang, crash, malformed output, missing executable) leave browsing fully functional and produce a visible explanation.
- **SC-004**: For objects within a configured scope, external metadata appears within 3 seconds of opening details under typical conditions, with an explicit pending state shown before it arrives.
- **SC-005**: A user can answer "which plugins are configured, and did their last call succeed?" inside the app in under 10 seconds, without reading log files.
- **SC-006**: Zero occurrences of storage credentials in plugin requests or in logs, verifiable by inspection and automated checks.
- **SC-007**: Attaching a provider for a new external backend (e.g., a different metadata service) requires only a configuration change plus the external provider itself — no change to the application.

## Assumptions

- Plugins are user-supplied local programs (or endpoints they front) explicitly declared in configuration. There is no plugin registry, marketplace, installation tooling, or auto-discovery in v1; trust is established by the explicit declaration, and plugins run with the user's own privileges.
- Plugins authenticate to their own backends themselves (e.g., an internal provisioning API or the image-storage service). The application never shares storage credentials with plugins (FR-008).
- Explicit MCP support (connecting an MCP server directly) is future work. v1 commits only to a channel-agnostic capability contract that an MCP bridge can later target (FR-016).
- "Image-storage metadata by image id" is one concrete instance of the generic object-metadata capability: extracting the image id from the object key and calling the image-storage service is the plugin's job, not the application's.
- User-invocable action plugins ("run command X on this object") are not part of v1; the capability surface is limited to data providers (discovery, metadata). The contract's capability model is expected to accommodate an action capability later.
- Caching follows the application's existing session semantics: results live until manual refresh; nothing expires on a timer.
- The application's read-only posture toward storage is unchanged by this feature.

## Out of Scope (v1)

- Implementing an MCP client or speaking the MCP protocol directly (only contract compatibility is required).
- Action/command plugins that perform user-invoked operations.
- Plugin distribution, installation, update, or sandboxing tooling beyond timeout enforcement and output validation.
- Any write path to storage.
