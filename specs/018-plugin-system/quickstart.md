# Quickstart: Plugin System — RED Test Sets

**Feature**: 018-plugin-system. TDD entry points per user story: write these failing
first (constitution III), then implement to green.

## Commands

```bash
make test                                   # full unit suite
go test ./internal/plugin/                  # runner + sanitizer + envelope
go test ./internal/config/ -run TestPlugins # config section
go test ./internal/ui/ -run TestPlugin      # UI merge / enrichment / status surface
make fmt vet lint check-readonly            # gates (check-readonly must stay green)
make test-integration                       # MinIO suite — must stay green, unchanged
```

## RED set 0 — core package foundations (shared by all stories)

`internal/plugin/plugin_test.go`, `runner_test.go`, `sanitize_test.go`:

1. Envelope round-trip: request encodes contractVersion=1, capability, connection
   (name/endpoint/userLabel/accessKeyId), target for object-metadata; **no secret field
   exists in the request type** (compile-level guarantee + marshaled-JSON assertion).
2. Runner happy path: `/bin/sh` fixture echoes a valid discovery response → outcome `ok`,
   names returned.
3. Timeout: fixture sleeps past a 100 ms test timeout → outcome `timeout`, process gone.
4. Nonzero exit → `exec_error`; stdout ignored.
5. Garbage stdout → `invalid_output`.
6. stdout > 1 MiB → `invalid_output` (read cap).
7. Soft failure `{"error": "..."}` → `contract_error`, reason captured ≤ 200 chars.
8. `contractVersion: 2` in response → `incompatible`.
9. Missing executable → `exec_error`; classified for `unavailable` status.
10. Owner-only gate: world-writable config path ⇒ runner refuses, no process spawned.
11. Sanitizer tables: CSI/OSC/C0/C1 stripped, UTF-8 preserved, newline collapse,
    length-cap with truncation marker.
12. Bucket-name validation tables: valid kept; uppercase/short/long/punctuation-edge
    rejected and counted.
13. slog capture: every invocation emits exactly one record (plugin, capability, target,
    duration_ms, outcome); record contains neither response payload nor argv.

## RED set 1 — US1 bucket discovery (P1)

`internal/config/plugins_test.go` + `internal/ui/plugins_test.go`:

1. Config: discovery declaration parses with defaults (timeout 5s, enabled);
   duplicate name / unknown capability / empty cmd / no connections ⇒ load errors;
   unknown connection name ⇒ warning + `unavailable`, config still loads.
2. UI merge (fake Runner): listing denied + discovery ok ⇒ list = pinned ∪ discovered,
   dedup, sorted; no ListBuckets call when pinned-only path already applies.
3. Listing available + discovery ok ⇒ pinned ∪ listed ∪ discovered (additive — Q3).
4. Discovery failure ⇒ pinned/listed intact + transient notice naming plugin and reason;
   second failure same session ⇒ no repeat notice.
5. Invalid names in response ⇒ discarded, notice carries discarded count.
6. Stale generation: discovery result for gen N arriving at gen N+1 ⇒ dropped.
7. Refresh `r` ⇒ re-invocation (cache invalidated); without refresh ⇒ cached, no second
   subprocess call (fake Runner call counter).
8. Disabled plugin ⇒ never invoked.

## RED set 2 — US2 metadata enrichment (P2)

`internal/ui/plugins_test.go` (+ details rendering assertions):

1. Match rule: object inside scope (connection+glob+keyPattern) invokes exactly the
   matching plugins; outside scope ⇒ zero invocations, no group rendered.
2. Group lifecycle in `App.View().Content`: `pending` → populated fields in plugin
   order under `From <plugin>` header.
3. Failure ⇒ `failed: <reason>`; empty fields array ⇒ empty-state text distinct from
   failure; NO_COLOR run keeps states distinguishable.
4. Field values flow through existing per-field reveal/copy path; truncated value
   carries marker and reveals in full.
5. Selection change before result ⇒ stale result dropped; cache hit on reselect (call
   counter static).
6. Two matching plugins ⇒ two groups, declaration order.
7. Rapid repeated selection of the same object ⇒ at most one in-flight invocation
   (call counter = 1 until the result lands).

## RED set 3 — US3 status surface (P3)

`internal/ui/plugins_test.go`:

1. `P` (and `:plugins`) opens modePlugins; Esc returns to previous mode; with zero
   declared plugins `P` is a no-op and hints line shows no plugin hint.
2. Rows render name/capability/scope/state; states `ok/failed/disabled/unavailable/
   incompatible` text-distinct under NO_COLOR.
3. `space` toggle: fake Connector records SetPluginEnabled; optimistic state flip;
   Connector error ⇒ state reverts + notice.
4. Disabled via toggle ⇒ subsequent loads skip the plugin immediately.
5. `Enter` reveals full sanitized error detail; footer + hints visible at 130×24 and
   floor sizes (height budget respected).
6. `r` retry re-invokes the selected plugin's last failed target; a plugin that
   returned `incompatible` is never invoked again within the session.

## Manual validation (after green)

1. Build (`make build`), declare `docs/plugins/discovery-static.sh` for a connection
   with `pathStyle: false` and no list permission → buckets appear ≤ 5 s; kill the
   script's data source → notice + `P` shows `failed`.
2. Declare `docs/plugins/image-storage-meta.sh` with a key pattern; open a matching
   object → `From image-storage-meta` group: pending → fields; copy a field.
3. `NO_COLOR=1` pass over the status surface and enrichment states.
4. 130×24 + narrow-terminal pass: footer/hints never scroll off on modePlugins.
5. `chmod 666` the config → plugins refuse with clear status; `chmod 600` restores.
