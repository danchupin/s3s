package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// fakeAPIErr is a minimal smithy.APIError for testing classify.
type fakeAPIErr struct{ code string }

func (e fakeAPIErr) Error() string                 { return e.code }
func (e fakeAPIErr) ErrorCode() string             { return e.code }
func (e fakeAPIErr) ErrorMessage() string          { return "" }
func (e fakeAPIErr) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"nil", nil, nil},
		{"canceled", context.Canceled, context.Canceled},
		{"deadline", context.DeadlineExceeded, context.DeadlineExceeded},
		{"wrapped canceled", fmt.Errorf("op: %w", context.Canceled), context.Canceled},
		{"NoSuchKey type", &types.NoSuchKey{}, ErrNotFound},
		{"NoSuchBucket type", &types.NoSuchBucket{}, ErrNotFound},
		{"NotFound type", &types.NotFound{}, ErrNotFound},
		{"api AccessDenied", fakeAPIErr{"AccessDenied"}, ErrAccessDenied},
		{"api Forbidden", fakeAPIErr{"Forbidden"}, ErrAccessDenied},
		{"api InvalidAccessKeyId", fakeAPIErr{"InvalidAccessKeyId"}, ErrAccessDenied},
		{"api SignatureDoesNotMatch", fakeAPIErr{"SignatureDoesNotMatch"}, ErrAccessDenied},
		{"api NoSuchKey code", fakeAPIErr{"NoSuchKey"}, ErrNotFound},
		{"api 404", fakeAPIErr{"404"}, ErrNotFound},
		{"api unknown code", fakeAPIErr{"ServiceUnavailable"}, ErrUnreachable},
		{"generic", errors.New("dial tcp: connection refused"), ErrUnreachable},
	}
	for _, c := range cases {
		got := classify(c.in)
		if c.want == nil {
			if got != nil {
				t.Errorf("%s: classify = %v, want nil", c.name, got)
			}
			continue
		}
		if !errors.Is(got, c.want) {
			t.Errorf("%s: classify = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestClassifyNoSecretLeak(t *testing.T) {
	// A generic error carrying a "secret" must not surface its text.
	err := classify(errors.New("AKIASECRETKEY signature mismatch detail"))
	if err == nil || err.Error() == "" {
		t.Fatal("expected a classified error")
	}
	// classify wraps only the sentinel — the original message is dropped.
	if got := err.Error(); got != ErrUnreachable.Error() {
		t.Errorf("classify leaked detail: %q", got)
	}
}
