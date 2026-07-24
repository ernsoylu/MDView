package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

var (
	keyCtrlO = tea.KeyMsg{Type: tea.KeyCtrlO}
	keyTab   = tea.KeyMsg{Type: tea.KeyTab}
)

func docWithLinks() string {
	var b strings.Builder
	b.WriteString("[jump](#target) and [ext](https://example.com/x)\n\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "filler %d\n\n", i)
	}
	b.WriteString("## Target\n\nyou made it\n")
	return b.String()
}

func newLinkModel(t *testing.T) Model {
	t.Helper()
	m := New(parser.Parse([]byte(docWithLinks())), theme.Plain(), "a.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	return mm.(Model)
}

func TestHintLabels(t *testing.T) {
	if got := makeLabels(3); got[0] != "a" || got[1] != "s" || got[2] != "d" {
		t.Errorf("labels(3) = %v", got)
	}
	two := makeLabels(30)
	seen := map[string]bool{}
	for _, l := range two {
		if len(l) != 2 || seen[l] {
			t.Fatalf("labels(30) not unique two-char: %v", two)
		}
		seen[l] = true
	}
}

func TestHintAnchorFollowAndJumplist(t *testing.T) {
	m := newLinkModel(t)
	m = press(m, runes("f"))
	if m.mode != modeHints || len(m.hints) != 2 {
		t.Fatalf("hints: mode=%v n=%d, want 2", m.mode, len(m.hints))
	}
	m = press(m, runes("a")) // first hint: the #target anchor
	if m.mode != modeNormal || m.offset == 0 {
		t.Fatalf("anchor follow: mode=%v offset=%d", m.mode, m.offset)
	}
	visible := strings.Join(m.rendered[m.offset:min(m.offset+m.contentHeight(), len(m.rendered))], "\n")
	if !strings.Contains(visible, "Target") {
		t.Errorf("Target heading not visible after anchor jump; viewport: %q", visible)
	}
	m = press(m, keyCtrlO)
	if m.offset != 0 {
		t.Errorf("ctrl+o: offset = %d, want 0", m.offset)
	}
	m = press(m, keyTab)
	if m.offset == 0 {
		t.Errorf("tab (forward) did not return to the anchor target")
	}
}

func TestHintExternalOpener(t *testing.T) {
	m := newLinkModel(t)
	var opened string
	m.opener = func(target string) error { opened = target; return nil }
	m = press(m, runes("f"))
	mm, cmd := m.Update(runes("s")) // second hint: the https link
	m = mm.(Model)
	if cmd == nil {
		t.Fatal("external link produced no command")
	}
	cmd()
	if opened != "https://example.com/x" {
		t.Errorf("opened %q, want the https link", opened)
	}
}

func TestFollowRelativeFileWithFragment(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("# B\n\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "filler %d\n\n", i)
	}
	b.WriteString("## Section Two\n\nfound it\n")
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	aSrc := "# A\n\n[to b](./b.md#section-two)\n"
	aPath := filepath.Join(dir, "a.md")
	if err := os.WriteFile(aPath, []byte(aSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(parser.Parse([]byte(aSrc)), theme.Plain(), "a.md", aPath)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = mm.(Model)

	m = press(m, runes("f"), runes("a"))
	if filepath.Base(m.path) != "b.md" {
		t.Fatalf("path = %q, want b.md", m.path)
	}
	if m.flash != "" {
		t.Fatalf("flash = %q", m.flash)
	}
	visible := strings.Join(m.rendered[m.offset:min(m.offset+m.contentHeight(), len(m.rendered))], "\n")
	if !strings.Contains(visible, "Section Two") {
		t.Errorf("fragment jump failed; viewport: %q", visible)
	}

	m = press(m, keyCtrlO)
	if filepath.Base(m.path) != "a.md" || m.offset != 0 {
		t.Errorf("ctrl+o: path=%q offset=%d, want back in a.md at top", m.path, m.offset)
	}
	m = press(m, keyTab)
	if filepath.Base(m.path) != "b.md" {
		t.Errorf("tab: path=%q, want b.md again", m.path)
	}
}

func TestWatchReloadPreservesAnchor(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("# C\n\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "filler %d\n\n", i)
	}
	b.WriteString("## Marker\n\noriginal text\n")
	path := filepath.Join(dir, "c.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(parser.Parse([]byte(b.String())), theme.Plain(), "c.md", path)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = mm.(Model)
	m.jumpToSourceLine(m.outline[len(m.outline)-1].SourceLine)
	if m.offset == 0 {
		t.Fatal("setup: expected to be scrolled down")
	}

	edited := strings.Replace(b.String(), "original text", "edited text", 1)
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	mm, _ = m.Update(fileChangedMsg{name: path})
	m = mm.(Model)

	visible := strings.Join(m.rendered[m.offset:min(m.offset+m.contentHeight(), len(m.rendered))], "\n")
	if !strings.Contains(visible, "edited text") {
		t.Errorf("reload did not pick up the edit; viewport: %q", visible)
	}
	if m.offset == 0 {
		t.Errorf("reload lost the anchored position")
	}
}

func TestEditorStdinGuard(t *testing.T) {
	m := newLinkModel(t) // path is ""
	mm, cmd := m.Update(runes("e"))
	m = mm.(Model)
	if cmd != nil {
		t.Error("editor command should not run for stdin documents")
	}
	if !strings.Contains(m.flash, "stdin") {
		t.Errorf("flash = %q, want a stdin explanation", m.flash)
	}
}

func TestEditorCmdForFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.md")
	if err := os.WriteFile(path, []byte("# D\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(parser.Parse([]byte("# D\n")), theme.Plain(), "d.md", path)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = mm.(Model)
	if _, cmd := m.Update(runes("e")); cmd == nil {
		t.Error("expected an editor command for a file-backed document")
	}
}

func TestLinkAtCell(t *testing.T) {
	lines := render.Render(parser.Parse([]byte("[ab](https://x) cd\n")), theme.Plain(), 80)
	ln := lines[0]
	if got := linkAtCell(ln, 0); got != "https://x" {
		t.Errorf("cell 0 = %q", got)
	}
	if got := linkAtCell(ln, 1); got != "https://x" {
		t.Errorf("cell 1 = %q", got)
	}
	if got := linkAtCell(ln, 4); got != "" {
		t.Errorf("cell 4 = %q, want no link", got)
	}
}

func TestOverlayLabel(t *testing.T) {
	lines := render.Render(parser.Parse([]byte("xx [link](https://u) yy\n")), theme.Plain(), 80)
	ln := lines[0]
	at := strings.Index(ln.Plain(), "link")
	th := theme.Plain()
	st := &th.HintLabel
	got := overlayLabel(ln, at, "a", st)
	if got.Plain() != "xx aink yy" {
		t.Errorf("overlay plain = %q, want %q", got.Plain(), "xx aink yy")
	}
	found := false
	for _, sp := range got.Spans {
		if sp.Text == "a" && sp.Style == st {
			found = true
		}
	}
	if !found {
		t.Error("label span with hint style not found")
	}
}

func TestWrappedLinkGetsOneHint(t *testing.T) {
	src := "[a very long link label that must wrap](https://example.com/long)\n"
	m := New(parser.Parse([]byte(src)), theme.Plain(), "w.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 12})
	m = mm.(Model)
	m = press(m, runes("f"))
	if len(m.hints) != 1 {
		t.Errorf("wrapped link produced %d hints, want 1", len(m.hints))
	}
}
