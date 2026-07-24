package render_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestMain(m *testing.M) {
	// Force the mono profile so golden output is identical everywhere.
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func renderFile(t *testing.T, name string, width int) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, ln := range render.Render(parser.Parse(src), theme.Plain(), width) {
		b.WriteString(ln.String())
		b.WriteString("\n")
	}
	return b.String()
}

func TestGolden(t *testing.T) {
	cases := []struct {
		file  string
		width int
	}{
		{"elements.md", 40},
		{"lists.md", 40},
		{"table_code.md", 60},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			got := renderFile(t, tc.file, tc.width)
			golden := filepath.Join("testdata", strings.TrimSuffix(tc.file, ".md")+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden file (run go test -update): %v", err)
			}
			if got != string(want) {
				t.Errorf("golden mismatch for %s\ngot:\n%s\nwant:\n%s", tc.file, got, want)
			}
		})
	}
}

func TestWidthInvariant(t *testing.T) {
	for _, file := range []string{"elements.md", "lists.md", "table_code.md"} {
		for _, width := range []int{20, 40, 79} {
			src, err := os.ReadFile(filepath.Join("testdata", file))
			if err != nil {
				t.Fatal(err)
			}
			for i, ln := range render.Render(parser.Parse(src), theme.Plain(), width) {
				if w := ln.Width(); w > width {
					t.Errorf("%s at width %d: line %d is %d cells wide: %q", file, width, i, w, ln.String())
				}
			}
		}
	}
}

func TestCJKWrap(t *testing.T) {
	src := []byte("日本語のテキストを折り返すテスト です\n")
	for _, ln := range render.Render(parser.Parse(src), theme.Plain(), 10) {
		if w := ln.Width(); w > 10 {
			t.Errorf("CJK line %q is %d cells wide, want <= 10", ln.String(), w)
		}
	}
}

func TestLongWordChunks(t *testing.T) {
	src := []byte("see https://example.com/a/very/long/path/that/cannot/fit/on/one/line/at/all\n")
	lines := render.Render(parser.Parse(src), theme.Plain(), 20)
	if len(lines) < 2 {
		t.Fatalf("expected the URL to chunk over multiple lines, got %d line(s)", len(lines))
	}
	var joined strings.Builder
	for _, ln := range lines {
		for _, sp := range ln.Spans {
			joined.WriteString(sp.Text)
		}
	}
	if !strings.Contains(strings.ReplaceAll(joined.String(), " ", ""), "that/cannot/fit") {
		t.Errorf("chunked URL lost content: %q", joined.String())
	}
}

func TestSourceLineMapping(t *testing.T) {
	src := []byte("# Title\n\npara\n\n## Second\n\n- item\n")
	lines := render.Render(parser.Parse(src), theme.Plain(), 40)
	find := func(sub string) render.Line {
		for _, ln := range lines {
			if strings.Contains(ln.String(), sub) {
				return ln
			}
		}
		t.Fatalf("no rendered line contains %q", sub)
		return render.Line{}
	}
	for _, tc := range []struct {
		sub  string
		want int
	}{
		{"Title", 1}, {"para", 3}, {"Second", 5}, {"item", 7},
	} {
		if got := find(tc.sub).SourceLine; got != tc.want {
			t.Errorf("SourceLine of %q = %d, want %d", tc.sub, got, tc.want)
		}
	}
}

func FuzzRender(f *testing.F) {
	for _, s := range []string{
		"# h\n\npara *em* [l](https://u)\n",
		"- a\n- b\n\n| a | b |\n|---|---|\n| 1 | 2 |\n",
		"```go\nfunc f() {}\n```\n",
		"> q\n>\n> > deep\n",
		"日本語 ~~strike~~ `code`\n",
		"1. a\n2. b\n\n---\n\n<div>\nx\n</div>\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, ln := range render.Render(parser.Parse([]byte(s)), theme.Plain(), 30) {
			for _, sp := range ln.Spans {
				if strings.ContainsRune(sp.Text, '\n') {
					t.Fatalf("newline inside span: %q", sp.Text)
				}
			}
		}
	})
}

// BenchmarkRenderProse isolates parse+layout cost from chroma highlighting.
func BenchmarkRenderProse(b *testing.B) {
	block := "# Section\n\nSome *paragraph* text with a [link](https://example.com) and `code` that wraps around nicely.\n\n- item one\n- item two\n\n> a quote with some wrapped text in it\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n"
	src := []byte(strings.Repeat(block, 1200)) // ~250 KB
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		render.Render(parser.Parse(src), theme.Plain(), 80)
	}
}

func BenchmarkRender(b *testing.B) {
	block := "# Section\n\nSome *paragraph* text with a [link](https://example.com) and `code` that wraps around nicely.\n\n- item one\n- item two\n\n```go\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n"
	src := []byte(strings.Repeat(block, 1200)) // ~250 KB
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		render.Render(parser.Parse(src), theme.Plain(), 80)
	}
}
