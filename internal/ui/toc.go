package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

// refilterTOC ranks outline entries against the fuzzy query; with an empty
// query every heading matches with score 0, keeping document order.
func (m *Model) refilterTOC() {
	type scored struct{ idx, score int }
	var arr []scored
	for i, e := range m.outline {
		if sc, ok := fuzzyMatch(m.tocQuery, e.Text); ok {
			arr = append(arr, scored{i, sc})
		}
	}
	sort.SliceStable(arr, func(a, b int) bool { return arr[a].score > arr[b].score })
	m.filtered = m.filtered[:0]
	for _, s := range arr {
		m.filtered = append(m.filtered, s.idx)
	}
	if m.tocSel >= len(m.filtered) {
		m.tocSel = 0
	}
}

func (m Model) updateTOC(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
	case tea.KeyEnter:
		if len(m.filtered) > 0 {
			m.jumpToSourceLine(m.outline[m.filtered[m.tocSel]].SourceLine)
		}
		m.mode = modeNormal
	case tea.KeyUp, tea.KeyCtrlP, tea.KeyShiftTab:
		if m.tocSel > 0 {
			m.tocSel--
		}
	case tea.KeyDown, tea.KeyCtrlN, tea.KeyTab:
		if m.tocSel < len(m.filtered)-1 {
			m.tocSel++
		}
	case tea.KeyBackspace:
		if m.tocQuery != "" {
			r := []rune(m.tocQuery)
			m.tocQuery = string(r[:len(r)-1])
		}
		m.refilterTOC()
	case tea.KeySpace:
		m.tocQuery += " "
		m.refilterTOC()
	case tea.KeyRunes:
		m.tocQuery += string(msg.Runes)
		m.tocSel = 0
		m.refilterTOC()
	}
	return m, nil
}

func (m Model) writeTOC(b *strings.Builder) {
	b.WriteString(" t› " + m.tocQuery + "\n")
	rows := m.contentHeight() - 1
	start := 0
	if rows > 0 && m.tocSel >= rows {
		start = m.tocSel - rows + 1
	}
	for i := 0; i < rows; i++ {
		fi := start + i
		switch {
		case len(m.filtered) == 0 && i == 0:
			b.WriteString("   (no matching headings)")
		case fi < len(m.filtered):
			e := m.outline[m.filtered[fi]]
			label := strings.Repeat("  ", e.Level-1) + e.Text
			maxw := m.width - 5
			if maxw < 4 {
				maxw = 4
			}
			label = runewidth.Truncate(label, maxw, "…")
			if fi == m.tocSel {
				b.WriteString(" " + m.th.StatusBar.Render("› "+label))
			} else {
				b.WriteString("   " + m.th.Heading[e.Level-1].Render(label))
			}
		}
		b.WriteString("\n")
	}
}
