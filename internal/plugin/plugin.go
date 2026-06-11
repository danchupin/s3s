package plugin

import (
	"context"
	"encoding/json"
	"time"
)

// ContractVersion is the envelope version this build speaks. A response
// declaring any other version is incompatible.
const ContractVersion = 1

// Exchange bounds. Exceeding a cap truncates with an explicit indicator; the
// 1 MiB read cap is a hard contract failure (defends against runaway producers
// before the JSON parse).
const (
	DefaultTimeout = 5 * time.Second
	MaxOutput      = 1 << 20 // stdout read cap, bytes
	MaxBuckets     = 5000    // discovered names per invocation
	MaxFields      = 64      // metadata fields per invocation
	MaxFieldValue  = 4096    // bytes per field value
	MaxErrDetail   = 200     // bytes of sanitized error reason retained
)

// Capability tags what a plugin provides. The envelope is shared across
// capabilities so one runner code path serves all of them.
type Capability string

const (
	BucketDiscovery Capability = "bucket-discovery"
	ObjectMetadata  Capability = "object-metadata"
)

// ValidCapability reports whether s names a known capability.
func ValidCapability(s string) bool {
	switch Capability(s) {
	case BucketDiscovery, ObjectMetadata:
		return true
	}
	return false
}

// Connection is the identity context a plugin receives: public identifiers
// only. The secret key, session tokens, or any other credential material are
// deliberately unrepresentable here — there is no field to hold them.
type Connection struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	UserLabel   string `json:"userLabel"`
	AccessKeyID string `json:"accessKeyId"`
}

// Target identifies the object an object-metadata invocation is about.
type Target struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

// Request is the single JSON object written to a plugin's stdin. Target is
// present iff the capability is object-metadata.
type Request struct {
	ContractVersion int        `json:"contractVersion"`
	Capability      Capability `json:"capability"`
	Connection      Connection `json:"connection"`
	Target          *Target    `json:"target,omitempty"`
}

// Field is one named metadata value; response order is preserved in display.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Response is the decoded plugin answer: exactly one of the capability's
// payload (Buckets / Fields) or Error is present in a valid response.
type Response struct {
	ContractVersion int
	Buckets         []string
	Fields          []Field
	Error           string
}

// respWire detects key PRESENCE (pointers): an empty fields array is a valid
// "nothing known" answer, distinct from an absent payload.
type respWire struct {
	ContractVersion *int      `json:"contractVersion"`
	Buckets         *[]string `json:"buckets"`
	Fields          *[]Field  `json:"fields"`
	Error           *string   `json:"error"`
}

// DecodeResponse parses and validates one plugin response against the invoked
// capability. Outcomes: OK for a valid payload, ContractError for a soft
// {"error": …}, Incompatible for a foreign contractVersion, InvalidOutput for
// everything else (unparsable JSON, missing version, wrong/both/no payload).
func DecodeResponse(data []byte, cap Capability) (Response, Outcome) {
	var w respWire
	if err := json.Unmarshal(data, &w); err != nil {
		return Response{}, OutcomeInvalidOutput
	}
	if w.ContractVersion == nil {
		return Response{}, OutcomeInvalidOutput
	}
	resp := Response{ContractVersion: *w.ContractVersion}
	if *w.ContractVersion != ContractVersion {
		return resp, OutcomeIncompatible
	}
	if w.Buckets != nil {
		resp.Buckets = *w.Buckets
	}
	if w.Fields != nil {
		resp.Fields = *w.Fields
	}
	if w.Error != nil {
		resp.Error = *w.Error
		if w.Buckets != nil || w.Fields != nil {
			return resp, OutcomeInvalidOutput // error and payload are mutually exclusive
		}
		return resp, OutcomeContractError
	}
	switch cap {
	case BucketDiscovery:
		if w.Buckets == nil || w.Fields != nil {
			return resp, OutcomeInvalidOutput
		}
	case ObjectMetadata:
		if w.Fields == nil || w.Buckets != nil {
			return resp, OutcomeInvalidOutput
		}
	default:
		return resp, OutcomeInvalidOutput
	}
	return resp, OutcomeOK
}

// Outcome classifies one invocation for status display and logging.
type Outcome string

const (
	OutcomeOK            Outcome = "ok"
	OutcomeTimeout       Outcome = "timeout"
	OutcomeExecError     Outcome = "exec_error"
	OutcomeContractError Outcome = "contract_error"
	OutcomeInvalidOutput Outcome = "invalid_output"
	OutcomeIncompatible  Outcome = "incompatible"
)

// Decl is the execution-relevant slice of a declared plugin. Timeout ≤ 0
// resolves to DefaultTimeout.
type Decl struct {
	Name    string
	Cmd     string
	Timeout time.Duration
}

// Result is one finished invocation. Payload strings are sanitized before they
// leave the runner; ErrDetail is sanitized and capped at MaxErrDetail.
type Result struct {
	Outcome      Outcome
	Buckets      []string // bucket-discovery payload
	Fields       []Field  // object-metadata payload
	ErrDetail    string
	Incompatible int  // contract version the plugin answered with, when Outcome is incompatible
	Unavailable  bool // the executable is missing — classified for the unavailable status
	Duration     time.Duration
}

// Runner executes one plugin invocation. The UI depends on this interface and
// injects a fake in tests; ExecRunner is the production subprocess runner.
type Runner interface {
	Invoke(ctx context.Context, d Decl, req Request) Result
}
