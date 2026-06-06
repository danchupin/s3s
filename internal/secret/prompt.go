package secret

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompt reads a secret from the terminal with no echo. It MUST run before the Bubble
// Tea program starts — the TUI owns the terminal afterward (Constitution V, 005 R12).
func Prompt(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("secret: prompt: %w", err)
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("secret: empty secret entered")
	}
	return s, nil
}
