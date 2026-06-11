package plugin

import (
	"strings"
	"testing"
)

func TestSanitizeStripsControlFlow(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", "hello"},
		{"utf8 survives", "фото — café 日本", "фото — café 日本"},
		{"csi stripped", "a\x1b[31mred\x1b[0mb", "aredb"},
		{"osc bel stripped", "x\x1b]0;title\x07y", "xy"},
		{"osc st stripped", "x\x1b]8;;http://e\x1b\\y", "xy"},
		{"bare escape pair", "a\x1bZb", "ab"},
		{"c0 stripped", "a\x01\x02b", "ab"},
		{"c1 stripped", "a\u0085\u009bb", "ab"},
		{"del stripped", "a\x7fb", "ab"},
		{"tab to space", "a\tb", "a b"},
		{"invalid utf8 replaced", "a\xffb", "a�b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Sanitize(tc.in, 0, false); got != tc.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeNewlineCollapse(t *testing.T) {
	if got := Sanitize("a\nb\r\nc", 0, true); got != "a b c" {
		t.Errorf("single-line collapse = %q, want %q", got, "a b c")
	}
	if got := Sanitize("a\nb", 0, false); got != "a\nb" {
		t.Errorf("multi-line keeps newline: %q", got)
	}
}

func TestSanitizeLengthCap(t *testing.T) {
	long := strings.Repeat("x", 50)
	got := Sanitize(long, 10, true)
	if !strings.HasSuffix(got, TruncationMarker) {
		t.Errorf("capped string must end with the truncation marker: %q", got)
	}
	if !strings.HasPrefix(got, "xxxxxxxxxx") || strings.Count(got, "x") != 10 {
		t.Errorf("cap should keep 10 bytes of payload: %q", got)
	}
	if got := Sanitize("short", 10, true); got != "short" {
		t.Errorf("under-cap string must pass through: %q", got)
	}
	// A multi-byte rune at the cap boundary is dropped whole, never split.
	got = Sanitize(strings.Repeat("ф", 50), 9, true) // "ф" is 2 bytes
	cut := strings.TrimSuffix(got, TruncationMarker)
	if !strings.HasSuffix(cut, "ф") || len(cut) > 9 {
		t.Errorf("rune-safe cap violated: %q (payload %d bytes)", got, len(cut))
	}
}

func TestValidBucketName(t *testing.T) {
	valid := []string{"abc", "my-bucket", "a.b-c", "images-prod", "b23", strings.Repeat("a", 63)}
	for _, n := range valid {
		if !ValidBucketName(n) {
			t.Errorf("ValidBucketName(%q) = false, want true", n)
		}
	}
	invalid := []string{
		"",                        // empty
		"ab",                      // too short
		strings.Repeat("a", 64),   // too long
		"UPPER",                   // uppercase
		"has space",               // space
		"-leading",                // non-alnum start
		"trailing-",               // non-alnum end
		".dot",                    // dot start
		"dot.",                    // dot end
		"under_score",             // underscore
		"weird\x1b[31mname-12345", // control bytes
	}
	for _, n := range invalid {
		if ValidBucketName(n) {
			t.Errorf("ValidBucketName(%q) = true, want false", n)
		}
	}
}

func TestFilterBuckets(t *testing.T) {
	names := []string{"beta", "alpha", "UPPER", "alpha", "x", "ok-name"}
	valid, discarded, truncated := FilterBuckets(names)
	if discarded != 2 { // UPPER + x invalid; the duplicate is deduped, not discarded
		t.Errorf("discarded = %d, want 2", discarded)
	}
	if truncated {
		t.Error("no truncation expected")
	}
	want := map[string]bool{"beta": true, "alpha": true, "ok-name": true}
	if len(valid) != len(want) {
		t.Fatalf("valid = %v", valid)
	}
	for _, n := range valid {
		if !want[n] {
			t.Errorf("unexpected name %q", n)
		}
	}
}

func TestFilterBucketsCap(t *testing.T) {
	names := make([]string, MaxBuckets+10)
	for i := range names {
		names[i] = "bucket-" + strings.Repeat("a", 3) + "-" + itoa(i)
	}
	valid, _, truncated := FilterBuckets(names)
	if !truncated {
		t.Error("expected truncation past the cap")
	}
	if len(valid) != MaxBuckets {
		t.Errorf("len(valid) = %d, want %d", len(valid), MaxBuckets)
	}
}

func TestCapFields(t *testing.T) {
	fields := make([]Field, MaxFields+5)
	for i := range fields {
		fields[i] = Field{Name: "f" + itoa(i), Value: "v"}
	}
	out, truncated := CapFields(fields)
	if !truncated || len(out) != MaxFields {
		t.Errorf("count cap: len=%d truncated=%v", len(out), truncated)
	}

	big := Field{Name: "big", Value: strings.Repeat("x", MaxFieldValue+100)}
	out, truncated = CapFields([]Field{big})
	if truncated {
		t.Error("a long value alone is not a count truncation")
	}
	if !strings.HasSuffix(out[0].Value, TruncationMarker) {
		t.Errorf("over-cap value must carry the marker: …%q", out[0].Value[len(out[0].Value)-30:])
	}

	dirty := Field{Name: "n\x1b[31m", Value: "v\x07alue"}
	out, _ = CapFields([]Field{dirty})
	if out[0].Name != "n" || out[0].Value != "value" {
		t.Errorf("fields must be sanitized: %+v", out[0])
	}
}

// itoa avoids strconv just for tests' readability in table builders.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
