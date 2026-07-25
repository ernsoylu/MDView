package ui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/mattn/go-runewidth"
	"gopkg.in/yaml.v3"
)

// action is one remappable normal-mode command. Overlay keys (esc, enter,
// backspace in search, the TOC and hint modes) are not remappable: they are
// the conventions that make an overlay dismissable, not preferences.
type action int

const (
	actNone action = iota
	actLineDown
	actLineUp
	actHalfDown
	actHalfUp
	actPageDown
	actPageUp
	actTop
	actBottom
	actSearch
	actNextMatch
	actPrevMatch
	actTOC
	actClearSearch
	actHints
	actJumpBack
	actJumpForward
	actEdit
	actYank
	actHelp
	actQuit
)

// commands is the single source of the keymap: it drives the defaults, the
// names accepted in keys.yaml, and the help overlay, so a remapped key
// cannot leave the help describing something else. Order and group are the
// help layout.
var commands = []struct {
	name  string
	act   action
	keys  []string
	desc  string
	group int
}{
	{"line-down", actLineDown, []string{"j", "down"}, "scroll down", 0},
	{"line-up", actLineUp, []string{"k", "up"}, "scroll up", 0},
	{"half-page-down", actHalfDown, []string{"d", "ctrl+d"}, "half page down", 0},
	{"half-page-up", actHalfUp, []string{"u", "ctrl+u"}, "half page up", 0},
	{"page-down", actPageDown, []string{" ", "pgdown", "ctrl+f"}, "page down", 0},
	{"page-up", actPageUp, []string{"b", "pgup", "ctrl+b"}, "page up", 0},
	{"top", actTop, []string{"g", "home"}, "go to top", 0},
	{"bottom", actBottom, []string{"G", "end"}, "go to bottom", 0},

	{"search", actSearch, []string{"/"}, "search", 1},
	{"next-match", actNextMatch, []string{"n"}, "next match", 1},
	{"prev-match", actPrevMatch, []string{"N"}, "previous match", 1},
	{"toc", actTOC, []string{"t"}, "table of contents", 1},
	{"clear-search", actClearSearch, []string{"esc"}, "clear search", 1},

	{"follow-link", actHints, []string{"f"}, "follow link", 2},
	{"jump-back", actJumpBack, []string{"ctrl+o"}, "jump back", 2},
	{"jump-forward", actJumpForward, []string{"tab"}, "jump forward", 2},
	{"edit", actEdit, []string{"e", "i"}, "edit at this line", 2},
	{"yank", actYank, []string{"y"}, "yank code block", 2},

	{"help", actHelp, []string{"?"}, "toggle this help", 3},
	{"quit", actQuit, []string{"q"}, "quit", 3},
}

// defaultKeymap builds the built-in bindings.
func defaultKeymap() map[string]action {
	m := make(map[string]action, 32)
	for _, c := range commands {
		for _, k := range c.keys {
			m[k] = c.act
		}
	}
	return m
}

// LoadKeys reads a keys.yaml. Listing keys for an action replaces that
// action's defaults wholesale; an empty list leaves it unbound. Unknown
// action names and keys claimed by two actions are errors rather than
// silent surprises. A missing file is not an error — the defaults stand.
func LoadKeys(path string) (map[string]action, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultKeymap(), nil
	}
	if err != nil {
		return nil, err
	}
	var spec map[string][]string
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&spec); err != nil {
		if errors.Is(err, io.EOF) { // fully commented template
			return defaultKeymap(), nil
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return buildKeymap(spec, path)
}

func buildKeymap(spec map[string][]string, path string) (map[string]action, error) {
	known := make(map[string]bool, len(commands))
	for _, c := range commands {
		known[c.name] = true
	}
	for name := range spec {
		if !known[name] {
			return nil, fmt.Errorf("%s: unknown action %q", path, name)
		}
	}
	// Walk commands rather than the spec map so a conflict is reported the
	// same way every run.
	m := make(map[string]action, 32)
	for _, c := range commands {
		keys, ok := spec[c.name]
		if !ok {
			keys = c.keys
		}
		for _, k := range keys {
			if prev, taken := m[k]; taken && prev != c.act {
				return nil, fmt.Errorf("%s: key %q is bound to both %q and %q",
					path, k, actionName(prev), c.name)
			}
			m[k] = c.act
		}
	}
	return m, nil
}

func actionName(a action) string {
	for _, c := range commands {
		if c.act == a {
			return c.name
		}
	}
	return "?"
}

// keyDisplay spells a bubbletea key name the way the help should show it.
var keyDisplay = map[string]string{
	" ": "space", "down": "↓", "up": "↑", "pgdown": "pgdn", "left": "←", "right": "→",
}

func showKey(k string) string {
	if d, ok := keyDisplay[k]; ok {
		return d
	}
	return k
}

// keysFor lists the keys bound to an action: the ones it holds by default
// first, then anything else the user bound, sorted so the help is stable
// across runs rather than following Go's map order.
func (m Model) keysFor(a action, defaults []string) []string {
	var out, extra []string
	for _, k := range defaults {
		if m.keys[k] == a {
			out = append(out, showKey(k))
		}
	}
	for k, act := range m.keys {
		if act == a && !slices.Contains(defaults, k) {
			extra = append(extra, showKey(k))
		}
	}
	slices.Sort(extra)
	return append(out, extra...)
}

const helpFooter = `  enter keeps a search · esc closes overlays
  ctrl+c always quits`

// helpKeysShown caps how many alternatives the overlay lists per action.
// Every binding still works; the help is a reminder, not an inventory, and
// a third alternative pushes the two-column layout past 80 columns.
const helpKeysShown = 2

// helpView renders the key help from the live keymap, so remapped keys are
// what the overlay shows. It lays out two columns per group, dropping to
// one when the terminal is too narrow to hold them.
func (m Model) helpView() string {
	type row struct{ keys, desc string }
	var order []int
	groups := map[int][]row{}
	keyW, descW := 0, 0
	for _, c := range commands {
		keys := m.keysFor(c.act, c.keys)
		if len(keys) == 0 {
			continue // deliberately unbound
		}
		joined := strings.Join(keys[:min(len(keys), helpKeysShown)], " / ")
		keyW = max(keyW, runewidth.StringWidth(joined))
		descW = max(descW, runewidth.StringWidth(c.desc))
		if _, seen := groups[c.group]; !seen {
			order = append(order, c.group)
		}
		groups[c.group] = append(groups[c.group], row{joined, c.desc})
	}

	pad := func(s string, w int) string {
		return s + strings.Repeat(" ", max(0, w-runewidth.StringWidth(s)))
	}
	cell := func(r row, last bool) string {
		s := "  " + pad(r.keys, keyW) + "  "
		if last {
			return s + r.desc
		}
		return s + pad(r.desc, descW)
	}
	// The View adds a leading space, and a wrapped line would break the grid.
	twoCol := m.width == 0 || 2*(keyW+descW+4)+1 <= m.width

	var b strings.Builder
	b.WriteString("mdv — keys\n")
	for _, g := range order {
		b.WriteString("\n")
		rows := groups[g]
		step := 1
		if twoCol {
			step = 2
		}
		for i := 0; i < len(rows); i += step {
			if twoCol && i+1 < len(rows) {
				b.WriteString(cell(rows[i], false) + cell(rows[i+1], true) + "\n")
			} else {
				b.WriteString(cell(rows[i], true) + "\n")
			}
		}
	}
	b.WriteString("\n" + helpFooter)
	return b.String()
}
