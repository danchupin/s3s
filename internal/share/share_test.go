package share

import (
	"strings"
	"testing"
)

// TestS3URI: canonical URIs; prefixes keep their trailing slash (017 US3/FR-014).
func TestS3URI(t *testing.T) {
	cases := []struct{ bucket, key, want string }{
		{"b", "docs/report.pdf", "s3://b/docs/report.pdf"},
		{"b", "docs/", "s3://b/docs/"},
		{"b", "", "s3://b/"},
	}
	for _, c := range cases {
		if got := S3URI(c.bucket, c.key); got != c.want {
			t.Errorf("S3URI(%q,%q) = %q, want %q", c.bucket, c.key, got, c.want)
		}
	}
}

// TestHTTPURLStyles: path-style vs virtual-host-style derive from the context's
// pathStyle flag (017 US3/FR-014, research D12).
func TestHTTPURLStyles(t *testing.T) {
	if got := HTTPURL("https://s3.example.com:9000", true, "b", "k.txt"); got != "https://s3.example.com:9000/b/k.txt" {
		t.Errorf("path-style = %q", got)
	}
	if got := HTTPURL("https://s3.example.com", false, "b", "k.txt"); got != "https://b.s3.example.com/k.txt" {
		t.Errorf("vhost-style = %q", got)
	}
}

// TestHTTPURLEscaping: keys with spaces, '+', unicode and '?' are percent-escaped so the
// copied URL actually works (017 US3, table-driven per contract).
func TestHTTPURLEscaping(t *testing.T) {
	cases := []struct{ key, wantSub string }{
		{"a b.txt", "a%20b.txt"},
		{"c+d.txt", "c%2Bd.txt"},
		{"данные.bin", "%D0%B4%D0%B0%D0%BD%D0%BD%D1%8B%D0%B5.bin"},
		{"q?.txt", "q%3F.txt"},
		{"dir/файл x.log", "dir/%D1%84%D0%B0%D0%B9%D0%BB%20x.log"}, // "/" stays a separator
	}
	for _, c := range cases {
		got := HTTPURL("https://h", true, "b", c.key)
		if !strings.Contains(got, c.wantSub) {
			t.Errorf("HTTPURL key %q = %q, want substring %q", c.key, got, c.wantSub)
		}
	}
}

// TestCLISnippet: a ready-to-run aws s3api download command carrying the endpoint.
func TestCLISnippet(t *testing.T) {
	got := CLISnippet("https://s3.example.com", "b", "docs/report.pdf")
	for _, want := range []string{"aws s3api get-object", "--endpoint-url https://s3.example.com", "--bucket b", "--key 'docs/report.pdf'", "report.pdf"} {
		if !strings.Contains(got, want) {
			t.Errorf("CLISnippet missing %q:\n%s", want, got)
		}
	}
}

// TestCurlSnippet: curl against a presigned URL, output named after the key's base.
func TestCurlSnippet(t *testing.T) {
	got := CurlSnippet("https://h/b/k.bin?X-Amz-Signature=abc", "dir/k.bin")
	if !strings.Contains(got, "curl") || !strings.Contains(got, "'https://h/b/k.bin?X-Amz-Signature=abc'") || !strings.Contains(got, "k.bin") {
		t.Errorf("CurlSnippet = %q", got)
	}
}
