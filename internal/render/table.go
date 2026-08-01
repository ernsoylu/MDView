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
	header, rows := r.collectTable(n)
	ncol := len(header)
	for _, row := range rows {
		if len(row) > ncol {
			ncol = len(row)
		}
	}
	if ncol == 0 {
		return nil
	}
	widths := measureTableWidths(header, rows, ncol)
	shrinkTableWidths(widths, ncol, width)

	srcLine := nodeLine(r.doc, n)
	out := []Line{tableBorder(widths, "┌", "┬", "┐", &r.th.TableBorder, srcLine)}
	if len(header) > 0 {
		out = append(out,
			tableRow(header, widths, ncol, &r.th.TableBorder, srcLine),
			tableBorder(widths, "├", "┼", "┤", &r.th.TableBorder, srcLine))
	}
	for _, row := range rows {
		out = append(out, tableRow(row, widths, ncol, &r.th.TableBorder, srcLine))
	}
	out = append(out, tableBorder(widths, "└", "┴", "┘", &r.th.TableBorder, srcLine))
	return out
}

func (r *renderer) collectTable(n *extast.Table) (header []tableCell, rows [][]tableCell) {
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
	return header, rows
}

func measureTableWidths(header []tableCell, rows [][]tableCell, ncol int) []int {
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
	return widths
}

// shrinkTableWidths reduces the widest columns until the table fits width.
// Total layout is per-column "│ cell " (3 + width) plus the closing "│".
func shrinkTableWidths(widths []int, ncol, width int) {
	total := func() int {
		t := ncol*3 + 1
		for _, w := range widths {
			t += w
		}
		return t
	}
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
}

func tableBorder(widths []int, left, mid, right string, st *lipgloss.Style, srcLine int) Line {
	spans := []Span{{Text: left, Style: st}}
	for i, w := range widths {
		sep := mid
		if i == len(widths)-1 {
			sep = right
		}
		spans = append(spans,
			Span{Text: strings.Repeat("─", w+2), Style: st},
			Span{Text: sep, Style: st})
	}
	return Line{Spans: spans, SourceLine: srcLine}
}

func tableRow(cells []tableCell, widths []int, ncol int, border *lipgloss.Style, srcLine int) Line {
	var spans []Span
	for i := 0; i < ncol; i++ {
		spans = append(spans, Span{Text: "│ ", Style: border})
		var content []Span
		align := extast.AlignNone
		if i < len(cells) {
			content = truncateSpans(cells[i].spans, widths[i])
			align = cells[i].align
		}
		pad := widths[i] - spansWidth(content)
		left, right := alignPad(align, pad)
		if left > 0 {
			spans = append(spans, Span{Text: strings.Repeat(" ", left)})
		}
		spans = append(spans, content...)
		spans = append(spans, Span{Text: strings.Repeat(" ", right) + " "})
	}
	spans = append(spans, Span{Text: "│", Style: border})
	return Line{Spans: spans, SourceLine: srcLine}
}

func alignPad(align extast.Alignment, pad int) (left, right int) {
	switch align {
	case extast.AlignRight:
		return pad, 0
	case extast.AlignCenter:
		return pad / 2, pad - pad/2
	default:
		return 0, pad
	}
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
