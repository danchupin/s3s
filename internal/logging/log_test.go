package logging

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesJSONToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "s3s.log")

	log, closer, err := New(path, slog.LevelInfo)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	log.Info("hello", "k", "v")
	if err := closer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	line := strings.TrimSpace(string(data))
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("log line is not JSON: %q (%v)", line, err)
	}
	if rec["msg"] != "hello" || rec["k"] != "v" {
		t.Fatalf("unexpected record: %v", rec)
	}
}

func TestNewRespectsLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s3s.log")

	log, closer, err := New(path, slog.LevelWarn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	log.Debug("debug-should-be-filtered")
	log.Warn("warn-should-appear")
	if err := closer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, "debug-should-be-filtered") {
		t.Error("debug record leaked despite Warn level")
	}
	if !strings.Contains(out, "warn-should-appear") {
		t.Error("warn record missing")
	}
}

func TestNewEmptyPathErrors(t *testing.T) {
	if _, _, err := New("", slog.LevelInfo); err == nil {
		t.Error("New(\"\") should error")
	}
}
