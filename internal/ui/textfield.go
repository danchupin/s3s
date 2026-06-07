package ui

import "strings"

// textField is the shared single-line, rune-aware text editor used by the add-connection
// form fields and the typed-confirm input (008 US3). The caret is a RUNE index (never a
// byte offset), so multi-byte input is never split. Render windows the value horizontally
// so the caret stays on screen, and masks to bullets for the secret field — without ever
// emitting a byte of the underlying value.
type textField struct {
	Value string
	Caret int // rune index, invariant 0 ≤ Caret ≤ runeLen(Value)
}

const caretGlyph = "▏"

// clamp keeps Caret a valid rune boundary (callers may set Value directly).
func (f *textField) clamp() {
	n := len([]rune(f.Value))
	if f.Caret < 0 {
		f.Caret = 0
	}
	if f.Caret > n {
		f.Caret = n
	}
}

// Insert places s at the caret and advances the caret by s's rune length (paste + typing).
func (f *textField) Insert(s string) {
	f.clamp()
	r := []rune(f.Value)
	ins := []rune(s)
	out := make([]rune, 0, len(r)+len(ins))
	out = append(out, r[:f.Caret]...)
	out = append(out, ins...)
	out = append(out, r[f.Caret:]...)
	f.Value = string(out)
	f.Caret += len(ins)
}

// Backspace removes the rune before the caret (no-op at start).
func (f *textField) Backspace() {
	f.clamp()
	if f.Caret == 0 {
		return
	}
	r := []rune(f.Value)
	f.Value = string(append(append([]rune{}, r[:f.Caret-1]...), r[f.Caret:]...))
	f.Caret--
}

// DeleteFwd removes the rune at the caret (no-op at end).
func (f *textField) DeleteFwd() {
	f.clamp()
	r := []rune(f.Value)
	if f.Caret >= len(r) {
		return
	}
	f.Value = string(append(append([]rune{}, r[:f.Caret]...), r[f.Caret+1:]...))
}

// Left / Right / Home / End move the caret, clamped to the value bounds.
func (f *textField) Left() {
	if f.Caret > 0 {
		f.Caret--
	}
}

func (f *textField) Right() {
	if f.Caret < len([]rune(f.Value)) {
		f.Caret++
	}
}

func (f *textField) Home() { f.Caret = 0 }

func (f *textField) End() { f.Caret = len([]rune(f.Value)) }

// Render returns a width-bounded window of the value with the caret glyph at its position.
// When masked, every rune is shown as a bullet (the value bytes never appear). The window
// always contains the caret so a long value scrolls horizontally with the cursor.
func (f textField) Render(width int, masked bool) string {
	f.clamp()
	var disp []rune
	if masked {
		disp = []rune(strings.Repeat("•", len([]rune(f.Value))))
	} else {
		disp = []rune(f.Value)
	}
	// Insert a sentinel for the caret so windowing keeps it visible; replaced on output.
	const sentinel = '\x00'
	withCaret := make([]rune, 0, len(disp)+1)
	withCaret = append(withCaret, disp[:f.Caret]...)
	withCaret = append(withCaret, sentinel)
	withCaret = append(withCaret, disp[f.Caret:]...)

	if width < 1 {
		width = 1
	}
	start := 0
	if len(withCaret) > width {
		// Keep the caret (at index f.Caret in withCaret) inside [start, start+width).
		if f.Caret >= width {
			start = f.Caret - width + 1
		}
		if start+width > len(withCaret) {
			start = len(withCaret) - width
		}
		if start < 0 {
			start = 0
		}
	}
	end := start + width
	if end > len(withCaret) {
		end = len(withCaret)
	}
	var b strings.Builder
	for _, r := range withCaret[start:end] {
		if r == sentinel {
			b.WriteString(caretGlyph)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
