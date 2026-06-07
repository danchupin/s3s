# Contract: Write-mode legibility, reversibility & prominent confirmation

**Stories**: US2, US4. FR-006..009, FR-014..017, FR-038. Source: `styles.go`, `commandbar.go`,
`writemode.go`, `app.go`, reuse `confirmview.go`.

## Badge color (FR-006)

`footerIdentityCompact` (`styles.go:411`): the separator space renders with `dimCellStyle`; only the tag
text (`[RO]`/`[RW]`) carries the state color. No whitespace is colored.

## Symmetric labels (FR-007/FR-008/FR-009)

`writeColumn` (`commandbar.go:213-222`): key sourced from `m.keys.WriteToggle`.
- disarmed (writable): "`<w>` enable write" (accent/warn)
- armed: "`<w>` → read-only" / "disable write" (accent)
- context read-only (forbidden): cue that writes are unavailable for this context.
Symmetric across all three states; the toggle key + current state always discoverable.

## Prominent arm confirmation (FR-014..017)

- New `armConfirmPopupView` reusing `confirmPopupView`/`popupBoxStyle`; overlaid in `View()` like the binary
  confirm. The `statusLine` `armConfirm` branch yields to it.
- Shows the consequence ("arm WRITE mode? mutations will be enabled") + confirm/cancel keys (cancel default).
- The read/write badge AND the border mode chip stay visible during the prompt (FR-017).
- Disarming stays instant — no confirmation (FR-016); `onArmConfirmKey`/`toggleWrite` logic unchanged; the
  `slog` write-toggle event preserved (constitution V).

## Mode chip (FR-038)

See `layout-visibility-contract.md` — `WRITE`/`RO` chip inset into the box top border, accent when armed,
neutral read-only; safety-redundant exception to the FR-033 dedup.

## Tests

`TestBadgeDoesNotColorAdjacentSpace`, `TestEnableWriteLabelWhenDisarmed`, `TestDisarmCueWhenArmed`,
`TestArmConfirmationIsCenteredPopup`, `TestArmConfirmBadgeAndChipStayVisible`, `TestDisarmIsInstantNoPopup`,
`TestReadonlyContextCue`, `TestWriteModeNoColor`.
