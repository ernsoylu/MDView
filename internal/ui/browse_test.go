package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/ernsoylu/MDView/internal/theme"
)

// tree builds a directory of markdown and noise to browse.
func tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"README.md",
		"guide.markdown",
		"notes.txt",            // not markdown
		"docs/install.md",      // nested
		"docs/img/diagram.png", // not markdown
		".hidden/secret.md",    // dot directory
		"node_modules/dep.md",  // dependency tree
		"vendor/other.md",      // dependency tree
		"src/deep/nested/api.md",
	}
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func rels(entries []FileEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = filepath.ToSlash(e.Rel)
	}
	return out
}

func TestFindMarkdown(t *testing.T) {
	entries, truncated, err := FindMarkdown(tree(t))
	if err != nil {
		t.Fatal(err)
	}
	if truncated {
		t.Error("a nine-file tree should not report truncation")
	}
	got := strings.Join(rels(entries), " ")
	want := "README.md docs/install.md guide.markdown src/deep/nested/api.md"
	if got != want {
		t.Errorf("found %q, want %q", got, want)
	}
}

func TestBrowserFiltersAndPicks(t *testing.T) {
	root := tree(t)
	entries, trunc, err := FindMarkdown(root)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBrowser(root, entries, trunc, theme.Plain())
	if len(b.filtered) != len(entries) {
		t.Fatalf("empty query matched %d of %d entries", len(b.filtered), len(entries))
	}

	// Typing narrows to the matching file.
	for _, r := range "install" {
		bm, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		b = bm.(Browser)
	}
	if len(b.filtered) != 1 {
		t.Fatalf("query %q matched %d entries, want 1", b.query, len(b.filtered))
	}
	bm, _ := b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b = bm.(Browser)
	got, ok := b.Chosen()
	if !ok || filepath.Base(got) != "install.md" {
		t.Errorf("picked %q, want docs/install.md", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("picked path %q should be absolute", got)
	}
}

// TestBrowserPrefersBasenameMatches is why refilter scores the basename
// separately: typing a filename should not rank a path that merely
// contains the letters above it.
func TestBrowserPrefersBasenameMatches(t *testing.T) {
	entries := []FileEntry{
		{Rel: "some/introduction/notes.md", Abs: "/a"},
		{Rel: "api.md", Abs: "/b"},
	}
	b := NewBrowser("/", entries, false, theme.Plain())
	for _, r := range "api" {
		bm, _ := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		b = bm.(Browser)
	}
	if len(b.filtered) == 0 {
		t.Fatal("no matches for \"api\"")
	}
	if got := entries[b.filtered[0]].Rel; got != "api.md" {
		t.Errorf("best match is %q, want api.md", got)
	}
}

func TestBrowserExitPaths(t *testing.T) {
	root := tree(t)
	entries, trunc, _ := FindMarkdown(root)

	b := NewBrowser(root, entries, trunc, theme.Plain())
	bm, _ := b.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := bm.(Browser).Chosen(); ok {
		t.Error("esc should leave without picking a file")
	}
	if bm.(Browser).HardQuit() {
		t.Error("esc is not a hard quit; it should not stop mdv coming back")
	}

	b = NewBrowser(root, entries, trunc, theme.Plain())
	bm, _ = b.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !bm.(Browser).HardQuit() {
		t.Error("ctrl+c should report a hard quit")
	}
}

// TestPagerReportsHardQuit is the other half of the browse loop: q comes
// back to the list, ctrl+c does not.
func TestPagerReportsHardQuit(t *testing.T) {
	m := keyModel(t, nil)
	if mm, _ := m.Update(runes("q")); mm.(Model).HardQuit() {
		t.Error("q should not report a hard quit")
	}
	if mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); !mm.(Model).HardQuit() {
		t.Error("ctrl+c should report a hard quit")
	}
}

func TestBrowserViewFitsWidth(t *testing.T) {
	root := tree(t)
	entries, trunc, _ := FindMarkdown(root)
	for _, w := range []int{20, 40, 80} {
		b := NewBrowser(root, entries, trunc, theme.Plain())
		bm, _ := b.Update(tea.WindowSizeMsg{Width: w, Height: 10})
		for _, ln := range strings.Split(bm.(Browser).View(), "\n") {
			if got := runewidth.StringWidth(ln); got > w {
				t.Errorf("at width %d a line is %d cells: %q", w, got, ln)
			}
		}
	}
}

func TestBrowserEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	entries, trunc, err := FindMarkdown(root)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBrowser(root, entries, trunc, theme.Plain())
	bm, _ := b.Update(tea.WindowSizeMsg{Width: 60, Height: 8})
	b = bm.(Browser)
	if !strings.Contains(b.View(), "no markdown files") {
		t.Errorf("an empty tree should say so: %q", b.View())
	}
	// Enter must not pick anything when there is nothing to pick.
	bm, _ = b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if _, ok := bm.(Browser).Chosen(); ok {
		t.Error("enter picked a file from an empty list")
	}
}

// TestPagerStatusBarFitsWidth covers the same row in the pager: a status
// bar wider than the terminal wraps and eats a line of the document.
func TestPagerStatusBarFitsWidth(t *testing.T) {
	for _, w := range []int{12, 20, 40, 80} {
		m := keyModel(t, nil)
		m.width = w
		m.name = "a-rather-long-file-name-for-the-status-bar.md"
		for _, ln := range strings.Split(m.View(), "\n") {
			if got := runewidth.StringWidth(ln); got > w {
				t.Errorf("at width %d a line is %d cells: %q", w, got, ln)
			}
		}
	}
}
