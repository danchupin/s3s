# Quickstart: Write Foundation & Safety (002)

How to exercise the first mutating operation once 002 is implemented. Builds on the
001 quickstart (config + a running MinIO).

## 1. A local MinIO

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin -e MINIO_ROOT_PASSWORD=password \
  minio/minio server /data --console-address ":9001"
```

## 2. Config with a writable and a protected context

`~/.config/s3s/config.yaml`:

```yaml
apiVersion: s3s/v1
clusters:
  - name: minio-local
    endpoint: http://127.0.0.1:9000
    region: us-east-1
    pathStyle: true
users:
  - name: dev
    accessKeyId: admin
    secretAccessKey: ${S3S_DEV_SECRET}
contexts:
  - name: local            # writable when --write is passed
    cluster: minio-local
    user: dev
  - name: local-ro         # protected: refuses mutations even with --write
    cluster: minio-local
    user: dev
    readonly: true
current-context: local
```

```bash
export S3S_DEV_SECRET=password
```

## 3. Default is still read-only

```bash
s3s                       # no --write → create-folder key shows a read-only hint
```

Attempting create-folder surfaces "context is read-only — start with --write" and
issues no request.

## 4. Enable writes and create a folder

```bash
s3s --write               # writable contexts now allow mutations
```

1. Enter a bucket.
2. Press the create-folder key, type `reports`, press `Enter`.
3. A **simple confirmation** appears (create-folder is reversible) → press `y`.
4. A spinner shows immediately (≤100 ms); the UI stays responsive.
5. On success the level refreshes and `reports/` appears as a folder.

Check the log:

```bash
tail -f "${XDG_STATE_HOME:-$HOME/.local/state}/s3s/s3s.log"
# mutation.start  action=create_folder bucket=... key=reports/ context=local
# mutation.done   action=create_folder ... outcome=ok
```

No secret appears in any line.

## 5. Verify the protection

```bash
s3s --write --context local-ro
```

Create-folder is still refused (read-only context wins over `--write`).

## 6. Run the tests

```bash
make test                 # unit: guard refusal, policy truth table, confirm tiers, create-folder (fake)
make test-integration     # real MinIO: create-folder visible after refresh; guard refuses without network
make check-readonly       # unchanged guard still passes (SDK mutations only in internal/storage)
```
