package render

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"image"
	"strings"
	"sync"

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

// mathResult is one memoized rasterization outcome.
type mathResult struct {
	m   image.Image
	err error
}

// maxCachedMath bounds a pathological document. Rasters are tens of KB, and
// past the cap the cache resets, degrading to the uncached behaviour rather
// than growing without limit.
const maxCachedMath = 256

// mathCache memoizes rasterization across renders. A render pass is
// otherwise dominated by it — ~3.8 ms per expression against ~78 µs to
// decode an image and ~73 µs to build its mosaic — so every resize redid
// all of it, on a frame the terminal was waiting for. The result is a pure
// function of the expression and the foreground it is tinted with.
//
// Failures are memoized too. go-latex cannot typeset sub- or superscripts
// at all, so those documents fall back to raw TeX on every frame, and
// re-attempting costs ~168 µs an expression.
//
// The mutex is not for the UI, which renders on one goroutine, but for
// tests and fuzzing, which do not.
var mathCache = struct {
	sync.Mutex
	entries map[string]mathResult
}{entries: map[string]mathResult{}}

// renderMath rasterizes TeX via go-latex and recolors it for the terminal
// background; fgKey separates light/dark variants in the kitty cache.
func renderMath(expr string) (image.Image, string, error) {
	fg := [3]uint8{30, 30, 36}
	fgKey := "light"
	if lipgloss.HasDarkBackground() {
		fg, fgKey = [3]uint8{224, 224, 230}, "dark"
	}
	key := fgKey + "|" + expr

	mathCache.Lock()
	got, hit := mathCache.entries[key]
	mathCache.Unlock()
	if hit {
		return got.m, fgKey, got.err
	}

	m, err := rasterizeMath(expr, fg)

	mathCache.Lock()
	if len(mathCache.entries) >= maxCachedMath {
		clear(mathCache.entries)
	}
	mathCache.entries[key] = mathResult{m: m, err: err}
	mathCache.Unlock()

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
