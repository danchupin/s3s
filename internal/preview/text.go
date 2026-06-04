// Package preview classifies and renders bounded object content for the preview
// pane: scrollable text, visual images, and a safe summary for binary. It has NO
// S3 dependency — callers fetch the bounded bytes and hand them in (FR-014/015/016).
package preview

import (
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
	// KindBinary is non-text, non-image content — shown as a safe summary.
	KindBinary Kind = iota
	// KindText is renderable as scrollable text.
	KindText
	// KindImage is renderable visually (half-block or graphics protocol).
	KindImage
)

func (k Kind) String() string {
	switch k {
	case KindText:
		return "text"
	case KindImage:
		return "image"
	default:
		return "binary"
	}
}

// Payload is bounded content ready for the preview pane (data-model PreviewPayload).
type Payload struct {
	Key         string
	ContentType string
	Data        []byte
	Truncated   bool // true if the object exceeds Limit
	Kind        Kind
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
		return KindText
	}
	// Unknown/octet-stream: sniff the bytes.
	if len(data) > 0 && !looksBinary(data) {
		return KindText
	}
	return KindBinary
}

// looksBinary reports whether data contains a NUL byte or is not valid UTF-8.
func looksBinary(data []byte) bool {
	if slices.Contains(data, 0) {
		return true
	}
	return !utf8.Valid(data)
}

// Build assembles a Payload, classifying the content.
func Build(key, contentType string, data []byte, truncated bool) Payload {
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
