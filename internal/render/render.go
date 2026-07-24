package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

type renderer struct {
	doc        *parser.Doc
	src        []byte
	th         theme.Theme
	listDepth  int
	styleCache map[chroma.TokenType]*lipgloss.Style
}

// Render lays out a parsed document at the given width.
func Render(doc *parser.Doc, th theme.Theme, width int) []Line {
	if width < 8 {
		width = 8
	}
	r := &renderer{
		doc:        doc,
		src:        doc.Source,
		th:         th,
		styleCache: make(map[chroma.TokenType]*lipgloss.Style),
	}
	return r.blocks(doc.Root, width, false)
}

// blocks renders the block children of parent, separated by blank lines
// unless tight (tight list items).
func (r *renderer) blocks(parent ast.Node, width int, tight bool) []Line {
	var out []Line
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		bl := r.block(c, width)
		if len(bl) == 0 {
			continue
		}
		if len(out) > 0 && !tight {
			out = append(out, Line{SourceLine: bl[0].SourceLine})
		}
		out = append(out, bl...)
	}
	return out
}

func (r *renderer) block(n ast.Node, width int) []Line {
	switch n := n.(type) {
	case *ast.Heading:
		return wrap(r.inlines(n, &r.th.Heading[n.Level-1]), width, r.nodeLine(n))
	case *ast.Paragraph:
		return wrap(r.inlines(n, nil), width, r.nodeLine(n))
	case *ast.TextBlock:
		return wrap(r.inlines(n, nil), width, r.nodeLine(n))
	case *ast.Blockquote:
		return r.blockquote(n, width)
	case *ast.List:
		return r.list(n, width)
	case *ast.FencedCodeBlock:
		return r.code(n.Lines(), string(n.Language(r.src)), width)
	case *ast.CodeBlock:
		return r.code(n.Lines(), "", width)
	case *ast.ThematicBreak:
		return []Line{{Spans: []Span{{Text: strings.Repeat("─", width), Style: &r.th.Rule}}}}
	case *extast.Table:
		return r.table(n, width)
	case *ast.HTMLBlock:
		return r.htmlBlock(n, width)
	default:
		return r.blocks(n, width, false)
	}
}

func (r *renderer) blockquote(n *ast.Blockquote, width int) []Line {
	innerW := width - 2
	if innerW < 4 {
		innerW = 4
	}
	inner := r.blocks(n, innerW, false)
	out := make([]Line, 0, len(inner))
	for _, ln := range inner {
		bar := Span{Text: "│ ", Style: &r.th.QuoteBar}
		if len(ln.Spans) == 0 {
			bar.Text = "│"
		}
		out = append(out, Line{Spans: prepend(bar, ln.Spans), SourceLine: ln.SourceLine})
	}
	return out
}

var bullets = []string{"•", "◦", "▪"}

func (r *renderer) list(n *ast.List, width int) []Line {
	depth := r.listDepth
	r.listDepth++
	defer func() { r.listDepth-- }()

	markerW := 2
	num := n.Start
	if n.IsOrdered() {
		items := 0
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			items++
		}
		markerW = len(strconv.Itoa(n.Start+items-1)) + 2
	}
	innerW := width - markerW
	if innerW < 4 {
		innerW = 4
	}

	var out []Line
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		content := r.blocks(c, innerW, n.IsTight)
		if len(content) == 0 {
			content = []Line{{SourceLine: r.nodeLine(c)}}
		}
		var marker string
		if n.IsOrdered() {
			marker = fmt.Sprintf("%*d. ", markerW-2, num)
			num++
		} else {
			marker = bullets[depth%len(bullets)] + " "
		}
		if len(out) > 0 && !n.IsTight {
			out = append(out, Line{SourceLine: content[0].SourceLine})
		}
		for i, ln := range content {
			switch {
			case i == 0:
				pre := Span{Text: marker, Style: &r.th.ListMarker}
				out = append(out, Line{Spans: prepend(pre, ln.Spans), SourceLine: ln.SourceLine})
			case len(ln.Spans) == 0:
				out = append(out, ln)
			default:
				pre := Span{Text: strings.Repeat(" ", markerW)}
				out = append(out, Line{Spans: prepend(pre, ln.Spans), SourceLine: ln.SourceLine})
			}
		}
	}
	return out
}

// prepend builds a right-sized span slice with p in front of spans.
func prepend(p Span, spans []Span) []Span {
	out := make([]Span, 0, len(spans)+1)
	out = append(out, p)
	return append(out, spans...)
}

func (r *renderer) code(ls *text.Segments, lang string, width int) []Line {
	chunkW := width - 2
	if chunkW < 4 {
		chunkW = 4
	}
	var sb strings.Builder
	starts := make([]int, 0, ls.Len())
	for i := 0; i < ls.Len(); i++ {
		sg := ls.At(i)
		starts = append(starts, sg.Start)
		sb.Write(sg.Value(r.src))
	}
	code := strings.TrimRight(sb.String(), "\n")
	if code == "" {
		return nil
	}
	var out []Line
	for i, row := range r.highlight(code, lang) {
		srcLine := 0
		if i < len(starts) {
			srcLine = r.doc.LineOf(starts[i])
		}
		chunks := chunkSpans(row, chunkW)
		if len(chunks) == 0 {
			out = append(out, Line{SourceLine: srcLine})
			continue
		}
		for _, chunk := range chunks {
			out = append(out, Line{Spans: prepend(Span{Text: "  "}, chunk), SourceLine: srcLine})
		}
	}
	return out
}

// highlight tokenizes code with chroma and returns one span row per code
// line, styled through the theme's chroma palette.
func (r *renderer) highlight(code, lang string) [][]Span {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		var rows [][]Span
		for _, l := range strings.Split(code, "\n") {
			rows = append(rows, []Span{{Text: expandTabs(l), Style: &r.th.CodeBlock}})
		}
		return rows
	}
	var rows [][]Span
	var cur []Span
	for tok := it(); tok != chroma.EOF; tok = it() {
		st := r.tokenStyle(tok.Type)
		for pi, part := range strings.Split(tok.Value, "\n") {
			if pi > 0 {
				rows = append(rows, cur)
				cur = nil
			}
			if part != "" {
				cur = append(cur, Span{Text: expandTabs(part), Style: st})
			}
		}
	}
	rows = append(rows, cur)
	return rows
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

func (r *renderer) tokenStyle(t chroma.TokenType) *lipgloss.Style {
	if st, ok := r.styleCache[t]; ok {
		return st
	}
	entry := r.th.Chroma.Get(t)
	st := r.th.CodeBlock
	if entry.Colour.IsSet() {
		st = st.Foreground(lipgloss.Color(entry.Colour.String()))
	}
	if entry.Bold == chroma.Yes {
		st = st.Bold(true)
	}
	if entry.Italic == chroma.Yes {
		st = st.Italic(true)
	}
	if entry.Underline == chroma.Yes {
		st = st.Underline(true)
	}
	r.styleCache[t] = &st
	return &st
}

func (r *renderer) htmlBlock(n *ast.HTMLBlock, width int) []Line {
	var out []Line
	emit := func(sg text.Segment) {
		txt := strings.TrimRight(string(sg.Value(r.src)), "\n")
		srcLine := r.doc.LineOf(sg.Start)
		chunks := chunkSpans([]Span{{Text: expandTabs(txt), Style: &r.th.Dim}}, width)
		if len(chunks) == 0 {
			out = append(out, Line{SourceLine: srcLine})
			return
		}
		for _, chunk := range chunks {
			out = append(out, Line{Spans: chunk, SourceLine: srcLine})
		}
	}
	ls := n.Lines()
	for i := 0; i < ls.Len(); i++ {
		emit(ls.At(i))
	}
	if n.HasClosure() {
		emit(n.ClosureLine)
	}
	return out
}

// nodeLine best-effort maps a node to the 1-based source line it starts on.
func (r *renderer) nodeLine(n ast.Node) int {
	if n == nil {
		return 0
	}
	if t, ok := n.(*ast.Text); ok {
		return r.doc.LineOf(t.Segment.Start)
	}
	if lb, ok := n.(interface{ Lines() *text.Segments }); ok && lb.Lines().Len() > 0 {
		sg := lb.Lines().At(0)
		return r.doc.LineOf(sg.Start)
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if l := r.nodeLine(c); l > 0 {
			return l
		}
	}
	return 0
}
