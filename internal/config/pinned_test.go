package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// 010 pinned buckets — config schema round-trip (T002).

const pinnedConfig = `apiVersion: s3s/v1
clusters:
  - name: scoped
    endpoint: https://bucket.avito-sd
    pathStyle: false
    buckets:
      - st-img-range-bucket-1416
      - other-bucket
users:
  - name: scoped
    accessKeyId: AK
    keychain: true
contexts:
  - name: scoped
    cluster: scoped
    user: scoped
current-context: scoped
`

func TestClusterBucketsLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pinnedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cl, ok := cfg.cluster("scoped")
	if !ok {
		t.Fatal("cluster scoped not found")
	}
	want := []string{"st-img-range-bucket-1416", "other-bucket"}
	if len(cl.Buckets) != len(want) {
		t.Fatalf("Buckets = %v, want %v", cl.Buckets, want)
	}
	for i := range want {
		if cl.Buckets[i] != want[i] {
			t.Errorf("Buckets[%d] = %q, want %q", i, cl.Buckets[i], want[i])
		}
	}
}

func TestClusterBucketsOmitEmpty(t *testing.T) {
	// A cluster with no pinned buckets must NOT emit a buckets: key (omitempty) — keeps
	// existing configs unchanged.
	cfg := &Config{
		APIVersion: "s3s/v1",
		Clusters:   []Cluster{{Name: "c", Endpoint: "http://h:9000"}},
		Users:      []User{{Name: "c", AccessKeyID: "AK", Keychain: true}},
		Contexts:   []Context{{Name: "c", Cluster: "c", User: "c"}},
	}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "buckets:") {
		t.Errorf("empty Buckets must not emit a buckets: key; got:\n%s", data)
	}
}

func TestAppendBucketPersistsAndDedupes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pinnedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Append a new bucket → live + disk grow.
	got, err := cfg.AppendBucket("scoped", "added-bucket")
	if err != nil {
		t.Fatalf("AppendBucket: %v", err)
	}
	want := []string{"st-img-range-bucket-1416", "other-bucket", "added-bucket"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("returned %v, want %v", got, want)
	}
	cl, _ := cfg.cluster("scoped")
	if strings.Join(cl.Buckets, ",") != strings.Join(want, ",") {
		t.Errorf("live cluster %v, want %v", cl.Buckets, want)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	rcl, _ := reloaded.cluster("scoped")
	if strings.Join(rcl.Buckets, ",") != strings.Join(want, ",") {
		t.Errorf("persisted %v, want %v", rcl.Buckets, want)
	}

	// Idempotent: appending an existing name does not duplicate.
	got2, err := cfg.AppendBucket("scoped", "added-bucket")
	if err != nil {
		t.Fatalf("AppendBucket dup: %v", err)
	}
	if len(got2) != 3 {
		t.Errorf("dup append changed list: %v", got2)
	}
}

func TestAddConnectionPersistsBuckets(t *testing.T) {
	keyring.MockInit()
	cfg := loadBase(t) // helper in connection_test.go
	if _, err := cfg.AddConnection(NewConnection{
		Name: "newc", Endpoint: "https://bucket.avito-sd", AccessKeyID: "AK",
		Buckets: []string{"a", "b"},
	}, "SK"); err != nil {
		t.Fatalf("AddConnection: %v", err)
	}
	reloaded, err := Load(cfg.Path())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	cl, _ := reloaded.cluster("newc")
	if strings.Join(cl.Buckets, ",") != "a,b" {
		t.Errorf("persisted buckets = %v, want [a b]", cl.Buckets)
	}
}

func TestAppendBucketRejectsEmptyAndUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(pinnedConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load(path)

	if _, err := cfg.AppendBucket("scoped", "   "); err == nil {
		t.Error("empty/whitespace bucket name must be rejected")
	}
	if _, err := cfg.AppendBucket("no-such-ctx", "b"); err == nil {
		t.Error("unknown context must be rejected")
	}
	// Config untouched after rejections.
	cl, _ := cfg.cluster("scoped")
	if len(cl.Buckets) != 2 {
		t.Errorf("config mutated on rejection: %v", cl.Buckets)
	}
}

func TestClusterBucketsMarshalReload(t *testing.T) {
	keyring.MockInit()
	cfg := &Config{
		APIVersion: "s3s/v1",
		Clusters:   []Cluster{{Name: "c", Endpoint: "http://h:9000", Buckets: []string{"a", "b"}}},
		Users:      []User{{Name: "c", AccessKeyID: "AK", Keychain: true}},
		Contexts:   []Context{{Name: "c", Cluster: "c", User: "c"}},
	}
	data, err := Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	cl, _ := reloaded.cluster("c")
	if len(cl.Buckets) != 2 || cl.Buckets[0] != "a" || cl.Buckets[1] != "b" {
		t.Errorf("round-trip Buckets = %v, want [a b]", cl.Buckets)
	}
}
