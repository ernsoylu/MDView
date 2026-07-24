// Package render turns a parsed markdown document into the styled-line IR:
// a flat slice of terminal rows the UI slices into a viewport. Each row
// carries styled spans (with optional hyperlink targets) and the source line
// it renders, which powers vim +N, resize re-anchoring, and navigation.
package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// Span is a styled run of text within one rendered line. Style points into
// the renderer's theme (nil = unstyled): spans are allocated in bulk, and
// carrying lipgloss.Style by value made the GC dominate render time.
type Span struct {
	Text  string
	Style *lipgloss.Style
	Link  string // when set, the span is wrapped in an OSC 8 hyperlink
}

// Line is a single terminal row plus the 1-based source line it renders.
// SourceLine is 0 for synthetic rows with no clear origin.
type Line struct {
	Spans      []Span
	SourceLine int
}

// String renders the line as an ANSI string. OSC 8 hyperlinks are emitted
// only when the color profile is not mono: a mono profile means output is
// piped or NO_COLOR is set, where raw escape sequences would be noise.
func (l Line) String() string {
	osc8 := lipgloss.ColorProfile() != termenv.Ascii
	var b strings.Builder
	for _, s := range l.Spans {
		t := s.Text
		if s.Style != nil {
			t = s.Style.Render(s.Text)
		}
		if s.Link != "" && osc8 {
			t = "\x1b]8;;" + s.Link + "\x1b\\" + t + "\x1b]8;;\x1b\\"
		}
		b.WriteString(t)
	}
	return b.String()
}

// Width returns the display width of the line's text in terminal cells.
func (l Line) Width() int {
	w := 0
	for _, s := range l.Spans {
		w += runewidth.StringWidth(s.Text)
	}
	return w
}

func spansWidth(spans []Span) int {
	w := 0
	for _, s := range spans {
		w += runewidth.StringWidth(s.Text)
	}
	return w
}
