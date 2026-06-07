package ui

import (
	"strings"
	"testing"
)

// --- 007 US6: progress bar for long operations ---

func TestProgressBarDeterminate(t *testing.T) {
	bar := progressBar(0.5, 20)
	if !strings.Contains(bar, "50%") {
		t.Errorf("determinate bar should show the percent; got %q", bar)
	}
	if !strings.Contains(bar, "█") || !strings.Contains(bar, "░") {
		t.Errorf("determinate bar should have a filled and an empty run; got %q", bar)
	}
	full := progressBar(1.0, 20)
	if !strings.Contains(full, "100%") {
		t.Errorf("full bar should read 100%%; got %q", full)
	}
}

func TestOpProgressDeterminateAndIndeterminate(t *testing.T) {
	// Known total (upload bytes) → determinate.
	if frac, ok := (opProgress{uploaded: 5, total: 10}).determinate(); !ok || frac != 0.5 {
		t.Errorf("byte progress should be determinate 0.5; got frac=%v ok=%v", frac, ok)
	}
	// Known count total (bulk) → determinate.
	if frac, ok := (opProgress{deleted: 2, failed: 1, total: 4}).determinate(); !ok || frac != 0.75 {
		t.Errorf("count progress should be determinate 0.75; got frac=%v ok=%v", frac, ok)
	}
	// Unknown total (recursive delete) → indeterminate, no fabricated percent (FR-037).
	if _, ok := (opProgress{deleted: 9}).determinate(); ok {
		t.Error("unknown total must be indeterminate")
	}
}

func TestOpProgressLineThresholdGate(t *testing.T) {
	m := App{op: &operation{kind: "upload", phase: phaseRunning, progress: opProgress{uploaded: 5, total: 10}}}
	m.loading = true

	// Before the threshold: no determinate bar (fast ops never flash one, SC-013).
	m.op.ticks = 0
	if line := m.opProgressLine(); strings.Contains(line, "%") {
		t.Errorf("below the threshold the bar must not show a percent; got %q", line)
	}
	// Past the threshold: the determinate bar with a percent appears (SC-012).
	m.op.ticks = progressThreshold
	if line := m.opProgressLine(); !strings.Contains(line, "%") {
		t.Errorf("past the threshold a determinate bar should appear; got %q", line)
	}
}

func TestOpProgressLineIndeterminateNoPercent(t *testing.T) {
	m := App{op: &operation{kind: "delete_recursive", phase: phaseRunning, ticks: 5, progress: opProgress{deleted: 3}}}
	m.loading = true
	line := m.opProgressLine()
	if strings.Contains(line, "%") {
		t.Errorf("an unknown-total op must not show a percent; got %q", line)
	}
	if !strings.Contains(line, "deleting") {
		t.Errorf("indeterminate progress should still label the work; got %q", line)
	}
}
