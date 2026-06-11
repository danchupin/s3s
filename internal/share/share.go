// Package share builds copyable artifacts from browse state: canonical URIs,
// addressing-style-aware HTTPS URLs, ready-to-run command snippets, and CSV/JSON report
// serializations (017 US3). Pure functions over already-validated inputs — no SDK, no
// TUI dependency (constitution I) — so a future plain CLI can reuse them verbatim.
package share

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// S3URI renders the canonical storage URI; a prefix keeps its trailing "/".
func S3URI(bucket, key string) string {
	return "s3://" + bucket + "/" + key
}

// escapeKey percent-escapes an object key per RFC 3986 (the SigV4 canonical-URI rules:
// everything but unreserved characters), keeping "/" as the separator — so the copied
// URL works for spaces, '+', unicode and '?' alike. url.PathEscape is NOT enough: it
// leaves '+' bare, which some backends decode as a space.
func escapeKey(key string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	var b strings.Builder
	for _, c := range []byte(key) {
		if c == '/' || strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// HTTPURL renders the direct HTTPS URL for (bucket, key) honoring the endpoint's
// addressing style: path-style `https://host/bucket/key` or virtual-host-style
// `https://bucket.host/key` (017 US3/FR-014, research D12).
func HTTPURL(endpoint string, pathStyle bool, bucket, key string) string {
	ep := strings.TrimRight(endpoint, "/")
	if pathStyle {
		return ep + "/" + bucket + "/" + escapeKey(key)
	}
	u, err := url.Parse(ep)
	if err != nil || u.Host == "" {
		return ep + "/" + bucket + "/" + escapeKey(key) // degenerate endpoint — fall back
	}
	u.Host = bucket + "." + u.Host
	return u.Scheme + "://" + u.Host + "/" + escapeKey(key)
}

// CLISnippet renders a ready-to-run aws-cli download command for the object.
func CLISnippet(endpoint, bucket, key string) string {
	return fmt.Sprintf("aws s3api get-object --endpoint-url %s --bucket %s --key '%s' '%s'",
		endpoint, bucket, key, path.Base(key))
}

// CurlSnippet renders a curl command fetching a presigned URL into the key's base name.
func CurlSnippet(presignedURL, key string) string {
	return fmt.Sprintf("curl -fLo '%s' '%s'", path.Base(key), presignedURL)
}
