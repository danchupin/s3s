package preview

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // register GIF decoder for image.Decode
	_ "image/jpeg" // register JPEG decoder
	"image/png"
	"strings"

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
// graphics protocol (kitty / iTerm2) produces a crisp, full-resolution image;
// ProtoNone/ProtoSixel (or any failure) fall back to ANSI half-block, which is
// low-resolution by nature (two pixels per character cell) but works everywhere.
// cols/rows are the target size in character cells. The protocol encoders are
// hand-rolled (no terminal queries) so they never block or panic inside the TUI.
func RenderImage(data []byte, proto Protocol, cols, rows int) (string, error) {
	switch proto {
	case ProtoITerm2:
		if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
			return "", err // not a decodable image → caller shows a summary
		}
		return renderITerm2(data, cols, rows), nil
	case ProtoKitty:
		if s, err := renderKitty(data, cols, rows); err == nil {
			return s, nil
		}
		return "", fmt.Errorf("kitty render failed")
	default: // ProtoNone, ProtoSixel
		return RenderHalfBlock(data, cols, rows)
	}
}

// renderITerm2 emits the iTerm2 inline-image escape (raw bytes, base64). width
// and height are in character cells; aspect ratio is preserved.
func renderITerm2(data []byte, cols, rows int) string {
	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("\x1b]1337;File=inline=1;size=%d;width=%d;height=%d;preserveAspectRatio=1:%s\x07",
		len(data), cols, rows, b64)
}

// renderKitty emits the kitty graphics protocol: a PNG payload, base64, chunked.
// A fixed image id replaces the previous image; q=2 suppresses the terminal's
// acknowledgement responses so they never leak into the TUI input stream.
func renderKitty(data []byte, cols, rows int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	const chunk = 4096
	var sb strings.Builder
	first := true
	for len(b64) > 0 {
		n := min(chunk, len(b64))
		part := b64[:n]
		b64 = b64[n:]
		more := 0
		if len(b64) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(&sb, "\x1b_Gf=100,a=T,i=1,q=2,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, part)
			first = false
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", more, part)
		}
	}
	return sb.String(), nil
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
