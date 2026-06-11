package ui

import (
	"testing"
	"time"
)

// TestRelTimeTable: the relative-time helper over fixed clocks (017 US2/FR-010, D14).
func TestRelTimeTable(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		d    time.Duration // age (now - t)
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{4 * 24 * time.Hour, "4d ago"},
		{70 * 24 * time.Hour, "2mo ago"},
		{3 * 365 * 24 * time.Hour, "3y ago"},
		{-3 * 24 * time.Hour, "in 3d"}, // future (Retain until / Expires)
	}
	for _, c := range cases {
		if got := relTime(now, now.Add(-c.d)); got != c.want {
			t.Errorf("relTime(age=%v) = %q, want %q", c.d, got, c.want)
		}
	}
	if got := relTime(now, time.Time{}); got != "" {
		t.Errorf("relTime(zero) = %q, want empty", got)
	}
}

// TestAppClockInjectable: App carries an injectable clock defaulting to time.Now
// (deterministic white-box view tests — 017 D14).
func TestAppClockInjectable(t *testing.T) {
	m := newApp(nil, nil, nil)
	if m.now == nil {
		t.Fatal("App.now must default to a usable clock")
	}
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return fixed }
	if !m.now().Equal(fixed) {
		t.Error("App.now must be overridable in tests")
	}
}
