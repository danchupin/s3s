package ui

import (
	"fmt"
	"strings"
)

// Progress feedback for long operations (007 US6): a Claude-Code-style determinate bar
// (filled/empty track + trailing percent) rendered inline in the footer status zone. An
// operation with an unknown total falls back to an indeterminate activity indicator (no
// fabricated percent, FR-037). The bar appears only after a brief "taking a while"
// threshold so fast operations never flash it (FR-035/SC-013).

// progressThreshold is how many spinner ticks must elapse during phaseRunning before the
// determinate bar is shown. Fast operations complete before this and show no bar.
const progressThreshold = 2

// determinate returns the completion fraction (0..1) and whether progress is known. It
// is unknown (indeterminate) when no total is set — e.g. a recursive delete that has not
// enumerated its objects up front (FR-037).
func (p opProgress) determinate() (float64, bool) {
	if p.total <= 0 {
		return 0, false
	}
	processed := p.uploaded // bytes (upload/download)
	if processed == 0 {
		processed = int64(p.deleted + p.failed) // counts (bulk)
	}
	frac := float64(processed) / float64(p.total)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return frac, true
}

// progressBar renders a determinate bar: a filled run (accent) + an empty run (dim) and a
// trailing percent, e.g. "[████████░░░░░░] 57%". width is the total cell width of the
// track (clamped to a sane minimum).
func progressBar(frac float64, width int) string {
	if width < 4 {
		width = 4
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	filled := int(frac*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	bar := accentStyle.Render(strings.Repeat("█", filled)) +
		dimCellStyle.Render(strings.Repeat("░", width-filled))
	return dimCellStyle.Render("[") + bar + dimCellStyle.Render("]") +
		dimCellStyle.Render(fmt.Sprintf(" %3d%%", int(frac*100+0.5)))
}
