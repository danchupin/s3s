package logging

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

const sentinel = "super-secret-key-AKIA123"

func TestSecretString(t *testing.T) {
	s := Secret(sentinel)
	if got := s.String(); got != redacted {
		t.Fatalf("String() = %q, want %q", got, redacted)
	}
}

func TestSecretFmtVerbs(t *testing.T) {
	s := Secret(sentinel)
	for _, verb := range []string{"%v", "%s", "%q"} {
		out := fmt.Sprintf(verb, s)
		if strings.Contains(out, sentinel) {
			t.Errorf("Sprintf(%q) leaked secret: %q", verb, out)
		}
		if !strings.Contains(out, redacted) {
			t.Errorf("Sprintf(%q) = %q, want it to contain %q", verb, out, redacted)
		}
	}
}

func TestSecretReveal(t *testing.T) {
	s := Secret(sentinel)
	if s.Reveal() != sentinel {
		t.Fatalf("Reveal() = %q, want %q", s.Reveal(), sentinel)
	}
}

func TestSecretIsEmpty(t *testing.T) {
	if !Secret("").IsEmpty() {
		t.Error("empty Secret should report IsEmpty() == true")
	}
	if Secret("x").IsEmpty() {
		t.Error("non-empty Secret should report IsEmpty() == false")
	}
}

func TestSecretSlogRedaction(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("auth", "secret", Secret(sentinel))
	if strings.Contains(buf.String(), sentinel) {
		t.Fatalf("slog output leaked secret: %s", buf.String())
	}
	if !strings.Contains(buf.String(), redacted) {
		t.Fatalf("slog output missing redaction marker: %s", buf.String())
	}
}
