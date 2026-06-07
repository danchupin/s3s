# Contract: Connection UI (delete hint, form editing, secret guidance)

## Add-connection form editing (FR-005, FR-006, FR-007, FR-008)

| Input | Focused field | Effect |
|-------|---------------|--------|
| printable key (`msg.Text`) | text field | `Insert(text)` at caret |
| `tea.PasteMsg` | text field | newline-sanitized `Insert(content)` at caret (atomic) |
| `←` / `→` | text field | `Left()` / `Right()` |
| `Home` / `End` | text field | `Home()` / `End()` |
| `Backspace` | text field | `Backspace()` at caret |
| `←/→/Home/End/paste` | boolean row | no-op (toggles unaffected; `space` still toggles) |
| `↑/↓/Tab` | any | move focus between fields (unchanged) |
| `Enter` / `Esc` | any | submit / cancel (unchanged) |

- Secret field edits identically but renders masked (`•`), wrapped in `logging.Secret` only
  at `draft()`; never written to config plaintext (FR-007, unchanged).
- Pasted content with a trailing newline must not submit the form or break the line.

## Secret + per-field guidance (FR-009, FR-010)

- When a field is focused, a hint line states its expectation.
- Secret field hint MUST: name the input (secret access key), state it is stored in the OS
  keychain, and note other sources (env var · cmd · AWS profile) are config-file-only.
- Hint MUST NOT imply the form resolves `${ENV}`/cmd/AWS-profile (it stores verbatim → keychain).

## Connection-list delete hint (FR-001, FR-002, FR-003, FR-012)

| Selection | Hint shown | On delete keystroke |
|-----------|------------|---------------------|
| non-active existing connection | `Ctrl+X delete` (active style) | start typed-name confirm (unchanged) |
| active connection | hint present (may be marked) | guard message: cannot delete active (FR-002) |
| `+ add connection` row | delete segment **absent** (not rendered) | no-op |
| empty list | delete segment **absent** (not rendered) | no-op |

- The hint is rendered **inline in the connections view** (NOT via the command-bar catalog).
- Remains visible/correct when the command bar collapses on a narrow terminal (FR-012).

## Acceptance (tests, written first)

1. Open form, paste `"https://h:9000\n"` into endpoint → field == `"https://h:9000"` (no newline, not submitted).
2. Type `"htps"`, `Left()`×2, `Insert("t")` → `"htps"`→ caret-correct middle insert (e.g. `"htt ps"` per ops) verifying mid-field edit.
3. Secret field: paste a 40-char key → masked render shows 40 `•`; `draft().Secret` redacts.
4. Connections list, non-active selected → View contains `Ctrl+X` + `delete`.
5. Add-row selected → View has no active delete hint.
6. Active connection selected, press Ctrl+X → notice "cannot delete the active connection".
