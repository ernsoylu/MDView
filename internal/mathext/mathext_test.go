package mathext

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func parse(t *testing.T, src string) ast.Node {
	t.Helper()
	md := goldmark.New(goldmark.WithExtensions(Extension))
	return md.Parser().Parse(text.NewReader([]byte(src)))
}

func collect(root ast.Node, kind ast.NodeKind) []ast.Node {
	var out []ast.Node
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == kind {
			out = append(out, n)
		}
		return ast.WalkContinue, nil
	})
	return out
}

func TestInlineMath(t *testing.T) {
	root := parse(t, "Euler: $e^{i\\pi} + 1 = 0$ indeed.\n")
	nodes := collect(root, KindInlineMath)
	if len(nodes) != 1 {
		t.Fatalf("%d inline math nodes, want 1", len(nodes))
	}
	if got := string(nodes[0].(*InlineMath).Value); got != `e^{i\pi} + 1 = 0` {
		t.Errorf("value = %q", got)
	}
}

func TestDollarAmountsStayText(t *testing.T) {
	for _, src := range []string{
		"It costs $5 and $10 in cash.\n",
		"Save $20 today, $30 tomorrow, $40 later.\n",
		"A lone $ sign.\n",
		"An empty $$ pair inline stays text.\n",
	} {
		if n := collect(parse(t, src), KindInlineMath); len(n) != 0 {
			t.Errorf("%q parsed as inline math", src)
		}
	}
}

func TestMathBlockMultiLine(t *testing.T) {
	src := "before\n\n$$\n\\int_0^1 x\\,dx\n= \\frac{1}{2}\n$$\n\nafter\n"
	blocks := collect(parse(t, src), KindMathBlock)
	if len(blocks) != 1 {
		t.Fatalf("%d math blocks, want 1", len(blocks))
	}
	b := blocks[0].(*MathBlock)
	var content strings.Builder
	for i := 0; i < b.Lines().Len(); i++ {
		sg := b.Lines().At(i)
		content.Write(sg.Value([]byte(src)))
	}
	got := content.String()
	if !strings.Contains(got, `\int_0^1`) || !strings.Contains(got, `\frac{1}{2}`) {
		t.Errorf("block content = %q", got)
	}
	if strings.Contains(got, "$$") {
		t.Errorf("delimiters leaked into content: %q", got)
	}
}

func TestMathBlockSingleLine(t *testing.T) {
	blocks := collect(parse(t, "$$E = mc^2$$\n\ntext\n"), KindMathBlock)
	if len(blocks) != 1 {
		t.Fatalf("%d math blocks, want 1", len(blocks))
	}
	b := blocks[0].(*MathBlock)
	if b.Lines().Len() != 1 {
		t.Fatalf("lines = %d, want 1", b.Lines().Len())
	}
	sg := b.Lines().At(0)
	if got := string(sg.Value([]byte("$$E = mc^2$$\n\ntext\n"))); got != "E = mc^2" {
		t.Errorf("content = %q", got)
	}
}

func TestNoMathUnaffected(t *testing.T) {
	src := "# Plain\n\nJust *markdown* here.\n"
	root := parse(t, src)
	if n := collect(root, KindInlineMath); len(n) != 0 {
		t.Error("phantom inline math")
	}
	if n := collect(root, KindMathBlock); len(n) != 0 {
		t.Error("phantom math block")
	}
}
