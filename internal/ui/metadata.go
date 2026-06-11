package ui

import (
	"fmt"
	"regexp"
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

// warnRow renders a label whose value carries a warning state ("unknown" — the gated
// header was absent). Text carries the state; the warn role is the accent (FR-009).
func warnRow(k, v string, w int) string {
	valW := w - metaKeyWidth
	if valW < 1 {
		valW = 1
	}
	return metaKeyStyle.Render(pad(k, metaKeyWidth)) + warnStyle.Render(truncate(v, valW)) + "\n"
}

// metaField is one label/value row of the details surface; warn marks the
// permission-gated "unknown" state. value is the FULL untruncated string — the render
// truncates, the field-copy path does not (017 US2/FR-012).
type metaField struct {
	label string
	value string
	warn  bool
}

// metaGroup is one named section of the details surface (017 US2/FR-008).
type metaGroup struct {
	title  string
	fields []metaField
}

// multipartETag matches the S3 multipart ETag shape `32hex-N` (optionally quoted) and
// captures the part count (research D15).
var multipartETag = regexp.MustCompile(`^"?[0-9a-f]{32}-(\d+)"?$`)

// etagValue annotates a multipart ETag with its part count and the not-a-content-hash
// note; a plain ETag passes through (017 US2/FR-011).
func etagValue(etag string) string {
	if m := multipartETag.FindStringSubmatch(etag); m != nil {
		return fmt.Sprintf("%s (multipart, %s parts — not a content hash)", etag, m[1])
	}
	return etag
}

// relTime renders a coarse relative form of t against now: "3d ago", "in 2mo", "just
// now"; "" for a zero time. Units: m/h/d/mo/y (017 US2/FR-010, research D14).
func relTime(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	future := d < 0
	if future {
		d = -d
	}
	var s string
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		s = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		s = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		s = fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		s = fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	default:
		s = fmt.Sprintf("%dy", int(d.Hours()/(24*365)))
	}
	if future {
		return "in " + s
	}
	return s + " ago"
}

// dualDate renders "3d ago · 2026-06-08 14:02" — relative AND exact (FR-010).
func dualDate(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return relTime(now, t) + " · " + formatDate(t)
}

// metaGroups assembles the grouped field rows for an object (017 US2/FR-008): named,
// ordered groups; optional unset fields are omitted; a group with nothing to show is
// dropped entirely. The permission-gated lock fields ALWAYS render — "unknown" when the
// header is absent (absence is information) — so Security & governance stays visible.
// The single source for BOTH the rendered pane (metaFieldRows) and the field-copy rows
// (collectFieldRows) — they cannot drift.
func metaGroups(md storage.ObjectMetadata, now time.Time) []metaGroup {
	opt := func(fields *[]metaField, label, value string) {
		if value != "" {
			*fields = append(*fields, metaField{label: label, value: value})
		}
	}
	gated := func(fields *[]metaField, label, value string) {
		if value == "" {
			*fields = append(*fields, metaField{label: label, value: "unknown", warn: true})
			return
		}
		*fields = append(*fields, metaField{label: label, value: value})
	}

	identity := []metaField{
		{label: "Key", value: sanitizeLabel(md.Key)},
		{label: "Size", value: fmt.Sprintf("%s (%d B)", humanSize(md.Size), md.Size)},
		{label: "Modified", value: orDash(dualDate(now, md.LastModified))},
		{label: "Type", value: orDash(md.ContentType)},
		{label: "Class", value: orDash(md.StorageClass)},
		{label: "ETag", value: orDash(etagValue(md.ETag))},
	}
	opt(&identity, "Version", md.VersionID)
	if md.DeleteMarker {
		identity = append(identity, metaField{label: "Delete marker", value: "yes"})
	}

	var security []metaField
	opt(&security, "Encryption", md.SSEAlgorithm)
	opt(&security, "KMS key", md.SSEKMSKeyID)
	gated(&security, "Lock", md.ObjectLockMode)
	opt(&security, "Retain until", dualDate(now, md.ObjectLockRetainTil))
	gated(&security, "Legal hold", md.ObjectLockLegalHold)
	opt(&security, "Replication", md.ReplicationStatus)
	opt(&security, "Restore", md.RestoreStatus)

	var delivery []metaField
	opt(&delivery, "Expires", md.LifecycleExpiration)
	opt(&delivery, "Encoding", md.ContentEncoding)
	opt(&delivery, "Cache", md.CacheControl)
	opt(&delivery, "Disposition", md.ContentDisposition)

	groups := []metaGroup{{title: "Identity & content", fields: identity}}
	if len(security) > 0 {
		groups = append(groups, metaGroup{title: "Security & governance", fields: security})
	}
	if len(delivery) > 0 {
		groups = append(groups, metaGroup{title: "Delivery", fields: delivery})
	}
	return groups
}

// metaFieldRows renders the grouped object field block at width w. The single source of
// truth for both the Enter object view AND the focus details pane (pane.go), so the
// grouped layout surfaces on both paths (016 US1 invariant, 017 US2 regroup). Fields are
// sanitized here; values keep their full form for the reveal/copy paths.
func metaFieldRows(md storage.ObjectMetadata, w int, now time.Time) string {
	var b strings.Builder
	for gi, g := range metaGroups(md, now) {
		if gi > 0 {
			b.WriteString("\n")
		}
		b.WriteString(colHeadStyle.Render(g.title) + "\n")
		for _, f := range g.fields {
			if f.warn {
				b.WriteString(warnRow(f.label, sanitizeLabel(f.value), w))
				continue
			}
			b.WriteString(metaRow(f.label, sanitizeLabel(f.value), w))
		}
	}
	return b.String()
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
	b.WriteString(metaFieldRows(*md, w, m.now()))

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
	// Attributed plugin groups follow every native group (provenance on-screen).
	if pg := m.enrichGroupsView(m.bucket, md.Key, w); pg != "" {
		b.WriteString("\n" + pg + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
