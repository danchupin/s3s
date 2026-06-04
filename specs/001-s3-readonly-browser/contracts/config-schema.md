# Contract: Config Schema (`~/.config/s3s/config.yaml`)

**Feature**: 001-s3-readonly-browser | kubectl-style, YAML.

Location resolved via XDG: `$XDG_CONFIG_HOME/s3s/config.yaml`, falling back to
`~/.config/s3s/config.yaml`. File expected to be user-protected (`0600`); s3s warns if more
permissive when secrets are stored inline.

## Schema

```yaml
apiVersion: s3s/v1

clusters:
  - name: minio-local
    endpoint: http://127.0.0.1:9000
    region: us-east-1
    pathStyle: true          # MinIO / Ceph RGW path-style; false => virtual-host/domain-style
    tlsSkipVerify: false      # explicit opt-in only, https endpoints
  - name: rgw-prod
    endpoint: https://rgw.example.com:8080
    region: us-east-1
    pathStyle: false          # domain (virtual-host) style

users:
  - name: dev
    accessKeyId: AKIAEXAMPLE
    secretAccessKey: ${S3S_DEV_SECRET}   # ${ENV} reference; resolved at load, never logged
    # sessionToken: optional STS token
  - name: public
    anonymous: true            # no credentials; public-bucket read access

contexts:
  - name: local
    cluster: minio-local
    user: dev
  - name: prod-public
    cluster: rgw-prod
    user: public

current-context: local
```

## Rules

- **Required**: at least one cluster, one user, one context, and a `current-context` that resolves.
- **Cross-refs**: every `context.cluster`/`context.user` MUST name an existing entry; names unique
  per list. Invalid references → load error with a clear, secret-free message.
- **Credentials** (FR-005, FR-005a):
  - `anonymous: true` ⇒ ignore key fields, build an anonymous client.
  - otherwise `accessKeyId` + `secretAccessKey` required; `sessionToken` optional.
  - `${ENV_VAR}` syntax pulls from the environment at load; environment values take precedence over
    inline values; missing referenced env var → load error.
  - Secrets are redacted everywhere (logs, error messages, any in-app display).
- **Active context precedence** (FR-002): `--context <name>` flag > `S3S_CONTEXT` env >
  `current-context`. The in-app switcher overrides at runtime without restart (SC-005).
- **TLS** (FR-004): `tlsSkipVerify` defaults false; true is an explicit per-cluster opt-in.
- **First run / empty config**: if the file is missing or has no contexts, s3s shows guidance on
  creating one instead of crashing (Edge Case).

## Validation surface (for tests)

- valid full config loads and resolves `current-context`
- dangling `cluster`/`user` reference rejected
- anonymous user with no keys accepted; non-anonymous user missing keys rejected
- `${ENV}` resolution + precedence over inline
- secret never appears in `String()`/log output (redaction test)
