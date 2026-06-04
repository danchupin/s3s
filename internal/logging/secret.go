// Package logging provides a file-based slog logger and a redacting Secret type.
//
// The TUI owns the terminal, so logs MUST go to a file, never stdout/stderr
// (Constitution V). Secrets MUST never appear in logs, errors, or any display
// (FR-005, FR-021).
package logging

import "log/slog"

// redacted is the placeholder shown wherever a Secret would otherwise render.
const redacted = "[REDACTED]"

// Secret is a string that never reveals its value through the fmt/log surfaces.
// It implements fmt.Stringer, fmt.Formatter, and slog.LogValuer so that %v, %s,
// String(), and structured logging all emit the redacted placeholder. Use Reveal
// only at the trust boundary (building an S3 client), never for logging or display.
type Secret string

// String implements fmt.Stringer — redacts. Covers %v and %s via the default path
// for types that are not fmt.Formatter, and direct String() calls.
func (s Secret) String() string { return redacted }

// Reveal returns the underlying secret value. Call ONLY where the raw credential
// is required (e.g. constructing the S3 client), never in logs or UI.
func (s Secret) Reveal() string { return string(s) }

// IsEmpty reports whether the secret has no value.
func (s Secret) IsEmpty() bool { return s == "" }

// LogValue implements slog.LogValuer — structured logging emits the redacted
// placeholder instead of the underlying value.
func (s Secret) LogValue() slog.Value { return slog.StringValue(redacted) }
