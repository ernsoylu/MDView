package render

import (
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/ernsoylu/MDView/internal/parser"
)

// CodeBlock is a fenced or indented code block with its source-line range,
// used by the yank command.
type CodeBlock struct {
	Start, End int // 1-based source lines, inclusive
	Content    string
}

// CodeBlocks lists the document's code blocks in order.
func CodeBlocks(doc *parser.Doc) []CodeBlock {
	var out []CodeBlock
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			if k := c.Kind(); k == ast.KindFencedCodeBlock || k == ast.KindCodeBlock {
				ls := c.Lines()
				if ls.Len() == 0 {
					continue
				}
				var b strings.Builder
				for i := 0; i < ls.Len(); i++ {
					sg := ls.At(i)
					b.Write(sg.Value(doc.Source))
				}
				first := ls.At(0)
				last := ls.At(ls.Len() - 1)
				out = append(out, CodeBlock{
					Start:   doc.LineOf(first.Start),
					End:     doc.LineOf(last.Stop - 1),
					Content: strings.TrimRight(b.String(), "\n"),
				})
				continue
			}
			walk(c)
		}
	}
	walk(doc.Root)
	return out
}
