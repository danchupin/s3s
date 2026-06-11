package preview

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func gz(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestGzipTransparentDecode: a gzipped text payload decompresses for preview, carrying
// the compressed→shown metadata (017 US5/FR-026, research D13).
func TestGzipTransparentDecode(t *testing.T) {
	plain := []byte("hello log line one\nhello log line two\n")
	p := Build("logs/app.log.gz", "application/octet-stream", gz(t, plain), false)

	if !bytes.Equal(p.Data, plain) {
		t.Errorf("decompressed data = %q, want %q", p.Data, plain)
	}
	if p.Kind != KindText {
		t.Errorf("Kind = %v, want text after re-classify", p.Kind)
	}
	if p.Compressed == nil {
		t.Fatal("Payload must carry the Compressed metadata")
	}
	if p.Compressed.From <= 0 || p.Compressed.Truncated {
		t.Errorf("Compressed = %+v, want From>0 Truncated=false", p.Compressed)
	}
}

// TestGzipBombCapped: a high-ratio payload is capped at Limit on OUTPUT —
// compression-bomb-safe by construction (017 FR-026).
func TestGzipBombCapped(t *testing.T) {
	big := bytes.Repeat([]byte("A"), int(Limit)+4096) // expands past the cap
	p := Build("bomb.gz", "", gz(t, big), false)

	if int64(len(p.Data)) > Limit {
		t.Fatalf("decompressed %d bytes, must cap at %d", len(p.Data), Limit)
	}
	if p.Compressed == nil || !p.Compressed.Truncated {
		t.Errorf("Compressed = %+v, want Truncated=true at the cap", p.Compressed)
	}
	if !p.Truncated {
		t.Error("Payload.Truncated must mark the capped decompression")
	}
}

// TestGzipBadMagicFallsBack: a .gz-named payload without the magic stays raw — no error
// surface (017 FR-026 silent fallback).
func TestGzipBadMagicFallsBack(t *testing.T) {
	raw := []byte("not actually gzip")
	p := Build("fake.gz", "", raw, false)
	if !bytes.Equal(p.Data, raw) || p.Compressed != nil {
		t.Errorf("non-gzip payload must pass through untouched: %+v", p)
	}
}

// TestGzipCorruptStreamFallsBack: magic present but the stream is corrupt → raw bytes,
// silently (017 FR-026).
func TestGzipCorruptStreamFallsBack(t *testing.T) {
	corrupt := append([]byte{0x1f, 0x8b}, []byte("garbage-after-magic")...)
	p := Build("broken.gz", "", corrupt, false)
	if !bytes.Equal(p.Data, corrupt) || p.Compressed != nil {
		t.Errorf("corrupt gzip must fall back to the raw bytes: %+v", p)
	}
}

// TestGzippedJSONPrettyPath: a gzipped JSON object re-classifies to KindJSON — the
// decompressed bytes re-enter Classify (017 FR-026 + FR-025).
func TestGzippedJSONPrettyPath(t *testing.T) {
	p := Build("data.json.gz", "", gz(t, []byte(`{"a":1,"b":[2,3]}`)), false)
	if p.Kind != KindJSON {
		t.Errorf("Kind = %v, want KindJSON after decompress + re-classify", p.Kind)
	}
	pretty, ok := Pretty(p)
	if !ok || !strings.Contains(pretty, "\n") {
		t.Errorf("gzipped JSON must pretty-print: ok=%v\n%s", ok, pretty)
	}
}
