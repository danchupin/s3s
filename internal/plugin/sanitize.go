package plugin

import (
	"sort"
	"strings"
)

// TruncationMarker is appended whenever a length cap cuts a plugin string, so
// truncation is always visible (never silent).
const TruncationMarker = "…(truncated)"

// Sanitize makes one plugin-supplied string safe to render: C0/C1 control
// characters, DEL, and ANSI escape sequences (CSI/OSC/two-byte) are stripped so
// plugin text can never alter terminal state; legitimate UTF-8 survives. When
// singleLine, newlines collapse to single spaces (tabs always become spaces).
// maxBytes > 0 caps the result without splitting a rune and appends
// TruncationMarker when it cuts.
func Sanitize(s string, maxBytes int, singleLine bool) string {
	s = strings.ToValidUTF8(s, "�")
	var b strings.Builder
	b.Grow(len(s))
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == 0x1b: // ESC — skip the whole sequence
			i += escapeLen(rs[i+1:])
		case r == '\n' || r == '\r':
			if singleLine {
				// Collapse a newline run (\r\n etc.) to one space.
				for i+1 < len(rs) && (rs[i+1] == '\n' || rs[i+1] == '\r') {
					i++
				}
				b.WriteByte(' ')
			} else if r == '\n' {
				b.WriteByte('\n')
			}
		case r == '\t':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			// C0 / DEL / C1 — dropped
		default:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if maxBytes > 0 && len(out) > maxBytes {
		cut := maxBytes
		for cut > 0 && !isRuneStart(out[cut]) {
			cut--
		}
		out = out[:cut] + TruncationMarker
	}
	return out
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// escapeLen returns how many runes after ESC belong to the escape sequence.
func escapeLen(rest []rune) int {
	if len(rest) == 0 {
		return 0
	}
	switch rest[0] {
	case '[': // CSI: parameters/intermediates, then one final byte in 0x40–0x7e
		return 1 + csiLen(rest[1:])
	case ']': // OSC: terminated by BEL or ST (ESC \)
		for i := 1; i < len(rest); i++ {
			if rest[i] == 0x07 {
				return i + 1
			}
			if rest[i] == 0x1b && i+1 < len(rest) && rest[i+1] == '\\' {
				return i + 2
			}
		}
		return len(rest)
	default: // two-byte escape (ESC + one char)
		return 1
	}
}

// csiLen returns how many runes a CSI body spans (through its final byte).
func csiLen(rest []rune) int {
	for i, r := range rest {
		if r >= 0x40 && r <= 0x7e {
			return i + 1
		}
	}
	return len(rest)
}

// ValidBucketName checks S3 bucket-name rules: 3–63 characters of lowercase
// letters, digits, dots, and hyphens, with an alphanumeric character at both
// ends. Discovered names failing this are discarded (and counted) by
// FilterBuckets.
func ValidBucketName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '.' || c == '-':
			if i == 0 || i == len(name)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// FilterBuckets prepares one discovery result for merging: every name is
// sanitized, invalid names are discarded and counted, duplicates collapse
// silently, the result is name-sorted, and MaxBuckets bounds it (truncated
// reports a cut).
func FilterBuckets(names []string) (valid []string, discarded int, truncated bool) {
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		n = Sanitize(n, 0, true)
		if !ValidBucketName(n) {
			discarded++
			continue
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		valid = append(valid, n)
	}
	sort.Strings(valid)
	if len(valid) > MaxBuckets {
		valid = valid[:MaxBuckets]
		truncated = true
	}
	return valid, discarded, truncated
}

// CapFields prepares one metadata result for display: field names and values
// are sanitized, values are capped at MaxFieldValue bytes (marker appended),
// and the field count is bounded by MaxFields (truncated reports a cut).
// Response order is preserved.
func CapFields(fields []Field) ([]Field, bool) {
	truncated := false
	if len(fields) > MaxFields {
		fields = fields[:MaxFields]
		truncated = true
	}
	out := make([]Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, Field{
			Name:  Sanitize(f.Name, 256, true),
			Value: Sanitize(f.Value, MaxFieldValue, true),
		})
	}
	return out, truncated
}
