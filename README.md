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
- **Read-only by default, runtime write toggle** — s3s starts read-only. Arm write at
  runtime with a hotkey (`w`) — arming takes a deliberate confirmation, disarming is
  instant — and while armed a **loud, high-contrast `[RW]` badge** shows on every
  screen so you can never mutate production thinking you were safe. `--write` just
  starts armed; a context marked `readonly: true` can never be armed. Mutations
  (create folder, delete, upload, copy, move/rename, recursive delete, bulk
  delete/copy) keep the two-tier confirmation (simple `y/N` vs typed target/count) and
  are confined to the storage layer by a CI guard.
- **Download & bulk operations** — pull a full object to local disk (a *read* — works
  read-only, against production); multi-select objects (`space`) and act on the batch:
  bulk download (mirrors the key hierarchy into local subdirs), bulk delete, bulk copy,
  each with a truthful per-item succeeded/failed summary.
- **Storage analytics (`du`)** — analyze a bucket or prefix and see the total size,
  object count, and a ranked largest-first breakdown of its immediate children (an
  `ncdu`-style view) with live progress and drill-down — what's eating space, in-TUI.
- **Sortable lists** — sort any level by name, size, or last-modified and toggle
  direction (`s` / `S`); the sort persists across navigation.
- **Secure credential sources** — a context resolves its secret from exactly one of:
  the OS keychain, an external command (`pass`, Vault, 1Password, sops…), an AWS shared
  profile, or the classic `${ENV}` reference — with a secure no-echo prompt fallback.
  Stop exporting a secret into every shell; the secret never lives on disk in plaintext.
  Manage keystore secrets with `s3s cred set|rotate|rm <context>`.
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
    buckets:                 # optional: pin specific buckets (see below)
      - my-bucket
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

### Scoped credentials (pinned buckets)

Some credentials can access specific buckets but cannot **list all buckets**
(`s3:ListAllMyBuckets`) — common with bucket-scoped Ceph RGW / MinIO keys, and with
domain-style endpoints where only `<bucket>.<host>` resolves. s3s normally opens at the bucket
list (a `ListBuckets` call), which fails for such credentials.

Pin the buckets you can reach with a `buckets:` list on the cluster:

```yaml
clusters:
  - name: scoped
    endpoint: https://bucket.example-rgw   # domain/virtual-hosted style → pathStyle: false
    pathStyle: false
    buckets:
      - my-bucket
      - another-bucket
```

When `buckets:` is set, s3s skips `ListBuckets` and shows exactly those names; open and switch
between them normally. You can also add buckets at runtime: on a scoped bucket list, choose the
`+ add bucket` row, type a name, and it is pinned to the connection (persisted to the config).
The in-app add-connection form has a `buckets` field for the same purpose. Connections that can
list buckets normally are unaffected (no `+ add bucket` row, no behavior change).

### Credential sources

A non-anonymous user names **exactly one** secret source (more than one is a config
error). The secret never lives on disk in plaintext and need not be exported into every
shell:

```yaml
users:
  - name: prod                    # OS keychain (store via: s3s cred set prod)
    accessKeyId: AKIAPROD
    keychain: true
  - name: vault                   # external command — owner-only config required
    accessKeyId: AKIAVLT
    cmd: "vault kv get -field=secret s3/prod"
  - name: aws                     # ~/.aws/credentials profile (static keys)
    awsProfile: prod
  - name: ci                      # classic ${ENV} — still works for automation
    accessKeyId: AKIACI
    secretAccessKey: ${S3S_CI_SECRET}
```

`s3s cred set|rotate|rm <context>` manages a context's secret in the OS keystore only.
If no source resolves, s3s prompts securely (no echo) at startup and offers to save to
the keystore. A group/world-readable config triggers a warning; a `cmd:` source is
*refused* on a group/world-writable config (it would let a tampered file run a command).

## Running

```bash
s3s                  # uses current-context (read-only by default)
s3s --context local  # explicit context
s3s --write          # START in write mode (toggle at runtime with `w`; readonly contexts stay protected)
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
`g`/`G`) still work and are listed in the help overlay (`?`). There is no action menu:
actions are direct single keys, grouped in an always-visible **command bar** at the bottom
split into three blocks — **info · read · write**. The write block stays visible even in a
read-only context (dimmed, `(w to arm)`) so the full capability set is always legible.

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | move selection |
| `→`/`l`/`Enter` | enter bucket/dir, or open an object (metadata + content) |
| `←`/`h`/`Esc` | back to parent (or clear an active filter/search); cancels an in-flight load |
| `g`/`Home`, `G`/`End` | jump to top / bottom |
| `/` | filter buckets / search a level by prefix; `Esc` clears |
| `space` | mark/unmark an object for multi-select (bulk variants act on the marked set) |
| `s` / `S` | cycle the sort column (name/size/modified) · toggle direction |
| `d` | download the selected object / marked set (a read — works read-only) |
| `a` | analyze (`du`) a bucket / folder / level (a read) |
| `r` | refresh the current list |
| `y` · `u` · `+` | copy · upload · new folder (write mode; safe — bare key) |
| `w` | **arm/disarm write** at runtime (confirm to arm; instant to disarm) |
| `c` | **connections** — switch context · add (via the "+ add connection" row) · delete · `1`–`9` jump to a context by number |
| `Tab` · `→` | cross focus into the objects pane (then `←`/`Esc` / `Tab` back) — two-pane browse |
| `?` | help (full keymap, incl. vim aliases, + connection details) · `q` / `Ctrl+C` quit |

### Dangerous actions (Ctrl chord + confirmation)

Destructive actions are **not** triggered by a bare key — they require a **Ctrl chord** so a
stray keystroke can never destroy data, and the confirmation strength scales with blast
radius:

| Chord | Action | Confirmation |
|-------|--------|--------------|
| `Ctrl+x` (object / marked set) | delete object(s) | binary `y/N` in a centered popup |
| `Ctrl+o` | move / rename | binary `y/N` in a centered popup |
| (copy/upload onto an existing key) | overwrite | binary `y/N` in a centered popup |
| `Ctrl+x` (folder) | recursive delete | type the exact **path** in a prominent inline form |
| `Ctrl+x` (bucket list) | delete bucket (empty-only) | type the exact **bucket name** |
| `Ctrl+x` (contexts screen) | delete connection | type the exact **connection name** |

Bucket delete requires an **empty** bucket — it never recursively purges. Deleting a
connection also removes its keychain secret; the **active** context cannot be deleted.

Long operations (download, recursive delete, bulk ops, `du`) show a determinate progress
bar with a percentage inline in the footer; fast operations show none.

Images render as ANSI half-block by default. Terminal graphics protocols
(kitty/iTerm2) are available behind `S3S_IMAGE_PROTOCOL=kitty|iterm2|auto` but are
experimental — Bubble Tea's cell renderer doesn't reliably pass them through.

Logs: `$XDG_STATE_HOME/s3s/s3s.log` (or `~/.local/state/s3s/s3s.log`).

## Roadmap

Larger items on the horizon (full list in [ROADMAP.md](./ROADMAP.md)):

- Full-quality image preview via an external viewer.
- Richer previews — syntax highlighting and a hex view for binaries.
- Copy key / S3 URI / ETag to the clipboard.
- Presigned URLs; bucket administration (policy/lifecycle/encryption/CORS);
  object versioning management; incomplete-multipart-upload cleanup.

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
