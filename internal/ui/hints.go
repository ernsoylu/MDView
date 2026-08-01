package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/ernsoylu/MDView/internal/render"
)

// hint is one followable link occurrence in the viewport.
type hint struct {
	label string
	line  int // rendered line index
	at    int // byte offset within the line's plain text
	url   string
}

const hintAlphabet = "asdfghjklqwertyuiopzxcvbnm"

func makeLabels(n int) []string {
	labels := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if n <= len(hintAlphabet) {
			labels = append(labels, string(hintAlphabet[i]))
			continue
		}
		labels = append(labels,
			string(hintAlphabet[(i/len(hintAlphabet))%len(hintAlphabet)])+string(hintAlphabet[i%len(hintAlphabet)]))
	}
	return labels
}

// startHints collects the link occurrences visible in the viewport and
// enters hint mode. A link wrapped onto the next line gets one hint only.
func (m *Model) startHints() {
	m.hints = nil
	m.hintPrefix = ""
	end := m.offset + m.contentHeight()
	if end > len(m.lines) {
		end = len(m.lines)
	}
	prevLast := ""
	for li := m.offset; li < end; li++ {
		prevLast = m.collectLineHints(li, prevLast)
	}
	if len(m.hints) == 0 {
		m.flash = "no links visible"
		return
	}
	for i, l := range makeLabels(len(m.hints)) {
		m.hints[i].label = l
	}
	m.mode = modeHints
}

// collectLineHints appends hints for links starting on line li. prevLast is
// the link URL ending the previous line (so a wrapped link is not re-hinted).
func (m *Model) collectLineHints(li int, prevLast string) string {
	spans := m.lines[li].Spans
	pos := 0
	runURL := ""
	for si, sp := range spans {
		if sp.Link != runURL {
			runURL = sp.Link
			if sp.Link != "" && (si != 0 || sp.Link != prevLast) {
				m.hints = append(m.hints, hint{line: li, at: pos, url: sp.Link})
			}
		}
		pos += len(sp.Text)
	}
	if len(spans) == 0 {
		return ""
	}
	return spans[len(spans)-1].Link
}

func (m Model) updateHints(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
	case tea.KeyBackspace:
		if m.hintPrefix != "" {
			r := []rune(m.hintPrefix)
			m.hintPrefix = string(r[:len(r)-1])
		}
	case tea.KeyRunes:
		m.hintPrefix += string(msg.Runes)
		live := 0
		var chosen *hint
		for i := range m.hints {
			h := &m.hints[i]
			if strings.HasPrefix(h.label, m.hintPrefix) {
				live++
				if h.label == m.hintPrefix {
					chosen = h
				}
			}
		}
		switch {
		case chosen != nil:
			m.mode = modeNormal
			return m, m.followLink(chosen.url)
		case live == 0:
			m.mode = modeNormal
			m.flash = "no matching hint"
		}
	}
	return m, nil
}

// overlayLabel draws a hint label into a line at a byte offset, consuming
// as many cells of the original text as the label occupies.
func overlayLabel(ln render.Line, at int, label string, st *lipgloss.Style) render.Line {
	out := make([]render.Span, 0, len(ln.Spans)+2)
	pos := 0
	need := runewidth.StringWidth(label)
	placed := false
	for _, sp := range ln.Spans {
		spStart, spEnd := pos, pos+len(sp.Text)
		pos = spEnd
		if spEnd <= at || (placed && need <= 0) {
			out = append(out, sp)
			continue
		}
		txt := sp.Text
		if spStart < at {
			out = append(out, render.Span{Text: txt[:at-spStart], Style: sp.Style, Link: sp.Link})
			txt = txt[at-spStart:]
		}
		if !placed {
			out = append(out, render.Span{Text: label, Style: st})
			placed = true
		}
		for need > 0 && txt != "" {
			r, size := utf8.DecodeRuneInString(txt)
			need -= runewidth.RuneWidth(r)
			txt = txt[size:]
		}
		if txt != "" && need <= 0 {
			out = append(out, render.Span{Text: txt, Style: sp.Style, Link: sp.Link})
		}
	}
	return render.Line{Spans: out, SourceLine: ln.SourceLine}
}
