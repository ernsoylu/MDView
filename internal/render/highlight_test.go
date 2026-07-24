package render_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

func firstLine(t *testing.T, md string) render.Line {
	t.Helper()
	lines := render.Render(parser.Parse([]byte(md)), theme.Plain(), 80)
	if len(lines) == 0 {
		t.Fatal("no lines rendered")
	}
	return lines[0]
}

func TestPlain(t *testing.T) {
	ln := firstLine(t, "some *styled* and [linked](https://x) text\n")
	if got, want := ln.Plain(), "some styled and linked text"; got != want {
		t.Errorf("Plain() = %q, want %q", got, want)
	}
}

func TestHighlightSplitsSpans(t *testing.T) {
	st := lipgloss.NewStyle().Reverse(true)
	ln := firstLine(t, "abc *def* ghi\n")
	plain := ln.Plain() // "abc def ghi"

	// Range crossing the styled span boundary: "c def g".
	start := strings.Index(plain, "c def g")
	hl := ln.Highlight([][2]int{{start, start + len("c def g")}}, &st)

	if got := hl.Plain(); got != plain {
		t.Fatalf("Highlight changed text: %q -> %q", plain, got)
	}
	// Every byte inside the range must carry the highlight style pointer.
	pos := 0
	for _, sp := range hl.Spans {
		spStart, spEnd := pos, pos+len(sp.Text)
		overlaps := spStart < start+len("c def g") && spEnd > start
		if overlaps && sp.Style != &st {
			t.Errorf("span %q inside range lacks highlight style", sp.Text)
		}
		if !overlaps && sp.Style == &st {
			t.Errorf("span %q outside range has highlight style", sp.Text)
		}
		pos = spEnd
	}
}

func TestHighlightPreservesLinks(t *testing.T) {
	st := lipgloss.NewStyle().Reverse(true)
	ln := firstLine(t, "[click here](https://example.com)\n")
	plain := ln.Plain()
	hl := ln.Highlight([][2]int{{0, len(plain)}}, &st)
	for _, sp := range hl.Spans {
		if sp.Link != "https://example.com" {
			t.Errorf("span %q lost its link: %q", sp.Text, sp.Link)
		}
	}
}

func TestHighlightMultipleRanges(t *testing.T) {
	st := lipgloss.NewStyle().Reverse(true)
	ln := firstLine(t, "one two one two one\n")
	plain := ln.Plain()
	var ranges [][2]int
	for off := 0; ; {
		i := strings.Index(plain[off:], "one")
		if i < 0 {
			break
		}
		ranges = append(ranges, [2]int{off + i, off + i + 3})
		off += i + 3
	}
	if len(ranges) != 3 {
		t.Fatalf("expected 3 ranges, got %d", len(ranges))
	}
	hl := ln.Highlight(ranges, &st)
	if got := hl.Plain(); got != plain {
		t.Fatalf("text changed: %q", got)
	}
	hits := 0
	for _, sp := range hl.Spans {
		if sp.Style == &st {
			hits++
			if sp.Text != "one" {
				t.Errorf("highlighted span = %q, want \"one\"", sp.Text)
			}
		}
	}
	if hits != 3 {
		t.Errorf("highlighted %d spans, want 3", hits)
	}
}

func TestHighlightNoRanges(t *testing.T) {
	ln := firstLine(t, "plain text\n")
	if got := ln.Highlight(nil, nil); got.Plain() != ln.Plain() {
		t.Errorf("no-range highlight changed the line")
	}
}

func TestOutline(t *testing.T) {
	src := "# One\n\ntext\n\n> ## Quoted Two\n\n### *Styled* Three\n"
	entries := render.Outline(parser.Parse([]byte(src)))
	want := []render.OutlineEntry{
		{Level: 1, Text: "One", SourceLine: 1},
		{Level: 2, Text: "Quoted Two", SourceLine: 5},
		{Level: 3, Text: "Styled Three", SourceLine: 7},
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for i, w := range want {
		if entries[i] != w {
			t.Errorf("entry %d = %+v, want %+v", i, entries[i], w)
		}
	}
}
