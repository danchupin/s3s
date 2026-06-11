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
millions of keys, inspect metadata, and preview files without leaving the
terminal. **Read-only by default — safe to point at production.** Writes are an
explicit, loudly-badged opt-in, and a per-context `readonly` flag keeps
protected environments untouchable.

</div>

## Why s3s

`s3cmd` and `mc` answer one question per invocation. s3s keeps you **in** the
storage — browsing, inspecting, and answering operator questions interactively,
without hammering the cluster to do it:

- 🛡️ **Cluster-safe by design.** Hovering a bucket shows its size and object
  count from a **budget-capped** background scan (default 20 000 objects) — an
  honest `≥` lower bound for anything bigger. The uncapped scan runs **only on
  an explicit keystroke** (`A`), streams progress, is cancellable, and partial
  progress is cached — never thrown away. A 100M-object production bucket is
  safe to browse.
- 🩺 **Operator health card** (`H`) — one screen answers "what is this bucket
  made of": age and size histograms, the storage-class spread (computed from
  the same scan pass — zero extra requests), **incomplete multipart uploads**
  (count, size, oldest age — the classic hidden cost), and a small-object
  index-pressure warning. Denied or unsupported probes say so explicitly —
  never rendered as a clean zero.
- 🔗 **Copy & share anything** (`Y`) — the S3 URI, a style-aware HTTPS URL, a
  ready-to-run `aws s3api` command, or a **presigned GET link** (15m/1h/24h/7d)
  minted entirely client-side and never logged. Export usage/health reports to
  CSV/JSON. Clipboard works over SSH (OSC52), and every value can be shown
  full-screen for manual copy.
- 🔍 **Payload-aware previews** — JSON/NDJSON pretty-printed (toggle raw with
  `p`), gzip transparently decompressed (bomb-safe), binaries hex-dumped,
  images rendered in-terminal — bounded to the first 5 MiB, no full downloads.
- 🗂️ **kubectl-style contexts** — clusters, users, and contexts in one YAML
  file; switch live (no restart) or jump by number (`1`–`9`). Secrets live in
  the **OS keychain** or come from an external command (`pass`, Vault,
  1Password…) — never on disk in plaintext.

## Features

- **Rich inline metadata** — the details pane shows object metadata in named
  groups (identity & content / security & governance / delivery): version,
  encryption + KMS key, replication, object-lock & legal-hold, lifecycle
  expiration — with relative + exact dates, a multipart-ETag explanation, and
  every field copyable in full. `a` expands a ranked largest-first usage
  breakdown, object tags, or the bucket configuration.
- **Tree navigation at any scale** — walk the key namespace by delimiter with
  on-demand pagination; never loads a whole bucket up front. Per-session cache,
  manual refresh (`r`).
- **Fast filter & search** — instant bucket-name filter; server-side prefix
  search within a level (debounced, complete results).
- **Sortable lists** — by name, size, or last-modified (`s`/`S`); persists
  across navigation.
- **Download & bulk operations** — pull objects to disk (a read — works in
  read-only mode); multi-select (`space`) for bulk download/delete/copy with
  truthful per-item summaries.
- **Opt-in writes with a safety model** — arm write at runtime (`w`, with
  confirmation); a loud `[RW]` badge shows on every screen while armed.
  Destructive actions need a **Ctrl chord** plus a confirmation that scales
  with blast radius (binary `y/N` → typed name for recursive/bucket deletes).
  A CI guard structurally confines mutations to the storage layer.
- **Non-blocking UI** — every backend call runs off the event loop; superseded
  loads are cancelled; long operations show progress and cancel cleanly.
- **Secrets never leak** — credentials are redacted everywhere; presigned URLs
  are never written to logs; logs go to a file only (the TUI owns the
  terminal).

## Installation

| Method | Command |
|--------|---------|
| Homebrew (macOS) | `brew install danchupin/tap/s3s` |
| Scoop (Windows) | `scoop bucket add danchupin https://github.com/danchupin/scoop-bucket && scoop install s3s` |
| Debian / Ubuntu | `sudo dpkg -i s3s_*_linux_amd64.deb` ([Releases](https://github.com/danchupin/s3s/releases/latest)) |
| RHEL / Fedora | `sudo rpm -i s3s_*_linux_amd64.rpm` |
| Alpine | `sudo apk add --allow-untrusted s3s_*_linux_amd64.apk` |
| Go | `go install github.com/danchupin/s3s/cmd/s3s@latest` |
| Binaries | grab a `.tar.gz`/`.zip` from the [Releases page](https://github.com/danchupin/s3s/releases/latest) |
| Source | `git clone https://github.com/danchupin/s3s && cd s3s && make build` (Go 1.25+) |

## Quick start

```bash
s3s config init     # interactive wizard: endpoint, addressing, credentials, context
s3s                 # browse (read-only by default)
```

No cluster handy? Spin up a local MinIO:

```bash
docker run -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin -e MINIO_ROOT_PASSWORD=password \
  minio/minio server /data --console-address ":9001"
```

```bash
s3s --context prod        # explicit context (> S3S_CONTEXT env > current-context)
s3s --config ~/work.yaml  # alternate config (or S3S_CONFIG env)
s3s --write               # start with write armed (readonly contexts stay protected)
```

## Configuration

Config lives at `~/.config/s3s/config.yaml` (XDG honored; override with
`--config`). Generate it with `s3s config init`, or write it by hand:

```yaml
apiVersion: s3s/v1
clusters:
  - name: minio-local
    endpoint: http://127.0.0.1:9000
    region: us-east-1
    pathStyle: true          # false => virtual-host/domain-style addressing
users:
  - name: dev
    accessKeyId: admin
    keychain: true           # secret in the OS keystore: s3s cred set local
  - name: public
    anonymous: true
contexts:
  - name: local
    cluster: minio-local
    user: dev
current-context: local

# optional tuning
usageScanBudget: 20000        # ambient usage-scan cap in objects; 0 = explicit-only
healthSmallObjectKiB: 128     # health-card small-object threshold
healthSmallObjectShare: 0.5   # warning fires above this share of small objects
```

```bash
chmod 600 ~/.config/s3s/config.yaml
s3s cred set local            # store the secret in the OS keystore (no echo)
```

A non-anonymous user names **exactly one** secret source: `keychain: true`
(macOS Keychain / Windows Credential Manager / Linux Secret Service; manage
with `s3s cred set|rotate|rm <context>`) or `cmd: "<command>"` whose stdout is
the secret (runs as argv, never a shell; requires an owner-only config; 10s
timeout). The secret never lives on disk in plaintext.

<details>
<summary><b>Scoped credentials (pinned buckets)</b> — keys that cannot <code>ListBuckets</code></summary>

Some credentials reach specific buckets but cannot list all buckets — common
with bucket-scoped RGW/MinIO keys and domain-style endpoints where only
`<bucket>.<host>` resolves. Pin the reachable buckets on the cluster:

```yaml
clusters:
  - name: scoped
    endpoint: https://bucket.example-rgw
    pathStyle: false
    buckets: [my-bucket, another-bucket]
```

s3s then skips `ListBuckets` and shows exactly those names; add more at runtime
via the `+ add bucket` row (persisted to the config).

</details>

<details>
<summary><b>External credential commands</b> — ready recipes</summary>

```bash
vault kv get -field=secret s3/prod                  # HashiCorp Vault
op read "op://Private/s3-prod/secret"               # 1Password CLI
pass show s3/prod                                   # pass
sops -d --extract '["secret"]' creds.yaml           # sops
secret-tool lookup service s3s account prod         # libsecret
security find-generic-password -w -s s3s -a prod    # macOS
```

</details>

<details>
<summary><b>Multiple configs</b> — work/personal, prod/staging</summary>

`--config <path>` or `S3S_CONFIG` applies to the TUI, `s3s cred`, and
`s3s config init`. Keychain secrets are namespaced per config, so two configs
that both define a `prod` context never share a secret.

</details>

### Plugins

External capability providers — executables you declare that supply data the
S3 protocol cannot: **bucket discovery** (e.g. a provisioning API listing the
buckets you were granted, when credentials can't `ListBuckets` or the endpoint
is domain-style-only) and **object metadata** (e.g. image-storage info keyed
by an id encoded in the object key, shown as a `From <plugin>` group in the
details pane). Strictly opt-in: no `plugins:` section, no plugin behavior.

```yaml
plugins:
  - name: corp-discovery
    capability: bucket-discovery
    cmd: "s3s-corp-discovery --cluster prod"   # shlex argv, never a shell
    timeout: 5s                                # optional, default 5s
    connections: [prod-rgw]
  - name: image-storage-meta
    capability: object-metadata
    cmd: "~/bin/image-storage-meta.sh"
    match:
      connections: [prod-rgw]
      buckets: ["images-*"]                    # glob; empty = any
      keyPattern: "^[0-9a-f]{32}"              # RE2; empty = any
```

A plugin reads one JSON request on stdin and writes one JSON response on
stdout (contract v1 — see [`docs/plugins/`](docs/plugins/) for the exchange
and two ready-to-copy stubs). Discovered names merge **additively** into the
bucket list (pinned ∪ listed ∪ discovered); failures never degrade browsing —
a transient notice points at the status surface (`P` / `:plugins`: per-plugin
outcome, enable/disable persisted to config, retry).

Security model: commands run as argv (never `sh -c`), only while the config
file is owner-only-writable (`chmod 600`) and owned by you; the request
carries identity context only (`accessKeyId` is the public identifier) — **the
secret key is never passed** in any field, env var, or argument; all
plugin-supplied text is sanitized before rendering; one log record per
invocation captures facts (plugin, capability, target, duration, outcome) and
never payloads or argv.

## Key bindings

Arrows are primary; vim aliases (`h/j/k/l`, `g/G`) work everywhere. The full
keymap lives in the help overlay (`?`) and the always-visible command bar.

| Key | Action |
|-----|--------|
| `↑`/`↓` · `→`/`Enter` · `←`/`Esc` | move · enter/open · back (also cancels an in-flight load) |
| `/` | filter buckets / search a level by prefix |
| `Enter` on an object | metadata + content side-by-side |
| `a` | more detail: usage breakdown · object tags · bucket config |
| `A` | **full usage scan** (uncapped — the only unbounded enumeration, always explicit) |
| `H` | **health card**: histograms · incomplete uploads · warnings |
| `Y` | **copy/share**: URI · URL · command · presigned link · export CSV/JSON |
| `p` | toggle pretty ↔ raw for JSON/NDJSON previews |
| `d` | download the selected object / marked set (a read) |
| `space` · `s`/`S` · `r` · `i` | multi-select · sort · refresh · reveal full identifier |
| `w` | arm/disarm write (confirm to arm; instant to disarm) |
| `P` | **plugin status** (shown only when plugins are declared): toggle · retry · error detail |
| `c` · `1`–`9` | connections manager · jump to context by number |
| `:` | command bar (`:scan`, `:health`, `:copy`, `:detail`, …) |
| `?` · `q` | help · quit |

Destructive actions (delete, move, recursive delete, bucket/connection delete)
are never on a bare key — they require a **Ctrl chord** (`Ctrl+x` / `Ctrl+o`)
plus a confirmation that scales with blast radius, up to typing the exact
target name. Bucket delete requires an empty bucket; the active connection
cannot be deleted.

Logs: `$XDG_STATE_HOME/s3s/s3s.log` (or `~/.local/state/s3s/s3s.log`).

## Roadmap

Planned work — syntax highlighting, bucket administration, versioning
management, incomplete-multipart cleanup, and more — lives in
**[ROADMAP.md](./ROADMAP.md)**.

## Development

```bash
make test               # unit tests (fake storage) — no Docker needed
make test-integration   # + real MinIO via testcontainers (needs Docker)
make fmt vet lint       # formatting, go vet, golangci-lint
make check-readonly     # structural read-only guard
```

- `internal/storage` — the `Storage` interface + aws-sdk-go-v2 impl (the only
  importer of `service/s3`) + an in-memory fake for unit tests.
- `internal/config` — kubectl-style YAML loader, validation, `config init`.
- `internal/cache` — per-session level cache (manual refresh only).
- `internal/preview` — text/JSON/image/binary classification, pretty-print,
  gunzip, hexdump, image rendering.
- `internal/share` — pure builders for copyable artifacts and report export.
- `internal/logging` — file slog handler + a redacting `Secret` type.
- `internal/ui` — Bubble Tea (v2) model; depends only on the storage interface.
- `cmd/s3s` — wiring: load config → build storage → run the TUI.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and
[aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2).

## License

[MIT](./LICENSE) © Daniil Chupin
