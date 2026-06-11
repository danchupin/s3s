package share

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

// exportDist is one labelled distribution bucket in the JSON form (stable names —
// contracts/copy-share-menu.md schema).
type exportDist struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Bytes int64  `json:"bytes"`
}

// exportReport is the stable JSON schema of a usage report (017 US3/FR-018). bounded
// marks lower-bound (partial) data so a consumer can never mistake it for exact.
type exportReport struct {
	Bucket     string       `json:"bucket"`
	Prefix     string       `json:"prefix"`
	TotalCount int          `json:"totalCount"`
	TotalBytes int64        `json:"totalBytes"`
	Bounded    bool         `json:"bounded"`
	ScanStart  time.Time    `json:"scanStart"`
	Age        []exportDist `json:"age"`
	Size       []exportDist `json:"size"`
	Classes    []exportDist `json:"classes"`
	Children   []exportDist `json:"children"`
}

func buildExport(rep *storage.UsageReport) exportReport {
	out := exportReport{
		Bucket: rep.Bucket, Prefix: rep.Prefix,
		TotalCount: rep.TotalCount, TotalBytes: rep.TotalSize,
		Bounded: !rep.Complete, ScanStart: rep.ScanStart,
	}
	for i, b := range rep.AgeDist {
		out.Age = append(out.Age, exportDist{Label: storage.AgeBucketLabels[i], Count: b.Count, Bytes: b.Size})
	}
	for i, b := range rep.SizeDist {
		out.Size = append(out.Size, exportDist{Label: storage.SizeBucketLabels[i], Count: b.Count, Bytes: b.Size})
	}
	for _, class := range sortedClassKeys(rep.ClassDist) {
		b := rep.ClassDist[class]
		out.Classes = append(out.Classes, exportDist{Label: class, Count: b.Count, Bytes: b.Size})
	}
	for _, c := range rep.Children {
		out.Children = append(out.Children, exportDist{Label: c.Name, Count: c.Count, Bytes: c.Size})
	}
	return out
}

// ExportJSON serializes the report with stable field names; bounded carries
// partial-ness (017 US3/FR-018).
func ExportJSON(rep *storage.UsageReport) []byte {
	out, _ := json.MarshalIndent(buildExport(rep), "", "  ") // struct of plain types — cannot fail
	return out
}

// ExportCSV serializes the report as `section,label,count,bytes,bounded` rows: the
// total, every distribution bucket, every class, every ranked child.
func ExportCSV(rep *storage.UsageReport) []byte {
	e := buildExport(rep)
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"section", "label", "count", "bytes", "bounded"})
	bounded := strconv.FormatBool(e.Bounded)
	row := func(section string, d exportDist) {
		_ = w.Write([]string{section, d.Label, strconv.Itoa(d.Count), strconv.FormatInt(d.Bytes, 10), bounded})
	}
	label := e.Prefix
	if label == "" {
		label = e.Bucket
	}
	row("total", exportDist{Label: label, Count: e.TotalCount, Bytes: e.TotalBytes})
	for _, d := range e.Age {
		row("age", d)
	}
	for _, d := range e.Size {
		row("size", d)
	}
	for _, d := range e.Classes {
		row("class", d)
	}
	for _, d := range e.Children {
		row("child", d)
	}
	w.Flush()
	return buf.Bytes()
}

// ReportFileName builds the deterministic export file name:
// s3s-report-<bucket>[-<prefix-slug>]-<YYYYMMDD-HHMMSS>.<format>; the slug replaces "/"
// with "-" and is truncated to 40 chars (research D11).
func ReportFileName(bucket, prefix string, ts time.Time, format string) string {
	slug := strings.Trim(strings.ReplaceAll(prefix, "/", "-"), "-")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	if slug != "" {
		slug = "-" + slug
	}
	return fmt.Sprintf("s3s-report-%s%s-%s.%s", bucket, slug, ts.Format("20060102-150405"), format)
}

// sortedClassKeys returns the class names sorted for stable output.
func sortedClassKeys(m map[string]storage.DistBucket) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
