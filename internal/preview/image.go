package preview

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif"  // register decoders for image.Decode
	_ "image/jpeg" // ...
	_ "image/png"  // ...
	"strings"

	termimg "github.com/blacktop/go-termimg"
	"github.com/eliukblau/pixterm/pkg/ansimage"
)

// Protocol identifies a terminal graphics capability (research §3).
type Protocol int

const (
	// ProtoNone means no graphics protocol — use ANSI half-block (works everywhere).
	ProtoNone Protocol = iota
	// ProtoKitty is the kitty graphics protocol (also Ghostty / WezTerm).
	ProtoKitty
	// ProtoITerm2 is the iTerm2 inline-images protocol.
	ProtoITerm2
	// ProtoSixel is the sixel graphics protocol.
	ProtoSixel
)

func (p Protocol) String() string {
	switch p {
	case ProtoKitty:
		return "kitty"
	case ProtoITerm2:
		return "iterm2"
	case ProtoSixel:
		return "sixel"
	default:
		return "none"
	}
}

// DetectProtocol resolves the best available graphics protocol from environment
// variables (research §3). env is an os.Getenv-like lookup (injectable for tests).
// Falls back to ProtoNone (→ half-block) when nothing is detected.
func DetectProtocol(env func(string) string) Protocol {
	term := strings.ToLower(env("TERM"))
	termProgram := env("TERM_PROGRAM")

	switch {
	case env("KITTY_WINDOW_ID") != "",
		strings.Contains(term, "kitty"),
		strings.Contains(term, "ghostty"),
		termProgram == "WezTerm",
		termProgram == "ghostty":
		return ProtoKitty
	case termProgram == "iTerm.app", env("LC_TERMINAL") == "iTerm2":
		return ProtoITerm2
	case strings.Contains(term, "sixel"):
		return ProtoSixel
	default:
		return ProtoNone
	}
}

// RenderImage renders image bytes for the given terminal protocol. A real
// graphics protocol (kitty/iTerm2/sixel) produces a crisp pixel image; ProtoNone
// (or any failure) falls back to ANSI half-block, which is low-resolution by
// nature (two pixels per character cell) but works everywhere. cols/rows are the
// target size in character cells.
func RenderImage(data []byte, proto Protocol, cols, rows int) (string, error) {
	if proto == ProtoNone {
		return RenderHalfBlock(data, cols, rows)
	}
	var tp termimg.Protocol
	switch proto {
	case ProtoKitty:
		tp = termimg.Kitty
	case ProtoITerm2:
		tp = termimg.ITerm2
	case ProtoSixel:
		tp = termimg.Sixel
	default:
		return RenderHalfBlock(data, cols, rows)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err // not a decodable image — caller shows a summary
	}
	r, err := termimg.GetRenderer(tp)
	if err != nil {
		return RenderHalfBlock(data, cols, rows)
	}
	out, err := r.Render(img, termimg.RenderOptions{Width: cols, Height: rows})
	if err != nil || out == "" {
		return RenderHalfBlock(data, cols, rows)
	}
	return out, nil
}

// RenderHalfBlock renders image bytes to a truecolor ANSI half-block string sized
// to fit within cols×rows character cells. Works in any 24-bit terminal — the
// safe default (research §3). Returns an error if the bytes are not a decodable
// image, so callers can fall back to a summary (FR-015).
func RenderHalfBlock(data []byte, cols, rows int) (string, error) {
	if cols < 1 {
		cols = 1
	}
	if rows < 2 {
		rows = 2
	}
	if rows%2 != 0 {
		rows++ // half-block packs 2 vertical pixels per cell
	}
	img, err := ansimage.NewScaledFromReader(
		bytes.NewReader(data), rows, cols, color.Black,
		ansimage.ScaleModeFit, ansimage.NoDithering,
	)
	if err != nil {
		return "", err
	}
	return img.Render(), nil
}
