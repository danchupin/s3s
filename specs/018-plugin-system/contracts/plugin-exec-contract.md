# Contract: Plugin Subprocess Exchange

**Feature**: 018-plugin-system | **Contract version**: 1

The normative contract between s3s and an external plugin process. A plugin is any
executable that fulfills this exchange. The envelope is channel-agnostic by design: a
future bridge (e.g., MCP) maps the same request/response objects onto another transport
without changing their shape.

## Process model

1. s3s splits the declared `cmd` string with POSIX shell-words rules (quotes honored,
   no variable expansion, no globbing) and executes it as argv. Never via a shell.
2. Execution is refused unless the s3s config file is owner-only-writable (mirrors the
   credential `cmd` source defense).
3. s3s writes exactly one JSON request object to the plugin's **stdin** and closes it.
4. The plugin writes exactly one JSON response object to **stdout** and exits.
5. stderr is ignored for data (reserved for the plugin's own diagnostics); it is never
   rendered and never logged by s3s.
6. A deadline applies to the whole exchange (default 5 s, per-plugin `timeout:`).
   On expiry s3s kills the process; outcome = `timeout`.
7. stdout is read up to 1 MiB; longer output ⇒ outcome `invalid_output`.

## Outcome classification

| Condition | Outcome |
|-----------|---------|
| exit 0 + valid success response | `ok` |
| exit 0 + valid `error` response | `contract_error` (soft failure, reason shown) |
| nonzero exit / spawn failure / unparsable cmd | `exec_error` |
| deadline exceeded | `timeout` |
| unparsable JSON, >1 MiB, schema violation | `invalid_output` |
| `contractVersion` ≠ 1 in response | `incompatible` → plugin disabled in status |

Every outcome except `ok` leaves the host view on its pre-plugin behavior (pinned/listed
buckets; native metadata only) and is reported via the transient notice + status surface.

## Request (s3s → plugin, stdin)

```json
{
  "contractVersion": 1,
  "capability": "bucket-discovery",
  "connection": {
    "name": "prod-rgw",
    "endpoint": "https://cluster.storage.example",
    "userLabel": "svc-images",
    "accessKeyId": "AKIAEXAMPLE123"
  }
}
```

```json
{
  "contractVersion": 1,
  "capability": "object-metadata",
  "connection": {
    "name": "prod-rgw",
    "endpoint": "https://cluster.storage.example",
    "userLabel": "svc-images",
    "accessKeyId": "AKIAEXAMPLE123"
  },
  "target": {
    "bucket": "images-prod",
    "key": "0af3c9d2e8b14f6790aa31c5d77e4b21.jpg"
  }
}
```

Guarantees:
- `accessKeyId` is a public identifier. **The secret key, session tokens, or any other
  credential material are never present in any field, in the environment added by s3s,
  or in argv constructed by s3s.**
- `target` present iff capability is `object-metadata`.
- Unknown future fields must be ignored by plugins (forward compatibility).

## Response (plugin → s3s, stdout)

### bucket-discovery success

```json
{"contractVersion": 1, "buckets": ["images-prod", "images-stage", "ml-datasets"]}
```

- s3s validates each name against S3 bucket-name rules (3–63 chars; lowercase letters,
  digits, `.`, `-`; alphanumeric at both ends). Invalid entries are discarded and
  counted; the count appears in the partial-result notice.
- ≤ 5 000 names used; excess truncated with indication.
- Result merges additively: `pinned ∪ listed (when listing available) ∪ discovered`.

### object-metadata success

```json
{
  "contractVersion": 1,
  "fields": [
    {"name": "Image ID", "value": "0af3c9d2e8b14f6790aa31c5d77e4b21"},
    {"name": "Dimensions", "value": "1024x768"},
    {"name": "Moderation", "value": "approved"},
    {"name": "Owner service", "value": "avito-images"}
  ]
}
```

- Field order is preserved in display.
- ≤ 64 fields; values ≤ 4 096 bytes; overruns truncated with explicit indication.
- An empty `fields` array is a valid "nothing known" answer — rendered as an empty
  group state, distinct from failure.

### Soft failure

```json
{"contractVersion": 1, "error": "provisioning API returned 503"}
```

- Mutually exclusive with payload fields. The reason is sanitized, truncated to 200
  chars, shown in status and in the first-failure notice.

## Sanitization (s3s-side, applies to ALL plugin strings)

Every string from a plugin (names, field names/values, error reasons) is sanitized
before entering the UI model: C0/C1 control characters and ANSI CSI/OSC/escape
sequences stripped; newlines collapsed to spaces on single-line surfaces; length caps
enforced with visible truncation. Plugin text can never alter terminal state.

## Exit-code convention for plugin authors

- `0` — response object written (success or soft `error`).
- nonzero — fatal: s3s reports `exec_error`; whatever was on stdout is ignored.

## Reference implementations (shipped in `docs/plugins/`)

- `discovery-static.sh` — discovery stub: emits names from a flat file; the template
  for wiring an internal provisioning API.
- `image-storage-meta.sh` — metadata stub: extracts the image id from the key, queries
  the image-storage endpoint with the caller's own token, maps the reply to `fields`.
