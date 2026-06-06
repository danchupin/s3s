# Quickstart: Storage Operations & Analytics (005)

How an engineer uses the new capabilities once 005 lands. Read-only by default; download and
analyze need no write mode.

## 1. Connect without exporting a secret (US6)

Configure a context's secret once in the OS keystore via the wizard:

```bash
s3s config init
#   endpoint, addressing, TLS …
#   credential source? [keychain | cmd | awsProfile | env]
#   > keychain
#   access key id: AKIA...
#   secret (no echo): ********        # stored in the OS keystore, NOT the config file
```

Or point a context at an existing secret store / profile by hand (exactly ONE source):

```yaml
users:
  - name: prod                       # keychain (stored via `s3s cred set prod`)
    accessKeyId: AKIAPROD
    keychain: true
  - name: vault                      # external command (owner-only config required)
    accessKeyId: AKIAVLT
    cmd: "vault kv get -field=secret s3/prod"
  - name: aws                        # ~/.aws/credentials profile (static keys)
    awsProfile: prod
  - name: ci                         # unchanged ${ENV} — still works for automation
    accessKeyId: AKIACI
    secretAccessKey: ${S3S_CI_SECRET}
```

Manage keystore secrets:

```bash
s3s cred set prod      # prompt (no echo) → store in OS keystore
s3s cred rotate prod
s3s cred rm prod
```

Now open a brand-new terminal — no `export` needed:

```bash
s3s --context prod     # connects; secret pulled from the keystore at launch
```

Notes: a config file readable by others triggers a warning (`chmod 600`); a `cmd:` source is
**refused** on a group/world-writable config. Configuring two sources for one user is a load
error. Secrets never appear in logs, the UI, or error text.

## 2. Download an object to disk (US1) — works read-only

```text
navigate to the object → a (action menu) → download
  → confirm overwrite if a local file exists
  → live progress (bytes / %), Esc cancels (no partial file left behind)
```

Default destination is the working directory (configurable); press the destination override to
pick a directory in the in-TUI file browser. No `--write` required — download is a read.

## 3. See what's eating space (US2) — works read-only

```text
select a bucket or folder → a → analyze
  → total size + object count for everything beneath
  → children ranked largest-first with size + % of parent (ncdu-style)
  → Enter drills into the heaviest child to find the exact consumer
  → Esc cancels a long scan (running totals shown; UI never freezes)
```

## 4. Act on many objects at once (US3)

```text
space            mark / unmark the current object (folders can't be marked)
                 header shows: "<n> selected · <combined size>"
a → download     pull all marked objects, recreating their key paths as local subdirs (read-only)
a → delete       requires WRITE armed; type the count to confirm; each delete logged
a → copy         requires WRITE armed
```

Leaving the level clears the selection. Recursive folder deletion stays its own dedicated action
(not via multi-select).

## 5. Sort lists (US4)

```text
sort key         cycle column: name → size → modified
direction key    toggle asc / desc
```

Sort persists across navigation for the session — e.g. set size-desc once and every level shows
the biggest first.

## 6. Arm write only when you mean it (US5)

```text
launch           s3s            → READ-ONLY (calm "RO")
write toggle     → confirm      → WRITE armed (loud, high-contrast "WRITE" badge on every screen)
write toggle     → instant      → back to READ-ONLY
s3s --write      → starts armed in WRITE from the first frame
```

A context marked `readonly: true` refuses to arm (and forces read-only if you switch into it
while armed). Every arm/disarm is logged. Mutating actions (single + bulk delete/copy) appear
only while armed.

## Verify (maps to Success Criteria)

- `s3s --context prod` in a fresh shell connects with no export/prompt (SC-011) and no plaintext
  secret on disk or in the environment (SC-012).
- Download a 200 MB object from a read-only context → byte-identical local file, progress shown
  (SC-001), cancellable (SC-006).
- Analyze a large prefix → totals + ranked consumers, cancellable mid-scan (SC-002).
- Mark 50 objects → one bulk action with a truthful per-item summary (SC-003).
- Size-desc sort surfaces the largest objects in two keystrokes (SC-004).
- While armed, the WRITE badge is unmistakable on every screen; a `readonly: true` context can
  never arm (SC-009).
