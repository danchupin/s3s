package storage

import (
	"context"
	"errors"
	"testing"

	smithy "github.com/aws/smithy-go"
)

func TestFakeGetObjectTagging(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	f.SeedObject("b", "tagged", FakeObject{Tags: map[string]string{"env": "prod"}})
	f.SeedObject("b", "none", FakeObject{})
	f.SeedObject("b", "denied", FakeObject{TagsDenied: true})

	if ot, err := f.GetObjectTagging(ctx, "b", "tagged"); err != nil || ot.Tags["env"] != "prod" {
		t.Errorf("tagged: ot=%+v err=%v", ot, err)
	}
	if ot, err := f.GetObjectTagging(ctx, "b", "none"); err != nil || len(ot.Tags) != 0 {
		t.Errorf("none: ot=%+v err=%v", ot, err)
	}
	if _, err := f.GetObjectTagging(ctx, "b", "denied"); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("denied: err=%v, want ErrAccessDenied", err)
	}
}

func TestFakeGetBucketConfigurationTriState(t *testing.T) {
	ctx := context.Background()
	f := NewFake()
	f.Seed("b", "x")
	f.Buckets["b"].BucketConfig = BucketConfig{
		Versioning: ConfigItem{State: ConfigConfigured, Detail: "Enabled"},
		// Encryption left zero → normalised to none.
	}
	f.Buckets["b"].UnsupportedGetConfigs = map[string]bool{"lifecycle": true}

	cfg, err := f.GetBucketConfiguration(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Versioning.State != ConfigConfigured || cfg.Versioning.Detail != "Enabled" {
		t.Errorf("versioning = %+v, want configured/Enabled", cfg.Versioning)
	}
	if cfg.Encryption.State != ConfigNone {
		t.Errorf("encryption = %+v, want none", cfg.Encryption)
	}
	if cfg.Lifecycle.State != ConfigUnsupported || !errors.Is(cfg.Lifecycle.Reason, ErrUnsupported) {
		t.Errorf("lifecycle = %+v, want unsupported", cfg.Lifecycle)
	}
}

// TestClassifyUnsupportedVsNone pins the riskiest FR-013 split: NotImplemented/501/405
// → unsupported; the *NotFound/*NotConfiguration family → none (NOT unsupported). MinIO
// cannot produce the unsupported branch, so it lives here (constitution IV scope note).
func TestClassifyUnsupportedVsNone(t *testing.T) {
	for _, code := range []string{"NotImplemented", "MethodNotAllowed"} {
		err := &smithy.GenericAPIError{Code: code, Message: "x"}
		if !errors.Is(classify(err), ErrUnsupported) {
			t.Errorf("classify(%s) = %v, want ErrUnsupported", code, classify(err))
		}
		if it := classifyConfig(err); it.State != ConfigUnsupported {
			t.Errorf("classifyConfig(%s).State = %v, want unsupported", code, it.State)
		}
	}
	for _, code := range []string{"NoSuchLifecycleConfiguration", "NoSuchTagSet", "ServerSideEncryptionConfigurationNotFoundError", "ReplicationConfigurationNotFoundError", "NoSuchPublicAccessBlockConfiguration"} {
		err := &smithy.GenericAPIError{Code: code, Message: "x"}
		if it := classifyConfig(err); it.State != ConfigNone {
			t.Errorf("classifyConfig(%s).State = %v, want none", code, it.State)
		}
		if errors.Is(classify(err), ErrUnsupported) {
			t.Errorf("classify(%s) must NOT be ErrUnsupported", code)
		}
	}
	if it := classifyConfig(&smithy.GenericAPIError{Code: "AccessDenied", Message: "x"}); it.State != ConfigDenied {
		t.Errorf("classifyConfig(AccessDenied).State = %v, want denied", it.State)
	}
}
