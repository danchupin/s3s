package plugin

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The request must carry the contract version, the capability, the identity
// context (name/endpoint/userLabel/accessKeyId), and — for object-metadata —
// the target. The secret key must be unrepresentable: no field of the request
// type can hold it, and the marshaled JSON key set must not contain one.
func TestRequestMarshalDiscovery(t *testing.T) {
	req := Request{
		ContractVersion: ContractVersion,
		Capability:      BucketDiscovery,
		Connection: Connection{
			Name:        "prod-rgw",
			Endpoint:    "https://cluster.storage.example",
			UserLabel:   "svc-images",
			AccessKeyID: "AKIAEXAMPLE123",
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["contractVersion"] != float64(1) {
		t.Errorf("contractVersion = %v, want 1", got["contractVersion"])
	}
	if got["capability"] != "bucket-discovery" {
		t.Errorf("capability = %v", got["capability"])
	}
	conn, ok := got["connection"].(map[string]any)
	if !ok {
		t.Fatalf("connection missing: %s", data)
	}
	for k, want := range map[string]string{
		"name": "prod-rgw", "endpoint": "https://cluster.storage.example",
		"userLabel": "svc-images", "accessKeyId": "AKIAEXAMPLE123",
	} {
		if conn[k] != want {
			t.Errorf("connection.%s = %v, want %q", k, conn[k], want)
		}
	}
	if _, ok := got["target"]; ok {
		t.Error("discovery request must not carry a target")
	}
}

func TestRequestMarshalMetadataTarget(t *testing.T) {
	req := Request{
		ContractVersion: ContractVersion,
		Capability:      ObjectMetadata,
		Connection:      Connection{Name: "prod-rgw"},
		Target:          &Target{Bucket: "images-prod", Key: "0af3.jpg"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	tgt, ok := got["target"].(map[string]any)
	if !ok {
		t.Fatalf("target missing: %s", data)
	}
	if tgt["bucket"] != "images-prod" || tgt["key"] != "0af3.jpg" {
		t.Errorf("target = %v", tgt)
	}
}

// No secret-ish key may exist in the marshaled request — asserted on the full
// recursive key set, and on the Connection type's field set (compile-level
// guarantee backed by reflection).
func TestRequestCarriesNoSecret(t *testing.T) {
	req := Request{
		ContractVersion: ContractVersion,
		Capability:      ObjectMetadata,
		Connection:      Connection{Name: "n", Endpoint: "e", UserLabel: "u", AccessKeyID: "ak"},
		Target:          &Target{Bucket: "b", Key: "k"},
	}
	data, _ := json.Marshal(req)
	lower := strings.ToLower(string(data))
	for _, bad := range []string{"secret", "sessiontoken", "password", "credential"} {
		if strings.Contains(lower, bad) {
			t.Errorf("request JSON contains %q: %s", bad, data)
		}
	}
	ct := reflect.TypeFor[Connection]()
	want := map[string]bool{"Name": true, "Endpoint": true, "UserLabel": true, "AccessKeyID": true}
	for i := 0; i < ct.NumField(); i++ {
		if !want[ct.Field(i).Name] {
			t.Errorf("unexpected Connection field %q — identity context only", ct.Field(i).Name)
		}
	}
}

func TestDecodeResponseVariants(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		cap     Capability
		outcome Outcome
	}{
		{"discovery ok", `{"contractVersion":1,"buckets":["a","b"]}`, BucketDiscovery, OutcomeOK},
		{"discovery empty ok", `{"contractVersion":1,"buckets":[]}`, BucketDiscovery, OutcomeOK},
		{"metadata ok", `{"contractVersion":1,"fields":[{"name":"n","value":"v"}]}`, ObjectMetadata, OutcomeOK},
		{"metadata empty ok", `{"contractVersion":1,"fields":[]}`, ObjectMetadata, OutcomeOK},
		{"soft error", `{"contractVersion":1,"error":"api 503"}`, BucketDiscovery, OutcomeContractError},
		{"error and payload", `{"contractVersion":1,"buckets":["a"],"error":"x"}`, BucketDiscovery, OutcomeInvalidOutput},
		{"missing payload", `{"contractVersion":1}`, BucketDiscovery, OutcomeInvalidOutput},
		{"wrong payload", `{"contractVersion":1,"fields":[]}`, BucketDiscovery, OutcomeInvalidOutput},
		{"version 2", `{"contractVersion":2,"buckets":["a"]}`, BucketDiscovery, OutcomeIncompatible},
		{"no version", `{"buckets":["a"]}`, BucketDiscovery, OutcomeInvalidOutput},
		{"garbage", `not json`, BucketDiscovery, OutcomeInvalidOutput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, outcome := DecodeResponse([]byte(tc.data), tc.cap)
			if outcome != tc.outcome {
				t.Fatalf("outcome = %s, want %s", outcome, tc.outcome)
			}
			if tc.name == "discovery ok" && len(resp.Buckets) != 2 {
				t.Errorf("buckets = %v", resp.Buckets)
			}
			if tc.name == "metadata ok" && (len(resp.Fields) != 1 || resp.Fields[0].Name != "n") {
				t.Errorf("fields = %v", resp.Fields)
			}
			if tc.name == "soft error" && resp.Error != "api 503" {
				t.Errorf("error = %q", resp.Error)
			}
			if tc.name == "version 2" && resp.ContractVersion != 2 {
				t.Errorf("version = %d, want 2", resp.ContractVersion)
			}
		})
	}
}
