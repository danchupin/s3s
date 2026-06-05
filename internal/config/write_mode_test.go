package config

import "testing"

// TestWriteModeFor covers the opt-out truth table: writable iff --write is on AND
// the context is not marked readonly (read-only always wins — FR-001/FR-002, SC-002).
func TestWriteModeFor(t *testing.T) {
	c := &Config{Contexts: []Context{
		{Name: "rw", Cluster: "x", User: "y"},
		{Name: "ro", Cluster: "x", User: "y", ReadOnly: true},
	}}

	cases := []struct {
		ctx  string
		flag bool
		want bool
	}{
		{"rw", false, false}, // writes off → read-only
		{"rw", true, true},   // writes on, not readonly → writable
		{"ro", false, false}, // writes off → read-only
		{"ro", true, false},  // writes on but readonly → still read-only (wins)
	}
	for _, tc := range cases {
		got, err := c.WriteModeFor(tc.ctx, tc.flag)
		if err != nil {
			t.Fatalf("WriteModeFor(%q,%v) error: %v", tc.ctx, tc.flag, err)
		}
		if got.Writable != tc.want {
			t.Errorf("WriteModeFor(%q,%v).Writable = %v, want %v", tc.ctx, tc.flag, got.Writable, tc.want)
		}
	}

	if _, err := c.WriteModeFor("nope", true); err == nil {
		t.Error("WriteModeFor on an unknown context should error")
	}
}
