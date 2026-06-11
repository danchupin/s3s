package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

// metaKeyWidth is the fixed label column width in the metadata pane.
const metaKeyWidth = 14

// metaRow renders one fixed-width label + truncated value line. Shared by the full-screen
// object view (metaPane) and the persistent details pane, so both render the
// same field column.
func metaRow(k, v string, w int) string {
	valW := w - metaKeyWidth
	if valW < 1 {
		valW = 1
	}
	return metaKeyStyle.Render(pad(k, metaKeyWidth)) + metaValStyle.Render(truncate(v, valW)) + "\n"
}

// metaFieldRows renders the object field block for the given metadata at width w. The
// single source of truth for both the Enter object view AND the focus details pane
// (pane.go), so enriching it surfaces the new fields on both paths (016 US1, FR-003).
//
// The core block (Key/Size/Modified/Type/Class/ETag) ALWAYS renders. The 016 enriched
// block below it is OMIT-EMPTY for optional fields, so a plain object adds no clutter
// and the pane stays compact. The permission-gated object-lock fields ALWAYS render —
// "unknown" when the header is absent (the caller may simply lack the retention/
// legal-hold read permission), distinct from a configured-but-empty "none".
func metaFieldRows(md storage.ObjectMetadata, w int) string {
	var b strings.Builder
	b.WriteString(metaRow("Key", sanitizeLabel(md.Key), w))
	b.WriteString(metaRow("Size", fmt.Sprintf("%s (%d B)", humanSize(md.Size), md.Size), w))
	b.WriteString(metaRow("Modified", formatDate(md.LastModified), w))
	b.WriteString(metaRow("Type", orDash(md.ContentType), w))
	b.WriteString(metaRow("Class", orDash(md.StorageClass), w))
	b.WriteString(metaRow("ETag", orDash(md.ETag), w))

	b.WriteString(optRow("Version", md.VersionID, w))
	if md.DeleteMarker {
		b.WriteString(metaRow("Delete marker", "yes", w))
	}
	b.WriteString(optRow("Encryption", md.SSEAlgorithm, w))
	b.WriteString(optRow("KMS key", md.SSEKMSKeyID, w))
	b.WriteString(optRow("Replication", md.ReplicationStatus, w))
	b.WriteString(optRow("Restore", md.RestoreStatus, w))
	b.WriteString(gatedRow("Lock", md.ObjectLockMode, w))
	b.WriteString(optRow("Retain until", optTime(md.ObjectLockRetainTil), w))
	b.WriteString(gatedRow("Legal hold", md.ObjectLockLegalHold, w))
	b.WriteString(optRow("Expires", md.LifecycleExpiration, w))
	b.WriteString(optRow("Encoding", md.ContentEncoding, w))
	b.WriteString(optRow("Cache", md.CacheControl, w))
	b.WriteString(optRow("Disposition", md.ContentDisposition, w))
	return b.String()
}

// optRow renders an optional field only when non-empty (omit-empty, FR-003).
func optRow(k, v string, w int) string {
	if v == "" {
		return ""
	}
	return metaRow(k, sanitizeLabel(v), w)
}

// gatedRow renders a permission-gated field ALWAYS: an absent value shows "unknown"
// (absence is information), never the optional "" placeholder (FR-003).
func gatedRow(k, v string, w int) string {
	if v == "" {
		v = "unknown"
	}
	return metaRow(k, sanitizeLabel(v), w)
}

// optTime formats a time for an optional row, or "" (omit) when zero.
func optTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return formatDate(t)
}

// metaPane renders the object metadata block within width w. It is the
// left column of the combined object view; access-denied and other errors surface
// in the footer (errorText).
func (m App) metaPane(w int) string {
	if m.meta == nil {
		if m.loading {
			return dimCellStyle.Render("metadata…")
		}
		if txt := m.errorText(); txt != "" {
			return errStyle.Render(truncate(txt, w))
		}
		return dimCellStyle.Render("(no metadata)")
	}
	md := m.meta
	var b strings.Builder
	b.WriteString(metaFieldRows(*md, w))

	if len(md.UserMetadata) > 0 {
		b.WriteString("\n" + colHeadStyle.Render("User metadata") + "\n")
		keys := make([]string, 0, len(md.UserMetadata))
		for k := range md.UserMetadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(metaRow(sanitizeLabel(k), sanitizeLabel(md.UserMetadata[k]), w))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
