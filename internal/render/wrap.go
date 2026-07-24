package render

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// frag is a piece of a word carrying one style.
type frag struct {
	text  string
	style *lipgloss.Style
	link  string
}

// wrap breaks a seg stream into lines of at most width columns, breaking at
// spaces and chunking words wider than a whole line (e.g. long URLs).
func wrap(segs []seg, width int, srcLine int) []Line {
	if width < 1 {
		width = 1
	}
	var lines []Line
	var cur []Span
	curW := 0
	pendingSpace := false

	flush := func() {
		lines = append(lines, Line{Spans: cur, SourceLine: srcLine})
		cur = nil
		curW = 0
		pendingSpace = false
	}

	var word []frag
	wordW := 0

	emitWord := func() {
		if wordW == 0 {
			return
		}
		spaceW := 0
		if pendingSpace {
			spaceW = 1
		}
		if curW+spaceW+wordW > width && curW > 0 {
			flush()
		}
		if wordW > width {
			// Chunk an oversize word across as many lines as needed.
			for _, f := range word {
				var b strings.Builder
				bw := 0
				emit := func() {
					if b.Len() > 0 {
						cur = append(cur, Span{Text: b.String(), Style: f.style, Link: f.link})
						curW += bw
						b.Reset()
						bw = 0
					}
				}
				for _, ch := range f.text {
					chW := runewidth.RuneWidth(ch)
					if curW+bw+chW > width && curW+bw > 0 {
						emit()
						flush()
					}
					b.WriteRune(ch)
					bw += chW
				}
				emit()
			}
		} else {
			if pendingSpace && curW > 0 {
				cur = append(cur, Span{Text: " ", Style: word[0].style, Link: word[0].link})
				curW++
			}
			for _, f := range word {
				cur = append(cur, Span{Text: f.text, Style: f.style, Link: f.link})
			}
			curW += wordW
		}
		word = nil
		wordW = 0
		pendingSpace = false
	}

	for _, s := range segs {
		if s.brk {
			emitWord()
			flush()
			continue
		}
		txt := s.text
		for txt != "" {
			i := strings.IndexAny(txt, " \t")
			if i == -1 {
				word = append(word, frag{text: txt, style: s.style, link: s.link})
				wordW += runewidth.StringWidth(txt)
				break
			}
			if i > 0 {
				word = append(word, frag{text: txt[:i], style: s.style, link: s.link})
				wordW += runewidth.StringWidth(txt[:i])
			}
			emitWord()
			pendingSpace = true
			txt = txt[i+1:]
		}
	}
	emitWord()
	if len(cur) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

// chunkSpans hard-breaks spans into rows of at most width columns, used for
// content that must not word-wrap (code lines, raw HTML).
func chunkSpans(spans []Span, width int) [][]Span {
	if width < 1 {
		width = 1
	}
	var rows [][]Span
	var cur []Span
	w := 0
	for _, s := range spans {
		var b strings.Builder
		bw := 0
		emit := func() {
			if b.Len() > 0 {
				cur = append(cur, Span{Text: b.String(), Style: s.Style, Link: s.Link})
				w += bw
				b.Reset()
				bw = 0
			}
		}
		for _, ch := range s.Text {
			chW := runewidth.RuneWidth(ch)
			if w+bw+chW > width && w+bw > 0 {
				emit()
				rows = append(rows, cur)
				cur = nil
				w = 0
			}
			b.WriteRune(ch)
			bw += chW
		}
		emit()
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}
	return rows
}

// truncateSpans cuts spans down to at most width columns, ending with an
// ellipsis when anything was cut.
func truncateSpans(spans []Span, width int) []Span {
	if spansWidth(spans) <= width {
		return spans
	}
	var out []Span
	w := 0
	for _, s := range spans {
		var b strings.Builder
		for _, ch := range s.Text {
			chW := runewidth.RuneWidth(ch)
			if w+chW > width-1 {
				if b.Len() > 0 {
					out = append(out, Span{Text: b.String(), Style: s.Style, Link: s.Link})
				}
				out = append(out, Span{Text: "…", Style: s.Style})
				return out
			}
			b.WriteRune(ch)
			w += chW
		}
		out = append(out, Span{Text: b.String(), Style: s.Style, Link: s.Link})
	}
	return out
}
