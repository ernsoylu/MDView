package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

// The rendered body is sanitized in the render package; these cases cover
// the chrome around it, where document-derived text also reaches the screen.

func modelFor(t *testing.T, src string) Model {
	t.Helper()
	m := New(parser.Parse([]byte(src)), theme.Plain(), "a.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	return mm.(Model)
}

// TestTOCSanitizesHeadings covers the TOC popup, which prints heading text
// straight from the document rather than through the line IR.
func TestTOCSanitizesHeadings(t *testing.T) {
	m := modelFor(t, "# safe\n\n## bad \x1b[31mheading\x1b[0m\n\ntext\n")
	m = press(m, runes("t"))
	if got := m.View(); strings.ContainsRune(got, 0x1b) {
		t.Errorf("escape reached the TOC popup: %q", got)
	}
}

// TestStatusBarSanitizesFlash covers the status bar, which shows link
// targets and error strings built from document content.
func TestStatusBarSanitizesFlash(t *testing.T) {
	m := modelFor(t, "text\n")
	m.flash = "no such anchor: #\x1b[31mgotcha"
	if got := m.View(); strings.ContainsRune(got, 0x1b) {
		t.Errorf("escape reached the status bar: %q", got)
	}
}

// TestStatusBarSanitizesName covers a filename carrying an escape.
func TestStatusBarSanitizesName(t *testing.T) {
	m := modelFor(t, "text\n")
	m.name = "weird\x1b[2Jname.md"
	if got := m.View(); strings.ContainsRune(got, 0x1b) {
		t.Errorf("escape reached the status bar via the filename: %q", got)
	}
}
