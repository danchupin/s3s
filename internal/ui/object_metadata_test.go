package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

// TestMetaFieldRowsEnrichedOmitAndGated drives the SHARED metaFieldRows directly —
// the single source consumed by BOTH the Enter object view and the focus pane (US1).
// Optional fields render only when set; the permission-gated lock fields always render
// ("unknown" when absent). FR-001/FR-002/FR-003.
func TestMetaFieldRowsEnrichedOmitAndGated(t *testing.T) {
	md := storage.ObjectMetadata{
		Key: "k", ContentType: "text/plain", StorageClass: "GLACIER", ETag: `"e"`,
		VersionID: "v1", SSEAlgorithm: "aws:kms", SSEKMSKeyID: "key-123",
		ObjectLockMode: "GOVERNANCE", // ObjectLockLegalHold + ReplicationStatus left empty
	}
	out := metaFieldRows(md, 80)
	for _, want := range []string{"Version", "v1", "Encryption", "aws:kms", "KMS key", "key-123", "Lock", "GOVERNANCE", "Legal hold", "unknown"} {
		if !strings.Contains(out, want) {
			t.Errorf("metaFieldRows missing %q:\n%s", want, out)
		}
	}
	for _, no := range []string{"Replication", "Restore", "Retain until", "Expires", "Encoding", "Cache", "Disposition", "Delete marker"} {
		if strings.Contains(out, no) {
			t.Errorf("omit-empty failed: unexpected %q present:\n%s", no, out)
		}
	}
}

// A plain object: optional fields omitted; the two gated lock fields ALWAYS render as
// "unknown" (absence is information, FR-003).
func TestMetaFieldRowsPlainGatedUnknown(t *testing.T) {
	md := storage.ObjectMetadata{Key: "k", ContentType: "text/plain", StorageClass: "STANDARD"}
	out := metaFieldRows(md, 80)
	if !strings.Contains(out, "Lock") || !strings.Contains(out, "Legal hold") || strings.Count(out, "unknown") < 2 {
		t.Errorf("plain object must always show gated Lock + Legal hold as unknown:\n%s", out)
	}
	for _, no := range []string{"Version", "Encryption", "KMS key", "Replication"} {
		if strings.Contains(out, no) {
			t.Errorf("plain object must omit optional %q:\n%s", no, out)
		}
	}
}

// TestMetaPaneShowsEnriched confirms the wiring: the Enter object view (metaPane)
// surfaces the enriched fields delivered via metadataMsg.
func TestMetaPaneShowsEnriched(t *testing.T) {
	f := storage.NewFake()
	f.SeedObject("b", "file.txt", storage.FakeObject{Data: []byte("hi")})
	m := enterTree(t, f, "b")
	m = press(m, "enter")
	md := storage.ObjectMetadata{
		Key: "file.txt", Size: 2, LastModified: time.Unix(1700000000, 0),
		ContentType: "text/plain", StorageClass: "STANDARD", ETag: `"e"`,
		VersionID: "v9", SSEAlgorithm: "AES256",
	}
	m = deliver(m, metadataMsg{gen: m.gen, md: md})
	v := viewOf(m)
	for _, want := range []string{"Version", "v9", "Encryption", "AES256"} {
		if !strings.Contains(v, want) {
			t.Errorf("Enter view missing enriched %q:\n%s", want, v)
		}
	}
}
