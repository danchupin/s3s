package storage

import (
	"context"
	"testing"
	"time"
)

// TestHeadObjectEnriched: the Fake (mirroring the real client) maps every 016
// enriched field from the seeded object into ObjectMetadata (US1, FR-001/FR-002).
func TestHeadObjectEnriched(t *testing.T) {
	f := NewFake()
	rt := time.Unix(1700000000, 0)
	f.SeedObject("b", "k", FakeObject{
		Data:                []byte("x"),
		SSEAlgorithm:        "aws:kms",
		SSEKMSKeyID:         "arn:key",
		VersionID:           "v1",
		DeleteMarker:        true,
		ReplicationStatus:   "COMPLETE",
		RestoreStatus:       "in progress",
		ObjectLockMode:      "GOVERNANCE",
		ObjectLockRetainTil: rt,
		ObjectLockLegalHold: "ON",
		LifecycleExpiration: "expiry-date=…",
		ContentEncoding:     "gzip",
		CacheControl:        "no-cache",
		ContentDisposition:  "inline",
	})
	md, err := f.HeadObject(context.Background(), "b", "k")
	if err != nil {
		t.Fatal(err)
	}
	if md.SSEAlgorithm != "aws:kms" || md.SSEKMSKeyID != "arn:key" || md.VersionID != "v1" || !md.DeleteMarker {
		t.Errorf("encryption/version/delete-marker not mapped: %+v", md)
	}
	if md.ObjectLockMode != "GOVERNANCE" || !md.ObjectLockRetainTil.Equal(rt) || md.ObjectLockLegalHold != "ON" {
		t.Errorf("object-lock not mapped: %+v", md)
	}
	if md.ReplicationStatus != "COMPLETE" || md.RestoreStatus != "in progress" {
		t.Errorf("replication/restore not mapped: %+v", md)
	}
	if md.ContentEncoding != "gzip" || md.CacheControl != "no-cache" || md.ContentDisposition != "inline" || md.LifecycleExpiration != "expiry-date=…" {
		t.Errorf("content-handling/lifecycle not mapped: %+v", md)
	}
}

func TestParseRestore(t *testing.T) {
	cases := map[string]string{
		"":                       "",
		`ongoing-request="true"`: "in progress",
		`ongoing-request="false", expiry-date="2026"`: "restored until 2026",
		`ongoing-request="false"`:                     "restored",
		"garbage":                                     "restored",
	}
	for in, want := range cases {
		if got := parseRestore(in); got != want {
			t.Errorf("parseRestore(%q) = %q, want %q", in, got, want)
		}
	}
}
