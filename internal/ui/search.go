package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// match is one search hit: byte offsets into the plain text of one
// rendered line.
type match struct {
	line, start, end int
}

// findMatches scans the plain rendered lines for a literal query with
// smart-case: an all-lowercase query is case-insensitive, any uppercase
// makes it exact. Matches never cross line breaks.
func findMatches(plain []string, query string) []match {
	if query == "" {
		return nil
	}
	needle := strings.ToLower(query)
	insensitive := needle == query
	if !insensitive {
		needle = query
	}
	var out []match
	for i, line := range plain {
		hay := line
		if insensitive {
			lowered := strings.ToLower(line)
			// Unicode case folding can change byte length (e.g. İ);
			// fall back to exact matching on such lines.
			if len(lowered) == len(line) {
				hay = lowered
			}
		}
		for off := 0; ; {
			j := strings.Index(hay[off:], needle)
			if j < 0 {
				break
			}
			s := off + j
			out = append(out, match{line: i, start: s, end: s + len(needle)})
			off = s + len(needle)
		}
	}
	return out
}

func (m *Model) clearSearch() {
	m.query = ""
	m.matches = nil
	m.lineRanges = nil
	m.cur = -1
}

// refreshSearch recomputes matches for the current query. cur lands on the
// first match at or after the search origin (while typing) or the viewport
// top, wrapping to the first match overall. With jump set the viewport
// scrolls to it, incremental-search style.
func (m *Model) refreshSearch(jump bool) {
	m.matches = findMatches(m.plain, m.query)
	m.lineRanges = make(map[int][][2]int, len(m.matches))
	for _, mt := range m.matches {
		m.lineRanges[mt.line] = append(m.lineRanges[mt.line], [2]int{mt.start, mt.end})
	}
	if len(m.matches) == 0 {
		m.cur = -1
		return
	}
	anchor := m.offset
	if m.mode == modeSearch {
		anchor = m.searchOrigin
	}
	m.cur = 0
	for i, mt := range m.matches {
		if mt.line >= anchor {
			m.cur = i
			break
		}
	}
	if jump {
		m.scrollToMatch(m.cur)
	}
}

// scrollToMatch makes match i current and scrolls it into view if needed.
func (m *Model) scrollToMatch(i int) {
	m.cur = i
	line := m.matches[i].line
	if line < m.offset || line >= m.offset+m.contentHeight() {
		m.offset = line - m.contentHeight()/4
	}
	m.scroll(0)
}

func (m *Model) nextMatch(dir int) {
	if len(m.matches) == 0 {
		return
	}
	n := len(m.matches)
	m.scrollToMatch(((m.cur+dir)%n + n) % n)
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.mode = modeNormal
	case tea.KeyEsc:
		m.mode = modeNormal
		m.offset = m.searchOrigin
		m.clearSearch()
	case tea.KeyBackspace:
		if m.query != "" {
			r := []rune(m.query)
			m.query = string(r[:len(r)-1])
		}
		if m.query == "" {
			m.offset = m.searchOrigin
			m.refreshSearch(false)
		} else {
			m.refreshSearch(true)
		}
	case tea.KeySpace:
		m.query += " "
		m.refreshSearch(true)
	case tea.KeyRunes:
		m.query += string(msg.Runes)
		m.refreshSearch(true)
	}
	return m, nil
}
