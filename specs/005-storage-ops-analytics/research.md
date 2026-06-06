# Phase 0 Research: Storage Operations & Analytics

All decisions below resolve the Technical Context unknowns. Format: Decision / Rationale /
Alternatives considered.

## R1 — Full-object download (US1) is a READ, not a write

**Decision**: Add `GetObject(ctx, bucket, key) (io.ReadCloser, error)` to `storage.Storage`
(the read interface), streaming the whole object. The UI writes the stream to a local
`<dest>.partial` temp file and atomically renames to `<dest>` on success; cancel/failure removes
the partial. Download is allowed in read-only contexts (FR-002).

**Rationale**: Downloading reads the remote and writes the *local* disk; it does not mutate S3.
Keeping it on the read interface means `readOnlyGuard` passes it straight through, no `--write`
needed, and `check-readonly.sh` stays green (`GetObject` is a read SDK symbol, and it lives in
`internal/storage` anyway). Temp-file + atomic rename guarantees no half-file ever looks complete
(FR-006); removing the partial on cancel satisfies FR-004.

**Alternatives**: (a) Reuse `GetObjectRange(0, size-1)` — needs a prior HeadObject for size and
re-implements full-stream semantics; rejected as indirect. (b) Put download behind the `Mutator`
interface — wrong: it would falsely require `--write` and contradict FR-002.

## R2 — `du` analytics (US2): one recursive read + client-side immediate-child bucketing

**Decision**: Add `UsageOf(ctx, bucket, prefix string, onProgress func(UsageProgress)) (UsageReport, error)`
to `storage.Storage`. Implementation paginates `ListObjectsV2` under `prefix` with **no
delimiter** (full recursive listing), accumulating: grand total size/count, and per-**immediate-
child** size/count (the child = the first path segment after `prefix`, treated as a sub-prefix if
it contains a further `/`, else a direct object). `onProgress` fires every N pages with running
totals; `ctx` cancellation stops paging and returns partial-but-truthful counts.

**Rationale**: A single recursive scan with in-memory bucketing is O(keys) calls-free beyond
pagination — far cheaper than per-child recursive `du`. Immediate-child ranking is exactly the
`ncdu` view (FR-009). Incremental `onProgress` + cancellable `ctx` gives the non-blocking,
running-total UX (FR-011, SC-002/006). Current-version-only accounting falls out naturally from a
plain list (matches the Assumption; versioned accounting is out of scope).

**Alternatives**: (a) Server-side inventory/metrics — not portable across Ceph RGW / MinIO,
rejected. (b) Recursive `du` per child prefix — N× the list calls, rejected. (c) Reuse the
existing recursive enumerator behind `DeleteRecursive` — that lives behind `Mutator`; instead
share a private paginating helper in `s3client.go` between the (write) recursive delete and the
(read) `UsageOf`.

## R3 — Dynamic read-only guard for the runtime write toggle (US5)

**Decision**: Move guarding from construction-time (`main.go` calls `storage.Guard`) to
**runtime** in the UI. The `Resolver`/`Backend` returns the **raw** (unguarded) client plus a
`ReadOnly bool` (true when the context is `readonly: true`). `App` holds `raw storage.Storage`,
`ctxReadOnly bool`, and `armed bool`; the derived `writable = armed && !ctxReadOnly`. A helper
`activeStore()` returns `storage.Guard(m.raw, m.writable)` and every operation uses it. The
`--write` flag sets the initial `armed`. Toggling arms (after a simple confirm) or disarms
(instant); context switch re-derives `ctxReadOnly` and thus `writable`.

**Rationale**: The guard stays the single runtime enforcement point (`guard.go` unchanged) — we
just choose the wrapper dynamically. `readonly: true` remains absolute because `writable` can
never be true for it. Operations already branch on `m.writable`; routing them through
`activeStore()` keeps a mutating call impossible while disarmed. UI remains SDK-free.

**Alternatives**: (a) Re-resolve the backend on every toggle (rebuild the S3 client) — wasteful
and drops connection state; rejected. (b) Keep construction-time guard and expose a setter on the
guard — mutable guard state is a foot-gun; rejected for the cleaner derive-and-wrap.

## R4 — Loud, always-on WRITE indicator (US5/FR-027)

**Decision**: A high-contrast `WRITE` badge (inverse/red background, bold — new `writeBadgeStyle`
in `styles.go`) rendered in the footer identity line on the normal views **and** injected into
the alt-screen overlays (action menu, help, object view) so it is present on *every* screen and
is never the first element dropped when width is tight. Read-only shows a calm `RO`. The badge
reflects the *current context's* `writable`, re-derived on context switch.

**Rationale**: FR-027 demands the danger state be impossible to miss and impossible to scroll
off. The footer identity line already carries `[RW|RO]`; this elevates it to a screaming badge
and guarantees presence on overlay screens that render without the normal footer.

**Alternatives**: A one-shot toast on arming — fails "persistent / every screen"; rejected. A
full-width colored top bar — heavier on the height budget; the badge is enough and cheaper.

## R5 — Per-level selection state (US3)

**Decision**: `App` holds `sel map[string]bool` keyed by object key, plus a derived count and
combined size computed from the loaded level. Only objects are insertable (folders rejected,
FR-014). A `Mark` key (space) toggles the current row. `sel` is cleared in `enterLevel`, the
back/up path, `applyContext`, and bucket entry (navigation = clear, FR-019). The selection count
+ size render in the box header/footer; marked rows get a visible marker glyph.

**Rationale**: A per-level map is the minimal state; clearing on navigation bounds the lifetime
and matches the spec's blast-radius decision (objects-only, no folder marking → recursive delete
stays its own action). Combined size is derivable from `m.level.objects`, no extra fetch.

**Alternatives**: Cross-level "cart" — explicitly out of scope; rejected. Marking folders to bulk-
delete — rejected by clarification (blast radius).

## R6 — Bulk actions reuse single-object ops (US3)

**Decision**: A `bulk` operation kind carries the ordered list of marked keys and an action
(`download|delete|copy`). It iterates, applying the existing per-item backend call
(`GetObject`→local for download, `RemoveObject` for delete, `CopyKey` for copy), reporting a
per-item `progressEvent` (done/failed + reason) over the existing channel; the run continues past
failures and ends with a truthful `succeeded/failed` summary (FR-018). Bulk download recreates
the key hierarchy as local subdirectories under the destination (FR-015a). Bulk delete uses the
typed confirmation tier with the **count** as the confirm target; each delete is logged before
execution (FR-017). Bulk delete/copy require `activeStore()` to be writable.

**Rationale**: Reuses proven, individually-tested mutations and the existing progress/cancel
machinery — no new safety framework (Assumption). Hierarchy-preserving download is the
clarified, collision-free layout.

**Alternatives**: A server-side batch `DeleteObjects` — a single typed confirm hiding hundreds of
deletes is higher-risk and the SDK batch-delete is a new write symbol; the per-item loop keeps
logging granular and reuses `RemoveObject`. Revisit only if throughput demands it.

## R7 — Stateless, session-persistent sort (US4)

**Decision**: `App` holds `sortCol` (name|size|modified) and `sortDir` (asc|desc), persisted for
the session (not reset on navigation, FR-020). Sorting is applied at **render time** to a copy of
the level's `dirs`+`objects` (consistent with the existing stateless `windowBounds` model). Dirs
(no size/date) sort by name and are grouped consistently (e.g. always above objects) when sorting
by size/modified (FR-021). A `Sort` key cycles the column; a modifier/second key toggles
direction. Sorting composes with the active search/filter (sorts the filtered set).

**Rationale**: Render-time sort keeps selection indices as the only mutable list state and makes
resize/reflow trivial (the architecture's existing principle). Session persistence matches the
k9s feel from the clarification.

**Alternatives**: Sort the cached `levelState` in place — couples cache to presentation and
complicates pagination merges; rejected.

## R8 — Credential sources & one-source-per-context (US6)

**Decision**: Extend the config `User` with mutually-exclusive source descriptors; **exactly one**
is allowed (validation error otherwise, FR-041): existing `secretAccessKey` (inline/`${ENV}`),
`secret: keychain`, `cmd: "<command>"`, or `awsProfile: "<name>"`. A new `internal/secret` package
resolves the chosen source to a secret at connection-build time; the secure prompt is the implicit
fallback only when the configured source yields nothing. `accessKeyId` (non-secret) stays in
config for keychain/cmd/env; `awsProfile` supplies both keys (+ optional session token).

**Rationale**: One explicit source removes precedence ambiguity (clarification) and is trivially
testable. Resolution in a dedicated package preserves Core/UI Separation and keeps the SDK out.

**Alternatives**: A precedence chain — rejected by clarification (hidden magic). A single
`source:` map field — more flexible but worse YAML ergonomics/validation; the discrete fields are
clearer.

## R9 — OS keystore library: `github.com/zalando/go-keyring`

**Decision**: Use `zalando/go-keyring` for store/fetch/remove. Namespacing: `service = "s3s"`,
`account = "<context>"` (or `<cluster>/<user>`). Linux requires a Secret Service provider (D-Bus);
when absent (headless), `Get` errors → s3s surfaces a clear message and the engineer uses
command/profile/env/prompt instead (FR-043, SC-015). Windows Credential Manager is best-effort.

**Rationale**: Pure-Go-ish, no cgo on Linux (D-Bus), widely used, covers macOS/Linux/Windows with
one API. Matches the threat model (secret in the OS keystore, never s3s files).

**Alternatives**: `99designs/keyring` (heavier, more backends incl. file/pass — but a file backend
reintroduces on-disk secrets we explicitly avoid); `keybase/go-keychain` (macOS only). Rejected
for breadth/simplicity reasons.

## R10 — `cmd:` execution safety (US6/FR-036)

**Decision**: Split the configured `cmd:` string into argv with POSIX shell-words rules
(`github.com/google/shlex` — honors single/double quotes and escapes, NO variable/glob expansion),
run via `exec.Command(argv[0], argv[1:]...)` (not `sh -c`), capture stdout, trim a trailing
newline → secret. An empty/unparseable split is a clear config error. **Before executing**, stat
the config file: refuse if it is group/world writable or not owned by the running uid, with a
clear message (blocks "attacker edits YAML → command runs at launch"). stderr and the command line
are never logged as secret material; the output is wrapped in `logging.Secret`.

**Rationale**: Owner-only gating is the cheap, strong defense the clarification chose. Argv (not
shell) execution avoids an extra injection surface. A short timeout via `ctx` prevents a hung
launch.

**Alternatives**: Always run + warn — rejected by clarification (warning ignored = RCE). Running
through `sh -c` — convenient but widens the injection surface; rejected.

## R11 — AWS shared-profile source (US6)

**Decision**: Parse `~/.aws/credentials` (honoring `AWS_SHARED_CREDENTIALS_FILE`) as INI for the
named profile's `aws_access_key_id` / `aws_secret_access_key` / optional `aws_session_token`
(static keys only). SSO / role assumption / `credential_process` are **out of scope** (documented)
— if a profile lacks static keys, surface a clear error.

**Rationale**: A tiny INI read covers the overwhelmingly common case (engineers already have
static-key profiles) without pulling the full SDK credential-provider chain into the config layer.
Keeps behavior explicit and testable against fixture files.

**Alternatives**: `aws-sdk-go-v2/config.LoadSharedConfigProfile` / full provider chain — drags in
SSO/process providers and async refresh semantics that conflict with "resolve once at launch";
rejected for this iteration.

## R12 — Secure prompt timing & context switch (US6)

**Decision**: The no-echo prompt (`golang.org/x/term.ReadPassword`) runs **before** the Bubble Tea
program starts (the TUI owns the terminal afterward). At startup, the active context's secret is
resolved — prompting if needed — then the program runs. Non-interactive sources resolve fine on a
later in-TUI **context switch**; if a switched-to context would require a prompt, s3s shows a clear
notice ("relaunch with this context to enter its secret") rather than corrupting the terminal.
After a successful prompt, offer to save into the keystore (FR-038).

**Rationale**: Honors Constitution V (TUI owns the terminal — no mid-frame prompts). Covers the
fresh-terminal / SSH startup case (SC-011/015) and keeps context switching non-interactive.

**Alternatives**: Suspend the TUI to prompt mid-session — fragile with the alt-screen renderer;
deferred. Pre-resolve every context's secret at startup — could fire many prompts/commands the
engineer never uses; rejected.

## R13 — Config-permissions warning (US6/FR-040)

**Decision**: On `config.Load`, stat the file; if group/world **readable** (or writable), emit a
one-line warning (stderr at startup) advising `chmod 600`. This is advisory for readability and
*enforced* (refusal) only for the `cmd:` source (R10).

**Rationale**: A readable config with a `${ENV}`/`awsProfile` reference is less dangerous than a
writable one feeding `cmd:`; warn broadly, refuse narrowly where the risk is code execution.

**Alternatives**: Hard-refuse any loose-perms config — too aggressive for existing setups; could
break CI. Warn-only everywhere — too weak for `cmd:`; the split (R10 enforce / R13 warn) is the
balance.
