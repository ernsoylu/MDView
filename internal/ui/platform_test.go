package ui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/theme"
)

func TestOpenerArgv(t *testing.T) {
	cases := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"linux", "xdg-open", []string{"https://x"}},
		{"freebsd", "xdg-open", []string{"https://x"}},
		{"darwin", "open", []string{"https://x"}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", "https://x"}},
	}
	for _, tc := range cases {
		name, args := openerArgv(tc.goos, "https://x")
		if name != tc.wantName || !slices.Equal(args, tc.wantArgs) {
			t.Errorf("openerArgv(%q) = %q %q, want %q %q",
				tc.goos, name, args, tc.wantName, tc.wantArgs)
		}
	}
}

func TestEditorArgs(t *testing.T) {
	cases := []struct {
		bin   string
		extra []string
		want  []string
	}{
		// The "+N file" convention, which most editors share.
		{"vim", nil, []string{"+12", "/tmp/a.md"}},
		{"nvim", nil, []string{"+12", "/tmp/a.md"}},
		{"nano", nil, []string{"+12", "/tmp/a.md"}},
		{"emacs", nil, []string{"+12", "/tmp/a.md"}},
		{"/usr/bin/vim", nil, []string{"+12", "/tmp/a.md"}},
		{"vim", []string{"-p"}, []string{"-p", "+12", "/tmp/a.md"}},
		// VS Code ignores a bare line number without --goto.
		{"code", nil, []string{"--goto", "/tmp/a.md:12"}},
		{"code-insiders", nil, []string{"--goto", "/tmp/a.md:12"}},
		{"codium", nil, []string{"--goto", "/tmp/a.md:12"}},
		{"/usr/share/code/bin/code", nil, []string{"--goto", "/tmp/a.md:12"}},
		// Windows ships the VS Code CLI as a .cmd shim, not a .exe.
		{"code.exe", nil, []string{"--goto", "/tmp/a.md:12"}},
		{"code.cmd", nil, []string{"--goto", "/tmp/a.md:12"}},
		{"CODE.EXE", nil, []string{"--goto", "/tmp/a.md:12"}},
		// These carry the line in the filename.
		{"subl", nil, []string{"/tmp/a.md:12"}},
		{"hx", nil, []string{"/tmp/a.md:12"}},
		{"helix", nil, []string{"/tmp/a.md:12"}},
		// Anything unrecognized falls back to the common convention.
		{"someeditor", nil, []string{"+12", "/tmp/a.md"}},
	}
	for _, tc := range cases {
		if got := editorArgs(tc.bin, tc.extra, "/tmp/a.md", 12); !slices.Equal(got, tc.want) {
			t.Errorf("editorArgs(%q, %q) = %q, want %q", tc.bin, tc.extra, got, tc.want)
		}
	}
}

// TestEditorArgsDoesNotAliasExtra guards the caller's slice: extra is
// parts[1:] of the split $EDITOR, and appending must not write through it.
func TestEditorArgsDoesNotAliasExtra(t *testing.T) {
	extra := make([]string, 1, 8) // spare capacity: append would write in place
	extra[0] = "-p"
	editorArgs("vim", extra, "/tmp/a.md", 3)
	if len(extra) != 1 || extra[0] != "-p" {
		t.Errorf("caller slice was mutated: %q", extra)
	}
}

func modelWithCode(t *testing.T) Model {
	t.Helper()
	m := New(parser.Parse([]byte("```go\nalpha()\n```\n")), theme.Plain(), "y.md", "")
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	return mm.(Model)
}

// TestYankReportsWriteFailure is the case that used to lie: an unreachable
// terminal produced a cheerful "yanked N code line(s)".
func TestYankReportsWriteFailure(t *testing.T) {
	m := modelWithCode(t)
	m.tty = func([]string) error { return errors.New("no /dev/tty") }

	_, cmd := m.Update(runes("y"))
	if cmd == nil {
		t.Fatal("y produced no command")
	}
	msg, ok := cmd().(flashMsg)
	if !ok {
		t.Fatalf("yank returned %#v, want a flash message", msg)
	}
	if !strings.Contains(string(msg), "no /dev/tty") {
		t.Errorf("yank message = %q, want the write error surfaced", msg)
	}
	if strings.Contains(string(msg), "yanked") {
		t.Errorf("yank claimed success after a failed write: %q", msg)
	}
}

func TestWriteTTYReportsFailure(t *testing.T) {
	m := modelWithCode(t)
	m.tty = func([]string) error { return errors.New("broken pipe") }

	msg := m.writeTTY("images", []string{"x"})()
	flash, ok := msg.(flashMsg)
	if !ok || !strings.Contains(string(flash), "images: broken pipe") {
		t.Errorf("writeTTY message = %#v, want the failure labelled and surfaced", msg)
	}

	m.tty = func([]string) error { return nil }
	if msg := m.writeTTY("images", []string{"x"})(); msg != nil {
		t.Errorf("a successful write produced %#v, want no message", msg)
	}
}
