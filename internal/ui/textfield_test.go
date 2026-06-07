package ui

import (
	"strings"
	"testing"
)

// textField is the shared rune-aware single-line editor (008 US3). These tests pin the
// contract in specs/008-connection-form-ux/contracts/text-input-contract.md.

func TestTextFieldInsertAtCaret(t *testing.T) {
	var f textField
	f.Insert("abc")
	if f.Value != "abc" || f.Caret != 3 {
		t.Fatalf("after Insert(abc): value=%q caret=%d, want abc/3", f.Value, f.Caret)
	}
	f.Left()
	f.Left()
	f.Insert("X")
	if f.Value != "aXbc" {
		t.Errorf("mid-field insert: value=%q, want aXbc", f.Value)
	}
	if f.Caret != 2 {
		t.Errorf("caret after insert = %d, want 2", f.Caret)
	}
}

func TestTextFieldBackspaceAtEnd(t *testing.T) {
	f := textField{Value: "abc", Caret: 3}
	f.End()
	f.Backspace()
	if f.Value != "ab" || f.Caret != 2 {
		t.Fatalf("End+Backspace: value=%q caret=%d, want ab/2", f.Value, f.Caret)
	}
}

func TestTextFieldDeleteFwdAtHome(t *testing.T) {
	f := textField{Value: "abc", Caret: 3}
	f.Home()
	f.DeleteFwd()
	if f.Value != "bc" || f.Caret != 0 {
		t.Fatalf("Home+DeleteFwd: value=%q caret=%d, want bc/0", f.Value, f.Caret)
	}
}

func TestTextFieldCaretBounds(t *testing.T) {
	f := textField{Value: "abc", Caret: 0}
	f.Left() // no-op at 0
	if f.Caret != 0 {
		t.Errorf("Left at 0 moved caret to %d", f.Caret)
	}
	f.End()
	f.Right() // no-op at end
	if f.Caret != 3 {
		t.Errorf("Right at end moved caret to %d", f.Caret)
	}
	f.Backspace() // removes c
	if f.Value != "ab" {
		t.Errorf("Backspace at end: %q, want ab", f.Value)
	}
}

func TestTextFieldRuneAware(t *testing.T) {
	var f textField
	f.Insert("héllo") // 5 runes, 6 bytes
	if f.Caret != 5 {
		t.Errorf("caret after multi-byte insert = %d, want 5 (rune count)", f.Caret)
	}
	f.Left()
	f.Backspace() // removes 'l' before caret-at-4
	if f.Value != "hélo" {
		t.Errorf("rune-aware backspace: %q, want hélo", f.Value)
	}
}

func TestTextFieldRenderWindowKeepsCaretVisible(t *testing.T) {
	f := textField{Value: "0123456789", Caret: 10}
	out := f.Render(3, false)
	// width 3 → tail of the value plus the caret must be visible; "9" (last char) present,
	// leading "0" not.
	if !strings.Contains(out, "9") {
		t.Errorf("Render(3) should show the tail incl. last char; got %q", out)
	}
	if strings.Contains(out, "0") {
		t.Errorf("Render(3) at end should have scrolled past the head; got %q", out)
	}
}

func TestTextFieldRenderMaskedLengthOnly(t *testing.T) {
	f := textField{Value: "secret42", Caret: 8}
	out := f.Render(40, true)
	if strings.Contains(out, "secret") || strings.Contains(out, "42") {
		t.Errorf("masked render leaked the value: %q", out)
	}
	if got := strings.Count(out, "•"); got != 8 {
		t.Errorf("masked render bullet count = %d, want 8 (rune length)", got)
	}
}
