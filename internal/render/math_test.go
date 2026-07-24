package render_test

import (
	"strings"
	"testing"

	"github.com/ernsoylu/MDView/internal/img"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

func plainOf(lines []render.Line) string {
	var b strings.Builder
	for _, ln := range lines {
		b.WriteString(ln.Plain())
		b.WriteString("\n")
	}
	return b.String()
}

func TestMathFallbackWhenImagesOff(t *testing.T) {
	lines := render.Render(parser.Parse([]byte("$$E = mc^2$$\n")), theme.Plain(), 40)
	joined := plainOf(lines)
	if !strings.Contains(joined, "$$") || !strings.Contains(joined, "E = mc^2") {
		t.Errorf("fallback should show raw TeX, got %q", joined)
	}
	if strings.ContainsAny(joined, "▀▄") {
		t.Errorf("no mosaic expected with images off: %q", joined)
	}
}

func TestMathHalfblock(t *testing.T) {
	doc := parser.Parse([]byte("$$f(x) = \\frac{\\sqrt{x}}{2\\pi}$$\n"))
	lines, tx := render.RenderDoc(doc, theme.Plain(), 60, render.Options{Images: render.ImagesHalfblock})
	if len(tx) != 0 {
		t.Errorf("halfblock produced %d transmissions", len(tx))
	}
	joined := plainOf(lines)
	if !strings.ContainsAny(joined, "▀▄") {
		t.Fatalf("expected mosaic glyph pixels, got %q", joined)
	}
	if strings.Contains(joined, "$$") {
		t.Errorf("rendered math should not show raw TeX: %q", joined)
	}
	for i, ln := range lines {
		if ln.Width() > 60 {
			t.Errorf("line %d is %d cells wide", i, ln.Width())
		}
	}
}

func TestMathKittyTransmitsOnce(t *testing.T) {
	doc := parser.Parse([]byte("$$x + y$$\n"))
	opts := render.Options{Images: render.ImagesKitty, IDs: img.NewRegistry()}
	lines, tx := render.RenderDoc(doc, theme.Plain(), 60, opts)
	if len(tx) != 1 {
		t.Fatalf("first render: %d transmissions, want 1", len(tx))
	}
	found := false
	for _, ln := range lines {
		if strings.ContainsRune(ln.Plain(), '\U0010EEEE') {
			found = true
		}
	}
	if !found {
		t.Error("no kitty placeholder cells for math")
	}
	if _, tx = render.RenderDoc(doc, theme.Plain(), 60, opts); len(tx) != 0 {
		t.Errorf("second render retransmitted %d times", len(tx))
	}
}

func TestMathUnsupportedTeXFallsBack(t *testing.T) {
	// go-latex v0.3.0 panics on constructs it does not typeset yet;
	// renderMath must recover and fall back to raw TeX. Superscripts and
	// subscripts are the practically important gap — update these cases
	// when upstream implements ast.Sup/ast.Sub.
	for _, src := range []string{
		"$$x^2 + y_i$$\n",
		"$$\\begin{bmatrix} a & b \\end{bmatrix}$$\n",
	} {
		doc := parser.Parse([]byte(src))
		lines, _ := render.RenderDoc(doc, theme.Plain(), 60, render.Options{Images: render.ImagesHalfblock})
		joined := plainOf(lines)
		if !strings.Contains(joined, "$$") {
			t.Errorf("%q should fall back to raw source, got %q", src, joined)
		}
		if strings.ContainsAny(joined, "▀▄") {
			t.Errorf("%q rendered a mosaic unexpectedly", src)
		}
	}
}

func TestInlineMathStaysRaw(t *testing.T) {
	doc := parser.Parse([]byte("Euler says $e^{i\\pi} = -1$ inline.\n"))
	lines, _ := render.RenderDoc(doc, theme.Plain(), 60, render.Options{Images: render.ImagesHalfblock})
	joined := plainOf(lines)
	if !strings.Contains(joined, "$e^{i\\pi} = -1$") {
		t.Errorf("inline math should render as raw TeX with delimiters: %q", joined)
	}
}
