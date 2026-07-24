// Package img loads images and renders them for terminals: half-block
// mosaic cells that work in any color terminal, and kitty graphics
// protocol transmissions with Unicode placeholders that survive scrolling.
package img

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"golang.org/x/image/draw"
)

// Load decodes a PNG, JPEG, or GIF file.
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	m, _, err := image.Decode(f)
	return m, err
}

// Fit computes the cell grid for an image. A half-block cell holds 1×2
// pixels and terminal cells are roughly twice as tall as wide, so rendered
// pixels come out square. Width is capped by maxCols and by the image's own
// pixel width (small images are not blown up); height by maxRows, shrinking
// the width proportionally when it binds.
func Fit(pxW, pxH, maxCols, maxRows int) (cols, rows int) {
	if pxW < 1 || pxH < 1 || maxCols < 1 || maxRows < 1 {
		return 0, 0
	}
	cols = maxCols
	if pxW < cols {
		cols = pxW
	}
	rows = (pxH*cols + pxW*2 - 1) / (pxW * 2)
	if rows > maxRows {
		cols = cols * maxRows / rows
		if cols < 1 {
			cols = 1
		}
		rows = maxRows
	}
	if rows < 1 {
		rows = 1
	}
	return cols, rows
}

// Cell is one terminal cell of a half-block mosaic: the top and bottom
// pixel colors, either of which may be transparent.
type Cell struct {
	TopSet, BottomSet bool
	Top, Bottom       [3]uint8
}

// Mosaic scales the image to cols×(rows*2) pixels and returns one Cell per
// terminal cell. Pixels with less than half alpha count as transparent.
func Mosaic(src image.Image, cols, rows int) [][]Cell {
	dst := image.NewRGBA(image.Rect(0, 0, cols, rows*2))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	out := make([][]Cell, rows)
	for y := 0; y < rows; y++ {
		out[y] = make([]Cell, cols)
		for x := 0; x < cols; x++ {
			top := dst.RGBAAt(x, y*2)
			bot := dst.RGBAAt(x, y*2+1)
			c := Cell{}
			if top.A >= 128 {
				c.TopSet = true
				c.Top = [3]uint8{top.R, top.G, top.B}
			}
			if bot.A >= 128 {
				c.BottomSet = true
				c.Bottom = [3]uint8{bot.R, bot.G, bot.B}
			}
			out[y][x] = c
		}
	}
	return out
}
