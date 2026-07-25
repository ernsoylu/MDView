package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/state"
	"github.com/ernsoylu/MDView/internal/theme"
)

// markerDoc has a "## Marker" heading far enough down to require scrolling.
func markerDoc() string {
	var b strings.Builder
	b.WriteString("# Top\n\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "filler %d\n\n", i)
	}
	b.WriteString("## Marker\n\nmarker body\n\n")
	for i := 40; i < 60; i++ {
		fmt.Fprintf(&b, "filler %d\n\n", i)
	}
	return b.String()
}

func TestPositionRestoreOnOpen(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	src := markerDoc()
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := parser.Parse([]byte(src))
	markerLine := 0
	for _, e := range render.Outline(doc) {
		if e.Text == "Marker" {
			markerLine = e.SourceLine
		}
	}
	if markerLine == 0 {
		t.Fatal("marker heading not found")
	}

	s := state.Open()
	s.Set(path, markerLine)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	m := New(doc, theme.Plain(), "doc.md", path).WithStore(state.Open())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = mm.(Model)
	if m.offset == 0 {
		t.Fatal("saved position was not restored")
	}
	if got := m.topSourceLine(); got != markerLine {
		t.Errorf("restored top source line = %d, want %d", got, markerLine)
	}
}

func TestQuitPersistsPosition(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	src := markerDoc()
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(parser.Parse([]byte(src)), theme.Plain(), "doc.md", path).WithStore(state.Open())
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = mm.(Model)
	m = press(m, tea.KeyMsg{Type: tea.KeyEnd}) // jump to bottom
	want := m.topSourceLine()
	if want <= 1 {
		t.Fatal("setup: expected to be scrolled down")
	}
	if _, cmd := m.Update(runes("q")); cmd == nil {
		t.Fatal("q returned no quit command")
	}
	if got := state.Open().Get(path); got != want {
		t.Errorf("persisted line = %d, want %d", got, want)
	}
}

func TestYankPicksNearestBlock(t *testing.T) {
	var b strings.Builder
	b.WriteString("```go\nalpha()\n```\n\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "filler %d\n\n", i)
	}
	b.WriteString("```go\nbeta()\ngamma()\n```\n")
	m := New(parser.Parse([]byte(b.String())), theme.Plain(), "y.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = mm.(Model)

	if cb := m.pickCodeBlock(); cb == nil || cb.Content != "alpha()" {
		t.Fatalf("at top: picked %+v, want alpha()", cb)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEnd})
	if cb := m.pickCodeBlock(); cb == nil || !strings.Contains(cb.Content, "beta()") {
		t.Fatalf("at bottom: picked %+v, want the beta block", cb)
	}
	// The confirmation now waits on the clipboard write, so drive the
	// command rather than reading the flash straight after the keypress.
	var wrote []string
	m.tty = func(chunks []string) error { wrote = append(wrote, chunks...); return nil }
	_, cmd := m.Update(runes("y"))
	if cmd == nil {
		t.Fatal("y produced no command")
	}
	msg, ok := cmd().(flashMsg)
	if !ok || !strings.Contains(string(msg), "2 code line") {
		t.Errorf("yank message = %#v, want a 2-line confirmation", msg)
	}
	if len(wrote) != 1 || !strings.HasPrefix(wrote[0], "\x1b]52;c;") {
		t.Errorf("clipboard write = %q, want one OSC 52 sequence", wrote)
	}
}

func TestMaxWidthCapsContent(t *testing.T) {
	m := New(parser.Parse([]byte(testDoc())), theme.Plain(), "w.md", "").WithMaxWidth(40)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = mm.(Model)
	for i, ln := range m.lines {
		if w := ln.Width(); w > 40 {
			t.Errorf("line %d is %d cells wide, want <= 40", i, w)
		}
	}
}
