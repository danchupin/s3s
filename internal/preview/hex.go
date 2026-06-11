package preview

import (
	"fmt"
	"strings"
)

// HexDump renders the classic offset + hex + printable-column dump, 16 bytes per row
// (017 US5/FR-027). Rows are ~76 chars — within the ≥80-column minimum, no wrapping.
func HexDump(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var b strings.Builder
	for off := 0; off < len(data); off += 16 {
		end := min(off+16, len(data))
		row := data[off:end]
		fmt.Fprintf(&b, "%08x  ", off)
		for i := range 16 {
			if i == 8 {
				b.WriteByte(' ')
			}
			if i < len(row) {
				fmt.Fprintf(&b, "%02x ", row[i])
			} else {
				b.WriteString("   ")
			}
		}
		b.WriteString(" |")
		for _, c := range row {
			if c >= 0x20 && c < 0x7f {
				b.WriteByte(c)
			} else {
				b.WriteByte('.')
			}
		}
		b.WriteString("|\n")
	}
	return b.String()
}
