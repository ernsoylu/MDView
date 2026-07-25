package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/ernsoylu/MDView/internal/config"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

func keyModel(t *testing.T, keys map[string]action) Model {
	t.Helper()
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString("line ")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString("\n\n")
	}
	m := New(parser.Parse([]byte(b.String())), theme.Plain(), "k.md", "").WithKeys(keys)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	return mm.(Model)
}

func TestDefaultKeymapDrivesScrolling(t *testing.T) {
	m := keyModel(t, nil)
	if m = press(m, runes("j")); m.offset != 1 {
		t.Errorf("j left offset at %d, want 1", m.offset)
	}
	if m = press(m, runes("k")); m.offset != 0 {
		t.Errorf("k left offset at %d, want 0", m.offset)
	}
}

// TestRemappedKeysReplaceDefaults is the point of the feature: a non-vim
// user binds their own keys and the old ones stop moving the viewport.
func TestRemappedKeysReplaceDefaults(t *testing.T) {
	keys, err := buildKeymap(map[string][]string{
		"line-down": {"ctrl+n"},
		"line-up":   {"ctrl+p"},
	}, "keys.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := keyModel(t, keys)

	if m2 := press(m, tea.KeyMsg{Type: tea.KeyCtrlN}); m2.offset != 1 {
		t.Errorf("ctrl+n left offset at %d, want 1", m2.offset)
	}
	if m2 := press(m, runes("j")); m2.offset != 0 {
		t.Errorf("j still scrolled after being rebound away (offset %d)", m2.offset)
	}
}

func TestUnbindingAnAction(t *testing.T) {
	keys, err := buildKeymap(map[string][]string{"line-down": {}}, "keys.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := keyModel(t, keys)
	if m2 := press(m, runes("j")); m2.offset != 0 {
		t.Errorf("j scrolled although line-down was unbound (offset %d)", m2.offset)
	}
	// Unrelated actions keep their defaults.
	if m2 := press(m, runes("G")); m2.offset == 0 {
		t.Error("G stopped working when an unrelated action was unbound")
	}
}

func TestKeymapErrors(t *testing.T) {
	if _, err := buildKeymap(map[string][]string{"fly": {"x"}}, "keys.yaml"); err == nil {
		t.Error("unknown action name was accepted")
	} else if !strings.Contains(err.Error(), "fly") {
		t.Errorf("error %q should name the offending action", err)
	}

	// A key claimed by a rebound action and a defaulted one is ambiguous.
	_, err := buildKeymap(map[string][]string{"yank": {"j"}}, "keys.yaml")
	if err == nil {
		t.Fatal("a key bound to two actions was accepted")
	}
	if !strings.Contains(err.Error(), "line-down") || !strings.Contains(err.Error(), "yank") {
		t.Errorf("error %q should name both actions", err)
	}
}

func TestCtrlCQuitsRegardlessOfKeymap(t *testing.T) {
	keys, err := buildKeymap(map[string][]string{"quit": {}}, "keys.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := keyModel(t, keys)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c must quit even with the quit action unbound")
	}
}

// TestHelpFollowsKeymap guards the reason the help is generated at all.
func TestHelpFollowsKeymap(t *testing.T) {
	keys, err := buildKeymap(map[string][]string{
		"line-down": {"ctrl+n"},
		"yank":      {},
	}, "keys.yaml")
	if err != nil {
		t.Fatal(err)
	}
	m := keyModel(t, keys)
	help := m.helpView()

	if !strings.Contains(help, "ctrl+n") {
		t.Error("help does not show the remapped key")
	}
	if strings.Contains(help, "yank code block") {
		t.Error("help still lists an unbound action")
	}
	for _, ln := range strings.Split(help, "\n") {
		if strings.HasPrefix(ln, "  j ") {
			t.Errorf("help still shows the replaced default: %q", ln)
		}
	}
}

func TestHelpFitsTerminalWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		m := keyModel(t, nil)
		// Tall enough that the whole overlay renders, footer included.
		m.width, m.height = w, 60
		m = press(m, runes("?"))
		if m.mode != modeHelp {
			t.Fatal("? did not open the help overlay")
		}
		for _, ln := range strings.Split(m.View(), "\n") {
			if got := runewidth.StringWidth(ln); got > w {
				t.Errorf("at width %d a line is %d cells: %q", w, got, ln)
			}
		}
	}
}

// TestSeededTemplateLoads pins the contract between the shipped keys.yaml
// template and this loader: a template that fails to parse would break
// every first run, and a fully commented one must mean "the defaults".
func TestSeededTemplateLoads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if _, err := config.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	path := filepath.Join(dir, ".MDView", "keys.yaml")
	keys, err := LoadKeys(path)
	if err != nil {
		t.Fatalf("the seeded template does not load: %v", err)
	}
	if len(keys) != len(defaultKeymap()) {
		t.Errorf("commented template yielded %d bindings, want %d (the defaults)",
			len(keys), len(defaultKeymap()))
	}
}

// TestSeededTemplateUncommentsCleanly checks the template's own examples
// parse: they are what users edit first.
func TestSeededTemplateUncommentsCleanly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if _, err := config.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	path := filepath.Join(dir, ".MDView", "keys.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var live strings.Builder
	for _, ln := range strings.Split(string(raw), "\n") {
		// Uncomment the binding lines, leave the prose comments out.
		if t := strings.TrimPrefix(ln, "#"); strings.Contains(t, ": [") {
			live.WriteString(t + "\n")
		}
	}
	if err := os.WriteFile(path, []byte(live.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	keys, err := LoadKeys(path)
	if err != nil {
		t.Fatalf("uncommenting the template's own examples fails: %v", err)
	}
	if len(keys) != len(defaultKeymap()) {
		t.Errorf("uncommented template yielded %d bindings, want %d",
			len(keys), len(defaultKeymap()))
	}
}
