package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	extast "github.com/yuin/goldmark/extension/ast"
)

type tableCell struct {
	spans []Span
	align extast.Alignment
}

func (r *renderer) table(n *extast.Table, width int) []Line {
	var header []tableCell
	var rows [][]tableCell
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		_, isHeader := c.(*extast.TableHeader)
		var base *lipgloss.Style
		if isHeader {
			base = &r.th.TableHeader
		}
		var cells []tableCell
		for tc := c.FirstChild(); tc != nil; tc = tc.NextSibling() {
			cell, ok := tc.(*extast.TableCell)
			if !ok {
				continue
			}
			cells = append(cells, tableCell{spans: cellSpans(r.inlines(cell, base)), align: cell.Alignment})
		}
		if isHeader {
			header = cells
		} else {
			rows = append(rows, cells)
		}
	}

	ncol := len(header)
	for _, row := range rows {
		if len(row) > ncol {
			ncol = len(row)
		}
	}
	if ncol == 0 {
		return nil
	}

	widths := make([]int, ncol)
	measure := func(cells []tableCell) {
		for i, c := range cells {
			if w := spansWidth(c.spans); w > widths[i] {
				widths[i] = w
			}
		}
	}
	measure(header)
	for _, row := range rows {
		measure(row)
	}
	for i := range widths {
		if widths[i] < 1 {
			widths[i] = 1
		}
	}
	// Total = per column "│ cell " (3 + width) plus the closing "│".
	total := func() int {
		t := ncol*3 + 1
		for _, w := range widths {
			t += w
		}
		return t
	}
	// Shrink the widest column until the table fits (cells truncate with …).
	for total() > width {
		mi, mw := 0, 0
		for i, w := range widths {
			if w > mw {
				mi, mw = i, w
			}
		}
		if mw <= 3 {
			break
		}
		widths[mi]--
	}

	srcLine := nodeLine(r.doc, n)
	border := func(left, mid, right string) Line {
		spans := []Span{{Text: left, Style: &r.th.TableBorder}}
		for i, w := range widths {
			sep := mid
			if i == len(widths)-1 {
				sep = right
			}
			spans = append(spans,
				Span{Text: strings.Repeat("─", w+2), Style: &r.th.TableBorder},
				Span{Text: sep, Style: &r.th.TableBorder})
		}
		return Line{Spans: spans, SourceLine: srcLine}
	}
	rowLine := func(cells []tableCell) Line {
		var spans []Span
		for i := 0; i < ncol; i++ {
			spans = append(spans, Span{Text: "│ ", Style: &r.th.TableBorder})
			var content []Span
			align := extast.AlignNone
			if i < len(cells) {
				content = truncateSpans(cells[i].spans, widths[i])
				align = cells[i].align
			}
			pad := widths[i] - spansWidth(content)
			left, right := 0, pad
			switch align {
			case extast.AlignRight:
				left, right = pad, 0
			case extast.AlignCenter:
				left, right = pad/2, pad-pad/2
			}
			if left > 0 {
				spans = append(spans, Span{Text: strings.Repeat(" ", left)})
			}
			spans = append(spans, content...)
			spans = append(spans, Span{Text: strings.Repeat(" ", right) + " "})
		}
		spans = append(spans, Span{Text: "│", Style: &r.th.TableBorder})
		return Line{Spans: spans, SourceLine: srcLine}
	}

	out := []Line{border("┌", "┬", "┐")}
	if len(header) > 0 {
		out = append(out, rowLine(header), border("├", "┼", "┤"))
	}
	for _, row := range rows {
		out = append(out, rowLine(row))
	}
	out = append(out, border("└", "┴", "┘"))
	return out
}

// cellSpans flattens a seg stream to single-line spans (breaks → spaces).
func cellSpans(segs []seg) []Span {
	var out []Span
	for _, s := range segs {
		if s.brk {
			out = append(out, Span{Text: " "})
			continue
		}
		if s.text == "" {
			continue
		}
		out = append(out, Span{Text: s.text, Style: s.style, Link: s.link})
	}
	return out
}
