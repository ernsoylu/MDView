// Package ui is the full-screen pager: a viewport over the rendered lines
// with vim-style keys, mouse wheel, a status bar, and a help overlay.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

const helpText = `mdv — keys

  j / ↓          scroll down       d / ctrl+d   half page down
  k / ↑          scroll up         u / ctrl+u   half page up
  space / pgdn   page down         b / pgup     page up
  g / home       go to top         G / end      go to bottom

  ?              toggle this help
  q / ctrl+c     quit`

type Model struct {
	doc  *parser.Doc
	th   theme.Theme
	name string

	lines    []render.Line
	rendered []string

	width, height int
	offset        int
	showHelp      bool
	ready         bool
}

func New(doc *parser.Doc, th theme.Theme, name string) Model {
	return Model{doc: doc, th: th, name: name}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.reflow()
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scroll(-3)
		case tea.MouseButtonWheelDown:
			m.scroll(3)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case "esc":
			m.showHelp = false
		case "j", "down":
			m.scroll(1)
		case "k", "up":
			m.scroll(-1)
		case "d", "ctrl+d":
			m.scroll(m.contentHeight() / 2)
		case "u", "ctrl+u":
			m.scroll(-m.contentHeight() / 2)
		case " ", "pgdown", "ctrl+f":
			m.scroll(m.contentHeight())
		case "b", "pgup", "ctrl+b":
			m.scroll(-m.contentHeight())
		case "g", "home":
			m.offset = 0
		case "G", "end":
			m.offset = m.maxOffset()
		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}
	var b strings.Builder
	if m.showHelp {
		helpLines := strings.Split(helpText, "\n")
		for i := 0; i < m.contentHeight(); i++ {
			if i < len(helpLines) {
				b.WriteString(" " + helpLines[i])
			}
			b.WriteString("\n")
		}
	} else {
		for i := 0; i < m.contentHeight(); i++ {
			if idx := m.offset + i; idx < len(m.rendered) {
				b.WriteString(" " + m.rendered[idx])
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(m.statusView())
	return b.String()
}

func (m Model) contentHeight() int {
	if m.height <= 1 {
		return 1
	}
	return m.height - 1
}

func (m Model) maxOffset() int {
	mo := len(m.rendered) - m.contentHeight()
	if mo < 0 {
		return 0
	}
	return mo
}

func (m *Model) scroll(delta int) {
	m.offset += delta
	if m.offset > m.maxOffset() {
		m.offset = m.maxOffset()
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// reflow re-renders at the current width and re-anchors the viewport to the
// source line that was previously at the top.
func (m *Model) reflow() {
	anchor := 0
	if m.ready {
		for i := m.offset; i < len(m.lines); i++ {
			if sl := m.lines[i].SourceLine; sl > 0 {
				anchor = sl
				break
			}
		}
	}
	w := m.width - 2
	if w > 120 {
		w = 120
	}
	m.lines = render.Render(m.doc, m.th, w)
	m.rendered = make([]string, len(m.lines))
	for i, ln := range m.lines {
		m.rendered[i] = ln.String()
	}
	m.ready = true
	if anchor > 0 {
		m.offset = m.maxOffset()
		for i, ln := range m.lines {
			if ln.SourceLine >= anchor {
				m.offset = i
				break
			}
		}
	}
	m.scroll(0)
}

func (m Model) statusView() string {
	var pos string
	switch {
	case m.maxOffset() == 0:
		pos = "ALL"
	case m.offset == 0:
		pos = "TOP"
	case m.offset >= m.maxOffset():
		pos = "BOT"
	default:
		pos = fmt.Sprintf("%d%%", m.offset*100/m.maxOffset())
	}
	left := " " + m.name + "  "
	right := pos + "  ? help  q quit "
	gap := m.width - runewidth.StringWidth(left) - runewidth.StringWidth(right)
	if gap < 1 {
		keep := m.width - runewidth.StringWidth(right) - 1
		if keep < 1 {
			keep = 1
		}
		left = runewidth.Truncate(left, keep, "… ")
		gap = m.width - runewidth.StringWidth(left) - runewidth.StringWidth(right)
		if gap < 0 {
			gap = 0
		}
	}
	return m.th.StatusBar.Render(left + strings.Repeat(" ", gap) + right)
}
