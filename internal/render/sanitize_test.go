package render_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

// hasControl reports C0/C1 controls, mirroring the renderer's own rule. Tab
// is allowed through; newlines are separators between rendered lines here.
func hasControl(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return (r < 0x20 && r != '\t' && r != '\n') || r == 0x7f || (r >= 0x80 && r <= 0x9f)
	}) >= 0
}

// TestSanitizeControlChars walks every block type that carries document text
// and asserts no control character survives into the IR. theme.Plain() emits
// no styling of its own, so any control found came from the document.
func TestSanitizeControlChars(t *testing.T) {
	const esc = "\x1b[31mRED\x1b[0m"
	cases := []struct {
		name string
		src  string
	}{
		{"paragraph", "text " + esc + " more\n"},
		{"heading", "# head " + esc + "\n"},
		{"fenced code", "```go\nx := \"" + esc + "\"\n```\n"},
		{"indented code", "    " + esc + "\n"},
		{"html block", "<div>" + esc + "</div>\n"},
		{"inline html", "a <b>" + esc + "</b> c\n"},
		{"code span", "a `" + esc + "` b\n"},
		{"emphasis", "*" + esc + "* and **" + esc + "**\n"},
		{"strikethrough", "~~" + esc + "~~\n"},
		{"blockquote", "> quoted " + esc + "\n"},
		{"list item", "- item " + esc + "\n- second\n"},
		{"ordered list", "1. item " + esc + "\n"},
		{"task list", "- [x] done " + esc + "\n"},
		{"table cell", "| a | b |\n|---|---|\n| " + esc + " | 2 |\n"},
		{"link label", "[" + esc + "](https://example.com)\n"},
		{"link target", "[label](https://example.com/" + esc + ")\n"},
		{"autolink", "<https://example.com/" + esc + ">\n"},
		{"image alt", "![" + esc + "](pic.png)\n"},
		{"inline math", "$x " + esc + " y$\n"},
		{"display math", "$$\n" + esc + "\n$$\n"},
		{"bare DEL", "text \x7f more\n"},
		{"C1 CSI", "text \u009b31m more\n"},
		{"NUL", "text \x00 more\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, ln := range render.Render(parser.Parse([]byte(tc.src)), theme.Plain(), 40) {
				for _, sp := range ln.Spans {
					if hasControl(sp.Text) {
						t.Errorf("control character survived into span: %q", sp.Text)
					}
				}
				if got := ln.String(); hasControl(got) {
					t.Errorf("control character survived into output: %q", got)
				}
			}
		})
	}
}

// TestOSC8LinkNoBreakout covers the sharpest case: a link destination is
// never displayed, but it is interpolated into an OSC 8 sequence. A raw
// ESC \ inside it would terminate the sequence early and hand everything
// after it to the terminal as input.
func TestOSC8LinkNoBreakout(t *testing.T) {
	defer lipgloss.SetColorProfile(termenv.Ascii) // TestMain's setting
	lipgloss.SetColorProfile(termenv.TrueColor)

	src := "[label](https://example.com/y\x1b\\INJECTED)\n"
	var out strings.Builder
	for _, ln := range render.Render(parser.Parse([]byte(src)), theme.Plain(), 40) {
		out.WriteString(ln.String())
	}
	got := out.String()

	if strings.Contains(got, "\x1b\\INJECTED") {
		t.Fatalf("link destination broke out of the OSC 8 sequence: %q", got)
	}
	if !strings.Contains(got, "%1B") {
		t.Errorf("expected the ESC to be percent-encoded, got %q", got)
	}
	// The label must still render, and the hyperlink must still be emitted.
	if !strings.Contains(got, "label") || !strings.Contains(got, "\x1b]8;;https://example.com/y") {
		t.Errorf("sanitizing dropped the hyperlink or its label: %q", got)
	}
}

// TestSanitizePreservesOrdinaryText guards against over-eager replacement:
// tabs, CJK, emoji, and combining marks must pass through untouched.
func TestSanitizePreservesOrdinaryText(t *testing.T) {
	const src = "日本語 café 🖼 naïve — dash\n\n```\n\tindented\n```\n"
	var out strings.Builder
	for _, ln := range render.Render(parser.Parse([]byte(src)), theme.Plain(), 60) {
		out.WriteString(ln.String())
		out.WriteString("\n")
	}
	got := out.String()
	for _, want := range []string{"日本語", "café", "🖼", "naïve", "— dash", "indented"} {
		if !strings.Contains(got, want) {
			t.Errorf("sanitizing mangled %q; got %q", want, got)
		}
	}
	if strings.ContainsRune(got, '�') {
		t.Errorf("ordinary text produced a replacement character: %q", got)
	}
}
