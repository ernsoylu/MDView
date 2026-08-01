package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
)

// maxBrowseEntries stops a walk that wandered somewhere enormous. A list
// this long is past the point of being browsable anyway, and the count in
// the status bar says when it bit.
const maxBrowseEntries = 5000

// skipDirs are trees that hold other people's markdown, not yours.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "target": true,
}

// FileEntry is one markdown file the browser can open: Rel is what the
// list shows, Abs is what gets read.
type FileEntry struct {
	Rel, Abs string
}

// EntriesFromPaths builds a browser list from explicit paths, for the
// several-files-on-the-command-line case. Each must exist and be readable.
func EntriesFromPaths(paths []string) ([]FileEntry, error) {
	out := make([]FileEntry, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(abs); err != nil {
			return nil, err
		} else if fi.IsDir() {
			return nil, fmt.Errorf("%s is a directory", p)
		}
		out = append(out, FileEntry{Rel: render.Sanitize(p), Abs: abs})
	}
	return out, nil
}

// FindMarkdown walks root for markdown files, skipping dot directories and
// dependency trees. Unreadable subdirectories are stepped over rather than
// failing the whole walk — a browser that shows most of a tree beats one
// that shows none of it.
func FindMarkdown(root string) ([]FileEntry, bool, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, false, err
	}
	var out []FileEntry
	truncated := false
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		entry, skip, done, walkErr := walkMarkdown(abs, path, d, err, len(out))
		if walkErr != nil {
			return walkErr
		}
		if done {
			truncated = true
			return filepath.SkipAll
		}
		if skip {
			return fs.SkipDir
		}
		if entry != nil {
			out = append(out, *entry)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, truncated, nil
}

// walkMarkdown classifies one WalkDir visit. done means the entry cap was
// hit; skip means the directory should be skipped.
func walkMarkdown(root, path string, d fs.DirEntry, err error, n int) (entry *FileEntry, skip, done bool, walkErr error) {
	if err != nil {
		if d != nil && d.IsDir() {
			return nil, true, false, nil
		}
		return nil, false, false, nil
	}
	if n >= maxBrowseEntries {
		return nil, false, true, nil
	}
	name := d.Name()
	if d.IsDir() {
		if path != root && (strings.HasPrefix(name, ".") || skipDirs[name]) {
			return nil, true, false, nil
		}
		return nil, false, false, nil
	}
	if !isMarkdownName(name) {
		return nil, false, false, nil
	}
	rel, rerr := filepath.Rel(root, path)
	if rerr != nil {
		rel = path
	}
	e := FileEntry{Rel: render.Sanitize(rel), Abs: path}
	return &e, false, false, nil
}

func isMarkdownName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown":
		return true
	}
	return false
}

// Browser is the file picker mdv shows when it is pointed at a directory,
// or started with no argument at all.
type Browser struct {
	root      string
	entries   []FileEntry
	truncated bool
	th        theme.Theme

	query    string
	filtered []int
	sel      int

	width, height int

	chosen   string // the file to open; "" when the user left instead
	hardQuit bool   // ctrl+c: do not come back here
}

// NewBrowser builds the picker over an already-scanned entry list.
func NewBrowser(root string, entries []FileEntry, truncated bool, th theme.Theme) Browser {
	b := Browser{root: root, entries: entries, truncated: truncated, th: th}
	b.refilter()
	return b
}

// Chosen returns the picked file, and whether the user picked one at all.
func (b Browser) Chosen() (string, bool) { return b.chosen, b.chosen != "" }

// HardQuit reports a ctrl+c, which should leave mdv rather than return to
// the list.
func (b Browser) HardQuit() bool { return b.hardQuit }

func (b Browser) Init() tea.Cmd { return nil }

// refilter ranks entries against the query. Matching the basename beats
// matching the path, so "read" finds README.md before docs/threading.md.
func (b *Browser) refilter() {
	type scored struct{ idx, score int }
	var arr []scored
	for i, e := range b.entries {
		sc, ok := fuzzyMatch(b.query, filepath.Base(e.Rel))
		if ok {
			sc += 10
		} else if sc, ok = fuzzyMatch(b.query, e.Rel); !ok {
			continue
		}
		arr = append(arr, scored{i, sc})
	}
	sort.SliceStable(arr, func(x, y int) bool { return arr[x].score > arr[y].score })
	b.filtered = b.filtered[:0]
	for _, s := range arr {
		b.filtered = append(b.filtered, s.idx)
	}
	if b.sel >= len(b.filtered) {
		b.sel = 0
	}
}

func (b Browser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width, b.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			b.hardQuit = true
			return b, tea.Quit
		case tea.KeyEsc:
			return b, tea.Quit
		case tea.KeyEnter:
			if len(b.filtered) > 0 {
				b.chosen = b.entries[b.filtered[b.sel]].Abs
			}
			return b, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP, tea.KeyShiftTab:
			if b.sel > 0 {
				b.sel--
			}
		case tea.KeyDown, tea.KeyCtrlN, tea.KeyTab:
			if b.sel < len(b.filtered)-1 {
				b.sel++
			}
		case tea.KeyBackspace:
			if b.query != "" {
				r := []rune(b.query)
				b.query = string(r[:len(r)-1])
			}
			b.refilter()
		case tea.KeySpace:
			b.query += " "
			b.refilter()
		case tea.KeyRunes:
			b.query += string(msg.Runes)
			b.sel = 0
			b.refilter()
		}
	}
	return b, nil
}

func (b Browser) rows() int {
	if b.height <= 2 {
		return 1
	}
	return b.height - 2
}

func (b Browser) View() string {
	if b.width == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(" open› " + b.query + "\n")

	rows := b.rows()
	start := 0
	if b.sel >= rows {
		start = b.sel - rows + 1
	}
	for i := 0; i < rows; i++ {
		fi := start + i
		switch {
		case len(b.entries) == 0 && i == 0:
			sb.WriteString("   (no markdown files under " + render.Sanitize(b.root) + ")")
		case len(b.filtered) == 0 && i == 0:
			sb.WriteString("   (nothing matches)")
		case fi < len(b.filtered):
			label := runewidth.Truncate(b.entries[b.filtered[fi]].Rel, max(4, b.width-5), "…")
			if fi == b.sel {
				sb.WriteString(" " + b.th.StatusBar.Render("› "+label))
			} else {
				sb.WriteString("   " + b.th.Dim.Render(label))
			}
		}
		sb.WriteString("\n")
	}

	count := fmt.Sprintf("%d/%d", len(b.filtered), len(b.entries))
	if b.truncated {
		count += "+"
	}
	right := count + "  enter open · esc quit "
	left := " " + render.Sanitize(b.root) + "  "
	sb.WriteString(b.th.StatusBar.Render(fitRow(left, right, b.width)))
	return sb.String()
}

// IsDir reports whether the path names a directory, so the CLI can tell a
// document from a tree to browse.
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
