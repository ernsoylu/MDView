// Package ui is the full-screen pager: a viewport over the rendered lines
// with vim-style keys, mouse wheel, incremental search, a fuzzy TOC jump,
// a status bar, and a help overlay.
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

  /              search (enter keep · esc cancel)
  n / N          next / previous match
  t              table of contents (type to filter)
  esc            clear search / close overlay

  ?              toggle this help
  q / ctrl+c     quit`

type mode int

const (
	modeNormal mode = iota
	modeHelp
	modeSearch
	modeTOC
)

type Model struct {
	doc  *parser.Doc
	th   theme.Theme
	name string

	lines    []render.Line
	rendered []string
	plain    []string
	outline  []render.OutlineEntry

	width, height int
	offset        int
	mode          mode
	ready         bool

	// search
	query        string
	matches      []match
	lineRanges   map[int][][2]int
	cur          int // index into matches; -1 when there are none
	searchOrigin int // offset when / was pressed, restored on esc

	// table of contents
	tocQuery string
	tocSel   int
	filtered []int // indices into outline, ranked
}

func New(doc *parser.Doc, th theme.Theme, name string) Model {
	return Model{doc: doc, th: th, name: name, outline: render.Outline(doc), cur: -1}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.reflow()
	case tea.MouseMsg:
		if m.mode == modeTOC {
			return m, nil
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scroll(-3)
		case tea.MouseButtonWheelDown:
			m.scroll(3)
		}
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeTOC:
			return m.updateTOC(msg)
		case modeHelp:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "?", "esc":
				m.mode = modeNormal
			}
			return m, nil
		}
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "?":
			m.mode = modeHelp
		case "esc":
			m.clearSearch()
		case "/":
			m.mode = modeSearch
			m.searchOrigin = m.offset
			m.query = ""
			m.refreshSearch(false)
		case "n":
			m.nextMatch(1)
		case "N":
			m.nextMatch(-1)
		case "t":
			m.mode = modeTOC
			m.tocQuery = ""
			m.tocSel = 0
			m.refilterTOC()
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
	switch m.mode {
	case modeHelp:
		helpLines := strings.Split(helpText, "\n")
		for i := 0; i < m.contentHeight(); i++ {
			if i < len(helpLines) {
				b.WriteString(" " + helpLines[i])
			}
			b.WriteString("\n")
		}
	case modeTOC:
		m.writeTOC(&b)
	default:
		for i := 0; i < m.contentHeight(); i++ {
			if idx := m.offset + i; idx < len(m.rendered) {
				b.WriteString(" " + m.renderLine(idx))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(m.statusView())
	return b.String()
}

// renderLine returns the ANSI string for one rendered line, overlaying
// search highlights when the line has matches.
func (m Model) renderLine(idx int) string {
	ranges := m.lineRanges[idx]
	if len(ranges) == 0 {
		return m.rendered[idx]
	}
	ln := m.lines[idx].Highlight(ranges, &m.th.SearchHit)
	if m.cur >= 0 {
		if mt := m.matches[m.cur]; mt.line == idx {
			ln = ln.Highlight([][2]int{{mt.start, mt.end}}, &m.th.SearchCurrent)
		}
	}
	return ln.String()
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

// jumpToSourceLine scrolls the first rendered line at or after the given
// source line to the top of the viewport.
func (m *Model) jumpToSourceLine(src int) {
	m.offset = m.maxOffset()
	for i, ln := range m.lines {
		if ln.SourceLine >= src {
			m.offset = i
			break
		}
	}
	m.scroll(0)
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
	m.plain = make([]string, len(m.lines))
	for i, ln := range m.lines {
		m.rendered[i] = ln.String()
		m.plain[i] = ln.Plain()
	}
	m.ready = true
	if anchor > 0 {
		m.jumpToSourceLine(anchor)
	}
	m.scroll(0)
	m.refreshSearch(false)
}

func (m Model) statusView() string {
	switch m.mode {
	case modeSearch:
		return m.statusLine(" /"+m.query, fmt.Sprintf("%d matches · enter keep · esc cancel ", len(m.matches)))
	case modeTOC:
		sel := 0
		if len(m.filtered) > 0 {
			sel = m.tocSel + 1
		}
		return m.statusLine(" contents", fmt.Sprintf("%d/%d · enter jump · esc close ", sel, len(m.filtered)))
	}
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
	right := pos + "  ? help  q quit "
	if m.query != "" {
		if len(m.matches) > 0 {
			right = fmt.Sprintf("[%d/%d]  ", m.cur+1, len(m.matches)) + right
		} else {
			right = "[no matches]  " + right
		}
	}
	return m.statusLine(" "+m.name+"  ", right)
}

func (m Model) statusLine(left, right string) string {
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
