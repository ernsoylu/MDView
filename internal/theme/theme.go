// Package theme maps semantic markdown elements to terminal styles. Color
// degradation (truecolor → 256 → 16 → mono) and NO_COLOR are handled by the
// lipgloss/termenv profile, not here.
package theme

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Heading       [6]lipgloss.Style
	Emph          lipgloss.Style
	Strong        lipgloss.Style
	Strike        lipgloss.Style
	CodeSpan      lipgloss.Style
	CodeBlock     lipgloss.Style // base style for code text without a token color
	Link          lipgloss.Style
	Image         lipgloss.Style
	QuoteBar      lipgloss.Style
	ListMarker    lipgloss.Style
	TableBorder   lipgloss.Style
	TableHeader   lipgloss.Style
	Rule          lipgloss.Style
	Dim           lipgloss.Style // raw HTML, image URLs, de-emphasized text
	StatusBar     lipgloss.Style
	SearchHit     lipgloss.Style // every search match in the viewport
	SearchCurrent lipgloss.Style // the match n/N is parked on
	HintLabel     lipgloss.Style // link-hint labels in follow mode
	Chroma        *chroma.Style  // syntax highlighting palette for code blocks
}

// Default is the built-in adaptive theme (Catppuccin Latte/Mocha accents).
func Default() Theme {
	heading := func(light, dark string) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: light, Dark: dark})
	}
	var t Theme
	t.Heading[0] = heading("#8839ef", "#cba6f7").Underline(true)
	t.Heading[1] = heading("#1e66f5", "#89b4fa")
	t.Heading[2] = heading("#179299", "#94e2d5")
	t.Heading[3] = heading("#df8e1d", "#f9e2af")
	t.Heading[4] = heading("#6c6f85", "#a6adc8")
	t.Heading[5] = heading("#6c6f85", "#a6adc8").Italic(true)
	t.Emph = lipgloss.NewStyle().Italic(true)
	t.Strong = lipgloss.NewStyle().Bold(true)
	t.Strike = lipgloss.NewStyle().Strikethrough(true)
	t.CodeSpan = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"})
	t.CodeBlock = lipgloss.NewStyle()
	t.Link = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"})
	t.Image = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.AdaptiveColor{Light: "#7287fd", Dark: "#b4befe"})
	t.QuoteBar = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"})
	t.ListMarker = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"})
	t.TableBorder = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9ca0b0", Dark: "#6c7086"})
	t.TableHeader = lipgloss.NewStyle().Bold(true)
	t.Rule = t.TableBorder
	t.Dim = lipgloss.NewStyle().Faint(true)
	t.StatusBar = lipgloss.NewStyle().Reverse(true)
	t.SearchHit = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"}).
		Background(lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"})
	t.SearchCurrent = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#1e1e2e"}).
		Background(lipgloss.AdaptiveColor{Light: "#fe640b", Dark: "#fab387"})
	t.HintLabel = lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#11111b"}).
		Background(lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"})
	t.Chroma = styles.Get("catppuccin-mocha")
	return t
}

// Plain has no styling at all. Used by golden tests, where it keeps output
// deterministic and readable.
func Plain() Theme {
	s := lipgloss.NewStyle()
	var t Theme
	for i := range t.Heading {
		t.Heading[i] = s
	}
	t.Emph, t.Strong, t.Strike, t.CodeSpan, t.CodeBlock = s, s, s, s, s
	t.Link, t.Image, t.QuoteBar, t.ListMarker = s, s, s, s
	t.TableBorder, t.TableHeader, t.Rule, t.Dim, t.StatusBar = s, s, s, s, s
	t.SearchHit, t.SearchCurrent, t.HintLabel = s, s, s
	t.Chroma = styles.Fallback
	return t
}
