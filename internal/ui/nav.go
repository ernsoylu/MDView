package ui

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/ernsoylu/MDView/internal/nav"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
)

type flashMsg string

type editorDoneMsg struct{ err error }

// srcLineAt returns the source line of the first mapped rendered line at or
// after the given offset.
func (m Model) srcLineAt(offset int) int {
	for i := offset; i < len(m.lines); i++ {
		if sl := m.lines[i].SourceLine; sl > 0 {
			return sl
		}
	}
	return 0
}

func (m Model) topSourceLine() int { return m.srcLineAt(m.offset) }

func (m Model) posHere() nav.Pos {
	return nav.Pos{Path: m.path, Offset: m.offset, SourceLine: m.topSourceLine()}
}

// restore returns to a jumplist position, switching documents if needed.
func (m *Model) restore(p nav.Pos) {
	if p.Path != m.path {
		if err := m.loadFile(p.Path); err != nil {
			m.flash = err.Error()
			return
		}
	}
	if p.Offset <= m.maxOffset() {
		m.offset = p.Offset
	} else if p.SourceLine > 0 {
		m.jumpToSourceLine(p.SourceLine)
	} else {
		m.offset = m.maxOffset()
	}
	m.scroll(0)
}

// loadFile switches the pager to another document, resetting position and
// search state.
func (m *Model) loadFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if m.store != nil && m.path != "" {
		m.store.Set(m.path, m.topSourceLine())
	}
	m.path = path
	m.name = path
	m.doc = parser.Parse(src)
	m.outline = render.Outline(m.doc)
	m.anchors = nav.Anchors(m.outline)
	m.codeBlocks = render.CodeBlocks(m.doc)
	m.clearSearch()
	m.offset = 0
	m.ready = false // reflow must not anchor to the previous document
	if m.store != nil {
		m.restoreLine = m.store.Get(path)
	}
	m.reflow()
	if m.watcher != nil {
		_ = m.watcher.Add(filepath.Dir(path))
	}
	return nil
}

// reload re-reads the current file in place, keeping the viewport anchored
// to the same source line (used by watch mode and editor return).
func (m *Model) reload() {
	src, err := os.ReadFile(m.path)
	if err != nil {
		m.flash = "reload: " + err.Error()
		return
	}
	m.doc = parser.Parse(src)
	m.outline = render.Outline(m.doc)
	m.anchors = nav.Anchors(m.outline)
	m.codeBlocks = render.CodeBlocks(m.doc)
	m.reflow()
}

// followLink routes a link target: #anchors jump in-document, relative
// markdown files open in mdv, everything else goes to the system opener.
// All in-app jumps push onto the jumplist first.
func (m *Model) followLink(target string) tea.Cmd {
	switch {
	case strings.HasPrefix(target, "#"):
		if line, ok := m.anchors[normalizeFragment(target[1:])]; ok {
			m.jump.Push(m.posHere())
			m.jumpToSourceLine(line)
		} else {
			m.flash = "no such anchor: " + target
		}
	case strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:"):
		return m.openExternalCmd(target)
	default:
		pathPart, frag, _ := strings.Cut(target, "#")
		if m.path == "" {
			m.flash = "cannot follow relative links from stdin"
			return nil
		}
		abs := pathPart
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(filepath.Dir(m.path), abs)
		}
		if ext := strings.ToLower(filepath.Ext(abs)); ext != ".md" && ext != ".markdown" {
			return m.openExternalCmd(abs)
		}
		from := m.posHere()
		if err := m.loadFile(abs); err != nil {
			m.flash = err.Error()
			return nil
		}
		m.jump.Push(from)
		if frag != "" {
			if line, ok := m.anchors[normalizeFragment(frag)]; ok {
				m.jumpToSourceLine(line)
			}
		}
	}
	return nil
}

func normalizeFragment(frag string) string {
	if u, err := url.PathUnescape(frag); err == nil {
		frag = u
	}
	return strings.ToLower(frag)
}

func (m Model) openExternalCmd(target string) tea.Cmd {
	open := m.opener
	return func() tea.Msg {
		if err := open(target); err != nil {
			return flashMsg("open: " + err.Error())
		}
		return nil
	}
}

// editorCmd suspends the TUI and runs $EDITOR (default vim) at the source
// line currently at the top of the viewport.
func (m *Model) editorCmd() tea.Cmd {
	if m.path == "" {
		m.flash = "no file to edit (stdin)"
		return nil
	}
	parts := strings.Fields(os.Getenv("EDITOR"))
	if len(parts) == 0 {
		parts = []string{"vim"}
	}
	line := m.topSourceLine()
	if line < 1 {
		line = 1
	}
	args := append(parts[1:], fmt.Sprintf("+%d", line), m.path)
	return tea.ExecProcess(exec.Command(parts[0], args...), func(err error) tea.Msg {
		return editorDoneMsg{err: err}
	})
}

// linkAtCell returns the hyperlink target under a terminal cell, if any.
func linkAtCell(ln render.Line, cell int) string {
	w := 0
	for _, sp := range ln.Spans {
		sw := runewidth.StringWidth(sp.Text)
		if cell < w+sw {
			return sp.Link
		}
		w += sw
	}
	return ""
}
