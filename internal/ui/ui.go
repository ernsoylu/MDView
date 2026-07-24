// Package ui is the full-screen pager: a viewport over the rendered lines
// with vim-style keys, mouse support, incremental search, a fuzzy TOC jump,
// link following with hints and a jumplist, editor integration, watch mode,
// a status bar, and a help overlay.
package ui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"

	"github.com/ernsoylu/MDView/internal/img"
	"github.com/ernsoylu/MDView/internal/nav"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/state"
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

  f              follow link (type its label; click works too)
  ctrl+o / tab   jump back / forward
  e / i          edit at current line ($EDITOR, default vim)
  y              yank nearest code block to the clipboard

  ?              toggle this help
  q / ctrl+c     quit`

type mode int

const (
	modeNormal mode = iota
	modeHelp
	modeSearch
	modeTOC
	modeHints
)

type Model struct {
	doc  *parser.Doc
	th   theme.Theme
	name string
	path string // absolute path of the current file; "" when reading stdin

	lines    []render.Line
	rendered []string
	plain    []string
	outline  []render.OutlineEntry
	anchors  map[string]int

	width, height int
	offset        int
	mode          mode
	ready         bool
	flash         string // transient status-bar message, cleared on keypress

	jump    nav.Jumplist
	watcher *fsnotify.Watcher
	opener  func(string) error // external opener, replaceable in tests

	imageMode render.ImageMode
	imgIDs    *img.Registry
	pendingTx []string // kitty transmissions awaiting a write to the tty

	store       *state.Store // reading-position persistence; nil disables
	restoreLine int          // saved position to apply on the next reflow
	maxWidth    int          // content width cap
	codeBlocks  []render.CodeBlock

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

	// link hints
	hints      []hint
	hintPrefix string
}

func New(doc *parser.Doc, th theme.Theme, name, path string) Model {
	m := Model{doc: doc, th: th, name: name, path: path, outline: render.Outline(doc), cur: -1}
	m.anchors = nav.Anchors(m.outline)
	m.opener = func(target string) error { return exec.Command("xdg-open", target).Start() }
	m.imageMode = detectImages()
	m.imgIDs = img.NewRegistry()
	m.maxWidth = 120
	m.codeBlocks = render.CodeBlocks(doc)
	if path != "" {
		if w, err := fsnotify.NewWatcher(); err == nil {
			if w.Add(filepath.Dir(path)) == nil {
				m.watcher = w
			} else {
				_ = w.Close()
			}
		}
	}
	return m
}

func (m Model) Init() tea.Cmd { return m.watchCmd() }

// WithStore attaches the reading-position store and queues restoring this
// document's saved position on the first layout.
func (m Model) WithStore(s *state.Store) Model {
	m.store = s
	if m.path != "" {
		m.restoreLine = s.Get(m.path)
	}
	return m
}

// WithMaxWidth caps the content width (default 120 columns).
func (m Model) WithMaxWidth(w int) Model {
	if w > 0 {
		m.maxWidth = w
	}
	return m
}

// quitCmd persists the reading position and exits.
func (m Model) quitCmd() tea.Cmd {
	if m.store != nil && m.path != "" {
		m.store.Set(m.path, m.topSourceLine())
		_ = m.store.Save()
	}
	return tea.Quit
}

// detectImages picks the best image path for this terminal: kitty graphics
// where advertised, half-block mosaic on any color terminal, off when mono
// (piped output, NO_COLOR, tests).
func detectImages() render.ImageMode {
	if lipgloss.ColorProfile() == termenv.Ascii {
		return render.ImagesOff
	}
	if img.KittySupported() {
		return render.ImagesKitty
	}
	return render.ImagesHalfblock
}

// Update wraps update so that any render pass that produced kitty
// transmissions flushes them to the terminal exactly once.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	nm, cmd := m.update(msg)
	model, ok := nm.(Model)
	if !ok || len(model.pendingTx) == 0 {
		return nm, cmd
	}
	tx := model.pendingTx
	model.pendingTx = nil
	return model, tea.Batch(cmd, writeTTY(tx))
}

// writeTTY writes raw escape sequences directly to the terminal, bypassing
// bubbletea's renderer: graphics transmissions must not be part of the
// diffed frame, or they would be re-sent on every repaint.
func writeTTY(chunks []string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()
		for _, c := range chunks {
			if _, err := io.WriteString(f, c); err != nil {
				return nil
			}
		}
		return nil
	}
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.reflow()
	case flashMsg:
		m.flash = string(msg)
	case editorDoneMsg:
		if msg.err != nil {
			m.flash = "editor: " + msg.err.Error()
		}
		m.reload()
	case fileChangedMsg:
		if m.path != "" && filepath.Base(msg.name) == filepath.Base(m.path) {
			m.reload()
		}
		return m, m.watchCmd()
	case tea.MouseMsg:
		if m.mode == modeTOC {
			return m, nil
		}
		switch {
		case msg.Button == tea.MouseButtonWheelUp:
			m.scroll(-3)
		case msg.Button == tea.MouseButtonWheelDown:
			m.scroll(3)
		case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && m.mode == modeNormal:
			if idx := m.offset + msg.Y; idx < len(m.lines) && msg.X >= 1 {
				if url := linkAtCell(m.lines[idx], msg.X-1); url != "" {
					return m, m.followLink(url)
				}
			}
		}
	case tea.KeyMsg:
		m.flash = ""
		if msg.Type == tea.KeyCtrlC {
			return m, m.quitCmd()
		}
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeTOC:
			return m.updateTOC(msg)
		case modeHints:
			return m.updateHints(msg)
		case modeHelp:
			switch msg.String() {
			case "q":
				return m, m.quitCmd()
			case "?", "esc":
				m.mode = modeNormal
			}
			return m, nil
		}
		switch msg.String() {
		case "q":
			return m, m.quitCmd()
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
		case "f":
			m.startHints()
		case "ctrl+o":
			if p, ok := m.jump.Back(m.posHere()); ok {
				m.restore(p)
			} else {
				m.flash = "at oldest jump"
			}
		case "tab": // terminals send Ctrl+I as Tab
			if p, ok := m.jump.Forward(m.posHere()); ok {
				m.restore(p)
			} else {
				m.flash = "at newest jump"
			}
		case "e", "i":
			if cmd := m.editorCmd(); cmd != nil {
				return m, cmd
			}
		case "y":
			if cmd := m.yank(); cmd != nil {
				return m, cmd
			}
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
// search highlights and hint labels as needed.
func (m Model) renderLine(idx int) string {
	ranges := m.lineRanges[idx]
	hasHint := false
	if m.mode == modeHints {
		for _, h := range m.hints {
			if h.line == idx {
				hasHint = true
				break
			}
		}
	}
	if len(ranges) == 0 && !hasHint {
		return m.rendered[idx]
	}
	ln := m.lines[idx]
	if len(ranges) > 0 {
		ln = ln.Highlight(ranges, &m.th.SearchHit)
		if m.cur >= 0 {
			if mt := m.matches[m.cur]; mt.line == idx {
				ln = ln.Highlight([][2]int{{mt.start, mt.end}}, &m.th.SearchCurrent)
			}
		}
	}
	if hasHint {
		// Overlay right-to-left so earlier byte offsets stay valid.
		for i := len(m.hints) - 1; i >= 0; i-- {
			h := m.hints[i]
			if h.line != idx || !strings.HasPrefix(h.label, m.hintPrefix) {
				continue
			}
			ln = overlayLabel(ln, h.at, h.label, &m.th.HintLabel)
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

// jumpToSourceLine scrolls the first content line at or after the given
// source line to the top of the viewport. Blank separator lines share the
// following block's source line and are skipped, so anchoring round-trips.
func (m *Model) jumpToSourceLine(src int) {
	m.offset = m.maxOffset()
	for i, ln := range m.lines {
		if ln.SourceLine >= src && len(ln.Spans) > 0 {
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
		anchor = m.topSourceLine()
	}
	w := m.width - 2
	if w > m.maxWidth {
		w = m.maxWidth
	}
	opts := render.Options{Images: m.imageMode, IDs: m.imgIDs}
	if m.path != "" {
		opts.BaseDir = filepath.Dir(m.path)
	}
	lines, tx := render.RenderDoc(m.doc, m.th, w, opts)
	m.lines = lines
	m.pendingTx = append(m.pendingTx, tx...)
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
	if m.restoreLine > 0 {
		m.jumpToSourceLine(m.restoreLine)
		m.restoreLine = 0
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
	case modeHints:
		return m.statusLine(" follow: "+m.hintPrefix, "type a label · esc cancel ")
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
	left := " " + m.name + "  "
	if m.flash != "" {
		left = " " + m.flash + "  "
	}
	return m.statusLine(left, right)
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
