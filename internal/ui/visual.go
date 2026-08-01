package ui

import (
	"encoding/base64"

	tea "github.com/charmbracelet/bubbletea"
)

// startVisual enters visual-line mode with both ends of the selection on the
// top visible line. Selection is by rendered line; yanking copies the
// markdown source lines the selection renders, so what lands on the
// clipboard is syntax, not styled text.
func (m *Model) startVisual() {
	if len(m.rendered) == 0 {
		return
	}
	m.mode = modeVisual
	m.vAnchor = m.offset
	m.vCursor = m.offset
}

// selBounds returns the selection as an inclusive rendered-line range.
func (m Model) selBounds() (lo, hi int) {
	if m.vAnchor <= m.vCursor {
		return m.vAnchor, m.vCursor
	}
	return m.vCursor, m.vAnchor
}

func (m Model) updateVisual(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.keys[msg.String()] {
	case actLineDown:
		m.moveVisual(1)
	case actLineUp:
		m.moveVisual(-1)
	case actHalfDown:
		m.moveVisual(m.contentHeight() / 2)
	case actHalfUp:
		m.moveVisual(-m.contentHeight() / 2)
	case actPageDown:
		m.moveVisual(m.contentHeight())
	case actPageUp:
		m.moveVisual(-m.contentHeight())
	case actTop:
		m.moveVisual(-len(m.rendered))
	case actBottom:
		m.moveVisual(len(m.rendered))
	case actYank:
		return m, m.yankSelection()
	case actVisual, actQuit:
		m.mode = modeNormal
	default:
		if msg.String() == "esc" {
			m.mode = modeNormal
		}
	}
	return m, nil
}

// moveVisual moves the cursor end of the selection, scrolling as needed to
// keep it visible.
func (m *Model) moveVisual(delta int) {
	m.vCursor += delta
	if m.vCursor > len(m.rendered)-1 {
		m.vCursor = len(m.rendered) - 1
	}
	if m.vCursor < 0 {
		m.vCursor = 0
	}
	if m.vCursor < m.offset {
		m.offset = m.vCursor
	}
	if m.vCursor >= m.offset+m.contentHeight() {
		m.offset = m.vCursor - m.contentHeight() + 1
	}
	m.scroll(0)
}

// yankSelection copies the source lines the selection renders via OSC 52.
// The rendered range maps to source through each line's SourceLine stamp;
// the end extends to the rest of the last selected block, because wrapped
// prose rows all carry their block's first source line and stopping there
// would silently drop the tail of the block.
func (m *Model) yankSelection() tea.Cmd {
	lo, hi := m.selBounds()
	m.mode = modeNormal
	srcLo, srcHi := m.selectionSourceRange(lo, hi)
	if srcLo == 0 {
		m.flash = "nothing to yank here"
		return nil
	}
	srcHi = m.extendSelectionEnd(lo, hi, srcLo, srcHi)
	text := m.doc.SourceLines(srcLo, srcHi)
	for len(text) > 1 && text[len(text)-1] == '\n' && text[len(text)-2] == '\n' {
		text = text[:len(text)-1] // block-end extension swept trailing blanks
	}
	n := countNewlines(text)
	seq := "\x1b]52;c;" + base64.StdEncoding.EncodeToString(text) + "\x07"
	write := m.tty
	return func() tea.Msg {
		if err := write([]string{seq}); err != nil {
			return flashMsg("yank: " + err.Error())
		}
		return flashMsg("yanked " + count(n, "line", "lines"))
	}
}

func (m Model) selectionSourceRange(lo, hi int) (srcLo, srcHi int) {
	for i := lo; i <= hi && i < len(m.lines); i++ {
		sl := m.lines[i].SourceLine
		if sl == 0 || len(m.lines[i].Spans) == 0 {
			continue // blank separators carry the next block's line
		}
		if srcLo == 0 || sl < srcLo {
			srcLo = sl
		}
		if sl > srcHi {
			srcHi = sl
		}
	}
	return srcLo, srcHi
}

// extendSelectionEnd grows srcHi to the end of the selected block, unless
// the selection sits entirely inside one code block (where SourceLine is
// already exact).
func (m Model) extendSelectionEnd(lo, hi, srcLo, srcHi int) int {
	// Code rows map one source line each, so a selection that begins and
	// ends inside the same code block is already exact — extending would
	// sweep in the closing fence.
	for i := range m.codeBlocks {
		cb := m.codeBlocks[i]
		if srcLo >= cb.Start && srcHi <= cb.End && srcHi >= cb.Start {
			return srcHi
		}
	}
	if next := m.srcLineAt(hi + 1); next > srcHi {
		return next - 1
	}
	if m.srcLineAt(hi+1) == 0 {
		return m.doc.LineCount()
	}
	return srcHi
}

func countNewlines(text []byte) int {
	n := 0
	for _, b := range text {
		if b == '\n' {
			n++
		}
	}
	return n
}
