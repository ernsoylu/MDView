// Package nav holds document navigation state: heading anchors with
// GitHub-style slugs, and the unified back/forward jumplist.
package nav

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ernsoylu/MDView/internal/render"
)

// Slug converts heading text to a GitHub-style anchor: lowercase, spaces
// become hyphens, and everything except letters, digits, `_` and `-` is
// dropped.
func Slug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	return b.String()
}

// Anchors maps each heading's slug to its source line, deduplicating
// repeats the way GitHub does: x, x-1, x-2, …
func Anchors(outline []render.OutlineEntry) map[string]int {
	anchors := make(map[string]int, len(outline))
	counts := make(map[string]int, len(outline))
	for _, e := range outline {
		s := Slug(e.Text)
		if n, seen := counts[s]; seen {
			counts[s] = n + 1
			s = fmt.Sprintf("%s-%d", s, n)
		} else {
			counts[s] = 1
		}
		anchors[s] = e.SourceLine
	}
	return anchors
}
