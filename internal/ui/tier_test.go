package ui

import "testing"

// TestLayoutTier asserts the normative width-tier boundaries (011 FR-013): Full ≥130,
// Dual 100–129, Single ≤99. Boundaries are inclusive as documented.
func TestLayoutTier(t *testing.T) {
	cases := []struct {
		w    int
		want tier
	}{
		{0, tierSingle},
		{80, tierSingle},
		{99, tierSingle}, // just below the Dual boundary
		{100, tierDual},  // paneSplitMin — Dual starts
		{120, tierDual},
		{129, tierDual}, // just below the Full boundary
		{130, tierFull}, // fullTierMin — Full starts
		{200, tierFull},
	}
	for _, c := range cases {
		if got := layoutTier(c.w); got != c.want {
			t.Errorf("layoutTier(%d) = %d, want %d", c.w, got, c.want)
		}
	}
}
