package ui

import (
	"strings"
	"testing"

	"github.com/danchupin/s3s/internal/storage"
)

// TestHotkeyGlyphsBold asserts advertised key glyphs render with the bold key style so the
// key stands out from its label (011 FR-022/FR-023). keyStyle fuses bold with the accent
// color; the command bar advertises the global "q quit" key through it.
func TestHotkeyGlyphsBold(t *testing.T) {
	// Bold must actually change the rendered bytes vs the plain accent style.
	if keyStyle.Render("q") == accentStyle.Render("q") {
		t.Fatalf("keyStyle must differ from accentStyle (bold not applied)")
	}
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, nil, nil)
	m.width, m.height = 120, 30
	v := viewOf(m)
	if !strings.Contains(v, keyStyle.Render("q")) {
		t.Errorf("quit key glyph must be rendered bold (keyStyle) in the command bar")
	}
}

// TestHelpGlyphsBold asserts the help surface renders its key column with the bold key
// style (011 FR-023): the Quit row's key column is keyStyle-rendered.
func TestHelpGlyphsBold(t *testing.T) {
	f := storage.NewFake()
	f.Seed("b", "a.txt")
	m := withBuckets(f, nil, nil)
	m.width, m.height = 120, 30
	want := keyStyle.Render(pad(formatKeys(m.keys.Quit), 17))
	got := strings.Join(m.helpLines(), "\n")
	if !strings.Contains(got, want) {
		t.Errorf("help surface must render the key column bold (keyStyle)")
	}
}
