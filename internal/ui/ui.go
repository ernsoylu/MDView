// Package ui is the full-screen pager: a viewport over the rendered lines
// with vim-style keys, mouse support, incremental search, a fuzzy TOC jump,
// link following with hints and a jumplist, editor integration, watch mode,
// a status bar, and a help overlay.
package ui

import (
	"fmt"
	"io"
	"os"
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

type mode int

const (
	modeNormal mode = iota
	modeHelp
	modeSearch
	modeTOC
	modeHints
	modeVisual
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
	opener  func(string) error   // external opener, replaceable in tests
	tty     func([]string) error // raw terminal writer, replaceable in tests

	imageMode  render.ImageMode
	mermaidCmd string // mermaid-cli path; "" leaves fences as code
	imgIDs     *img.Registry
	pendingTx  []string // kitty transmissions awaiting a write to the tty

	store       *state.Store // reading-position persistence; nil disables
	restoreLine int          // saved position to apply on the next reflow
	maxWidth    int          // content width cap
	editor      string       // editor override; "" falls back to $EDITOR
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

	// visual-line selection: anchor and cursor as rendered-line indices
	vAnchor, vCursor int

	lineNums bool // show a source-line-number gutter
	gutter   int  // gutter width in cells, 0 when numbers are off

	keys map[string]action // normal-mode bindings, overridable via keys.yaml

	// hardQuit distinguishes ctrl+c from q: started from the browser, q
	// goes back to the list and ctrl+c leaves mdv.
	hardQuit bool
}

// HardQuit reports whether the pager was left with ctrl+c rather than the
// quit key.
func (m Model) HardQuit() bool { return m.hardQuit }

func New(doc *parser.Doc, th theme.Theme, name, path string) Model {
	m := Model{doc: doc, th: th, name: name, path: path, outline: render.Outline(doc), cur: -1}
	m.anchors = nav.Anchors(m.outline)
	m.opener = systemOpen
	m.tty = writeRawTTY
	m.keys = defaultKeymap()
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

// WithEditor overrides the editor used by the e/i keys.
func (m Model) WithEditor(ed string) Model {
	if ed != "" {
		m.editor = ed
	}
	return m
}

// WithImages overrides the detected image mode.
func (m Model) WithImages(mode render.ImageMode) Model {
	m.imageMode = mode
	return m
}

// WithMermaid enables ```mermaid rendering through the given mermaid-cli.
func (m Model) WithMermaid(cmd string) Model {
	m.mermaidCmd = cmd
	return m
}

// WithKeys replaces the normal-mode bindings.
func (m Model) WithKeys(keys map[string]action) Model {
	if len(keys) > 0 {
		m.keys = keys
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
	return model, tea.Batch(cmd, model.writeTTY("images", tx))
}

// writeRawTTY writes escape sequences straight to the terminal, bypassing
// bubbletea's renderer: graphics transmissions must not be part of the
// diffed frame, or they would be re-sent on every repaint.
func writeRawTTY(chunks []string) error {
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for _, c := range chunks {
		if _, err := io.WriteString(f, c); err != nil {
			return err
		}
	}
	return nil
}

// writeTTY performs a raw terminal write as a command, reporting failure in
// the status bar. Silence here used to make a dropped image or an
// unreachable clipboard indistinguishable from success.
func (m Model) writeTTY(what string, chunks []string) tea.Cmd {
	write := m.tty
	return func() tea.Msg {
		if err := write(chunks); err != nil {
			return flashMsg(what + ": " + err.Error())
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
			if idx := m.offset + msg.Y; idx < len(m.lines) && msg.X >= 1+m.gutter {
				if url := linkAtCell(m.lines[idx], msg.X-1-m.gutter); url != "" {
					return m, m.followLink(url)
				}
			}
		}
	case tea.KeyMsg:
		m.flash = ""
		if msg.Type == tea.KeyCtrlC {
			m.hardQuit = true
			return m, m.quitCmd()
		}
		switch m.mode {
		case modeSearch:
			return m.updateSearch(msg)
		case modeTOC:
			return m.updateTOC(msg)
		case modeHints:
			return m.updateHints(msg)
		case modeVisual:
			return m.updateVisual(msg)
		case modeHelp:
			// esc closes an overlay whatever it is bound to elsewhere.
			switch act := m.keys[msg.String()]; {
			case act == actQuit:
				return m, m.quitCmd()
			case act == actHelp, msg.String() == "esc":
				m.mode = modeNormal
			}
			return m, nil
		}
		switch m.keys[msg.String()] {
		case actQuit:
			return m, m.quitCmd()
		case actHelp:
			m.mode = modeHelp
		case actClearSearch:
			m.clearSearch()
		case actSearch:
			m.mode = modeSearch
			m.searchOrigin = m.offset
			m.query = ""
			m.refreshSearch(false)
		case actNextMatch:
			m.nextMatch(1)
		case actPrevMatch:
			m.nextMatch(-1)
		case actTOC:
			m.mode = modeTOC
			m.tocQuery = ""
			m.tocSel = 0
			m.refilterTOC()
		case actHints:
			m.startHints()
		case actJumpBack:
			if p, ok := m.jump.Back(m.posHere()); ok {
				m.restore(p)
			} else {
				m.flash = "at oldest jump"
			}
		case actJumpForward: // terminals send Ctrl+I as Tab
			if p, ok := m.jump.Forward(m.posHere()); ok {
				m.restore(p)
			} else {
				m.flash = "at newest jump"
			}
		case actEdit:
			if cmd := m.editorCmd(); cmd != nil {
				return m, cmd
			}
		case actYank:
			if cmd := m.yank(); cmd != nil {
				return m, cmd
			}
		case actVisual:
			m.startVisual()
		case actLineNumbers:
			m.lineNums = !m.lineNums
			m.reflow()
		case actLineDown:
			m.scroll(1)
		case actLineUp:
			m.scroll(-1)
		case actHalfDown:
			m.scroll(m.contentHeight() / 2)
		case actHalfUp:
			m.scroll(-m.contentHeight() / 2)
		case actPageDown:
			m.scroll(m.contentHeight())
		case actPageUp:
			m.scroll(-m.contentHeight())
		case actTop:
			m.offset = 0
		case actBottom:
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
		helpLines := strings.Split(m.helpView(), "\n")
		for i := 0; i < m.contentHeight(); i++ {
			if i < len(helpLines) {
				// A wrapped line would push the rest of the overlay down.
				b.WriteString(" " + runewidth.Truncate(helpLines[i], m.width-1, "…"))
			}
			b.WriteString("\n")
		}
	case modeTOC:
		m.writeTOC(&b)
	default:
		for i := 0; i < m.contentHeight(); i++ {
			if idx := m.offset + i; idx < len(m.rendered) {
				b.WriteString(" " + m.gutterFor(idx) + m.renderLine(idx))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString(m.statusView())
	return b.String()
}

// gutterFor formats the line-number gutter cell for one rendered line: the
// source line number on the first row that renders it, blanks on wrapped
// continuations and synthetic rows.
func (m Model) gutterFor(idx int) string {
	if m.gutter == 0 {
		return ""
	}
	sl := m.lines[idx].SourceLine
	if sl == 0 || (idx > 0 && m.lines[idx-1].SourceLine == sl) {
		return strings.Repeat(" ", m.gutter)
	}
	return m.th.Dim.Render(fmt.Sprintf("%*d", m.gutter-1, sl)) + " "
}

// renderLine returns the ANSI string for one rendered line, overlaying
// search highlights, hint labels, and the visual selection as needed.
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
	inSel := false
	if m.mode == modeVisual {
		lo, hi := m.selBounds()
		inSel = idx >= lo && idx <= hi
	}
	if len(ranges) == 0 && !hasHint && !inSel {
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
	if inSel {
		if len(ln.Spans) == 0 {
			// An empty row still needs a visible cell to read as selected.
			ln = render.Line{Spans: []render.Span{{Text: " "}}, SourceLine: ln.SourceLine}
		}
		ln = ln.Highlight([][2]int{{0, len(ln.Plain())}}, &m.th.Selection)
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
	m.gutter = 0
	if m.lineNums {
		m.gutter = len(fmt.Sprint(m.doc.LineCount())) + 1
	}
	w := m.width - 2 - m.gutter
	if w > m.maxWidth {
		w = m.maxWidth
	}
	opts := render.Options{Images: m.imageMode, IDs: m.imgIDs, MermaidCmd: m.mermaidCmd}
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
		return m.statusLine(" /"+m.query, count(len(m.matches), "match", "matches")+" · enter keep · esc cancel ")
	case modeTOC:
		sel := 0
		if len(m.filtered) > 0 {
			sel = m.tocSel + 1
		}
		return m.statusLine(" contents", fmt.Sprintf("%d/%d · enter jump · esc close ", sel, len(m.filtered)))
	case modeHints:
		return m.statusLine(" follow: "+m.hintPrefix, "type a label · esc cancel ")
	case modeVisual:
		lo, hi := m.selBounds()
		return m.statusLine(" -- VISUAL LINE --", count(hi-lo+1, "line", "lines")+" · y yank · esc cancel ")
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

// statusLine lays out the one-row status bar. Both halves are sanitized:
// the left side carries document-derived text — filenames, link targets in
// flash messages — that must not reach the terminal as escape sequences.
func (m Model) statusLine(left, right string) string {
	return m.th.StatusBar.Render(fitRow(render.Sanitize(left), render.Sanitize(right), m.width))
}

// count renders a number with the right form of its noun, so a status bar
// says "1 match" rather than "1 matches".
func count(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// fitRow lays left and right on a row of at most width cells, trimming the
// left side first and the right side too when even it will not fit — a
// status bar wider than the terminal wraps and costs a line of content.
func fitRow(left, right string, width int) string {
	if width < 1 {
		return ""
	}
	if runewidth.StringWidth(right) >= width {
		return runewidth.Truncate(right, width, "…")
	}
	left = runewidth.Truncate(left, width-runewidth.StringWidth(right), "… ")
	gap := width - runewidth.StringWidth(left) - runewidth.StringWidth(right)
	return left + strings.Repeat(" ", max(0, gap)) + right
}
