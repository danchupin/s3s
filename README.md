# s3s — read-only S3 browser (TUI)

An interactive, keyboard-driven terminal UI to browse S3-compatible object storage
(Ceph RGW, MinIO) **read-only**: pick a cluster via a kubectl-style context, list
buckets, walk the key namespace as a tree by the `/` delimiter with on-demand
paging, inspect object metadata, preview content (text + images), and narrow a
level with a server-side prefix search.

Read-only is structural, not just policy: the storage interface exposes no
mutating method, and a CI guard (`scripts/check-readonly.sh`) fails the build if
any write-capable S3 symbol appears outside `internal/storage`.

## Install / build

```bash
go build -o bin/s3s ./cmd/s3s
```

Requires Go 1.25+. For local testing you also need Docker (MinIO via testcontainers).

## Configuration

### Generate a config interactively

```bash
s3s config init                       # writes to the default XDG path
s3s config init --config ./my.yaml    # custom path
```

The wizard prompts for the cluster endpoint, addressing/TLS, credentials, and
context name, then merges the result into the existing config (existing entries
are preserved). The secret is stored as a `${ENV}` reference — never written to
disk — and the wizard prints the `export` line to set it.

### Manual config

Config lives at `$XDG_CONFIG_HOME/s3s/config.yaml` (default `~/.config/s3s/config.yaml`).
Override with `--config <path>`.

```yaml
apiVersion: s3s/v1
clusters:
  - name: minio-local
    endpoint: http://127.0.0.1:9000
    region: us-east-1
    pathStyle: true          # path-style; false => virtual-host/domain style
    tlsSkipVerify: false     # explicit opt-in, https only
users:
  - name: dev
    accessKeyId: admin
    secretAccessKey: ${S3S_DEV_SECRET}   # ${ENV} resolved at load; never logged
  - name: public
    anonymous: true          # public buckets, no signing
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

Active-context precedence: `--context <name>` > `S3S_CONTEXT` env > `current-context`.

## Run

```bash
go run ./cmd/s3s                  # uses current-context
go run ./cmd/s3s --context local  # explicit context
```

### Local MinIO to try it

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin -e MINIO_ROOT_PASSWORD=password \
  minio/minio server /data --console-address ":9001"
```

## Keybindings

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | move selection |
| `→`/`l`/`Enter` | enter bucket/dir, or open an object (metadata + content together) |
| `←`/`h`/`Esc` | back to parent (or clear active search) |
| `g` / `G` | jump to top / bottom |
| `/` | filter buckets / search a level (prefix, ~300 ms debounce); `Esc` clears |
| `i` / `p` | open the object view (same as Enter on an object) |
| `r` | refresh current level (discard cache) |
| `c` | switch context |
| `x` | cancel in-flight load |
| `?` | help overlay |
| `q` / `Ctrl+C` | quit |

Images preview using a terminal graphics protocol (kitty / iTerm2) when one is
detected from the environment — this gives a crisp, full-resolution image. On
terminals without a supported protocol (incl. sixel) it falls back to ANSI
half-block, which is inherently low-resolution (two pixels per character cell).
Force half-block with `S3S_IMAGE_PROTOCOL=off` if the protocol path causes
artifacts. Previews are bounded to the first 5 MiB; larger objects show a
truncation notice.

### Filtering & quick context switch

In the bucket list press `/` to filter buckets by name (live, case-insensitive);
`Esc` clears. Press a digit `1`–`9` to jump straight to that context (shown in the
header's numbered list).

Logs are written to a file (`$XDG_STATE_HOME/s3s/s3s.log` or
`~/.local/state/s3s/s3s.log`) — never the terminal. Secrets are always redacted.

## Tests

```bash
make test               # unit tests (fake storage) — no Docker needed
make test-integration   # + real MinIO via testcontainers (needs Docker)
make fmt vet lint       # formatting, vet, golangci-lint
make check-readonly     # structural read-only guard
```

Integration tests `t.Skip` automatically when the Docker provider is unreachable.

## Architecture

- `internal/storage` — read-only `Storage` interface + aws-sdk-go-v2 impl (the
  ONLY importer of `service/s3`) + in-memory fake for unit tests.
- `internal/config` — kubectl-style YAML loader, `${ENV}` resolution, validation.
- `internal/cache` — per-session, TTL-free level cache (manual refresh only).
- `internal/preview` — text/image/binary classification, half-block image render.
- `internal/logging` — file slog handler + redacting `Secret` type.
- `internal/ui` — Bubble Tea (v2) model; depends only on the storage interface.
  Every backend call runs in a `tea.Cmd`; superseded loads are cancelled and
  their stale results dropped via a generation id.
- `cmd/s3s` — wiring: load config → build storage → run the TUI.
