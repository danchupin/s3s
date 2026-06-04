# Quickstart: Read-Only S3 Browser (TUI)

**Feature**: 001-s3-readonly-browser

## Prerequisites

- Go 1.24+
- A reachable S3-compatible backend (Ceph RGW or MinIO). For local dev/tests, Docker (MinIO via
  testcontainers / `docker run`).

## Bootstrap (one-time)

```bash
go mod init github.com/dochupin/s3s   # adjust module path if different
go get github.com/aws/aws-sdk-go-v2 \
       github.com/aws/aws-sdk-go-v2/config \
       github.com/aws/aws-sdk-go-v2/credentials \
       github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/charmbracelet/bubbletea/v2 \
       github.com/charmbracelet/bubbles/v2 \
       github.com/charmbracelet/lipgloss/v2
go get github.com/eliukblau/pixterm \
       github.com/blacktop/go-termimg
go get go.yaml.in/yaml/v3
go get github.com/testcontainers/testcontainers-go/modules/minio   # test-only
go mod tidy
```

## Local MinIO for trying it out

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin -e MINIO_ROOT_PASSWORD=password \
  minio/minio server /data --console-address ":9001"
```

## Config

`~/.config/s3s/config.yaml` (see `contracts/config-schema.md`):

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
  - name: local
    cluster: minio-local
    user: dev
current-context: local
```

```bash
chmod 600 ~/.config/s3s/config.yaml
export S3S_DEV_SECRET=password
```

## Run

```bash
go run ./cmd/s3s                 # uses current-context
go run ./cmd/s3s --context local # explicit context
```

Navigate: `↑/↓` move, `→/Enter` drill in, `←/Esc` back, `/` search, `i` metadata, `p` preview,
`r` refresh, `c` switch context, `?` help, `Ctrl+C` quit. (Full map: `contracts/tui-contract.md`.)

## Tests

```bash
go test ./...                          # unit tests (fake storage) — no Docker needed
go test -tags=integration ./...        # + real MinIO via testcontainers (needs Docker)
```

Integration tests `t.Skip` automatically when the Docker provider is unreachable.

## Validate against the spec

- US1: launch → buckets visible (SC-001).
- US2: drill into nested prefixes, scroll triggers next page only on demand (SC-002, SC-003).
- US3: `i` shows size/type/last-modified/etc. (FR-013).
- US4: `/` narrows the level server-side, complete results (FR-017).
- US5: `p` previews text and images; >5 MiB shows truncated; non-capable terminal falls back
  (FR-014/015/016).
- Read-only: capture backend access logs during a full session → zero write requests (SC-009).
