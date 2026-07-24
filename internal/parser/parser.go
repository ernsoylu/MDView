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
