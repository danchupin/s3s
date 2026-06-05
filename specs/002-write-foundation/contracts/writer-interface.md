# Contract: Storage Mutating Interface + Read-Only Guard

**Package**: `internal/storage` | **Feature**: 002-write-foundation

Extends the read-only `storage.Storage` boundary (001) with the first **mutating**
capability. All write code — the interface method, its `PutObject` call, and the
read-only guard — lives only in `internal/storage`. `scripts/check-readonly.sh`
(unchanged) keeps SDK mutation symbols confined here.

## Go interface

```go
package storage

import (
    "context"
    "errors"
)

// Mutator adds write capability on top of Storage. Implemented by the real
// client and by Fake; wrapped by readOnlyGuard.
type Mutator interface {
    // CreateFolder creates an empty folder at (bucket, prefix) by putting a
    // zero-length object whose key is prefix normalised to exactly one trailing
    // "/". Returns ErrReadOnly (no network call) when the backend is read-only.
    // FR-009, FR-010.
    CreateFolder(ctx context.Context, bucket, prefix string) error
}

// ErrReadOnly is returned by mutating methods when the active backend is
// read-only (context readonly: true, or --write not set). The call never reaches
// the network, so storage is provably unchanged. FR-003, FR-011, FR-012.
var ErrReadOnly = errors.New("storage: backend is read-only")
```

The concrete client and `Fake` satisfy both `Storage` and `Mutator`. The UI depends
on these interfaces, never the SDK.

## Behaviour contract

- **Key shaping**: `CreateFolder(ctx, b, "reports")` and
  `CreateFolder(ctx, b, "reports/")` both create key `reports/`. Nested:
  `a/b` → `a/b/`. Implementation normalises to exactly one trailing `/`.
- **Validation** (before any network call): reject empty/whitespace prefix; reject
  control characters; return a distinct, classifiable error so the UI can show
  guidance (FR-010). On a name that already exists as a folder or collides with an
  existing object key, surface a clear "already exists" error — never overwrite.
- **Read-only refusal**: when wrapped by `readOnlyGuard` in read-only mode, return
  `ErrReadOnly` immediately; the underlying client is not called (SC-002, SC-007).
- **Errors**: reuse `classify` → `ErrNotFound`/`ErrAccessDenied`/`ErrUnreachable`/
  `ErrInvalidConfig`, plus `ErrReadOnly`. Errors MUST NOT embed secrets (FR-008).
- **Idempotency**: putting the same `prefix/` twice is not an error at the S3 level;
  the UI's "already exists" check is advisory (best-effort, from the current
  listing), not a hard precondition.

## Read-only guard

```go
// readOnlyGuard wraps a backend and refuses every mutation. Reads pass through.
type readOnlyGuard struct{ Storage } // embeds the real backend's read methods

func (readOnlyGuard) CreateFolder(context.Context, string, string) error {
    return ErrReadOnly
}

// Guard wraps b in a read-only guard unless policy.Writable.
func Guard(b Storage, policy WritePolicy) Storage { ... }
```

- When `policy.Writable == false`, `Guard` returns a `readOnlyGuard` whose mutating
  methods all return `ErrReadOnly`; reads delegate to the wrapped backend.
- When `policy.Writable == true`, `Guard` returns the backend unwrapped.
- Resolution happens at construction (in the resolver closure that rebuilds the
  backend on context switch), so the UI holds an already-correct backend.

## Construction

No change to `ClientConfig` or `New` (001). Read-only is enforced by `Guard`, not by
the client. The resolver composes them:

```go
clientCfg, err := cfg.ClientConfig(name)        // unchanged
backend, err := storage.New(clientCfg)          // unchanged
policy, err := cfg.WritePolicyFor(name, writeFlag) // new method; existing Resolve untouched
backend = storage.Guard(backend, policy)        // read-only unless Writable
```

## Test contract

- **Unit (fake)**: `Fake` implements `CreateFolder` (mutates its in-memory map).
  Tests assert: key normalisation, validation rejects, "already exists" detection,
  and that `Guard(fake, {Writable:false}).CreateFolder(...) == ErrReadOnly` with the
  fake's contents unchanged.
- **Integration (`//go:build integration`, MinIO)**: create `reports/` on a
  writable backend → re-list the level → the folder appears as a common prefix;
  the guard refuses a mutation on a read-only-wrapped backend without contacting the
  server; access-denied path returns `ErrAccessDenied`, storage unchanged.
- **Guard test**: writing through `readOnlyGuard` never reaches a stub client (use a
  client that fails the test if any mutating method is invoked).
