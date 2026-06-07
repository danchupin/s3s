# Feature Specification: Pinned Buckets for Scoped Connections

**Feature Branch**: `010-pinned-buckets`

**Created**: 2026-06-07

**Status**: Draft

**Input**: User description: "Scoped/pinned buckets for connections whose S3 credentials lack the service-level s3:ListAllMyBuckets permission. s3s always starts at list-all-buckets; for bucket-scoped credentials that returns access-denied and dead-ends the user. Allow a connection to declare the bucket names it can access, skip list-all-buckets, browse and switch between those buckets directly (like `mc ls alias/bucket`). Also stop mislabelling every connection-test failure as 'unreachable', and make the test tolerate access-denied-on-listing."

## Background & Motivation

s3s opens every session at the **bucket list**, which it populates with a *list-all-buckets*
request. That request needs a service-level "list all my buckets" privilege. Many real
credentials — especially bucket-scoped keys on Ceph RGW / MinIO — are granted access to a
specific set of buckets but **not** the privilege to enumerate all of them. For such a connection,
s3s today dead-ends three ways:

1. The add-connection **reachability test** uses list-all-buckets, so it fails.
2. The test failure is shown as a blanket **"unreachable"** regardless of the real cause
   (access-denied, not-found, invalid-config), so the user is misdirected.
3. Even if the connection is saved anyway, the **bucket list** itself calls list-all-buckets and
   fails — and there is **no way to type or otherwise reach a specific bucket**.

Other tools (mc, s3cmd) work with such credentials because they **address a named bucket
directly** and never call list-all-buckets. A concrete, verified instance: an Avito Ceph RGW
where domain-style addressing means each provisioned bucket resolves as
`<bucket>.bucket.avito-sd`, while the apex host `bucket.avito-sd` (which list-all-buckets dials)
has no DNS record at all — so listing can never succeed, yet every individual bucket is fully
reachable.

This feature lets a connection **declare the bucket names it can use**, so s3s skips
list-all-buckets and goes straight to those buckets — and lets the user **add and open more
buckets by name at runtime in the UI**, so a single connection is the entry point to many buckets
(no separate connection per bucket, no config-file editing).

## Clarifications

### Session 2026-06-07

- Q: How is the in-app "add bucket" action triggered in the bucket list (US2/FR-013)? → A: A
  "+ add bucket" row at the end of the bucket list (mirrors the existing "+ add connection" row);
  Enter on it opens a single-field name input.
- Q: When credentials CAN list all buckets (list-all works), where is the runtime "add bucket"
  affordance available — so an accidental pin never hides the rest? → A: Scoped only. The
  "+ add bucket" row is shown only when the connection already has pinned buckets OR its
  list-all-buckets attempt failed / was denied / returned empty; it is hidden on a connection whose
  list-all succeeds with results.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse and switch between named buckets with scoped credentials (Priority: P1)

A user has credentials that can access several specific buckets but cannot list all buckets. They
configure a connection with the list of bucket names they can reach. When they open that
connection, s3s shows exactly those buckets — without attempting to list all buckets — and lets
them select, filter, open, and switch between them just like the normal bucket list.

**Why this priority**: This is the core value and the MVP. Without it, scoped-credential users
cannot use s3s at all. With it (even configured manually), they can browse their data end-to-end.

**Independent Test**: Configure a connection with two pinned bucket names against a backend whose
credentials deny list-all-buckets; open the connection; confirm both names appear, no
list-all-buckets call is made, selecting one enters its contents, and switching to the other
works.

**Acceptance Scenarios**:

1. **Given** a connection that pins buckets "alpha" and "beta" and credentials that deny
   list-all-buckets, **When** the user opens the connection, **Then** the bucket list shows
   exactly "alpha" and "beta" and no list-all-buckets request is issued.
2. **Given** the pinned bucket list is displayed, **When** the user selects "alpha" and opens it,
   **Then** the contents of "alpha" load and the user can navigate into prefixes.
3. **Given** the user is browsing inside "alpha", **When** they return to the bucket list and open
   "beta", **Then** the contents of "beta" load without any list-all-buckets request.
4. **Given** a pinned bucket list, **When** the user filters the bucket list, **Then** filtering
   narrows the pinned names exactly as it does for a normally-listed bucket set.
5. **Given** a pinned bucket list, **When** the user refreshes the bucket view, **Then** the same
   pinned names are shown and still no list-all-buckets request is issued.

---

### User Story 2 - Reach different buckets from one connection, in the UI (Priority: P1)

While browsing a connection's bucket list, the user can add and open a bucket by name directly in
the app — without editing any config file — so a single connection serves as the entry point to
many buckets. The newly added bucket appears in the bucket list, opens to its contents, and is
remembered for that connection in future sessions.

**Why this priority**: This is what makes the feature convenient and is explicitly required.
Scoped credentials typically grant access to several buckets; the user must be able to hop between
them under one connection rather than creating a connection per bucket. Together with User Story 1
it forms the real MVP.

**Independent Test**: Open a connection's bucket list, invoke the add-bucket affordance, type a
valid bucket name; confirm it is added to the list, opens to its contents, and is still present
after restarting the app and reopening the connection.

**Acceptance Scenarios**:

1. **Given** a connection's bucket list is shown, **When** the user invokes the add-bucket
   affordance and enters a valid bucket name, **Then** the name is added to the bucket list and can
   be opened to browse its contents.
2. **Given** a bucket was added in-app, **When** the user restarts the app and reopens the same
   connection, **Then** the added bucket is still listed (the addition persisted with the
   connection).
3. **Given** the add-bucket input, **When** the user enters a name already present, **Then** no
   duplicate is created and the existing entry is selected/used.
4. **Given** the add-bucket input, **When** the user cancels, **Then** the bucket list is
   unchanged.
5. **Given** a connection with several buckets, **When** the user switches between all of them,
   **Then** each opens directly and no list-all-buckets request is issued.

---

### User Story 3 - Add and save a scoped connection through the in-app form (Priority: P2)

A user adds a new connection in the app, enters the endpoint, credentials, and the names of the
buckets they can access, and saves it. The reachability test validates against one of the named
buckets (not list-all-buckets). If the backend resolves and answers — even if it denies broader
listing — the connection is treated as reachable and saved, so the user lands in a working browse
session.

**Why this priority**: Makes the feature usable without hand-editing config files; turns the
add-connection flow from a dead-end into a success path for scoped credentials.

**Independent Test**: In the add-connection form, enter a reachable endpoint, scoped credentials,
and a valid pinned bucket name; run the test; confirm it passes (or is savable) rather than
reporting "unreachable", and that after save the app shows the pinned bucket list.

**Acceptance Scenarios**:

1. **Given** the add-connection form with a pinned bucket name entered, **When** the test runs and
   the named bucket is reachable, **Then** the test passes and the connection saves.
2. **Given** the add-connection form with pinned buckets and credentials that deny list-all-buckets
   but allow the named bucket, **When** the test runs, **Then** the result is "reachable" (not
   "unreachable") and the connection is savable.
3. **Given** a saved scoped connection, **When** the user is taken into the browser, **Then** the
   pinned bucket list is shown (per User Story 1).
4. **Given** the user enters several bucket names separated by commas/spaces with stray whitespace
   and duplicates, **When** the connection is saved, **Then** the stored list is trimmed,
   de-duplicated, and preserves the entered order.

---

### User Story 4 - See the real reason a connection test failed (Priority: P3)

When a connection test fails, the user sees a message that reflects the actual cause — access
denied, not found, unreachable, or invalid configuration — instead of a single generic
"unreachable". The "press again to save anyway" escape hatch is preserved.

**Why this priority**: Pure diagnosis quality. It does not unblock scoped credentials by itself,
but it stops actively misleading users (e.g. an access/DNS/config problem reported as a network
outage) and shortens debugging.

**Independent Test**: Drive the form's test against each failure cause and confirm the displayed
message matches the cause and still offers "save anyway".

**Acceptance Scenarios**:

1. **Given** a test that fails because credentials are rejected, **When** the result is shown,
   **Then** the message says access was denied (not "unreachable").
2. **Given** a test that fails because the host cannot be reached, **When** the result is shown,
   **Then** the message says the backend is unreachable.
3. **Given** any test failure, **When** the result is shown, **Then** the user can still press
   again to save the connection anyway.

---

### Edge Cases

- **Pinned bucket that does not exist / is inaccessible**: entering it surfaces the normal
  per-bucket error (not-found / access-denied) for that bucket; other pinned buckets are
  unaffected.
- **Connection that both pins buckets and could list all**: the pinned list takes precedence
  (deterministic, honors explicit user intent); list-all-buckets is not called.
- **Empty pinned list**: behavior is identical to today (list-all-buckets).
- **Whitespace / duplicate / empty entries** in the entered names: normalized away; order of
  remaining names preserved.
- **Refresh on a pinned bucket list**: re-renders the pinned names; does not turn into a
  list-all-buckets call.
- **A pinned bucket is deleted (where writes are enabled)**: the pin may become stale; this is
  acceptable and user-correctable by editing the connection. (No automatic pin pruning in v1.)
- **Adding (in-app) a bucket the credentials cannot access**: the name is still added to the list;
  opening it surfaces the normal per-bucket access-denied. (Same stale-pin tolerance.)
- **Adding (in-app) a bucket on a scoped connection that currently has no pins** (list-all
  failed/denied/empty, so the "+ add bucket" row is shown): the connection switches to the pinned
  model for that name (and any later additions); it no longer calls list-all-buckets while pins
  exist. A connection whose list-all succeeds never shows the row and cannot be switched this way.
- **Removing/unpinning a bucket from the set**: out of scope for v1 (user edits the connection
  config). This deliberately avoids overloading the existing bucket-delete (destructive) key.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A connection MUST be able to declare an ordered list of bucket names it can access
  ("pinned buckets"). The list is optional and defaults to empty.
- **FR-002**: When a connection has one or more pinned buckets, s3s MUST populate the bucket list
  from that list and MUST NOT issue a list-all-buckets request for that connection.
- **FR-003**: A pinned bucket list MUST support the same interactions as a normally-listed bucket
  set: selection movement, filter, open/enter to browse, and switching between buckets.
- **FR-004**: When a connection has no pinned buckets, s3s MUST behave exactly as today (populate
  the bucket list via list-all-buckets).
- **FR-005**: The add-connection form MUST let the user enter the pinned bucket list (zero or more
  names in one field).
- **FR-006**: Entered bucket names MUST be normalized before persistence and display — leading and
  trailing whitespace trimmed, empty entries dropped, duplicates removed — while preserving the
  order in which distinct names were entered.
- **FR-007**: Pinned buckets MUST be persisted with the connection and MUST be present after a
  restart and after an in-session context switch into that connection.
- **FR-008**: When a connection has pinned buckets, the add-connection reachability test MUST
  validate against a pinned bucket (a minimal, bounded read of that bucket) instead of
  list-all-buckets.
- **FR-009**: The reachability test MUST treat an access-denied outcome as "reachable but
  unprivileged" — a passing/savable result, not a hard failure — because the endpoint resolved and
  the server answered. Unreachable, not-found, and invalid-configuration outcomes remain failures
  that offer "save anyway".
- **FR-010**: When a connection test fails, the message shown MUST reflect the classified cause
  (access denied / not found / unreachable / invalid configuration) rather than a single generic
  "unreachable" string, and MUST preserve the "press again to save anyway" affordance.
- **FR-011**: Pinned-bucket browsing MUST function under domain/virtual-hosted-style addressing
  (so each named bucket is addressed at its own host), since that is a primary motivating
  configuration.
- **FR-012**: The feature MUST NOT introduce any new write or mutating storage operation; it relies
  only on existing read operations. (Persisting a pinned bucket is a configuration write, not a
  storage/S3 write.)
- **FR-013**: The bucket list MUST offer an in-app "add bucket" affordance rendered as a
  "+ add bucket" row at the end of the list (mirroring the "+ add connection" row); selecting it
  opens a single-field name input, and opening the resulting name MUST browse that bucket's
  contents directly — all without editing a config file.
- **FR-013a**: The "+ add bucket" row MUST be shown only for a "scoped" bucket list — i.e. when the
  connection already has pinned buckets OR its list-all-buckets attempt failed / was denied /
  returned empty. It MUST be hidden when list-all-buckets succeeds with results, so a connection
  that can enumerate its buckets never gains hidden buckets via an accidental pin.
- **FR-014**: A bucket added in-app MUST be persisted to the active connection's pinned set, so it
  is present when the connection is reopened in a later session. Persistence MUST happen off the UI
  event loop and MUST NOT block the interface.
- **FR-015**: An in-app bucket addition MUST apply the same normalization as FR-006 (trim, drop
  empty, dedupe); adding a name already present MUST NOT create a duplicate.
- **FR-016**: Adding the first bucket to a connection that had no pins (only possible when the
  "+ add bucket" row is shown per FR-013a — i.e. list-all unavailable/denied/empty) MUST switch
  that connection to the pinned model (subsequent bucket-list loads use the pinned set, not
  list-all-buckets). A connection whose list-all succeeds never reaches this state via the UI.

### Key Entities *(include if feature involves data)*

- **Connection**: an existing configured endpoint + credentials + options. Gains one new optional
  attribute: an ordered list of **Pinned Bucket Names**.
- **Pinned Bucket Name**: a bucket name the connection's credentials can access directly, without
  enumerating all buckets. Used to render the bucket list and to target the reachability test. The
  list is editable in two places: at connection creation (the add-connection form) and at runtime
  (the bucket-list add affordance), both feeding the same per-connection set.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user whose credentials cannot list all buckets can open a configured connection and
  browse a named bucket's contents without editing any config file by hand and without
  encountering a list-all-buckets failure.
- **SC-002**: Adding such a connection through the in-app form results in a saved, immediately
  browsable connection (the add flow no longer dead-ends for scoped credentials).
- **SC-003**: No connection-test failure is presented as "unreachable" when the real cause is
  access-denied, not-found, or invalid-configuration — the message matches the cause in 100% of
  these cases.
- **SC-004**: A connection without pinned buckets exhibits no behavioral change from the previous
  release (no regression in the list-all-buckets flow).
- **SC-005**: While a pinned connection is active — including selecting, opening, switching
  between, and refreshing pinned buckets — zero list-all-buckets requests are issued.
- **SC-006**: A user can reach a second, different bucket under the same connection from within the
  UI, without editing any config file, and that bucket remains available after restarting the app.

## Assumptions

- Target backends are S3-compatible (Ceph RGW / MinIO) where bucket-scoped credentials and direct
  per-bucket addressing are common and expected.
- Domain/virtual-hosted-style addressing (bucket name in the host) is a valid, in-use
  configuration and must be supported; e.g. the Avito RGW where `<bucket>.bucket.avito-sd` resolves
  per provisioned bucket while the apex host is unlistable.
- The user knows the names of the buckets their credentials can access (the same precondition that
  mc/s3cmd impose when addressing a bucket directly).
- Pinned buckets are a per-connection property; two connections to the same endpoint may pin
  different sets. The set is grown both at creation (form) and at runtime (in-app add), and
  persists with the connection.
- A pinned bucket that no longer exists or becomes inaccessible surfaces normal per-bucket errors
  when entered; a stale pin is acceptable and user-correctable.
- **Out of scope**: exposing TLS-skip-verify in the connection form; any change to the read-only
  posture/guard; auto-discovery or suggestion of accessible buckets; pruning stale pins; an in-app
  un-pin/remove-from-set action (deferred to avoid overloading the destructive bucket-delete key).
- Governed by Constitution v1.0.0; no amendment expected. All changes confined to the UI,
  config, and the connection-test seam — no new storage-interface methods and no relaxation of the
  structural read-only guard.
