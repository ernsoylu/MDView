package img

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
)

// KittySupported reports whether the terminal understands the kitty
// graphics protocol (kitty itself and ghostty advertise via environment).
func KittySupported() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	term := os.Getenv("TERM")
	return strings.Contains(term, "kitty") || strings.Contains(term, "ghostty")
}

// Registry hands out stable 24-bit image ids per cache key, so an image is
// transmitted once and reused across re-renders.
type Registry struct {
	ids  map[string]uint32
	next uint32
}

func NewRegistry() *Registry { return &Registry{ids: map[string]uint32{}, next: 1} }

// ID returns the id for key, allocating one and reporting fresh the first
// time the key is seen.
func (r *Registry) ID(key string) (id uint32, fresh bool) {
	if id, ok := r.ids[key]; ok {
		return id, false
	}
	id = r.next & 0xFFFFFF
	r.next++
	r.ids[key] = id
	return id, true
}

const chunkSize = 4096

// Transmit encodes the image as PNG and returns the chunked APC sequence
// that transmits it for virtual placement (U=1) over cols×rows cells. The
// caller writes it to the terminal once; the placeholder cells then show
// the image wherever they scroll.
func Transmit(m image.Image, id uint32, cols, rows int) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		return "", err
	}
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	var b strings.Builder
	first := true
	for len(data) > 0 {
		n := chunkSize
		if n > len(data) {
			n = len(data)
		}
		chunk := data[:n]
		data = data[n:]
		more := 0
		if len(data) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(&b, "\x1b_Ga=T,U=1,f=100,q=2,i=%d,c=%d,r=%d,m=%d;%s\x1b\\", id, cols, rows, more, chunk)
			first = false
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	return b.String(), nil
}

// placeholderRune marks a cell to be filled by a virtually-placed image.
const placeholderRune = '\U0010EEEE'

// PlaceholderRows returns one string per cell row of the placeholder grid.
// Each cell is the placeholder rune plus row and column diacritics; the
// terminal ties it to the image via the foreground color, which the caller
// must set to the 24-bit image id.
func PlaceholderRows(id uint32, cols, rows int) []string {
	_ = id // the id travels in the foreground color, not the text
	if rows > len(rowColDiacritics) {
		rows = len(rowColDiacritics)
	}
	if cols > len(rowColDiacritics) {
		cols = len(rowColDiacritics)
	}
	out := make([]string, rows)
	for y := 0; y < rows; y++ {
		var b strings.Builder
		for x := 0; x < cols; x++ {
			b.WriteRune(placeholderRune)
			b.WriteRune(rowColDiacritics[y])
			b.WriteRune(rowColDiacritics[x])
		}
		out[y] = b.String()
	}
	return out
}
