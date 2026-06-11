package preview

import (
	"strings"
	"testing"
)

// TestHexDumpFormat: offset + hex bytes + printable column, 16 bytes per row, with
// non-printables dotted (017 US5/FR-027).
func TestHexDumpFormat(t *testing.T) {
	data := append([]byte("ABCDEFGHIJKLMNOP"), 0x00, 0x01, 'Z')
	out := HexDump(data)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("rows = %d, want 2 (16 bytes per row):\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "00000000") {
		t.Errorf("row 0 must start with the offset:\n%s", lines[0])
	}
	if !strings.Contains(lines[0], "41 42 43") || !strings.Contains(lines[0], "|ABCDEFGHIJKLMNOP|") {
		t.Errorf("row 0 must carry hex + printable columns:\n%s", lines[0])
	}
	if !strings.Contains(lines[1], "00 01 5a") || !strings.Contains(lines[1], "|..Z|") {
		t.Errorf("row 1 must dot non-printables:\n%s", lines[1])
	}
}

// TestHexDumpEmpty: no payload → no rows (header/summary is the caller's concern).
func TestHexDumpEmpty(t *testing.T) {
	if out := HexDump(nil); out != "" {
		t.Errorf("HexDump(nil) = %q, want empty", out)
	}
}
