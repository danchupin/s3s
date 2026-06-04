package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// New builds a slog.Logger that writes JSON records to the file at path, creating
// parent directories as needed. It NEVER writes to stdout/stderr — the TUI owns
// the terminal (Constitution V). The returned closer flushes and closes the file.
func New(path string, level slog.Level) (*slog.Logger, io.Closer, error) {
	if path == "" {
		return nil, nil, fmt.Errorf("logging: empty log path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("logging: create log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("logging: open log file: %w", err)
	}
	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	return slog.New(handler), f, nil
}

// NewWriter builds a JSON slog.Logger over an arbitrary writer. Useful for tests
// and for wiring a custom sink; production code uses New for the file path.
func NewWriter(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
