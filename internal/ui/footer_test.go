package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/danchupin/s3s/internal/storage"
)

func TestBucketFilter(t *testing.T) {
	f := storage.NewFake()
	f.Seed("assets")
	f.Seed("archive")
	f.Seed("backups")
	m := withBuckets(f, []string{"ctx"}, nil)

	m = press(m, "/")
	if !m.searching {
		t.Fatal("'/' should open the bucket filter")
	}
	m = press(m, "a") // filter "a" → assets, archive, backups (all contain 'a')
	if len(m.filteredBuckets()) != 3 {
		t.Fatalf("filter 'a' = %d buckets, want 3", len(m.filteredBuckets()))
	}
	m = press(m, "r") // "ar" → only archive contains "ar"
	got := m.filteredBuckets()
	if len(got) != 1 || got[0].Name != "archive" {
		t.Fatalf("filter 'ar' = %+v, want [archive]", got)
	}

	// Esc clears the filter, restoring all buckets.
	m = press(m, "esc")
	if m.bucketFilter != "" || len(m.filteredBuckets()) != 3 {
		t.Fatalf("esc should clear filter; got %q / %d", m.bucketFilter, len(m.filteredBuckets()))
	}
}

func TestDigitContextSwitch(t *testing.T) {
	f := storage.NewFake()
	f.Seed("a")
	other := storage.NewFake()
	other.Seed("b")
	resolve := func(name string) (Backend, error) {
		if name == "two" {
			return Backend{Store: other}, nil
		}
		return Backend{Store: f}, nil
	}
	m := withBuckets(f, []string{"one", "two", "three"}, resolve)
	m.ctxName = "one"

	m = press(m, "2") // jump to second context
	if m.ctxName != "two" {
		t.Fatalf("digit '2' should switch to second context, got %q", m.ctxName)
	}
}

func TestFooterFitsWidthAndShowsHints(t *testing.T) {
	f := storage.NewFake()
	f.Seed("hot")
	m := New(Backend{Store: f, Cluster: "c", User: "u",
		Endpoint: "https://very-long-endpoint.example.storage.internal:9000", Region: "us-east-1"},
		"my-context", []string{"my-context"}, nil, 0)
	m.width, m.height = 60, 16
	m = deliver(m, bucketsMsg{gen: m.gen, buckets: []storage.Bucket{{Name: "hot"}}})

	v := m.View().Content
	for _, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line exceeds width 60 (=%d): %q", w, line)
		}
	}
	if !strings.Contains(v, "quit") || !strings.Contains(v, "filter") {
		t.Errorf("footer hints missing from view:\n%s", v)
	}
}

func TestBoxLongTitleNoOverflow(t *testing.T) {
	f := storage.NewFake()
	m := withBuckets(f, []string{"ctx"}, nil)
	m.width, m.height = 40, 14
	m.mode = modeObject
	longKey := strings.Repeat("verylongsegment/", 12) + "file.bin"
	md := storage.ObjectMetadata{Key: longKey, ContentType: "application/octet-stream"}
	m.meta = &md

	v := m.View().Content
	for _, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w > 40 {
			t.Errorf("long-title line exceeds width 40 (=%d)", w)
		}
	}
}
