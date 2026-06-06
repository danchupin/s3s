# Phase 1 Data Model: Storage Operations & Analytics

Entities from the spec mapped to concrete Go types and their home packages. Field lists are the
design intent; exact names are finalized in code (test-first).

## Storage layer (`internal/storage`)

### UsageReport / UsageChild / UsageProgress (US2)

Result of `UsageOf` — read-only recursive aggregation.

```go
// UsageChild is one immediate child (sub-prefix or direct object) of the analyzed prefix.
type UsageChild struct {
    Name    string // child segment (sub-prefix has a trailing "/", object does not)
    IsDir   bool   // true => sub-prefix, false => direct object
    Size    int64  // bytes beneath this child (recursive for a sub-prefix)
    Count   int    // object count beneath this child
}

// UsageReport is the aggregate for one analyzed prefix.
type UsageReport struct {
    Bucket     string
    Prefix     string
    TotalSize  int64
    TotalCount int
    Children   []UsageChild // ranked largest-first by Size (FR-009)
    Complete   bool         // false => cancelled/partial (FR-011)
}

// UsageProgress is a running tick during a long scan (FR-011).
type UsageProgress struct {
    ScannedCount int
    ScannedSize  int64
}
```

Validation / rules: empty prefix ⇒ `TotalSize=0, TotalCount=0, Children=nil, Complete=true`
(FR-012, not an error). `Children` is sorted by `Size` desc, ties broken by `Name`. Cancellation
returns the partial report with `Complete=false` plus `ctx.Err()`.

### Storage interface additions (read-only)

```go
type Storage interface {
    // ... existing read methods ...
    GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) // US1 (full stream)
    UsageOf(ctx context.Context, bucket, prefix string, onProgress func(UsageProgress)) (UsageReport, error) // US2
}
```

Both pass straight through `readOnlyGuard` (reads). `Fake` implements both for unit tests.

## Credential resolution (`internal/secret`, `internal/config`)

### SourceKind / Source (US6)

```go
type SourceKind int
const (
    SourceInline   SourceKind = iota // secretAccessKey: literal or ${ENV}
    SourceKeychain                   // secret: keychain
    SourceCommand                    // cmd: "<command argv>"
    SourceAWSProfile                 // awsProfile: "<name>"
    // SourcePrompt is implicit (fallback), never declared in config.
)

// Source is the single configured credential descriptor for a user/context.
type Source struct {
    Kind      SourceKind
    Ref       string         // env var name | keychain account | command line | profile name
    Inline    logging.Secret // SourceInline only (already env-resolved)
}
```

Rules (FR-041): a `config.User` MUST resolve to exactly one `Source`; declaring more than one of
{`secretAccessKey`, `secret: keychain`, `cmd:`, `awsProfile:`} is a `config.Validate` error.
Anonymous users have no source.

### ResolvedCredential

```go
// ResolvedCredential is the outcome of resolving a Source (held only in memory, redacted).
type ResolvedCredential struct {
    AccessKeyID  string         // from config (non-secret) or aws profile
    SecretKey    logging.Secret // never persisted to an s3s file (FR-035)
    SessionToken logging.Secret // aws profile only (optional)
}
```

`ResolveSecret(ctx, src, accessKeyID, opts) (ResolvedCredential, error)`:
keychain→go-keyring Get; command→exec+perms-gate (FR-036); awsProfile→INI parse; inline→passthrough;
unavailable→clear error (FR-043); nothing→prompt fallback (startup only, R12).

### config.User additions

```go
type User struct {
    Name            string
    Anonymous       bool
    AccessKeyID     string
    SecretAccessKey logging.Secret // SourceInline (existing; ${ENV} still works — FR-042)
    Keychain        bool           `yaml:"keychain,omitempty"`   // SourceKeychain
    Command         string         `yaml:"cmd,omitempty"`        // SourceCommand
    AWSProfile      string         `yaml:"awsProfile,omitempty"` // SourceAWSProfile
}
```

### config.Config addition (US1 download default)

```go
// DownloadDir is the default local directory for downloads; "" => current working directory.
// Overridden at runtime by the S3S_DOWNLOAD_DIR env var (env wins). Per-download override via
// the in-TUI file browser (FR-007).
DownloadDir string `yaml:"downloadDir,omitempty"`
```

## UI layer (`internal/ui`)

### Session write state (US5) — fields on `App`

```go
raw         storage.Storage // unguarded client for the active context
ctxReadOnly bool            // context marked readonly:true (absolute lock, FR-028)
armed       bool            // runtime write-arm intent (toggle / --write initial)
// derived: writable = armed && !ctxReadOnly  (replaces the static m.writable)
```

`activeStore() storage.Storage` = `storage.Guard(raw, writable)`; all operations call it.
Transitions logged (FR-032). Context switch re-derives `ctxReadOnly`/`writable` (FR-029).

### Selection set (US3) — fields on `App`

```go
sel map[string]bool // marked OBJECT keys in the current level (folders excluded, FR-014)
// derived: selCount = len(sel); selSize = Σ size of marked objects in m.level
```

Cleared on every navigation (enter level, back/up, context switch, bucket entry — FR-019).

### Download transfer (US1) — `download.go`

```go
type downloadJob struct {
    bucket, key string
    destPath    string // final path; written via destPath+".partial" then renamed
    total       int64  // from HeadObject (progress denominator)
    transferred int64  // running
}
```

State machine: confirm-overwrite (if local file exists, FR-005) → running (progress + cancel) →
done (rename) | failed/cancelled (remove partial). Reuses `operation`/`progressEvent`/`opCh`, with
a non-`Mutator` dispatch path (download is a read).

### Bulk operation result (US3) — `bulk.go`

```go
type bulkItemResult struct {
    key    string
    ok     bool
    reason string // populated when !ok
}
type bulkResult struct {
    action     string // download | delete | copy
    succeeded  int
    failed     int
    items      []bulkItemResult
}
```

Continues past failures (FR-018); destructive bulk delete uses typed confirm on the count and logs
each op (FR-017).

### Sort order (US4) — fields on `App`

```go
type sortCol int
const (sortName sortCol = iota; sortSize; sortModified)
sortBy  sortCol // session-persistent (FR-020)
sortAsc bool
```

Applied at render time to a copy of `level.dirs`+`level.objects`; dirs ordered consistently when
sorting by size/modified (FR-021).

### New modes / keys

```go
modeUsage // du results view (ranked children + totals + progress); Enter drills down (FR-013)
```

Key additions (`keys.go`) — interaction primitives only: `Mark` (space), `Sort` (+ direction
toggle), `WriteToggle`. Download / analyze / bulk are **menu-only** (no dedicated keys) to keep
the footer uncluttered (FR-023). All advertised via the action menu / help; arrows stay primary nav.
