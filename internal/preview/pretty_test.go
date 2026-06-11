package preview

import (
	"strings"
	"testing"
)

// TestClassifyJSONKinds: a single JSON value → KindJSON; ≥2 newline-delimited values →
// KindNDJSON; broken JSON stays plain text (017 US5/FR-025).
func TestClassifyJSONKinds(t *testing.T) {
	if k := Classify("application/json", []byte(`{"a":1}`)); k != KindJSON {
		t.Errorf("single object = %v, want KindJSON", k)
	}
	nd := []byte("{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n")
	if k := Classify("application/x-ndjson", nd); k != KindNDJSON {
		t.Errorf("ndjson = %v, want KindNDJSON", k)
	}
	// Content-type lies / no type: sniffed text that parses as JSON still upgrades.
	if k := Classify("", []byte(`[1,2,3]`)); k != KindJSON {
		t.Errorf("sniffed array = %v, want KindJSON", k)
	}
	if k := Classify("application/json", []byte(`{"a":1`)); k != KindText {
		t.Errorf("broken JSON = %v, want plain KindText", k)
	}
	if k := Classify("text/plain", []byte("just words")); k != KindText {
		t.Errorf("plain text = %v, want KindText", k)
	}
}

// TestPrettyJSON: 2-space indentation; the raw bytes stay untouched on the payload
// (the toggle returns the byte-identical original — 017 FR-025).
func TestPrettyJSON(t *testing.T) {
	raw := []byte(`{"name":"s3s","tags":["tui","s3"]}`)
	p := Build("x.json", "application/json", raw, false)
	pretty, ok := Pretty(p)
	if !ok {
		t.Fatal("Pretty must succeed on valid JSON")
	}
	want := "{\n  \"name\": \"s3s\",\n  \"tags\": [\n    \"tui\",\n    \"s3\"\n  ]\n}"
	if pretty != want {
		t.Errorf("pretty:\n%s\nwant:\n%s", pretty, want)
	}
	if string(p.Data) != string(raw) {
		t.Error("Payload.Data must stay byte-identical (raw toggle source)")
	}
}

// TestPrettyNDJSON: each record pretty-printed in order (017 FR-025).
func TestPrettyNDJSON(t *testing.T) {
	p := Build("x.ndjson", "application/x-ndjson", []byte("{\"a\":1}\n{\"b\":2}\n"), false)
	pretty, ok := Pretty(p)
	if !ok {
		t.Fatal("Pretty must succeed on NDJSON")
	}
	ia, ib := strings.Index(pretty, `"a"`), strings.Index(pretty, `"b"`)
	if ia < 0 || ib < 0 || ia > ib {
		t.Errorf("records must keep order:\n%s", pretty)
	}
	if !strings.Contains(pretty, "  \"a\": 1") {
		t.Errorf("records must be indented:\n%s", pretty)
	}
}

// TestPrettyNonJSONDeclines: Pretty reports !ok for non-JSON kinds — the caller falls
// back to the raw render with no error banner (017 FR-025).
func TestPrettyNonJSONDeclines(t *testing.T) {
	p := Build("a.txt", "text/plain", []byte("plain"), false)
	if _, ok := Pretty(p); ok {
		t.Error("Pretty must decline non-JSON payloads")
	}
}
