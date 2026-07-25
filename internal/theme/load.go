package theme

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// styleSpec is one style entry in a YAML theme file. Colors are hex or
// ANSI numbers, passed straight to lipgloss.
type styleSpec struct {
	Fg            string `yaml:"fg"`
	Bg            string `yaml:"bg"`
	Bold          bool   `yaml:"bold"`
	Italic        bool   `yaml:"italic"`
	Underline     bool   `yaml:"underline"`
	Faint         bool   `yaml:"faint"`
	Reverse       bool   `yaml:"reverse"`
	Strikethrough bool   `yaml:"strikethrough"`
}

func (s styleSpec) style() lipgloss.Style {
	st := lipgloss.NewStyle()
	if s.Fg != "" {
		st = st.Foreground(lipgloss.Color(s.Fg))
	}
	if s.Bg != "" {
		st = st.Background(lipgloss.Color(s.Bg))
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Underline {
		st = st.Underline(true)
	}
	if s.Faint {
		st = st.Faint(true)
	}
	if s.Reverse {
		st = st.Reverse(true)
	}
	if s.Strikethrough {
		st = st.Strikethrough(true)
	}
	return st
}

// themeFile is the YAML schema. Every key is optional; present keys
// replace the default theme's style for that element.
type themeFile struct {
	Heading1      *styleSpec `yaml:"heading1"`
	Heading2      *styleSpec `yaml:"heading2"`
	Heading3      *styleSpec `yaml:"heading3"`
	Heading4      *styleSpec `yaml:"heading4"`
	Heading5      *styleSpec `yaml:"heading5"`
	Heading6      *styleSpec `yaml:"heading6"`
	Emph          *styleSpec `yaml:"emph"`
	Strong        *styleSpec `yaml:"strong"`
	Strike        *styleSpec `yaml:"strike"`
	CodeSpan      *styleSpec `yaml:"codespan"`
	CodeBlock     *styleSpec `yaml:"codeblock"`
	Link          *styleSpec `yaml:"link"`
	Image         *styleSpec `yaml:"image"`
	QuoteBar      *styleSpec `yaml:"quotebar"`
	ListMarker    *styleSpec `yaml:"listmarker"`
	TableBorder   *styleSpec `yaml:"tableborder"`
	TableHeader   *styleSpec `yaml:"tableheader"`
	Rule          *styleSpec `yaml:"rule"`
	Dim           *styleSpec `yaml:"dim"`
	StatusBar     *styleSpec `yaml:"statusbar"`
	SearchHit     *styleSpec `yaml:"searchhit"`
	SearchCurrent *styleSpec `yaml:"searchcurrent"`
	HintLabel     *styleSpec `yaml:"hintlabel"`
	Selection     *styleSpec `yaml:"selection"`
	Chroma        string     `yaml:"chroma"`
}

// Load reads a YAML theme file whose keys overlay the default theme.
// Unknown keys are errors (they are almost always typos); unknown chroma
// style names fall back to chroma's default palette.
func Load(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}
	var f themeFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) { // empty or fully commented: no overrides
			return Default(), nil
		}
		return Theme{}, fmt.Errorf("theme %s: %w", path, err)
	}
	t := Default()
	apply := func(dst *lipgloss.Style, src *styleSpec) {
		if src != nil {
			*dst = src.style()
		}
	}
	for i, src := range []*styleSpec{f.Heading1, f.Heading2, f.Heading3, f.Heading4, f.Heading5, f.Heading6} {
		apply(&t.Heading[i], src)
	}
	apply(&t.Emph, f.Emph)
	apply(&t.Strong, f.Strong)
	apply(&t.Strike, f.Strike)
	apply(&t.CodeSpan, f.CodeSpan)
	apply(&t.CodeBlock, f.CodeBlock)
	apply(&t.Link, f.Link)
	apply(&t.Image, f.Image)
	apply(&t.QuoteBar, f.QuoteBar)
	apply(&t.ListMarker, f.ListMarker)
	apply(&t.TableBorder, f.TableBorder)
	apply(&t.TableHeader, f.TableHeader)
	apply(&t.Rule, f.Rule)
	apply(&t.Dim, f.Dim)
	apply(&t.StatusBar, f.StatusBar)
	apply(&t.SearchHit, f.SearchHit)
	apply(&t.SearchCurrent, f.SearchCurrent)
	apply(&t.HintLabel, f.HintLabel)
	apply(&t.Selection, f.Selection)
	if f.Chroma != "" {
		t.Chroma = styles.Get(f.Chroma)
	}
	return t, nil
}
