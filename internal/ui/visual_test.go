package ui

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

const visualDoc = `# Title

Some **bold** and a [link](https://example.com).

` + "```go\none()\ntwo()\n```\n"

func newVisualModel(t *testing.T, src string) Model {
	t.Helper()
	m := New(parser.Parse([]byte(src)), theme.Plain(), "v.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	return mm.(Model)
}

// yankPayload runs one keypress, executes the command it returns, and
// decodes the OSC 52 payload written to the tty.
func yankPayload(t *testing.T, m Model, key tea.Msg) (string, string) {
	t.Helper()
	var wrote []string
	m.tty = func(chunks []string) error { wrote = append(wrote, chunks...); return nil }
	mm, cmd := m.Update(key)
	if cmd == nil {
		t.Fatal("yank produced no command")
	}
	msg, _ := cmd().(flashMsg)
	if len(wrote) != 1 || !strings.HasPrefix(wrote[0], "\x1b]52;c;") {
		t.Fatalf("clipboard write = %q, want one OSC 52 sequence", wrote)
	}
	b64 := strings.TrimSuffix(strings.TrimPrefix(wrote[0], "\x1b]52;c;"), "\x07")
	dec, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("payload is not base64: %v", err)
	}
	if mm.(Model).mode != modeNormal {
		t.Errorf("mode after yank = %v, want normal", mm.(Model).mode)
	}
	return string(dec), string(msg)
}

func TestVisualSelectAllYanksWholeSource(t *testing.T) {
	m := newVisualModel(t, visualDoc)
	m = press(m, runes("v"))
	if m.mode != modeVisual {
		t.Fatalf("mode after v = %v, want visual", m.mode)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyEnd}) // G: extend to the bottom
	if got := m.vCursor; got != len(m.rendered)-1 {
		t.Fatalf("cursor after G = %d, want %d", got, len(m.rendered)-1)
	}
	got, flash := yankPayload(t, m, runes("y"))
	if got != visualDoc {
		t.Errorf("yanked %q, want the exact markdown source %q", got, visualDoc)
	}
	if !strings.Contains(flash, "yanked") {
		t.Errorf("flash = %q, want a yank confirmation", flash)
	}
}

func TestVisualYankCopiesCodeLinesExactly(t *testing.T) {
	m := newVisualModel(t, visualDoc)
	// Park the top of the viewport on the rendered row of one().
	for i, p := range m.plain {
		if strings.Contains(p, "one()") {
			m.offset = i
			break
		}
	}
	if m.offset == 0 {
		t.Fatal("one() not found in the rendered document")
	}
	m = press(m, runes("V"), runes("j")) // one() and two()
	got, _ := yankPayload(t, m, runes("y"))
	if got != "one()\ntwo()\n" {
		t.Errorf("yanked %q, want the two code source lines", got)
	}
}

func TestVisualEscCancelsAndMotionsScroll(t *testing.T) {
	m := newTestModel(t, 80, 12)
	m = press(m, runes("v"), runes("V"))
	if m.mode != modeNormal {
		t.Fatalf("v then V should leave visual mode, mode = %v", m.mode)
	}
	m = press(m, runes("v"), tea.KeyMsg{Type: tea.KeyEnd})
	if m.vCursor < m.offset || m.vCursor >= m.offset+m.contentHeight() {
		t.Errorf("cursor %d scrolled out of view [%d, %d)", m.vCursor, m.offset, m.offset+m.contentHeight())
	}
	if !strings.Contains(m.View(), "VISUAL LINE") {
		t.Error("status bar does not announce visual mode")
	}
	m = press(m, keyEsc)
	if m.mode != modeNormal {
		t.Errorf("esc left mode = %v, want normal", m.mode)
	}
}

func TestLineNumbersToggleAndGutter(t *testing.T) {
	m := newTestModel(t, 80, 12)
	if m.gutter != 0 {
		t.Fatalf("gutter = %d before toggling, want 0", m.gutter)
	}
	m = press(m, runes("#"))
	want := len(fmt.Sprint(m.doc.LineCount())) + 1
	if m.gutter != want {
		t.Fatalf("gutter = %d, want %d", m.gutter, want)
	}
	for i, ln := range m.lines {
		if w := ln.Width(); w > 80-2-m.gutter {
			t.Errorf("line %d is %d cells wide, want <= %d", i, w, 80-2-m.gutter)
		}
	}
	if g := m.gutterFor(0); !strings.Contains(g, "1") {
		t.Errorf("gutter for the first line = %q, want the number 1", g)
	}
	top := m.View()[:strings.Index(m.View(), "\n")]
	if !strings.Contains(top, " 1 ") {
		t.Errorf("first view row %q lacks its line number", top)
	}
	m = press(m, runes("#"))
	if m.gutter != 0 {
		t.Errorf("gutter = %d after toggling off, want 0", m.gutter)
	}
}
