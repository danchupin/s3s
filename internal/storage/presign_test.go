package storage

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestPresignGetTTLPresetsOnly: exactly 15m/1h/24h/7d are accepted; anything else is
// ErrInvalidConfig (017 US3/FR-015 — defense in depth under the UI picker).
func TestPresignGetTTLPresetsOnly(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "k.txt", FakeObject{Data: []byte("x")})

	for _, ttl := range []time.Duration{15 * time.Minute, time.Hour, 24 * time.Hour, 7 * 24 * time.Hour} {
		if _, _, err := f.PresignGet(context.Background(), "b", "k.txt", ttl); err != nil {
			t.Errorf("PresignGet(ttl=%v) = %v, want ok", ttl, err)
		}
	}
	for _, ttl := range []time.Duration{0, time.Minute, 8 * 24 * time.Hour, -time.Hour} {
		if _, _, err := f.PresignGet(context.Background(), "b", "k.txt", ttl); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("PresignGet(ttl=%v) = %v, want ErrInvalidConfig", ttl, err)
		}
	}
}

// TestPresignGetFakeShape: the Fake yields a deterministic URL embedding bucket/key/ttl
// and performs ZERO backend calls (presign is client-side — 017 FR-015).
func TestPresignGetFakeShape(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "dir/k.txt", FakeObject{Data: []byte("x")})

	u, warn, err := f.PresignGet(context.Background(), "b", "dir/k.txt", time.Hour)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if !strings.Contains(u, "b") || !strings.Contains(u, "dir/k.txt") || !strings.Contains(u, "3600") {
		t.Errorf("fake URL = %q, want bucket/key/ttl embedded", u)
	}
	if warn != "" {
		t.Errorf("warn = %q, want empty without expiring creds", warn)
	}
	if f.ListBucketsCalls != 0 || f.ListLevelCalls != 0 || f.UsagePages != 0 {
		t.Error("presign must record zero backend calls")
	}
}

// TestPresignGetCredExpiryWarn: credentials expiring before now+ttl yield a non-empty
// warning (017 US3/FR-017).
func TestPresignGetCredExpiryWarn(t *testing.T) {
	f := NewFake()
	f.SeedObject("b", "k.txt", FakeObject{Data: []byte("x")})
	f.PresignCredsExpireIn = 30 * time.Minute

	_, warn, err := f.PresignGet(context.Background(), "b", "k.txt", 24*time.Hour)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if warn == "" {
		t.Error("creds expiring before the link's ttl must yield a warning")
	}
	if _, warn2, _ := f.PresignGet(context.Background(), "b", "k.txt", 15*time.Minute); warn2 != "" {
		t.Errorf("ttl inside the creds' lifetime must not warn, got %q", warn2)
	}
}

// TestPresignGetRealClientOffline: the REAL client signs entirely client-side — a URL
// with SigV4 query params is produced against an unreachable endpoint, proving zero
// network involvement (017 FR-015, research D9).
func TestPresignGetRealClientOffline(t *testing.T) {
	st, err := New(ClientConfig{
		Endpoint:    "http://127.0.0.1:1", // nothing listens — any network call would fail
		Region:      "us-east-1",
		PathStyle:   true,
		AccessKeyID: "AK", SecretKey: "SK",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	u, _, err := st.PresignGet(context.Background(), "b", "dir/k v+1.txt", 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	parsed, perr := url.Parse(u)
	if perr != nil {
		t.Fatalf("URL parse: %v", perr)
	}
	q := parsed.Query()
	if q.Get("X-Amz-Expires") != "900" {
		t.Errorf("X-Amz-Expires = %q, want 900", q.Get("X-Amz-Expires"))
	}
	if q.Get("X-Amz-Signature") == "" {
		t.Error("URL must carry a SigV4 signature")
	}
	if !strings.HasPrefix(u, "http://127.0.0.1:1/b/") {
		t.Errorf("path-style URL expected, got %q", u)
	}
}
