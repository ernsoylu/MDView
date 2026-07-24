// Package config manages ~/.MDView: a config.yaml with viewer settings
// and a theme.yaml the user can edit. Both are seeded with commented
// templates on first run.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config are the settings from ~/.MDView/config.yaml. Command-line flags
// take precedence over all of them.
type Config struct {
	Theme  string `yaml:"theme"`  // default, plain, or a theme file (relative to ~/.MDView)
	Width  int    `yaml:"width"`  // max content width; 0 = auto
	Editor string `yaml:"editor"` // overrides $EDITOR for the e/i keys
	Images string `yaml:"images"` // auto, kitty, halfblock, off
}

// Dir returns the configuration directory, "" when home is undetectable.
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".MDView")
}

// ThemePath is the user theme file inside the config directory.
func ThemePath() string {
	d := Dir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "theme.yaml")
}

// Ensure creates ~/.MDView and seeds config.yaml and theme.yaml with
// commented templates when missing, then loads config.yaml. Missing or
// empty files mean defaults; only a malformed existing config is an error
// (the caller warns and continues with defaults).
func Ensure() (Config, error) {
	d := Dir()
	if d == "" {
		return Config{}, nil
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return Config{}, nil
	}
	seed(filepath.Join(d, "config.yaml"), configTemplate)
	seed(filepath.Join(d, "theme.yaml"), themeTemplate)

	data, err := os.ReadFile(filepath.Join(d, "config.yaml"))
	if err != nil {
		return Config{}, nil
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		if errors.Is(err, io.EOF) { // fully commented / empty
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("%s: %w", filepath.Join(d, "config.yaml"), err)
	}
	return c, nil
}

func seed(path, content string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.WriteFile(path, []byte(content), 0o644)
}

const configTemplate = `# mdv configuration (~/.MDView/config.yaml)
# Command-line flags override these settings.

# Theme: "default", "plain", or a YAML theme file. Relative paths resolve
# against this directory. Leave unset to use ~/.MDView/theme.yaml.
#theme: ""

# Maximum content width in columns. 0 means terminal width, capped at 120.
#width: 0

# Editor for the e/i keys. Empty uses $EDITOR, falling back to vim.
#editor: ""

# Image rendering: auto (kitty where supported, else half-block mosaic),
# kitty, halfblock, or off.
#images: auto
`

const themeTemplate = `# mdv theme (~/.MDView/theme.yaml)
# Uncomment keys to override the built-in adaptive theme. Colors are hex
# ("#88c0d0") or ANSI numbers ("12"). A present key replaces that
# element's style entirely. See the repo's examples/nord.yaml for a
# complete theme.
#
#heading1: { fg: "#cba6f7", bold: true, underline: true }
#heading2: { fg: "#89b4fa", bold: true }
#heading3: { fg: "#94e2d5", bold: true }
#heading4: { fg: "#f9e2af", bold: true }
#heading5: { fg: "#a6adc8", bold: true }
#heading6: { fg: "#a6adc8", bold: true, italic: true }
#emph: { italic: true }
#strong: { bold: true }
#strike: { strikethrough: true }
#codespan: { fg: "#f38ba8" }
#codeblock: {}
#link: { fg: "#89b4fa", underline: true }
#image: { fg: "#b4befe", italic: true }
#quotebar: { fg: "#a6e3a1" }
#listmarker: { fg: "#cba6f7" }
#tableborder: { fg: "#6c7086" }
#tableheader: { bold: true }
#rule: { fg: "#6c7086" }
#dim: { faint: true }
#statusbar: { reverse: true }
#searchhit: { fg: "#1e1e2e", bg: "#f9e2af" }
#searchcurrent: { fg: "#1e1e2e", bg: "#fab387" }
#hintlabel: { fg: "#11111b", bg: "#f38ba8", bold: true }
#chroma: catppuccin-mocha
`
