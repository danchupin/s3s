# s3s

**A fast, keyboard-driven terminal browser for S3-compatible object storage —
think `k9s`, but for your buckets.** Point it at Ceph RGW or MinIO, switch
clusters like kubectl contexts, and walk millions of keys, inspect metadata, and
preview files without ever leaving the terminal — fully read-only, so you can
explore production safely.

> ⚠️ **Alpha.** Usable day-to-day but rough edges remain; flags, config, and the
> UI may change. Feedback and issues welcome.

The interface borrows from two tools we love: **k9s** (bordered resource tables,
fast navigation, context switching) and **Claude Code** (warm color palette, a
compact multi-line status footer).

## Features

- **kubectl-style contexts** — define clusters, users, and contexts in one YAML
  file; switch between them live (no restart) or jump by number (`1`–`9`).
- **Read-only by construction** — the storage interface exposes no mutating
  method, and a CI guard fails the build if any write-capable S3 symbol appears
  outside the storage layer. Safe to point at prod.
- **Tree navigation** — walk the key namespace by the `/` delimiter with
  on-demand pagination; never loads a whole bucket up front. Per-session cache
  with manual refresh.
- **Combined object view** — press `Enter` on an object to see its metadata and
  content side-by-side in one screen (no separate steps).
- **Inline previews** — scrollable text and visual images (ANSI half-block,
  works in any 24-bit terminal), bounded to the first 5 MiB with a truncation
  notice; safe summary for binaries.
- **Fast filter & search** — filter buckets by name instantly; server-side
  prefix search within a level (debounced, complete results — not just what's
  loaded).
- **Non-blocking UI** — every backend call runs off the event loop; superseded
  loads are cancelled, in-flight loads show a spinner and can be cancelled.
- **Secrets never leak** — credentials are redacted everywhere; logs go to a file
  only (the TUI owns the terminal). `${ENV}` references keep keys out of config.

## Install

```bash
go build -ldflags "-X main.version=$(git describe --tags --always)" -o bin/s3s ./cmd/s3s
```

Requires Go 1.25+. For the integration tests you also need Docker (MinIO via
testcontainers).

## Configuration

Config lives at `$XDG_CONFIG_HOME/s3s/config.yaml` (default
`~/.config/s3s/config.yaml`); override with `--config <path>`.

### Generate it interactively

```bash
s3s config init                       # write to the default XDG path
s3s config init --config ./my.yaml    # custom path
```

The wizard asks for the endpoint, addressing/TLS, credentials, and context name,
then merges into any existing config. The secret is stored as a `${ENV}`
reference (never written to disk) and the wizard prints the `export` line to set
it.

### Or write it by hand

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

Active-context precedence: `--context <name>` > `S3S_CONTEXT` env >
`current-context`.

## Run

```bash
s3s                  # uses current-context
s3s --context local  # explicit context
```

### A local MinIO to try it

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin -e MINIO_ROOT_PASSWORD=password \
  minio/minio server /data --console-address ":9001"
```

## Keybindings

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | move selection |
| `→`/`l`/`Enter` | enter bucket/dir, or open an object (metadata + content) |
| `←`/`h`/`Esc` | back to parent (or clear an active filter/search) |
| `g` / `G` | jump to top / bottom |
| `/` | filter buckets / search a level by prefix; `Esc` clears |
| `r` | refresh current level (discard cache) |
| `c` | switch context · `1`–`9` jump to a context by number |
| `x` | cancel in-flight load |
| `?` | help · `q` / `Ctrl+C` quit |

Images render as ANSI half-block by default. Terminal graphics protocols
(kitty/iTerm2) are available behind `S3S_IMAGE_PROTOCOL=kitty|iterm2|auto` but are
experimental — Bubble Tea's cell renderer doesn't reliably pass them through.

Logs: `$XDG_STATE_HOME/s3s/s3s.log` (or `~/.local/state/s3s/s3s.log`).

## Roadmap (selected)

Larger items on the horizon (full list in [ROADMAP.md](./ROADMAP.md)):

- Full-quality image preview via an external viewer.
- Sortable lists (by name / size / last-modified).
- Richer previews — syntax highlighting and a hex view for binaries.
- Copy key / S3 URI / ETag to the clipboard.

## Development

```bash
make test               # unit tests (fake storage) — no Docker needed
make test-integration   # + real MinIO via testcontainers (needs Docker)
make fmt vet lint       # formatting, go vet, golangci-lint
make check-readonly     # structural read-only guard
```

Integration tests `t.Skip` automatically when Docker is unreachable.

### Architecture

- `internal/storage` — read-only `Storage` interface + aws-sdk-go-v2 impl (the
  only importer of `service/s3`) + an in-memory fake for unit tests.
- `internal/config` — kubectl-style YAML loader, `${ENV}` resolution, validation,
  and the `config init` wizard.
- `internal/cache` — per-session, TTL-free level cache (manual refresh only).
- `internal/preview` — text/image/binary classification and image rendering.
- `internal/logging` — file slog handler + a redacting `Secret` type.
- `internal/ui` — Bubble Tea (v2) model; depends only on the storage interface.
- `cmd/s3s` — wiring: load config → build storage → run the TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and
[aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2).
