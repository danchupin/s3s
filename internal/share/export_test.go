package share

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/danchupin/s3s/internal/storage"
)

func sampleReport(complete bool) *storage.UsageReport {
	rep := &storage.UsageReport{
		Bucket: "b", Prefix: "logs/",
		TotalSize: 300, TotalCount: 3,
		Children: []storage.UsageChild{
			{Name: "2026/", IsDir: true, Size: 200, Count: 2},
			{Name: "app.log", Size: 100, Count: 1},
		},
		Complete:  complete,
		Bounded:   !complete,
		ScanStart: time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
		ClassDist: map[string]storage.DistBucket{"STANDARD": {Count: 3, Size: 300}},
	}
	rep.AgeDist[0] = storage.DistBucket{Count: 3, Size: 300}
	rep.SizeDist[0] = storage.DistBucket{Count: 3, Size: 300}
	return rep
}

// TestExportCSV: the CSV carries section,label,count,bytes,bounded rows — totals,
// distributions, children — and the bounded flag for partial data (017 US3/FR-018).
func TestExportCSV(t *testing.T) {
	out := string(ExportCSV(sampleReport(false)))
	for _, want := range []string{
		"section,label,count,bytes,bounded",
		"total,logs/,3,300,true",
		"age,<1d,3,300,true",
		"size,<128KiB,3,300,true",
		"class,STANDARD,3,300,true",
		"child,2026/,2,200,true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("CSV missing %q:\n%s", want, out)
		}
	}
	exact := string(ExportCSV(sampleReport(true)))
	if !strings.Contains(exact, "total,logs/,3,300,false") {
		t.Errorf("exact CSV must carry bounded=false:\n%s", exact)
	}
}

// TestExportJSONRoundTrip: the JSON form unmarshals back with stable field names and
// carries the bounded flag (017 US3/FR-018).
func TestExportJSONRoundTrip(t *testing.T) {
	raw := ExportJSON(sampleReport(false))
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("ExportJSON not valid JSON: %v", err)
	}
	if got["bucket"] != "b" || got["prefix"] != "logs/" {
		t.Errorf("identity fields: %v", got)
	}
	if got["bounded"] != true {
		t.Errorf("bounded flag missing/false: %v", got["bounded"])
	}
	if _, ok := got["age"]; !ok {
		t.Errorf("age distribution missing: %v", got)
	}
}

// TestReportFileName: deterministic naming with the prefix slug truncated to 40 chars
// (017 US3, research D11).
func TestReportFileName(t *testing.T) {
	ts := time.Date(2026, 6, 11, 15, 4, 5, 0, time.UTC)
	if got := ReportFileName("b", "", ts, "csv"); got != "s3s-report-b-20260611-150405.csv" {
		t.Errorf("bucket-only name = %q", got)
	}
	got := ReportFileName("b", "logs/2026/app/", ts, "json")
	if !strings.HasPrefix(got, "s3s-report-b-logs-2026-app-") || !strings.HasSuffix(got, ".json") {
		t.Errorf("prefixed name = %q", got)
	}
	long := strings.Repeat("verylongsegment/", 8) // slug >40 chars before truncation
	name := ReportFileName("b", long, ts, "csv")
	if len(name) > len("s3s-report-b-")+40+len("-20260611-150405.csv") {
		t.Errorf("slug not truncated: %q (len %d)", name, len(name))
	}
}
