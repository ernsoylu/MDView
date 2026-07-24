package render_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ernsoylu/MDView/internal/img"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

// writePNG creates an 8×4 solid red test image.
func writePNG(t *testing.T, dir, name string) string {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			m.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, m); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestImageHalfblock(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "pic.png")
	doc := parser.Parse([]byte("before\n\n![tiny logo](pic.png)\n\nafter\n"))
	opts := render.Options{BaseDir: dir, Images: render.ImagesHalfblock}
	lines, tx := render.RenderDoc(doc, theme.Plain(), 40, opts)
	if len(tx) != 0 {
		t.Errorf("halfblock produced %d transmissions", len(tx))
	}
	var blocks, caption int
	for _, ln := range lines {
		s := ln.Plain()
		if strings.Contains(s, "▀") {
			blocks++
			if ln.Width() > 8 {
				t.Errorf("mosaic row wider than the 8px image: %d cells", ln.Width())
			}
		}
		if strings.Contains(s, "tiny logo") && strings.Contains(s, "pic.png") {
			caption++
		}
	}
	// 8×4 px at 8 cols → 2 pixel rows per cell → 2 rows.
	if blocks != 2 {
		t.Errorf("%d mosaic rows, want 2", blocks)
	}
	if caption != 1 {
		t.Errorf("caption rows = %d, want 1", caption)
	}
}

func TestImageKittyTransmitsOnce(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "pic.png")
	doc := parser.Parse([]byte("![k](pic.png)\n"))
	reg := img.NewRegistry()
	opts := render.Options{BaseDir: dir, Images: render.ImagesKitty, IDs: reg}

	lines, tx := render.RenderDoc(doc, theme.Plain(), 40, opts)
	if len(tx) != 1 {
		t.Fatalf("first render: %d transmissions, want 1", len(tx))
	}
	found := false
	for _, ln := range lines {
		if strings.ContainsRune(ln.Plain(), '\U0010EEEE') {
			found = true
			if ln.Width() != 8 {
				t.Errorf("placeholder row width = %d cells, want 8", ln.Width())
			}
		}
	}
	if !found {
		t.Error("no placeholder cells rendered")
	}

	_, tx = render.RenderDoc(doc, theme.Plain(), 40, opts)
	if len(tx) != 0 {
		t.Errorf("second render retransmitted %d times", len(tx))
	}
}

func TestImageFallbacks(t *testing.T) {
	dir := t.TempDir()
	cases := []struct{ name, md string }{
		{"remote", "![r](https://example.com/x.png)\n"},
		{"missing", "![m](nope.png)\n"},
	}
	for _, tc := range cases {
		lines, _ := render.RenderDoc(parser.Parse([]byte(tc.md)), theme.Plain(), 40,
			render.Options{BaseDir: dir, Images: render.ImagesHalfblock})
		joined := ""
		for _, ln := range lines {
			joined += ln.Plain() + "\n"
		}
		if !strings.Contains(joined, "🖼 [Image:") {
			t.Errorf("%s: expected placeholder fallback, got %q", tc.name, joined)
		}
		if strings.Contains(joined, "▀") {
			t.Errorf("%s: unexpected mosaic", tc.name)
		}
	}
}

func TestImagesOffUnchanged(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, dir, "pic.png")
	lines := render.Render(parser.Parse([]byte("![x](pic.png)\n")), theme.Plain(), 40)
	joined := ""
	for _, ln := range lines {
		joined += ln.Plain()
	}
	if !strings.Contains(joined, "🖼 [Image: x]") || strings.Contains(joined, "▀") {
		t.Errorf("Render without options must keep the placeholder: %q", joined)
	}
}
