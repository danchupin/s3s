// Package preview classifies and renders bounded object content for the preview
// pane: scrollable text, visual images, and a safe summary for binary. It has NO
// S3 dependency — callers fetch the bounded bytes and hand them in (FR-014/015/016).
package preview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

// Limit is the maximum number of bytes fetched for a preview (FR-014/016).
const Limit int64 = 5 * 1024 * 1024

// Kind classifies preview content for the renderer.
type Kind int

const (
	// KindBinary is non-text, non-image content — shown as a hex dump.
	KindBinary Kind = iota
	// KindText is renderable as scrollable text.
	KindText
	// KindImage is renderable visually (half-block or graphics protocol).
	KindImage
	// KindJSON is ONE parseable JSON value — pretty-printable (017 US5).
	KindJSON
	// KindNDJSON is ≥2 newline-delimited parseable JSON values (017 US5).
	KindNDJSON
)

func (k Kind) String() string {
	switch k {
	case KindText:
		return "text"
	case KindImage:
		return "image"
	case KindJSON:
		return "json"
	case KindNDJSON:
		return "ndjson"
	default:
		return "binary"
	}
}

// Payload is bounded content ready for the preview pane (data-model PreviewPayload).
type Payload struct {
	Key         string
	ContentType string
	Data        []byte
	Truncated   bool // true if the object exceeds Limit (or a capped decompression)
	Kind        Kind
	// Compressed is set when Data was transparently gunzipped (017 US5/FR-026).
	Compressed *Compressed
}

// textualTypes are MIME types treated as text beyond the text/* family.
var textualTypes = map[string]bool{
	"application/json":         true,
	"application/xml":          true,
	"application/javascript":   true,
	"application/x-javascript": true,
	"application/yaml":         true,
	"application/x-yaml":       true,
	"application/x-sh":         true,
	"application/toml":         true,
	"application/x-ndjson":     true,
	"application/ld+json":      true,
}

// normalizeCT strips parameters and lowercases a content type.
func normalizeCT(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// Classify decides the Kind from the content type and (when ambiguous) the bytes.
// An empty content type is sniffed via http.DetectContentType. A NUL byte forces
// binary. octet-stream falls back to a UTF-8/printability check.
func Classify(contentType string, data []byte) Kind {
	ct := normalizeCT(contentType)
	if ct == "" || ct == "application/octet-stream" {
		ct = normalizeCT(http.DetectContentType(data))
	}

	if strings.HasPrefix(ct, "image/") {
		return KindImage
	}
	if strings.HasPrefix(ct, "text/") || textualTypes[ct] {
		if looksBinary(data) {
			return KindBinary
		}
		return jsonKind(data)
	}
	// Unknown/octet-stream: sniff the bytes.
	if len(data) > 0 && !looksBinary(data) {
		return jsonKind(data)
	}
	return KindBinary
}

// jsonKind upgrades textual content to KindJSON (one parseable value) or KindNDJSON
// (≥2 newline-delimited parseable values); anything else stays plain text — a
// truncated/broken payload silently falls back to the raw render (017 FR-025).
func jsonKind(data []byte) Kind {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return KindText
	}
	if json.Valid(trimmed) {
		return KindJSON
	}
	lines := nonEmptyLines(trimmed)
	if len(lines) < 2 {
		return KindText
	}
	for _, l := range lines {
		if !json.Valid(l) {
			return KindText
		}
	}
	return KindNDJSON
}

// nonEmptyLines splits data on newlines, dropping blank lines.
func nonEmptyLines(data []byte) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

// Pretty renders a JSON/NDJSON payload indented (2 spaces); ok=false for any other
// kind or a parse failure — the caller falls back to the raw bytes with no error
// banner (017 US5/FR-025). Payload.Data is never modified (the raw-toggle source).
func Pretty(p Payload) (string, bool) {
	switch p.Kind {
	case KindJSON:
		var buf bytes.Buffer
		if err := json.Indent(&buf, bytes.TrimSpace(p.Data), "", "  "); err != nil {
			return "", false
		}
		return buf.String(), true
	case KindNDJSON:
		var b strings.Builder
		for _, l := range nonEmptyLines(p.Data) {
			var buf bytes.Buffer
			if err := json.Indent(&buf, l, "", "  "); err != nil {
				return "", false
			}
			b.WriteString(buf.String())
			b.WriteByte('\n')
		}
		return strings.TrimRight(b.String(), "\n"), true
	}
	return "", false
}

// looksBinary reports whether data contains a NUL byte or is not valid UTF-8.
func looksBinary(data []byte) bool {
	if slices.Contains(data, 0) {
		return true
	}
	return !utf8.Valid(data)
}

// Build assembles a Payload, classifying the content. A gzip payload (magic bytes —
// the primary signal; the .gz name and Content-Encoding merely corroborate) is
// transparently decompressed with the output capped at Limit, and the decompressed
// bytes re-enter classification so a gzipped JSON pretty-prints (017 US5/FR-026). A
// failed decode falls back to the raw bytes silently.
func Build(key, contentType string, data []byte, truncated bool) Payload {
	if isGzip(data) {
		if out, capped, err := gunzipCapped(data, Limit); err == nil {
			return Payload{
				Key:         key,
				ContentType: contentType,
				Data:        out,
				Truncated:   truncated || capped,
				Kind:        Classify("", out), // re-classify the decompressed bytes
				Compressed:  &Compressed{From: int64(len(data)), Truncated: capped},
			}
		}
	}
	return Payload{
		Key:         key,
		ContentType: contentType,
		Data:        data,
		Truncated:   truncated,
		Kind:        Classify(contentType, data),
	}
}

// Summary returns a safe one-line description for binary (or any) content — never
// dumps raw bytes to the terminal (FR-015).
func Summary(p Payload) string {
	ct := p.ContentType
	if ct == "" {
		ct = http.DetectContentType(p.Data)
	}
	s := fmt.Sprintf("%s content — %d bytes, type %s", p.Kind, len(p.Data), ct)
	if p.Truncated {
		s += " (preview truncated at 5 MiB)"
	}
	return s
}
