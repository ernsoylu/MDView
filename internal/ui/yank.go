package ui

import (
	"encoding/base64"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ernsoylu/MDView/internal/render"
)

// pickCodeBlock returns the first code block at or below the top of the
// viewport, nil when there is none.
func (m Model) pickCodeBlock() *render.CodeBlock {
	top := m.topSourceLine()
	for i := range m.codeBlocks {
		if m.codeBlocks[i].End >= top {
			return &m.codeBlocks[i]
		}
	}
	return nil
}

// yank copies the picked code block to the system clipboard via OSC 52.
// The confirmation waits on the write: reporting a clipboard the terminal
// never received is worse than reporting nothing.
func (m *Model) yank() tea.Cmd {
	cb := m.pickCodeBlock()
	if cb == nil {
		m.flash = "no code block at or below the viewport"
		return nil
	}
	lines := strings.Count(cb.Content, "\n") + 1
	seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(cb.Content)) + "\x07"
	write := m.tty
	return func() tea.Msg {
		if err := write([]string{seq}); err != nil {
			return flashMsg("yank: " + err.Error())
		}
		return flashMsg(fmt.Sprintf("yanked %d code line(s)", lines))
	}
}
