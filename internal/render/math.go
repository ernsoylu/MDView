package render

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"image"
	"strings"

	"codeberg.org/go-latex/latex/drawtex/drawimg"
	"codeberg.org/go-latex/latex/mtex"
	"github.com/charmbracelet/lipgloss"

	"github.com/ernsoylu/MDView/internal/img"
	"github.com/ernsoylu/MDView/internal/mathext"
)

const (
	mathFontSize = 28
	mathDPI      = 130
	mathMaxRows  = 8 // display math stays a modest banner, not a poster
)

// mathBlock renders $$...$$ through the image pipeline, falling back to
// the raw TeX styled like a code block when images are off or the
// expression is outside go-latex's mathtext subset.
func (r *renderer) mathBlock(n *mathext.MathBlock, width int) []Line {
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		sg := n.Lines().At(i)
		b.Write(sg.Value(r.src))
	}
	expr := strings.TrimSpace(strings.ReplaceAll(b.String(), "\n", " "))
	if expr == "" {
		return nil
	}
	srcLine := nodeLine(r.doc, n)

	if r.opts.Images != ImagesOff {
		if m, fgKey, err := renderMath(expr); err == nil {
			bounds := m.Bounds()
			cols, rows := img.Fit(bounds.Dx(), bounds.Dy(), width, mathMaxRows)
			key := fmt.Sprintf("math|%08x|%s", crc32.ChecksumIEEE([]byte(expr)), fgKey)
			if lines := r.pictureLines(m, key, cols, rows, srcLine); lines != nil {
				return lines
			}
		}
	}

	cw := width - 2
	if cw < 4 {
		cw = 4
	}
	out := []Line{{Spans: []Span{{Text: "  $$", Style: &r.th.CodeSpan}}, SourceLine: srcLine}}
	for _, chunk := range chunkSpans([]Span{{Text: expr, Style: &r.th.CodeSpan}}, cw) {
		out = append(out, Line{Spans: prepend(Span{Text: "  "}, chunk), SourceLine: srcLine})
	}
	return append(out, Line{Spans: []Span{{Text: "  $$", Style: &r.th.CodeSpan}}, SourceLine: srcLine})
}

// maxCachedMath bounds a pathological document; rasters are tens of KB.
const maxCachedMath = 256

var mathCache = newRasterCache(maxCachedMath)

// renderMath rasterizes TeX via go-latex and recolors it for the terminal
// background; fgKey separates light/dark variants in the kitty cache.
func renderMath(expr string) (image.Image, string, error) {
	fg := [3]uint8{30, 30, 36}
	fgKey := "light"
	if lipgloss.HasDarkBackground() {
		fg, fgKey = [3]uint8{224, 224, 230}, "dark"
	}
	m, err := mathCache.do(fgKey+"|"+expr, func() (image.Image, error) {
		return rasterizeMath(expr, fg)
	})
	return m, fgKey, err
}

// rasterizeMath draws TeX tinted with fg. go-latex v0.3.0 panics (not
// errors) on TeX outside its subset — notably superscripts and subscripts
// are not typeset yet — so recover turns any panic into an error and the
// caller falls back to raw TeX.
func rasterizeMath(expr string, fg [3]uint8) (m image.Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("mtex: %v", r)
		}
	}()
	var buf bytes.Buffer
	if err := mtex.Render(drawimg.NewRenderer(&buf), "$"+expr+"$", mathFontSize, mathDPI, nil); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(&buf)
	if err != nil {
		return nil, err
	}
	return img.AlphaFromLuminance(src, fg), nil
}
