package render

import (
	"github.com/yuin/goldmark/ast"

	"github.com/ernsoylu/MDView/internal/parser"
)

// OutlineEntry is one heading of the document, for the TOC jump.
type OutlineEntry struct {
	Level      int // 1–6
	Text       string
	SourceLine int
}

// Outline lists the document's headings in order, wherever they appear
// (including inside blockquotes and lists).
func Outline(doc *parser.Doc) []OutlineEntry {
	var out []OutlineEntry
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if h, ok := c.(*ast.Heading); ok {
				out = append(out, OutlineEntry{
					Level:      h.Level,
					Text:       Sanitize(nodeText(h, doc.Source)),
					SourceLine: nodeLine(doc, h),
				})
				continue
			}
			walk(c)
		}
	}
	walk(doc.Root)
	return out
}
