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
- **Inline metadata & usage, cluster-safe** — the details pane shows rich object metadata
  in named groups (identity & content / security & governance / delivery), with relative
  + exact dates, a multipart-ETag explanation, and every field copyable in full. The
  bucket/prefix total size + object count comes from a background scan that is
  **budget-capped** (default 20 000 objects, `usageScanBudget` in the config; `0` turns
  ambient scanning off) so hovering a 100M-object bucket never hammers the cluster:
  bigger targets show an honest `≥` lower bound, and the **uncapped scan runs only on an
  explicit `A`** (`:scan`) — with progress, cancellable, and partial progress is cached,
  never thrown away. Press `a` ("more detail") to expand a ranked largest-first
  breakdown, an object's tags, or a bucket's configuration. Non-standard storage classes
  are marked in the listing (`i` reveals the full class).
- **Operator health card** (`H` / `:health`) — one full-screen answer to "what is this
  bucket made of": age and size histograms plus the storage-class spread (computed from
  the same scan pass — zero extra requests), **incomplete multipart uploads** (count,
  size, oldest age — the classic hidden cost), and a small-object index-pressure warning
  (`healthSmallObjectKiB` / `healthSmallObjectShare` knobs). Denied/unsupported probes
  say so explicitly — never rendered as a clean zero.
- **Copy & share menu** (`Y` / `:copy`) — copy the S3 URI, a style-aware HTTPS URL, a
  ready-to-run `aws s3api` download command, or a **presigned GET link** (15m/1h/24h/7d,
  minted entirely client-side, never logged — with a warning when your credentials expire
  before the link). Export the current usage/health report to CSV/JSON in your download
  dir. Clipboard is best-effort OSC52 (works over SSH); every value can also be shown
  full-screen for manual copy.
- **Sortable lists** — sort any level by name, size, or last-modified and toggle
  direction (`s` / `S`); the sort persists across navigation.
- **Two secure credential sources** — a context resolves its secret from exactly one of:
  the **OS keychain** (the default; macOS Keychain / Windows Credential Manager / Linux
  Secret Service) or an **external command** (`pass`, Vault, 1Password, sops…) — with a
  secure no-echo prompt fallback. The secret never lives on disk in plaintext. Manage
  keystore secrets with `s3s cred set|rotate|rm <context>`.
- **Tree navigation** — walk the key namespace by the `/` delimiter with
  on-demand pagination; never loads a whole bucket up front. Per-session cache
  with manual refresh.
- **Combined object view** — press `Enter` on an object to see its metadata and
  content side-by-side in one screen (no separate steps).
- **Payload-aware previews** — JSON/NDJSON pretty-printed (toggle raw with `p`),
  gzip transparently decompressed (bomb-safe, both sizes shown), binaries hex-dumped
  with offset + ASCII columns, scrollable text, and visual images (ANSI half-block,
  works in any 24-bit terminal) — all bounded to the first 5 MiB with a truncation
  notice.
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
then merges into any existing config. The credential source defaults to the **OS
keychain**: the wizard reads the secret with no echo and stores it in the keystore
(never on disk). Choose `cmd` instead to name an external command.

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
    keychain: true           # secret in the OS keystore (store via: s3s cred set local)
  - name: public
    anonymous: true          # public buckets, no signing
contexts:
  - name: local
    cluster: minio-local
    user: dev
current-context: local
# optional knobs (017):
# usageScanBudget: 20000        # ambient usage-scan cap in objects; 0 = explicit-only (A/:scan)
# healthSmallObjectKiB: 128     # health-card small-object threshold
# healthSmallObjectShare: 0.5   # warning fires above this share of small objects
```

```bash
chmod 600 ~/.config/s3s/config.yaml
s3s cred set local           # store the secret in the OS keystore (no echo)
```

Active-context precedence: `--context <name>` > `S3S_CONTEXT` env >
`current-context`.

**Multiple configs.** Point s3s at an alternate config file with `--config <path>` or
the `S3S_CONFIG` env var (precedence: `--config` > `S3S_CONFIG` > default
`~/.config/s3s/config.yaml`). It applies to the TUI, `s3s cred`, and `s3s config init`,
so you can keep separate work/personal or prod/staging configs. Keychain secrets are
isolated per config, so two configs that both define a `prod` context never share a
secret. An explicitly named missing config is an error (only the default path opens the
first-run add-connection form).

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

A non-anonymous user names **exactly one** secret source — `keychain` or `cmd` (more than
one, or neither, is a config error). The secret never lives on disk in plaintext.

```yaml
users:
  - name: prod                    # OS keychain (store via: s3s cred set prod)
    accessKeyId: AKIAPROD
    keychain: true
  - name: vault                   # external command — owner-only config required
    accessKeyId: AKIAVLT
    cmd: "vault kv get -field=secret s3/prod"
```

#### `keychain` (the default)

The same `keychain: true` field works on every desktop OS — s3s uses the platform's
native secret store:

| OS | Backed by |
|----|-----------|
| macOS | login Keychain |
| Windows | Credential Manager |
| Linux / BSD desktop | Secret Service over D-Bus (GNOME Keyring / KWallet) |

Manage the secret with `s3s cred set|rotate|rm <context>` (the OS keystore only — never the
config file). If the keystore has no entry, s3s prompts securely (no echo) at startup and
offers to save it.

> **Headless Linux** (no Secret Service / D-Bus): the keychain is unavailable, and s3s
> emits a clear error pointing you at a `cmd` source — it never falls back to a plaintext
> secret.

#### `cmd` (the escape hatch)

The command's **stdout** is the secret. It runs as argv (never a shell), the config must be
`chmod 600` (a `cmd:` source is *refused* on a group/world-writable config — a tampered file
must not run a command), and it is bounded by a 10s timeout. Ready recipes:

```bash
vault kv get -field=secret s3/prod                 # HashiCorp Vault
op read "op://Private/s3-prod/secret"               # 1Password CLI
pass show s3/prod                                   # pass
sops -d --extract '["secret"]' creds.yaml           # sops
secret-tool lookup service s3s account prod         # libsecret
security find-generic-password -w -s s3s -a prod    # macOS
```

Either source can be set up in three ways: `s3s config init`, by hand in the config, or the
**in-app add-connection form** (`c` → `+ add connection`) — its `source` row toggles
(`space`) between `keychain` and `cmd`, and the credential field below becomes either the
masked secret or the command line.

## Running

```bash
s3s                          # uses current-context (read-only by default)
s3s --context local          # explicit context
s3s --config ~/work.yaml     # use an alternate config (or set S3S_CONFIG)
s3s --write                  # START in write mode (toggle at runtime with `w`; readonly contexts stay protected)
s3s --version                # print version
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
| `a` | more detail: expand usage breakdown · object tags · bucket config (a read) |
| `A` | **full usage scan** (uncapped) of the focused bucket/folder — the only unbounded enumeration, always explicit |
| `Y` | **copy/share menu**: S3 URI · HTTPS URL · download command · presigned link · export CSV/JSON |
| `H` | **health card**: age/size/class histograms · incomplete multipart uploads · warnings |
| `p` | object view: toggle pretty ↔ raw for JSON/NDJSON previews |
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

Long operations (download, recursive delete, bulk ops, the usage scan) show a determinate progress
bar with a percentage inline in the footer; fast operations show none.

Images render as ANSI half-block by default. Terminal graphics protocols
(kitty/iTerm2) are available behind `S3S_IMAGE_PROTOCOL=kitty|iterm2|auto` but are
experimental — Bubble Tea's cell renderer doesn't reliably pass them through.

Logs: `$XDG_STATE_HOME/s3s/s3s.log` (or `~/.local/state/s3s/s3s.log`).

## Roadmap

Larger items on the horizon (full list in [ROADMAP.md](./ROADMAP.md)):

- Full-quality image preview via an external viewer.
- Syntax highlighting for text previews.
- Bucket administration (policy/lifecycle/encryption/CORS); object versioning
  management; incomplete-multipart-upload **cleanup** (surfacing them shipped in the
  health card — aborting them belongs to the write iteration).

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
- `internal/preview` — text/JSON/image/binary classification, pretty-print, gunzip,
  hexdump, and image rendering.
- `internal/share` — pure builders for copyable artifacts (URIs, URLs, command
  snippets) and CSV/JSON report export.
- `internal/logging` — file slog handler + a redacting `Secret` type.
- `internal/ui` — Bubble Tea (v2) model; depends only on the storage interface.
- `cmd/s3s` — wiring: load config → build storage → run the TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and
[aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2).

## License

[MIT](./LICENSE) © Daniil Chupin
