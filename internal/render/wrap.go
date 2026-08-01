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
	w := &wrapState{width: width, srcLine: srcLine}
	for _, s := range segs {
		if s.brk {
			w.emitWord()
			w.flush()
			continue
		}
		w.feedText(s)
	}
	w.emitWord()
	if len(w.cur) > 0 || len(w.lines) == 0 {
		w.flush()
	}
	return w.lines
}

type wrapState struct {
	width, srcLine int
	lines          []Line
	cur            []Span
	curW           int
	pendingSpace   bool
	word           []frag
	wordW          int
}

func (w *wrapState) flush() {
	w.lines = append(w.lines, Line{Spans: w.cur, SourceLine: w.srcLine})
	w.cur = nil
	w.curW = 0
	w.pendingSpace = false
}

func (w *wrapState) emitWord() {
	if w.wordW == 0 {
		return
	}
	spaceW := 0
	if w.pendingSpace {
		spaceW = 1
	}
	if w.curW+spaceW+w.wordW > w.width && w.curW > 0 {
		w.flush()
	}
	if w.wordW > w.width {
		w.chunkWord()
	} else {
		w.appendWord()
	}
	w.word = nil
	w.wordW = 0
	w.pendingSpace = false
}

func (w *wrapState) appendWord() {
	if w.pendingSpace && w.curW > 0 {
		w.cur = append(w.cur, Span{Text: " ", Style: w.word[0].style, Link: w.word[0].link})
		w.curW++
	}
	for _, f := range w.word {
		w.cur = append(w.cur, Span{Text: f.text, Style: f.style, Link: f.link})
	}
	w.curW += w.wordW
}

// chunkWord hard-breaks an oversize word across as many lines as needed.
func (w *wrapState) chunkWord() {
	for _, f := range w.word {
		var b strings.Builder
		bw := 0
		emit := func() {
			if b.Len() > 0 {
				w.cur = append(w.cur, Span{Text: b.String(), Style: f.style, Link: f.link})
				w.curW += bw
				b.Reset()
				bw = 0
			}
		}
		for _, ch := range f.text {
			chW := runewidth.RuneWidth(ch)
			if w.curW+bw+chW > w.width && w.curW+bw > 0 {
				emit()
				w.flush()
			}
			b.WriteRune(ch)
			bw += chW
		}
		emit()
	}
}

func (w *wrapState) feedText(s seg) {
	txt := s.text
	for txt != "" {
		i := strings.IndexAny(txt, " \t")
		if i == -1 {
			w.word = append(w.word, frag{text: txt, style: s.style, link: s.link})
			w.wordW += runewidth.StringWidth(txt)
			return
		}
		if i > 0 {
			w.word = append(w.word, frag{text: txt[:i], style: s.style, link: s.link})
			w.wordW += runewidth.StringWidth(txt[:i])
		}
		w.emitWord()
		w.pendingSpace = true
		txt = txt[i+1:]
	}
}

// chunkSpans hard-breaks spans into rows of at most width columns, used for
// content that must not word-wrap (code lines, raw HTML). These spans are
// built straight from the source rather than through inlines, so control
// characters are replaced here as the text is rechunked.
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
			if isControl(ch) {
				ch = replacement
			}
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
