// Package plugin is the UI-agnostic core of the external capability-provider
// boundary: user-declared executables that supply data the storage protocol
// cannot (bucket discovery for listing-denied connections, object metadata from
// external systems).
//
// A plugin is invoked as an argv-exec'd subprocess — the command line is split
// with POSIX shell-words rules and executed directly, never via a shell — that
// receives one JSON request on stdin and answers with one JSON response on
// stdout, under a versioned, channel-agnostic envelope. The package owns the
// envelope types, the exec runner (timeouts, output caps, outcome
// classification, owner-only config gate), and the sanitization/validation
// pure functions every plugin-supplied string must pass before it may enter a
// UI model.
//
// The package never imports the AWS SDK or any UI code; the UI depends on the
// Runner interface and injects a fake in tests.
package plugin
