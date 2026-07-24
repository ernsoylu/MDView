package ui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// testDoc has two "needle" hits far apart and three headings, so search
// navigation and TOC jumps both have to scroll.
func testDoc() string {
	var b strings.Builder
	b.WriteString("# One\n\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "filler paragraph %d\n\n", i)
	}
	b.WriteString("## Two\n\nneedle alpha here\n\n")
	for i := 30; i < 60; i++ {
		fmt.Fprintf(&b, "filler paragraph %d\n\n", i)
	}
	b.WriteString("### Three\n\nneedle Beta here\n")
	return b.String()
}

func newTestModel(t *testing.T, w, h int) Model {
	t.Helper()
	m := New(parser.Parse([]byte(testDoc())), theme.Plain(), "test.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return mm.(Model)
}

func press(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		mm, _ := m.Update(msg)
		m = mm.(Model)
	}
	return m
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

var (
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
	keyUp    = tea.KeyMsg{Type: tea.KeyUp}
)

func TestIncrementalSearchJumpsAndEscRestores(t *testing.T) {
	m := newTestModel(t, 80, 12)
	if m.offset != 0 {
		t.Fatalf("initial offset = %d", m.offset)
	}
	m = press(m, runes("/"))
	if m.mode != modeSearch {
		t.Fatalf("mode = %v after /", m.mode)
	}
	m = press(m, runes("needle"))
	if len(m.matches) != 2 {
		t.Fatalf("%d matches, want 2", len(m.matches))
	}
	if m.offset == 0 {
		t.Error("incremental search did not scroll to the first match")
	}
	m = press(m, keyEsc)
	if m.mode != modeNormal || m.offset != 0 || m.matches != nil || m.query != "" {
		t.Errorf("esc did not restore: mode=%v offset=%d matches=%v query=%q",
			m.mode, m.offset, m.matches, m.query)
	}
}

func TestSearchNextPrevWrap(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("/"), runes("needle"), keyEnter)
	if m.mode != modeNormal || m.cur != 0 {
		t.Fatalf("after enter: mode=%v cur=%d", m.mode, m.cur)
	}
	m = press(m, runes("n"))
	if m.cur != 1 {
		t.Fatalf("n: cur = %d, want 1", m.cur)
	}
	line := m.matches[m.cur].line
	if line < m.offset || line >= m.offset+m.contentHeight() {
		t.Errorf("current match line %d not visible at offset %d", line, m.offset)
	}
	m = press(m, runes("n"))
	if m.cur != 0 {
		t.Errorf("n wrap: cur = %d, want 0", m.cur)
	}
	m = press(m, runes("N"))
	if m.cur != 1 {
		t.Errorf("N wrap: cur = %d, want 1", m.cur)
	}
}

func TestSearchSmartCase(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("/"), runes("beta"), keyEnter)
	if len(m.matches) != 1 {
		t.Errorf("lowercase beta: %d matches, want 1 (insensitive)", len(m.matches))
	}
	m = press(m, runes("/"), runes("BETA"), keyEnter)
	if len(m.matches) != 0 {
		t.Errorf("uppercase BETA: %d matches, want 0 (exact)", len(m.matches))
	}
}

func TestSearchHighlightInView(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("/"), runes("needle"), keyEnter)
	view := m.View()
	if !strings.Contains(view, "needle") {
		t.Errorf("current match line not visible in view")
	}
	if !strings.Contains(view, "[1/2]") {
		t.Errorf("status bar lacks match position, view tail: %q", view[len(view)-80:])
	}
}

func TestTOCFuzzyFilterAndJump(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("t"))
	if m.mode != modeTOC || len(m.filtered) != 3 {
		t.Fatalf("open TOC: mode=%v filtered=%d, want 3 headings", m.mode, len(m.filtered))
	}
	m = press(m, runes("thr"))
	if len(m.filtered) != 1 || m.outline[m.filtered[0]].Text != "Three" {
		t.Fatalf("filter thr: %d entries", len(m.filtered))
	}
	m = press(m, keyEnter)
	if m.mode != modeNormal {
		t.Fatalf("enter did not close TOC")
	}
	// The last heading cannot reach the viewport top (offset clamps at the
	// end of the document), but it must be visible.
	visible := false
	for i := m.offset; i < m.offset+m.contentHeight() && i < len(m.rendered); i++ {
		if strings.Contains(m.rendered[i], "Three") {
			visible = true
			break
		}
	}
	if !visible {
		t.Errorf("Three heading not visible after jump (offset %d)", m.offset)
	}
}

func TestTOCSelectionMoves(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("t"), keyDown)
	if m.tocSel != 1 {
		t.Errorf("down: sel = %d, want 1", m.tocSel)
	}
	m = press(m, keyUp)
	if m.tocSel != 0 {
		t.Errorf("up: sel = %d, want 0", m.tocSel)
	}
	m = press(m, keyEsc)
	if m.mode != modeNormal {
		t.Errorf("esc did not close TOC")
	}
}

func TestResizeKeepsSearch(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("/"), runes("needle"), keyEnter)
	m = press(m, tea.WindowSizeMsg{Width: 40, Height: 20})
	if len(m.matches) != 2 {
		t.Errorf("after resize: %d matches, want 2 (query persists)", len(m.matches))
	}
}
