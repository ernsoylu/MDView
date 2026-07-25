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

type fileEntry struct {
	rel, abs string
}

// FindMarkdown walks root for markdown files, skipping dot directories and
// dependency trees. Unreadable subdirectories are stepped over rather than
// failing the whole walk — a browser that shows most of a tree beats one
// that shows none of it.
func FindMarkdown(root string) ([]fileEntry, bool, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, false, err
	}
	var out []fileEntry
	truncated := false
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if len(out) >= maxBrowseEntries {
			truncated = true
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if path != abs && (strings.HasPrefix(name, ".") || skipDirs[name]) {
				return fs.SkipDir
			}
			return nil
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".md", ".markdown":
			rel, rerr := filepath.Rel(abs, path)
			if rerr != nil {
				rel = path
			}
			out = append(out, fileEntry{rel: render.Sanitize(rel), abs: path})
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out, truncated, nil
}

// Browser is the file picker mdv shows when it is pointed at a directory,
// or started with no argument at all.
type Browser struct {
	root      string
	entries   []fileEntry
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
func NewBrowser(root string, entries []fileEntry, truncated bool, th theme.Theme) Browser {
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
		sc, ok := fuzzyMatch(b.query, filepath.Base(e.rel))
		if ok {
			sc += 10
		} else if sc, ok = fuzzyMatch(b.query, e.rel); !ok {
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
				b.chosen = b.entries[b.filtered[b.sel]].abs
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
			label := runewidth.Truncate(b.entries[b.filtered[fi]].rel, max(4, b.width-5), "…")
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
