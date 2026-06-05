<div align="center">

# s3s

**A fast, keyboard-driven terminal browser for S3-compatible object storage —
think [`k9s`](https://github.com/derailed/k9s), but for your buckets.**

[![Release](https://img.shields.io/github/v/release/danchupin/s3s?sort=semver)](https://github.com/danchupin/s3s/releases/latest)
[![CI](https://github.com/danchupin/s3s/actions/workflows/ci.yml/badge.svg)](https://github.com/danchupin/s3s/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/danchupin/s3s)](https://goreportcard.com/report/github.com/danchupin/s3s)
[![Go version](https://img.shields.io/github/go-mod/go-version/danchupin/s3s)](./go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

Point it at Ceph RGW or MinIO, switch clusters like kubectl contexts, and walk
millions of keys, inspect metadata, and preview files without ever leaving the
terminal. **Read-only by default** — safe to point at production — with opt-in
writes (`--write`) for the small, growing set of mutating operations, and a
per-context `readonly` flag that keeps protected environments untouchable.

</div>

> ⚠️ **Alpha.** Usable day-to-day but rough edges remain; flags, config, and the
> UI may change. Feedback and issues welcome.

The interface borrows from two tools we love: **k9s** (bordered resource tables,
fast navigation, context switching) and **Claude Code** (warm color palette, a
compact multi-line status footer).

<!-- TODO: drop a screenshot or asciinema GIF here, e.g. assets/demo.gif -->

## Screenshots

<!--
Add a screenshot/GIF to make the README pop, then reference it here:

![s3s browsing a bucket](assets/screenshot.png)
-->

_Coming soon — a recording of context switching, tree navigation, and previews._

## Features

- **kubectl-style contexts** — define clusters, users, and contexts in one YAML
  file; switch between them live (no restart) or jump by number (`1`–`9`).
- **Read-only by default, safe writes opt-in** — without `--write` nothing can be
  mutated. With it, the write operations (create folder, delete object, upload a
  local file, copy, move/rename, recursive folder delete) run behind a confirmation
  gate — a simple `y/N` for reversible actions and a typed confirmation of the exact
  target for destructive ones (delete, move, overwrite, recursive delete). A context
  marked `readonly: true` refuses changes even under `--write`. All S3 mutations are
  confined to the storage layer by a CI guard.
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

## Installation

### Homebrew (macOS)

```bash
brew install danchupin/tap/s3s
```

### Scoop (Windows)

```powershell
scoop bucket add danchupin https://github.com/danchupin/scoop-bucket
scoop install s3s
```

### Debian / Ubuntu (`.deb`)

Grab the latest `.deb` from the [Releases page](https://github.com/danchupin/s3s/releases/latest), then:

```bash
sudo dpkg -i s3s_*_linux_amd64.deb
```

### RHEL / Fedora (`.rpm`)

```bash
sudo rpm -i s3s_*_linux_amd64.rpm
```

### Alpine (`.apk`)

```bash
sudo apk add --allow-untrusted s3s_*_linux_amd64.apk
```

### Go

```bash
go install github.com/danchupin/s3s/cmd/s3s@latest
```

### Prebuilt binaries

Download a `.tar.gz` / `.zip` for your OS/arch from the
[Releases page](https://github.com/danchupin/s3s/releases/latest), extract, and
put `s3s` on your `PATH`.

### Building From Source

Requires Go 1.25+.

```bash
git clone https://github.com/danchupin/s3s
cd s3s
make build   # -> bin/s3s
```

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

## Running

```bash
s3s                  # uses current-context (read-only)
s3s --context local  # explicit context
s3s --write          # enable mutating operations (readonly contexts stay protected)
s3s --version        # print version
```

### A local MinIO to try it

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin -e MINIO_ROOT_PASSWORD=password \
  minio/minio server /data --console-address ":9001"
```

## Key Bindings

Arrow keys are the primary, advertised navigation; the vim aliases (`h`/`j`/`k`/`l`,
`g`/`G`) still work and are listed in the help overlay (`?`). The write operations and
refresh live behind a single contextual **action menu** opened with `a` — the footer
advertises just `a actions`, not a wall of per-op keys.

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | move selection |
| `→`/`l`/`Enter` | enter bucket/dir, or open an object (metadata + content) |
| `←`/`h`/`Esc` | back to parent (or clear an active filter/search); cancels an in-flight load |
| `g`/`Home`, `G`/`End` | jump to top / bottom |
| `/` | filter buckets / search a level by prefix; `Esc` clears |
| `a` | **action menu** — contextual operations for the selection (see below) |
| `c` | switch context · `1`–`9` jump to a context by number |
| `?` | help (full keymap, incl. vim aliases, + connection details) · `q` / `Ctrl+C` quit |

The action menu (`a`) lists only what applies to the current selection and context:

| Menu item | Notes |
|-----------|-------|
| refresh | reload the current list (always available, incl. the bucket list) |
| new folder | write mode |
| delete | object selected — write mode; typed confirm |
| upload here | write mode; opens a local file browser |
| copy | object selected — write mode |
| move / rename | object selected — write mode; typed confirm |
| recursive delete | folder selected — write mode; typed confirm |

In a read-only context the menu offers only `refresh`. The footer stays at most three
rows — a compact identity line (`● context [RW|RO] · cluster`), one contextual hint row
(capped at six, with a `? more` cue when narrow), and a status line.

Images render as ANSI half-block by default. Terminal graphics protocols
(kitty/iTerm2) are available behind `S3S_IMAGE_PROTOCOL=kitty|iterm2|auto` but are
experimental — Bubble Tea's cell renderer doesn't reliably pass them through.

Logs: `$XDG_STATE_HOME/s3s/s3s.log` (or `~/.local/state/s3s/s3s.log`).

## Roadmap

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

## License

[MIT](./LICENSE) © Daniil Chupin
