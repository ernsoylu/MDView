// Package parser wraps goldmark configured for CommonMark 0.31.2 + GFM and
// pairs the AST with a byte-offset → source-line index, which the renderer
// uses to stamp every rendered row with the markdown line it came from.
package parser

import (
	"sort"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"

	"github.com/ernsoylu/MDView/internal/mathext"
)

// Doc is a parsed markdown document together with its source, which goldmark
// AST nodes reference by byte offset.
type Doc struct {
	Root        ast.Node
	Source      []byte
	lineOffsets []int
}

func Parse(source []byte) *Doc {
	md := goldmark.New(goldmark.WithExtensions(extension.GFM, mathext.Extension))
	root := md.Parser().Parse(text.NewReader(source))
	offsets := []int{0}
	for i, b := range source {
		if b == '\n' && i+1 < len(source) {
			offsets = append(offsets, i+1)
		}
	}
	return &Doc{Root: root, Source: source, lineOffsets: offsets}
}

// LineOf returns the 1-based source line containing the given byte offset.
func (d *Doc) LineOf(offset int) int {
	return sort.SearchInts(d.lineOffsets, offset+1)
}

// LineCount returns the number of source lines.
func (d *Doc) LineCount() int { return len(d.lineOffsets) }

// SourceLines returns the source text of the 1-based line range [from, to],
// clamped to the document and always newline-terminated.
func (d *Doc) SourceLines(from, to int) []byte {
	if from < 1 {
		from = 1
	}
	if to > len(d.lineOffsets) {
		to = len(d.lineOffsets)
	}
	if from > to {
		return nil
	}
	start := d.lineOffsets[from-1]
	end := len(d.Source)
	if to < len(d.lineOffsets) {
		end = d.lineOffsets[to]
	}
	out := d.Source[start:end]
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(append([]byte(nil), out...), '\n')
	}
	return out
}
