# Contract: Copy & Share Menu (US3)

## Invocation

- `Y` (`keys.CopyMenu`) opens the overlay; `:copy` opens the same overlay (command synonym,
  single dispatcher). `Esc` closes without action.
- Menu items are focus-aware:

| Focus | Items |
|---|---|
| object (browse selection or modeObject) | S3 URI · HTTPS URL · download command · presigned link… · copy a field… |
| bucket | S3 URI (`s3://bucket/`) |
| prefix/dir | S3 URI (`s3://bucket/prefix/`) |
| modeHealth / usage report visible | + export CSV · export JSON |

## Artifacts (built by `internal/share`, pure)

- `S3URI`: `s3://<bucket>/<key>`; prefixes keep the trailing `/`.
- `HTTPURL`: honors the context's `PathStyle` (`internal/config/config.go:57`):
  path-style `https://<host>/<bucket>/<escaped-key>`, vhost `https://<bucket>.<host>/<escaped-key>`.
  Keys percent-escaped (space, `+`, unicode, `?` — table-driven unit).
- download command: `aws s3api get-object --endpoint-url <ep> --bucket <b> --key <k> <basename>`.
- presigned: TTL picker (sub-overlay) with exactly `15m · 1h (default) · 24h · 7d`;
  generation via `storage.PresignGet` in a `tea.Cmd` (cred command may block — constitution
  II); on success the URL is copied + a curl snippet offered; `warn` (cred expiry before TTL)
  renders in the footer (warn role).
- export: `share.ExportCSV/ExportJSON` bytes written via `tea.Cmd` to
  `<DownloadDir>/s3s-report-<bucket>[-<prefix-slug>]-<ts>.{csv,json}`; temp-write+rename;
  failure → temp removed + footer error.

## Clipboard discipline

- Copy = the existing best-effort OSC52 command (the reveal path,
  `internal/ui/reveal.go:10,43,81`) — works locally and over SSH.
- Fallback: when the artifact cannot be confirmed copied (OSC52 is fire-and-forget), the menu
  ALWAYS also shows the value in the reveal popup on request ("show value") for manual copy —
  no silent failure (FR-019).
- Footer confirmation names the artifact and target: `copied HTTPS URL — logs/2026/app.log`
  (never bare "copied").
- The presigned URL: shown/copied only; NEVER written to slog by any path (assert: log file
  free of `X-Amz-Signature` after the flow — unit hooks the logger).

## Consistency (constitution VII)

- Overlay reuses the connections-manager list pattern (selection, Enter-to-act, Esc-cancel);
  labels use the shared key-glyph + verb vocabulary; no new palette roles.
- `keys.CopyMenu("Y")` registered in `defaultKeys` + help (`Actions` section) + hintbar where
  relevant; hints derive from the keymap (`keyHint`), never hardcoded.

## Test obligations (RED first)

1. Menu item matrix per focus (object/bucket/prefix/health) — exact items, no more.
2. Builder units in `internal/share`: URI/URL escaping table; path vs vhost; snippets.
3. TTL picker: only 4 presets; Enter default = 1h; result lands in OSC52 cmd payload.
4. Cred-expiry warn: Fake creds expiring in 30m + TTL 24h → footer warn present.
5. Export: file exists with exact name pattern in DownloadDir; CSV golden; JSON
   round-trips; `bounded:true` carried for partial reports; failure path leaves no file.
6. Footer confirmation strings per artifact kind.
7. Log redaction: presign flow leaves no URL/signature in the log buffer.
