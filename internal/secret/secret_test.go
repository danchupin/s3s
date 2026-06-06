package secret

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestResolveInlineRedacts: an inline secret resolves and stays redacted in String/fmt
// while Reveal returns the real value (005 FR-039, SC-014).
func TestResolveInlineRedacts(t *testing.T) {
	res, err := Resolve(context.Background(), Request{Kind: Inline, AccessKeyID: "AK", Ref: "top-secret"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.AccessKeyID != "AK" || res.SecretKey.Reveal() != "top-secret" {
		t.Errorf("resolved wrong: ak=%q reveal=%q", res.AccessKeyID, res.SecretKey.Reveal())
	}
	if strings.Contains(res.SecretKey.String(), "top-secret") {
		t.Errorf("secret leaked via String(): %q", res.SecretKey.String())
	}
	if strings.Contains(fmt.Sprintf("%v %s", res.SecretKey, res.SecretKey), "top-secret") {
		t.Error("secret leaked via fmt verbs")
	}
}

// TestResolveEmptyInline: an empty inline secret is a clear error, never an empty
// success (005 FR-043).
func TestResolveEmptyInline(t *testing.T) {
	if _, err := Resolve(context.Background(), Request{Kind: Inline, Ref: ""}); err == nil {
		t.Error("empty inline secret should error")
	}
}
