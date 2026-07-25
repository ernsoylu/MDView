package render

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/ast"

	"github.com/ernsoylu/MDView/internal/img"
)

// soleImage returns the paragraph's image when the paragraph consists of
// exactly one image node — the case that renders as a block image.
func soleImage(n ast.Node) *ast.Image {
	if n.ChildCount() != 1 {
		return nil
	}
	im, _ := n.FirstChild().(*ast.Image)
	return im
}

// imageBlock renders an image-only paragraph as half-block mosaic or kitty
// placeholder cells plus a dim caption. It returns nil — falling back to
// the inline placeholder — for remote URLs, unreadable files, or when
// images are off.
func (r *renderer) imageBlock(n *ast.Image, width int) []Line {
	if r.opts.Images == ImagesOff || r.opts.BaseDir == "" {
		return nil
	}
	target := string(n.Destination)
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return nil // remote images stay placeholders
	}
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.opts.BaseDir, path)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil
	}
	m, err := img.Load(path)
	if err != nil {
		return nil
	}
	maxRows := r.opts.MaxImageRows
	if maxRows <= 0 {
		maxRows = 24
	}
	b := m.Bounds()
	cols, rows := img.Fit(b.Dx(), b.Dy(), width, maxRows)
	if cols < 1 || rows < 1 {
		return nil
	}
	srcLine := nodeLine(r.doc, n.Parent()) // the enclosing paragraph
	keyBase := fmt.Sprintf("%s|%d", path, fi.ModTime().UnixNano())
	out := r.pictureLines(m, keyBase, cols, rows, srcLine)
	if out == nil {
		return nil
	}

	alt := nodeText(n, r.src)
	if alt == "" {
		alt = "image"
	}
	caption := []seg{{text: sanitize("🖼 " + alt + " (" + target + ")"), style: &r.th.Dim}}
	return append(out, wrap(caption, width, srcLine)...)
}

// pictureLines renders a decoded image over a cols×rows cell grid in the
// active image mode. keyBase identifies the picture for kitty's
// transmit-once registry; the grid size is appended so resizes retransmit.
func (r *renderer) pictureLines(m image.Image, keyBase string, cols, rows, srcLine int) []Line {
	if cols < 1 || rows < 1 {
		return nil
	}
	switch r.opts.Images {
	case ImagesKitty:
		if r.opts.IDs == nil {
			return nil
		}
		id, fresh := r.opts.IDs.ID(fmt.Sprintf("%s|%dx%d", keyBase, cols, rows))
		if fresh {
			tx, err := img.Transmit(m, id, cols, rows)
			if err != nil {
				return nil
			}
			r.transmissions = append(r.transmissions, tx)
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%06X", id)))
		var out []Line
		for _, row := range img.PlaceholderRows(id, cols, rows) {
			out = append(out, Line{Spans: []Span{{Text: row, Style: &style}}, SourceLine: srcLine})
		}
		return out
	case ImagesHalfblock:
		return mosaicLines(img.Mosaic(m, cols, rows), srcLine)
	}
	return nil
}

// mosaicLines converts mosaic cells to IR lines, merging runs of identical
// cells into single spans.
func mosaicLines(cells [][]img.Cell, srcLine int) []Line {
	out := make([]Line, 0, len(cells))
	for _, row := range cells {
		var spans []Span
		for x := 0; x < len(row); {
			c := row[x]
			n := x + 1
			for n < len(row) && row[n] == c {
				n++
			}
			runLen := n - x
			var span Span
			switch {
			case c.TopSet && c.BottomSet:
				s := lipgloss.NewStyle().Foreground(hexColor(c.Top)).Background(hexColor(c.Bottom))
				span = Span{Text: strings.Repeat("▀", runLen), Style: &s}
			case c.TopSet:
				s := lipgloss.NewStyle().Foreground(hexColor(c.Top))
				span = Span{Text: strings.Repeat("▀", runLen), Style: &s}
			case c.BottomSet:
				s := lipgloss.NewStyle().Foreground(hexColor(c.Bottom))
				span = Span{Text: strings.Repeat("▄", runLen), Style: &s}
			default:
				span = Span{Text: strings.Repeat(" ", runLen)}
			}
			spans = append(spans, span)
			x = n
		}
		out = append(out, Line{Spans: spans, SourceLine: srcLine})
	}
	return out
}

// hexColor formats a cell colour. This runs up to twice per mosaic cell —
// tens of thousands of times for a full-width picture — so it spells out
// the hex rather than paying for a format string.
func hexColor(c [3]uint8) lipgloss.Color {
	const digits = "0123456789abcdef"
	b := [7]byte{'#'}
	for i, v := range c {
		b[1+i*2] = digits[v>>4]
		b[2+i*2] = digits[v&0x0f]
	}
	return lipgloss.Color(string(b[:]))
}
