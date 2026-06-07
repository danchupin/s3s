# Quickstart: Pinned Buckets (010)

## What it does
Lets a connection whose credentials **can't list all buckets** (no `s3:ListAllMyBuckets`) still
browse the buckets it *can* reach: declare the bucket names, s3s skips `ListBuckets` and shows them
directly. Add more buckets at runtime in the UI. Works with domain/virtual-hosted-style endpoints
(e.g. Avito RGW `<bucket>.bucket.avito-sd`, whose apex is unlistable).

## User flow

### Create a scoped connection (form)
1. Open connections → `+ add connection`.
2. Fill `name`, `endpoint` (e.g. `https://bucket.avito-sd`), `access key id`, `secret`. Leave
   `path-style` **off** for domain-style.
3. In the new **`buckets`** field, type the bucket names you can access, comma/space-separated:
   `st-img-range-bucket-1416, some-other-bucket`.
4. `Enter`. The test probes the first named bucket (not list-all). `AccessDenied` is treated as
   reachable → it saves. A real failure shows the *actual* reason ("Backend unreachable…",
   "Not found…") and still offers "press Enter again to save anyway".
5. You land in the bucket list showing exactly your pinned buckets.

### Add another bucket at runtime
1. On the bucket list of a scoped connection, move to the `+ add bucket` row (last row) and `Enter`.
2. Type the bucket name, `Enter`. It is persisted to the connection and appears in the list.
3. `Enter` it to browse; switch between all your buckets freely — no `ListBuckets` is ever called.

### Edit config by hand (equivalent)
```yaml
clusters:
  - name: avito-staging
    endpoint: https://bucket.avito-sd
    pathStyle: false
    buckets:
      - st-img-range-bucket-1416
```

## Verify (manual)
- A pinned connection never logs a `ListBuckets` call; `~/.local/state/s3s/s3s.log` shows
  `list level` for the bucket you open, not `list buckets`.
- A non-pinned connection is unchanged.

## Test (automated)
```bash
make test                                   # all unit tests (UI white-box + config + fake)
go test ./internal/ui/  -run TestPinned     # pinned-bucket UI tests
go test ./internal/config/ -run Bucket      # config round-trip + AppendBucket
make fmt vet lint check-readonly            # gates — check-readonly MUST stay green
```
Key cases (see contracts/): pinned list renders with 0 `ListBuckets` calls; `+ add bucket` row shows
only for scoped lists; runtime add persists + reflects; probe uses `ListLevel(MaxKeys=1)`;
`AccessDenied` → save; non-AccessDenied errors show the classified message.

## Boundaries
- Un-pin / remove a bucket from the set: not in v1 — edit the config file.
- TLS-skip-verify is still not in the form (out of scope).
- A working list-all connection never shows `+ add bucket` (no accidental hiding of buckets).
