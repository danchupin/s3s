# s3s plugins

A plugin is any executable you declare in the `plugins:` section of the s3s
config. s3s runs it as a subprocess — the `cmd` line is split with POSIX
shell-words rules and exec'd directly (never via a shell) — writes **one JSON
request to stdin**, reads **one JSON response from stdout** (up to 1 MiB), and
applies a deadline (default 5 s, per-plugin `timeout:`).

The normative contract lives in
[`specs/018-plugin-system/contracts/plugin-exec-contract.md`](../../specs/018-plugin-system/contracts/plugin-exec-contract.md).
The essentials:

## Request (stdin)

```json
{
  "contractVersion": 1,
  "capability": "bucket-discovery",
  "connection": {
    "name": "prod-rgw",
    "endpoint": "https://cluster.storage.example",
    "userLabel": "svc-images",
    "accessKeyId": "AKIAEXAMPLE123"
  },
  "target": {"bucket": "images-prod", "key": "0af3….jpg"}
}
```

- `target` is present only for `object-metadata`.
- `accessKeyId` is a public identifier. **The secret key is never passed** — in
  any field, in the environment, or in argv. A plugin that needs to call an
  internal API authenticates with its own token.

## Response (stdout)

```json
{"contractVersion": 1, "buckets": ["images-prod", "ml-datasets"]}
{"contractVersion": 1, "fields": [{"name": "Moderation", "value": "approved"}]}
{"contractVersion": 1, "error": "provisioning API returned 503"}
```

- Exactly one of the payload (`buckets` / `fields`) or `error`.
- Exit `0` after writing the response (even a soft `error`); a nonzero exit is
  reported as `exec_error` and stdout is ignored.
- stderr is yours for diagnostics — s3s never renders or logs it.

## Security model

- Plugins run **only** when the config file is owner-only-writable (`chmod
  600`) and owned by you — the same gate as the credential `cmd` source.
- Declaration is explicit opt-in per entry; nothing is auto-discovered.
- Every plugin string is sanitized (control/escape sequences stripped, lengths
  capped) before it can reach the terminal.

## Shipped examples

- [`discovery-static.sh`](discovery-static.sh) — bucket discovery from a flat
  file; the template for wiring a provisioning API.
- [`image-storage-meta.sh`](image-storage-meta.sh) — object metadata: extracts
  an image id from the key and maps an image-storage answer to fields.
