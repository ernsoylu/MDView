package theme

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func writeTheme(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadOverlaysDefault(t *testing.T) {
	th, err := Load(writeTheme(t, `
heading1: { fg: "#ff0000", bold: true }
emph: { underline: true }
chroma: dracula
`))
	if err != nil {
		t.Fatal(err)
	}
	if !th.Heading[0].GetBold() {
		t.Error("heading1 lost bold")
	}
	if fg := th.Heading[0].GetForeground(); fg != lipgloss.Color("#ff0000") {
		t.Errorf("heading1 fg = %v", fg)
	}
	if !th.Emph.GetUnderline() || th.Emph.GetItalic() {
		t.Error("emph should be replaced wholesale: underline on, italic off")
	}
	// Untouched keys keep their defaults.
	if !th.Strong.GetBold() {
		t.Error("strong lost its default bold")
	}
	if th.Chroma == nil || th.Chroma.Name != "dracula" {
		t.Errorf("chroma = %v", th.Chroma)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	if _, err := Load(writeTheme(t, "headnig1: { bold: true }\n")); err == nil {
		t.Error("typo'd key should be an error")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Error("missing file should error")
	}
}
