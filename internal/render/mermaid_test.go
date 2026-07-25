package render

// Internal test: the point is the subprocess contract and the fallback,
// both of which need the unexported Options plumbing.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ernsoylu/MDView/internal/img"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

const mermaidDoc = "# D\n\n```mermaid\ngraph TD;\n  A-->B;\n```\n"

// fakeMmdc writes a real PNG at the -o path, standing in for mermaid-cli
// so the subprocess path is exercised without a Node toolchain.
func fakeMmdc(t *testing.T, behaviour string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub renderer is a shell script")
	}
	dir := t.TempDir()

	// A 2x2 PNG, base64'd, for the stub to emit.
	src, err := os.ReadFile(filepath.Join("..", "..", "examples", "gradient.png"))
	if err != nil {
		t.Fatal(err)
	}
	png := filepath.Join(dir, "canned.png")
	if err := os.WriteFile(png, src, 0o644); err != nil {
		t.Fatal(err)
	}

	script := "#!/bin/sh\n"
	switch behaviour {
	case "ok":
		script += `out=""
while [ $# -gt 0 ]; do
  case "$1" in -o) out="$2"; shift 2;; *) shift;; esac
done
cp ` + png + ` "$out"
`
	case "fail":
		script += "echo 'Parse error on line 2' >&2\nexit 1\n"
	}
	bin := filepath.Join(dir, "mmdc")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func renderMermaidDoc(t *testing.T, cmd string) []Line {
	t.Helper()
	mermaidCache.reset()
	doc := parser.Parse([]byte(mermaidDoc))
	lines, _ := RenderDoc(doc, theme.Plain(), 60, Options{
		Images:     ImagesHalfblock,
		IDs:        img.NewRegistry(),
		MermaidCmd: cmd,
	})
	return lines
}

func plainLines(lines []Line) string {
	var b strings.Builder
	for _, ln := range lines {
		b.WriteString(ln.Plain())
		b.WriteString("\n")
	}
	return b.String()
}

// TestMermaidDisabledStaysCode is the default: without the config key the
// fence must render exactly as it did before.
func TestMermaidDisabledStaysCode(t *testing.T) {
	got := plainLines(renderMermaidDoc(t, ""))
	if !strings.Contains(got, "graph TD;") {
		t.Errorf("disabled mermaid should stay a code block, got %q", got)
	}
}

func TestMermaidRendersThroughTheImagePipeline(t *testing.T) {
	got := plainLines(renderMermaidDoc(t, fakeMmdc(t, "ok")))
	if strings.Contains(got, "graph TD;") {
		t.Errorf("rendered mermaid should not also show its source: %q", got)
	}
	if !strings.Contains(got, "▀") {
		t.Errorf("expected mosaic cells from the rendered diagram, got %q", got)
	}
}

// TestMermaidFailureFallsBackToCode: a diagram that does not compile must
// leave the reader with the source, not a blank space.
func TestMermaidFailureFallsBackToCode(t *testing.T) {
	got := plainLines(renderMermaidDoc(t, fakeMmdc(t, "fail")))
	if !strings.Contains(got, "graph TD;") {
		t.Errorf("a failed render should fall back to the code block, got %q", got)
	}
}

// TestMermaidImagesOffStaysCode: piped output has no images at all, so the
// fence must stay text there too.
func TestMermaidImagesOffStaysCode(t *testing.T) {
	mermaidCache.reset()
	doc := parser.Parse([]byte(mermaidDoc))
	lines, _ := RenderDoc(doc, theme.Plain(), 60, Options{
		Images:     ImagesOff,
		MermaidCmd: fakeMmdc(t, "ok"),
	})
	if !strings.Contains(plainLines(lines), "graph TD;") {
		t.Error("with images off the fence should stay a code block")
	}
}

// TestMermaidMemoized guards the reason the cache exists: mmdc drives a
// browser, so a resize must not re-run it per frame.
func TestMermaidMemoized(t *testing.T) {
	bin := fakeMmdc(t, "ok")
	counter := bin + ".count"

	// Wrap the stub so each invocation leaves a mark.
	wrapper := filepath.Join(t.TempDir(), "mmdc")
	script := fmt.Sprintf("#!/bin/sh\necho x >> %s\nexec %s \"$@\"\n", counter, bin)
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	mermaidCache.reset()
	doc := parser.Parse([]byte(mermaidDoc))
	opts := Options{Images: ImagesHalfblock, IDs: img.NewRegistry(), MermaidCmd: wrapper}
	for i := 0; i < 4; i++ {
		RenderDoc(doc, theme.Plain(), 60, opts)
	}

	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("the stub renderer never ran: %v", err)
	}
	if runs := strings.Count(string(data), "x"); runs != 1 {
		t.Errorf("mermaid ran %d times across four renders, want 1", runs)
	}
}
