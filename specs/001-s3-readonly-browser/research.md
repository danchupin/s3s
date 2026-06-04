# Phase 0 Research: Read-Only S3 Browser (TUI)

**Feature**: 001-s3-readonly-browser | **Date**: 2026-06-04

All library facts verified against current (mid-2026) sources. Two notable surprises: Bubble Tea
**v2 is now stable**, and `gopkg.in/yaml.v3` is **archived** with a maintained drop-in successor.

---

## 1. S3 SDK

- **Decision**: `github.com/aws/aws-sdk-go-v2` (`config`, `credentials`, `service/s3`).
- **Rationale**: First-class config knobs for every requirement, portable across Ceph RGW and
  MinIO (both speak the AWS S3 API). Explicit endpoint + path-style toggle and per-request `Range`
  on `GetObject` are native; credential primitives map directly onto the
  static/session-token/anonymous matrix.
- **Concrete APIs**:
  - Custom endpoint + addressing (per-client, the deprecated global resolver is avoided):
    ```go
    client := s3.NewFromConfig(cfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String("https://rgw.example.com:8080")
        o.UsePathStyle = true // true=path-style host/bucket/key; false=virtual-host bucket.host/key
    })
    ```
  - Anonymous (public buckets): `cfg.Credentials = aws.AnonymousCredentials{}`.
  - Static + optional session token:
    `credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken)` (3rd arg `""`
    when none).
  - Listing: `s3.ListObjectsV2Input{Bucket, Prefix, Delimiter: aws.String("/"), ContinuationToken}`;
    paginate with `s3.NewListObjectsV2Paginator`. `CommonPrefixes` → tree "directories".
  - Ranged preview: `s3.GetObjectInput{Bucket, Key, Range: aws.String("bytes=0-5242879")}`.
  - Metadata: `HeadObject` → `ContentLength`, `ContentType`, `LastModified`, `ETag`,
    `StorageClass`, `Metadata` (user metadata).
- **Alternatives considered**: `github.com/minio/minio-go/v7` — capable equivalent (path vs DNS via
  `BucketLookup`, `opts.SetRange`, `ListObjects` non-recursive emits common prefixes). Leaner deps,
  valid second choice; AWS SDK chosen for the explicit endpoint/credential surface and to track the
  canonical S3 contract.

## 2. Bubble Tea ecosystem

- **Decision**: `github.com/charmbracelet/bubbletea/v2` + `bubbles/v2` + `lipgloss/v2`.
- **Status**: Bubble Tea **v2 stable** (≥ v2.0.x, 2026). Elm architecture unchanged
  (`Model`/`Update`/`View`, `tea.Cmd func() tea.Msg`). v2 ships the new "Cursed" renderer; Lip Gloss
  is "pure" (Bubble Tea owns I/O). Pin all three Charm libs to v2 (mixing v1/v2 paths breaks).
- **Components used**: `list`, `viewport` (text/metadata panes), `textinput` (search), `spinner`
  (loading), `key` (keymaps); `lipgloss` for layout/styling.
- **Async pattern** (core of Constitution II): wrap each S3 call in a `tea.Cmd`:
  ```go
  func loadLevel(ctx context.Context, st storage.Storage, q storage.LevelQuery, gen int) tea.Cmd {
      return func() tea.Msg {
          page, err := st.ListLevel(ctx, q)
          if err != nil { return levelErrMsg{gen, err} }
          return levelLoadedMsg{gen, page}
      }
  }
  ```
  Return the cmd from `Update`; runtime runs it on a goroutine, feeds the `tea.Msg` back. Use
  `tea.Batch` for parallel loads (Head + ranged Get).
- **Cancellation**: store a `context.CancelFunc` per active load in the model; on navigation/new
  search, cancel the old, create a fresh `context.WithCancel`, and tag each load with a
  generation/sequence ID so stale messages are dropped. `tea.WithContext` is program-level
  shutdown, separate from per-load cancellation — use both.

## 3. Image preview in the terminal

- **Decision**: default to ANSI **half-block** (`▀`) 24-bit rendering; opt into a graphics protocol
  (kitty > iTerm2 > sixel) when detected.
- **Default (works everywhere)**: half-block encodes two vertical pixels per cell via
  foreground+background truecolor — it *is* cells, so it composes cleanly with Bubble Tea's
  cell-based renderer. Lib: `github.com/eliukblau/pixterm/pkg/ansimage` (truecolor lower-half-block,
  PNG/JPEG/GIF/etc.). Alt: `github.com/qeesung/image2ascii`.
- **Enhanced (optional)**: `github.com/blacktop/go-termimg` — kitty + iTerm2 + sixel with automatic
  protocol detection and fallback (kitty/iTerm2 ~2.5ms, sixel ~90ms). Alt: `BourgeoisBear/rasterm`.
- **Detection (env)**: kitty → `KITTY_WINDOW_ID` or `TERM` contains `kitty` (also Ghostty/WezTerm);
  iTerm2 → `TERM_PROGRAM=iTerm.app` / `LC_TERMINAL=iTerm2`; sixel → `TERM` matches `*sixel*` or DA1
  query. Fall back to half-block if none.
- **Bubble Tea friction (flagged)**: protocol escape blobs occupy pixel regions the cell renderer
  doesn't model → artifacts on scroll/resize. Mitigation: half-block is the safe default; for
  protocol images, redraw into a dedicated region on every relevant `tea.Msg` and manage placement
  manually. **v1 ships half-block; protocol path is a clearly-isolated optional enhancement.**

## 4. Config file (YAML)

- **Decision**: `go.yaml.in/yaml/v3` — officially maintained, byte-for-byte drop-in successor to
  the now-**archived** `gopkg.in/yaml.v3` (same API, security fixes). One-line import swap.
- **Alternatives**: `sigs.k8s.io/yaml` (YAML→JSON, struct `json` tags; its `goyaml.v3` subpackage is
  deprecated in favor of `go.yaml.in/yaml/v3`); `github.com/goccy/go-yaml` (richer parsing/errors,
  heavier API).
- **Kubeconfig-style modeling**: no special lib — plain structs. `clusters` (endpoint, region,
  path-style), `users` (access key / secret / optional session token / `anonymous: true`),
  `contexts` (bind cluster+user), `current-context`. Resolve at startup → build §1 client config.

## 5. Integration testing (real backend)

- **Decision**: `github.com/testcontainers/testcontainers-go/modules/minio` — spin a real MinIO
  container per test run.
- **Rationale**: Constitution IV mandates a real backend. Testcontainers gives programmatic
  lifecycle, random ports, dynamic endpoint injection — better than static docker-compose, lighter
  than LocalStack. MinIO is one of the two target backends → most representative.
- **Usage**: `minio.Run(ctx, "minio/minio:RELEASE.<pinned>", minio.WithUsername(...),
  minio.WithPassword(...))`; endpoint via `ConnectionString`/`Endpoint`, fed as `BaseEndpoint` +
  `UsePathStyle=true` + static creds. Gate: build tag `//go:build integration` and/or `t.Skip` when
  the Docker provider is unreachable, so `go test ./...` stays green without Docker.
- **Unit mocking (complementary)**: narrow storage interface in core; real impl wraps the SDK;
  units use a hand-written fake.

## 6. Other decisions

- **Module layout**: `cmd/s3s` (wiring), `internal/storage` (interface + sole SDK importer),
  `internal/config`, `internal/preview`, `internal/cache`, `internal/ui`. UI depends on the storage
  interface only (Constitution I).
- **Logging**: stdlib `log/slog` JSON handler → file. **Never** stdout/stderr (Bubble Tea owns the
  terminal). Optionally pair with `tea.LogToFile`.
- **Read-only guarantee (structural, not just discipline)**:
  1. Storage interface exposes only read methods (`ListBuckets`, `ListLevel`, `HeadObject`,
     `GetObjectRange`). No Put/Delete/Create/Copy methods exist → UI cannot call a mutation.
  2. CI guard (grep/lint) forbids `s3.*Put*`/`*Delete*`/`*Create*`/`*Copy*` symbols outside
     `internal/storage` — build fails if a write API ever appears.
  3. Prefer anonymous or read-scoped credentials; bucket policy/IAM is the runtime backstop.

---

### Resolved unknowns

All Technical Context items resolved; **no NEEDS CLARIFICATION remain**. Version pins (exact MinIO
release tag, Charm v2 patch versions) are finalized at `go mod tidy` time.
