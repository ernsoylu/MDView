package render

import (
	"html"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"

	"github.com/ernsoylu/MDView/internal/mathext"
	"github.com/ernsoylu/MDView/internal/theme"
)

// seg is a styled fragment of inline content before line breaking.
type seg struct {
	text  string
	style *lipgloss.Style
	link  string
	brk   bool // hard line break
}

// inlines flattens the inline children of a block node into a seg stream.
// base may be nil for unstyled text. Every inline path bottoms out here, so
// this is where document text loses its control characters.
func (r *renderer) inlines(parent ast.Node, base *lipgloss.Style) []seg {
	var out []seg
	r.inlineChildren(parent, base, "", &out)
	for i := range out {
		out[i].text = sanitize(out[i].text)
	}
	return out
}

// merged combines a theme style with the inherited surrounding style. With
// no surrounding style the theme pointer is shared as-is, so the common case
// allocates nothing.
func merged(specific, base *lipgloss.Style) *lipgloss.Style {
	if base == nil {
		return specific
	}
	m := specific.Inherit(*base)
	return &m
}

func (r *renderer) inlineChildren(parent ast.Node, style *lipgloss.Style, link string, out *[]seg) {
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		switch n := c.(type) {
		case *ast.Text:
			r.appendText(n, style, link, out)
		case *ast.String:
			*out = append(*out, seg{text: string(n.Value), style: style, link: link})
		case *ast.CodeSpan:
			r.appendCodeSpan(n, style, link, out)
		case *ast.Emphasis:
			r.inlineChildren(n, merged(emphStyle(&r.th, n.Level), style), link, out)
		case *extast.Strikethrough:
			r.inlineChildren(n, merged(&r.th.Strike, style), link, out)
		case *ast.Link:
			r.inlineChildren(n, merged(&r.th.Link, style), string(n.Destination), out)
		case *ast.AutoLink:
			url := string(n.URL(r.src))
			*out = append(*out, seg{text: string(n.Label(r.src)), style: merged(&r.th.Link, style), link: url})
		case *ast.Image:
			r.appendImage(n, style, out)
		case *extast.TaskCheckBox:
			*out = append(*out, seg{text: taskGlyph(n.IsChecked), style: &r.th.ListMarker})
		case *mathext.InlineMath:
			// Inline math always shows its raw TeX, styled like a code span.
			*out = append(*out, seg{text: "$" + string(n.Value) + "$", style: merged(&r.th.CodeSpan, style), link: link})
		case *ast.RawHTML:
			r.appendRawHTML(n, out)
		default:
			r.inlineChildren(c, style, link, out)
		}
	}
}

func emphStyle(th *theme.Theme, level int) *lipgloss.Style {
	if level >= 2 {
		return &th.Strong
	}
	return &th.Emph
}

func taskGlyph(checked bool) string {
	if checked {
		return "[✓] "
	}
	return "[ ] "
}

func (r *renderer) appendText(n *ast.Text, style *lipgloss.Style, link string, out *[]seg) {
	*out = append(*out, seg{text: unescape(string(n.Segment.Value(r.src))), style: style, link: link})
	if n.HardLineBreak() {
		*out = append(*out, seg{brk: true})
	} else if n.SoftLineBreak() {
		*out = append(*out, seg{text: " ", style: style, link: link})
	}
}

func (r *renderer) appendCodeSpan(n *ast.CodeSpan, style *lipgloss.Style, link string, out *[]seg) {
	// Code span children carry raw segments where line endings are
	// literal newline bytes; CommonMark says they become spaces.
	st := merged(&r.th.CodeSpan, style)
	for gc := n.FirstChild(); gc != nil; gc = gc.NextSibling() {
		if t, ok := gc.(*ast.Text); ok {
			txt := strings.ReplaceAll(string(t.Segment.Value(r.src)), "\n", " ")
			*out = append(*out, seg{text: txt, style: st, link: link})
		}
	}
}

func (r *renderer) appendImage(n *ast.Image, style *lipgloss.Style, out *[]seg) {
	alt := nodeText(n, r.src)
	if alt == "" {
		alt = "image"
	}
	url := string(n.Destination)
	*out = append(*out,
		seg{text: "🖼 [Image: " + alt + "]", style: merged(&r.th.Image, style), link: url},
		seg{text: " (" + url + ")", style: &r.th.Dim, link: url},
	)
}

func (r *renderer) appendRawHTML(n *ast.RawHTML, out *[]seg) {
	var b strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		sg := n.Segments.At(i)
		b.Write(sg.Value(r.src))
	}
	*out = append(*out, seg{text: strings.ReplaceAll(b.String(), "\n", " "), style: &r.th.Dim})
}

// nodeText collects the plain text of a node's descendants (e.g. image alt).
func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch t := c.(type) {
			case *ast.Text:
				b.WriteString(unescape(string(t.Segment.Value(src))))
			case *ast.String:
				b.Write(t.Value)
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}

// unescape resolves CommonMark backslash escapes and HTML entity references
// in inline text — goldmark leaves both to the output writer, which this
// renderer replaces. Backslash escapes win over entities, so \&amp; stays
// the literal text "&amp;".
func unescape(s string) string {
	if !strings.ContainsAny(s, `\&`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && util.IsPunct(s[i+1]) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if c == '&' {
			// CommonMark entities always end in ';'; the longest HTML5
			// entity name is ~32 chars.
			if j := strings.IndexByte(s[i:min(len(s), i+34)], ';'); j > 1 {
				if cand := s[i : i+j+1]; html.UnescapeString(cand) != cand {
					b.WriteString(html.UnescapeString(cand))
					i += j
					continue
				}
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}
