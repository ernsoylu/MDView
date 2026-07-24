package img

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestFit(t *testing.T) {
	cases := []struct {
		pxW, pxH, maxCols, maxRows, cols, rows int
	}{
		{100, 100, 50, 24, 50, 25}, // square image → rows = cols/2, capped next
		{100, 50, 40, 24, 40, 10},  // wide image fits
		{16, 16, 80, 24, 16, 8},    // small image is not upscaled
		{100, 400, 40, 10, 5, 10},  // tall image shrinks width to honor maxRows
	}
	for i, tc := range cases {
		cols, rows := Fit(tc.pxW, tc.pxH, tc.maxCols, tc.maxRows)
		if i == 0 {
			// 50 cols of a square image wants 25 rows > 24: width shrinks.
			if rows > 24 {
				t.Errorf("case 0: rows = %d exceeds cap", rows)
			}
			continue
		}
		if cols != tc.cols || rows != tc.rows {
			t.Errorf("Fit case %d = (%d, %d), want (%d, %d)", i, cols, rows, tc.cols, tc.rows)
		}
	}
	if c, r := Fit(0, 10, 10, 10); c != 0 || r != 0 {
		t.Errorf("degenerate input = (%d, %d), want (0, 0)", c, r)
	}
}

func TestMosaicColorsAndTransparency(t *testing.T) {
	// 2×4 pixels → 2 cols, 2 rows. Left column red over green then blue
	// over transparent; right column all transparent.
	m := image.NewRGBA(image.Rect(0, 0, 2, 4))
	set := func(x, y int, c color.RGBA) { m.SetRGBA(x, y, c) }
	red := color.RGBA{255, 0, 0, 255}
	green := color.RGBA{0, 255, 0, 255}
	blue := color.RGBA{0, 0, 255, 255}
	set(0, 0, red)
	set(0, 1, green)
	set(0, 2, blue)
	// (0,3) and the whole right column stay zero (transparent).

	cells := Mosaic(m, 2, 2)
	if len(cells) != 2 || len(cells[0]) != 2 {
		t.Fatalf("grid = %dx%d, want 2x2", len(cells), len(cells[0]))
	}
	c := cells[0][0]
	if !c.TopSet || !c.BottomSet || c.Top != [3]uint8{255, 0, 0} || c.Bottom != [3]uint8{0, 255, 0} {
		t.Errorf("cell (0,0) = %+v, want red over green", c)
	}
	c = cells[1][0]
	if !c.TopSet || c.BottomSet || c.Top != [3]uint8{0, 0, 255} {
		t.Errorf("cell (1,0) = %+v, want blue over transparent", c)
	}
	c = cells[0][1]
	if c.TopSet || c.BottomSet {
		t.Errorf("cell (0,1) = %+v, want fully transparent", c)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	a1, fresh := r.ID("a")
	if !fresh || a1 == 0 {
		t.Fatalf("first id = %d fresh=%v", a1, fresh)
	}
	a2, fresh := r.ID("a")
	if fresh || a2 != a1 {
		t.Errorf("repeat id = %d fresh=%v, want %d false", a2, fresh, a1)
	}
	b, fresh := r.ID("b")
	if !fresh || b == a1 {
		t.Errorf("new key id = %d fresh=%v", b, fresh)
	}
}

func TestTransmitSingleChunk(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 2, 2))
	tx, err := Transmit(m, 7, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tx, "\x1b_G") || !strings.HasSuffix(tx, "\x1b\\") {
		t.Fatalf("not an APC sequence: %q", tx)
	}
	for _, want := range []string{"a=T", "U=1", "f=100", "q=2", "i=7", "c=3", "r=2", "m=0"} {
		if !strings.Contains(tx, want) {
			t.Errorf("transmission lacks %q", want)
		}
	}
}

func TestTransmitChunking(t *testing.T) {
	// Noise-free gradients compress well; use a big enough image that the
	// base64 payload still exceeds one chunk.
	m := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			m.SetRGBA(x, y, color.RGBA{uint8(x * y % 256), uint8(x % 256), uint8(y % 256), 255})
		}
	}
	tx, err := Transmit(m, 1, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tx, "\x1b\\")
	parts = parts[:len(parts)-1] // trailing empty piece
	if len(parts) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(parts))
	}
	if !strings.Contains(parts[0], "m=1") {
		t.Errorf("first chunk lacks m=1: %.60q", parts[0])
	}
	for _, p := range parts[1 : len(parts)-1] {
		if !strings.HasPrefix(p, "\x1b_Gm=1;") {
			t.Errorf("middle chunk header wrong: %.30q", p)
		}
	}
	if !strings.HasPrefix(parts[len(parts)-1], "\x1b_Gm=0;") {
		t.Errorf("last chunk header wrong: %.30q", parts[len(parts)-1])
	}
}

func TestPlaceholderRows(t *testing.T) {
	rows := PlaceholderRows(5, 3, 2)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	first := []rune(rows[0])
	if len(first) != 9 { // 3 cells × (placeholder + 2 diacritics)
		t.Fatalf("row 0 has %d runes, want 9", len(first))
	}
	if first[0] != placeholderRune || first[1] != rowColDiacritics[0] || first[2] != rowColDiacritics[0] {
		t.Errorf("cell (0,0) runes = %U", first[:3])
	}
	if first[5] != rowColDiacritics[1] { // second cell's column diacritic
		t.Errorf("cell (0,1) column diacritic = %U, want %U", first[5], rowColDiacritics[1])
	}
	second := []rune(rows[1])
	if second[1] != rowColDiacritics[1] {
		t.Errorf("row 1 row-diacritic = %U, want %U", second[1], rowColDiacritics[1])
	}
}

func TestKittySupported(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "xterm-256color")
	if KittySupported() {
		t.Error("plain xterm should not report kitty support")
	}
	t.Setenv("TERM", "xterm-kitty")
	if !KittySupported() {
		t.Error("xterm-kitty should report support")
	}
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("KITTY_WINDOW_ID", "3")
	if !KittySupported() {
		t.Error("KITTY_WINDOW_ID should report support")
	}
}
