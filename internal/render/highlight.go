package render

import "github.com/charmbracelet/lipgloss"

// Plain returns the line's text without styling. Byte offsets into this
// string are what Highlight expects as ranges.
func (l Line) Plain() string {
	if len(l.Spans) == 1 {
		return l.Spans[0].Text
	}
	var b []byte
	for _, s := range l.Spans {
		b = append(b, s.Text...)
	}
	return string(b)
}

// Highlight returns a copy of the line where the byte ranges [start, end)
// of its plain text are restyled as st, splitting spans as needed. Ranges
// must be sorted and non-overlapping. Hyperlink targets are preserved.
func (l Line) Highlight(ranges [][2]int, st *lipgloss.Style) Line {
	if len(ranges) == 0 {
		return l
	}
	var out []Span
	pos := 0
	ri := 0
	for _, sp := range l.Spans {
		spStart, spEnd := pos, pos+len(sp.Text)
		cut := spStart
		for cut < spEnd {
			for ri < len(ranges) && ranges[ri][1] <= cut {
				ri++
			}
			segEnd := spEnd
			inHit := false
			switch {
			case ri >= len(ranges) || ranges[ri][0] >= spEnd:
				// no range intersects the rest of this span
			case ranges[ri][0] > cut:
				segEnd = ranges[ri][0]
			default:
				if e := ranges[ri][1]; e < spEnd {
					segEnd = e
				}
				inHit = true
			}
			style := sp.Style
			if inHit {
				style = st
			}
			if txt := sp.Text[cut-spStart : segEnd-spStart]; txt != "" {
				out = append(out, Span{Text: txt, Style: style, Link: sp.Link})
			}
			cut = segEnd
		}
		pos = spEnd
	}
	return Line{Spans: out, SourceLine: l.SourceLine}
}
