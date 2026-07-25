package render

import (
	"context"
	"fmt"
	"hash/crc32"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/ast"

	"github.com/ernsoylu/MDView/internal/img"
)

const (
	// mermaidTimeout bounds the renderer. mmdc drives a headless browser,
	// so seconds are normal and a hang is not worth waiting out.
	mermaidTimeout   = 30 * time.Second
	mermaidMaxRows   = 24
	maxCachedMermaid = 64 // diagrams are larger than math rasters
)

var mermaidCache = newRasterCache(maxCachedMermaid)

// mermaidBlock renders a ```mermaid fence as a picture, falling back to the
// ordinary highlighted code block when rendering is off, the renderer is
// missing, or the diagram does not compile.
func (r *renderer) mermaidBlock(n *ast.FencedCodeBlock, width int) []Line {
	if r.opts.MermaidCmd == "" || r.opts.Images == ImagesOff {
		return nil
	}
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		sg := n.Lines().At(i)
		b.Write(sg.Value(r.src))
	}
	src := strings.TrimSpace(b.String())
	if src == "" {
		return nil
	}

	theme := "default"
	if lipgloss.HasDarkBackground() {
		theme = "dark"
	}
	key := fmt.Sprintf("%08x|%s|%s", crc32.ChecksumIEEE([]byte(src)), theme, r.opts.MermaidCmd)
	m, err := mermaidCache.do(key, func() (image.Image, error) {
		return rasterizeMermaid(r.opts.MermaidCmd, src, theme)
	})
	if err != nil {
		return nil
	}
	bounds := m.Bounds()
	cols, rows := img.Fit(bounds.Dx(), bounds.Dy(), width, mermaidMaxRows)
	return r.pictureLines(m, "mermaid|"+key, cols, rows, nodeLine(r.doc, n))
}

// rasterizeMermaid shells out to mermaid-cli. The diagram source comes from
// the document, so it is written to a file and passed by path — never
// through a shell — and the renderer is only ever reached when the user has
// turned it on.
func rasterizeMermaid(bin, src, theme string) (image.Image, error) {
	dir, err := os.MkdirTemp("", "mdv-mermaid")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	in := filepath.Join(dir, "diagram.mmd")
	out := filepath.Join(dir, "diagram.png")
	if err := os.WriteFile(in, []byte(src), 0o600); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), mermaidTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-i", in, "-o", out, "-b", "transparent", "-t", theme)
	if combined, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("mermaid: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	return img.Load(out)
}

// LookupMermaid resolves the mermaid renderer on PATH, returning "" when it
// is not installed. Kept here so the CLI does not need to know the binary's
// name.
func LookupMermaid() string {
	path, err := exec.LookPath("mmdc")
	if err != nil {
		return ""
	}
	return path
}
