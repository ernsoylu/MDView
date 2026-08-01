// Package mathext is a small goldmark extension parsing LaTeX math:
// $...$ inline and $$...$$ display blocks. Math is not CommonMark or GFM,
// so the rules are conservative — documents without math are unaffected,
// and dollar amounts like "$5 and $10" stay plain text.
package mathext

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// InlineMath is a $...$ span; Value holds the TeX between the dollars.
type InlineMath struct {
	ast.BaseInline
	Value []byte
}

var KindInlineMath = ast.NewNodeKind("InlineMath")

func (n *InlineMath) Kind() ast.NodeKind { return KindInlineMath }
func (n *InlineMath) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Value": string(n.Value)}, nil)
}

// MathBlock is a $$...$$ display block; the TeX lines are in Lines().
type MathBlock struct {
	ast.BaseBlock
	closed bool // single-line $$...$$ closes at Open time
}

var KindMathBlock = ast.NewNodeKind("MathBlock")

func (n *MathBlock) Kind() ast.NodeKind            { return KindMathBlock }
func (n *MathBlock) IsRaw() bool                   { return true }
func (n *MathBlock) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

type inlineParser struct{}

func (p inlineParser) Trigger() []byte { return []byte{'$'} }

func (p inlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) < 3 || line[0] != '$' || line[1] == '$' {
		return nil
	}
	// Opening $ must be followed by non-space; closing $ must be preceded
	// by non-space and must exist on the same line.
	if util.IsSpace(line[1]) {
		return nil
	}
	end := -1
	for j := 2; j < len(line); j++ {
		if line[j] == '$' && line[j-1] != '\\' && !util.IsSpace(line[j-1]) {
			end = j
			break
		}
		if line[j] == '\n' {
			break
		}
	}
	if end < 0 {
		return nil
	}
	n := &InlineMath{Value: bytes.Clone(line[1:end])}
	n.AppendChild(n, ast.NewTextSegment(text.NewSegment(seg.Start+1, seg.Start+end)))
	block.Advance(end + 1)
	return n
}

type blockParser struct{}

func (p blockParser) Trigger() []byte { return []byte{'$'} }

func (p blockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || pos+1 >= len(line) || line[pos] != '$' || line[pos+1] != '$' {
		return nil, parser.NoChildren
	}
	n := &MathBlock{}
	rest := bytes.TrimSpace(line[pos+2:])
	if len(rest) >= 2 && bytes.HasSuffix(rest, []byte("$$")) {
		// Single-line $$content$$: keep the inner segment only.
		start := seg.Start + pos + 2
		stop := seg.Start + bytes.LastIndex(line, []byte("$$"))
		if stop > start {
			n.Lines().Append(text.NewSegment(start, stop))
		}
		n.closed = true
	}
	return n, parser.NoChildren
}

func (p blockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	n := node.(*MathBlock)
	if n.closed {
		return parser.Close
	}
	line, seg := reader.PeekLine()
	if bytes.HasPrefix(bytes.TrimSpace(line), []byte("$$")) {
		reader.Advance(seg.Len() - 1)
		return parser.Close
	}
	node.Lines().Append(seg)
	return parser.Continue | parser.NoChildren
}

func (p blockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	// MathBlock lines are finalized in Continue; the closer $$ is not kept.
}
func (p blockParser) CanInterruptParagraph() bool { return true }
func (p blockParser) CanAcceptIndentedLine() bool { return false }

type extender struct{}

// Extension enables $...$ and $$...$$ math parsing.
var Extension goldmark.Extender = extender{}

func (extender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(util.Prioritized(blockParser{}, 701)),
		parser.WithInlineParsers(util.Prioritized(inlineParser{}, 501)),
	)
}
