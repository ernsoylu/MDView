package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

// captureStdout runs f with os.Stdout redirected, returning what it wrote.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	f()
	_ = w.Close()
	os.Stdout = saved
	return <-done
}

// dump is what "curl … | mdv" falls back to when no terminal can supply
// the keys, so it has to produce the document under any width setting.
func TestDumpWidth(t *testing.T) {
	doc := parser.Parse([]byte("# H\n\n" + strings.Repeat("word ", 40) + "\n"))

	cases := []struct {
		name      string
		columns   string
		widthFlag int
		want      int
	}{
		{"default", "", 0, 80},
		{"COLUMNS honoured", "40", 0, 40},
		{"flag beats COLUMNS", "40", 30, 30},
		// A COLUMNS of 0 or junk must not become the width: this shell
		// exports COLUMNS=0, and a width of 0 renders one column of text.
		{"COLUMNS zero ignored", "0", 0, 80},
		{"COLUMNS junk ignored", "not-a-number", 0, 80},
		{"COLUMNS negative ignored", "-5", 0, 80},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("COLUMNS", tc.columns)
			assertDumpWidth(t, doc, tc.widthFlag, tc.want)
		})
	}
}

func assertDumpWidth(t *testing.T, doc *parser.Doc, widthFlag, want int) {
	t.Helper()
	out := captureStdout(t, func() { dump(doc, theme.Plain(), widthFlag) })
	if strings.TrimSpace(out) == "" {
		t.Fatal("dump produced nothing")
	}
	widest := 0
	for _, ln := range strings.Split(out, "\n") {
		if n := len([]rune(ln)); n > widest {
			widest = n
		}
	}
	if widest > want {
		t.Errorf("widest line is %d cells, want <= %d", widest, want)
	}
	// Guard against a width so small the text is shredded: real
	// wrapping should get reasonably close to the target.
	if want >= 30 && widest < want/2 {
		t.Errorf("widest line is only %d cells at width %d; text looks over-wrapped", widest, want)
	}
}

func TestDumpRendersContent(t *testing.T) {
	doc := parser.Parse([]byte("# Title\n\nsome **bold** text\n"))
	t.Setenv("COLUMNS", "")
	out := captureStdout(t, func() { dump(doc, theme.Plain(), 0) })
	for _, want := range []string{"Title", "some bold text"} {
		if !strings.Contains(out, want) {
			t.Errorf("dump output missing %q; got %q", want, out)
		}
	}
}
