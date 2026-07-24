package nav

import (
	"testing"

	"github.com/ernsoylu/MDView/internal/render"
)

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello, World!", "hello-world"},
		{"Deep  Nesting", "deep--nesting"}, // each space becomes a hyphen
		{"Émigré Café", "émigré-café"},
		{"snake_case and-hyphen", "snake_case-and-hyphen"},
		{"100% *Styled*", "100-styled"},
	}
	for _, tc := range cases {
		if got := Slug(tc.in); got != tc.want {
			t.Errorf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAnchorsDedupe(t *testing.T) {
	outline := []render.OutlineEntry{
		{Level: 1, Text: "Setup", SourceLine: 1},
		{Level: 2, Text: "Setup", SourceLine: 10},
		{Level: 2, Text: "Setup", SourceLine: 20},
	}
	a := Anchors(outline)
	for slug, want := range map[string]int{"setup": 1, "setup-1": 10, "setup-2": 20} {
		if got, ok := a[slug]; !ok || got != want {
			t.Errorf("anchors[%q] = %d (ok=%v), want %d", slug, got, ok, want)
		}
	}
}

func TestJumplist(t *testing.T) {
	var j Jumplist
	if _, ok := j.Back(Pos{}); ok {
		t.Fatal("Back on empty list succeeded")
	}
	a := Pos{Path: "a.md", Offset: 0, SourceLine: 1}
	b := Pos{Path: "a.md", Offset: 40, SourceLine: 90}
	c := Pos{Path: "b.md", Offset: 5, SourceLine: 12}

	j.Push(a) // at a, jumping away
	j.Push(b) // at b, jumping away

	got, ok := j.Back(c) // currently at c
	if !ok || got != b {
		t.Fatalf("Back = %+v (%v), want %+v", got, ok, b)
	}
	got, ok = j.Back(b)
	if !ok || got != a {
		t.Fatalf("Back = %+v (%v), want %+v", got, ok, a)
	}
	got, ok = j.Forward(a)
	if !ok || got != b {
		t.Fatalf("Forward = %+v (%v), want %+v", got, ok, b)
	}
	// A fresh push clears forward history.
	j.Push(b)
	if _, ok := j.Forward(b); ok {
		t.Error("Forward after Push should be empty")
	}
}
