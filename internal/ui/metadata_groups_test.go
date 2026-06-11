package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

var testNow = time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

// enrichedMeta returns an object populating every group (017 US2 fixtures).
func enrichedMeta() storage.ObjectMetadata {
	return storage.ObjectMetadata{
		Key:          "docs/report.pdf",
		Size:         2048,
		LastModified: testNow.Add(-3 * 24 * time.Hour),
		ContentType:  "application/pdf",
		StorageClass: "STANDARD",
		ETag:         "d41d8cd98f00b204e9800998ecf8427e",

		VersionID:           "v1abc",
		SSEAlgorithm:        "aws:kms",
		SSEKMSKeyID:         "arn:aws:kms:eu-west-1:123456789012:key/abcdef-1234",
		ReplicationStatus:   "COMPLETE",
		ObjectLockMode:      "GOVERNANCE",
		ObjectLockLegalHold: "OFF",
		ContentEncoding:     "gzip",
		CacheControl:        "max-age=3600",
	}
}

// TestMetaGroupHeadersOrder: the pane renders named groups in stable order and omits a
// group with no populated fields (017 US2/FR-008).
func TestMetaGroupHeadersOrder(t *testing.T) {
	v := stripANSI(metaFieldRows(enrichedMeta(), 80, testNow))
	iID := strings.Index(v, "Identity")
	iSec := strings.Index(v, "Security & governance")
	iDel := strings.Index(v, "Delivery")
	if iID < 0 || iSec < 0 || iDel < 0 {
		t.Fatalf("missing group headers (id=%d sec=%d del=%d):\n%s", iID, iSec, iDel, v)
	}
	if iID >= iSec || iSec >= iDel {
		t.Errorf("group order wrong (id=%d sec=%d del=%d)", iID, iSec, iDel)
	}

	// No delivery fields → the Delivery header disappears entirely.
	md := enrichedMeta()
	md.ContentEncoding, md.CacheControl, md.ContentDisposition, md.LifecycleExpiration = "", "", "", ""
	if v := stripANSI(metaFieldRows(md, 80, testNow)); strings.Contains(v, "Delivery") {
		t.Errorf("empty Delivery group must be omitted:\n%s", v)
	}
}

// TestMetaFieldStatesDistinctByText: unknown (gated header absent) ≠ "—" (core unset) —
// the distinction is carried by TEXT, so it survives NO_COLOR (017 US2/FR-009).
func TestMetaFieldStatesDistinctByText(t *testing.T) {
	md := enrichedMeta()
	md.ObjectLockMode = "" // gated + absent → "unknown"
	md.ContentType = ""    // core unset → "—"
	md.SSEAlgorithm = ""   // optional unset → row omitted
	v := stripANSI(metaFieldRows(md, 80, testNow))

	if !strings.Contains(v, "unknown") {
		t.Errorf("gated absent field must read 'unknown':\n%s", v)
	}
	if !strings.Contains(v, "—") {
		t.Errorf("core unset field must read '—':\n%s", v)
	}
	if strings.Contains(v, "Encryption") {
		t.Errorf("optional unset field must be omitted entirely:\n%s", v)
	}
	// denied / unsupported are carried by the existing tri-state labels — text-distinct
	// from both 'unknown' and '—' (shared vocabulary, constitution VII).
	if l := configLabel(storage.ConfigItem{State: storage.ConfigDenied}); !strings.Contains(l, "denied") {
		t.Errorf("denied label = %q", l)
	}
	if l := configLabel(storage.ConfigItem{State: storage.ConfigUnsupported}); l != "unsupported" {
		t.Errorf("unsupported label = %q", l)
	}
}

// TestMultipartETagAnnotation: a multipart ETag (32hex-N) is explained; a plain ETag is
// left alone (017 US2/FR-011, research D15).
func TestMultipartETagAnnotation(t *testing.T) {
	md := enrichedMeta()
	md.ETag = `"9bb58f26192e4ba00f01e2e7b136bbd8-7"`
	v := stripANSI(metaFieldRows(md, 120, testNow))
	if !strings.Contains(v, "multipart, 7 parts") || !strings.Contains(v, "not a content hash") {
		t.Errorf("multipart ETag must carry the parts annotation:\n%s", v)
	}

	plain := enrichedMeta() // 32-hex ETag, no suffix
	if v := stripANSI(metaFieldRows(plain, 120, testNow)); strings.Contains(v, "multipart") {
		t.Errorf("plain ETag must not be annotated:\n%s", v)
	}
}

// TestModifiedDualDate: the Modified row carries BOTH the relative and the exact form
// (017 US2/FR-010).
func TestModifiedDualDate(t *testing.T) {
	v := stripANSI(metaFieldRows(enrichedMeta(), 100, testNow))
	if !strings.Contains(v, "3d ago") {
		t.Errorf("Modified must carry the relative form:\n%s", v)
	}
	if !strings.Contains(v, "2026-06-08") {
		t.Errorf("Modified must keep the exact timestamp:\n%s", v)
	}
}
