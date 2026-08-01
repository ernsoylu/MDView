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

	// Mermaid renders ```mermaid fences through mermaid-cli. Off by
	// default: it runs a headless browser over content that came from the
	// document, which is a bigger thing to opt into than a colour scheme.
	Mermaid bool `yaml:"mermaid"`
}

const configFile = "config.yaml"

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

// KeysPath is the user keymap file inside the config directory.
func KeysPath() string {
	d := Dir()
	if d == "" {
		return ""
	}
	return filepath.Join(d, "keys.yaml")
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
	seed(filepath.Join(d, configFile), configTemplate)
	seed(filepath.Join(d, "theme.yaml"), themeTemplate)
	seed(filepath.Join(d, "keys.yaml"), keysTemplate)

	cfgPath := filepath.Join(d, configFile)
	data, err := os.ReadFile(cfgPath)
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
		return Config{}, fmt.Errorf("%s: %w", cfgPath, err)
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

# Render mermaid fences as diagrams. Needs mermaid-cli (mmdc) on PATH.
# Off by default: mmdc drives a headless browser over content that came
# from the document, so this is opt-in. Without it the fence stays a
# syntax-highlighted code block.
#mermaid: false
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

const keysTemplate = `# mdv keymap (~/.MDView/keys.yaml)
# Uncomment an action to replace its default keys entirely. An empty list
# leaves the action unbound. Unknown action names, and a key claimed by two
# actions, are errors. ctrl+c always quits regardless of what is set here.
#
# Key names follow bubbletea: single characters ("j", "G"), "ctrl+d",
# "alt+x", "up", "down", "left", "right", "home", "end", "pgup", "pgdown",
# "tab", "esc", and " " for space.

#line-down: [j, down]
#line-up: [k, up]
#half-page-down: [d, ctrl+d]
#half-page-up: [u, ctrl+u]
#page-down: [" ", pgdown, ctrl+f]
#page-up: [b, pgup, ctrl+b]
#top: [g, home]
#bottom: [G, end]

#search: [/]
#next-match: [n]
#prev-match: [N]
#toc: [t]
#clear-search: [esc]

#follow-link: [f]
#jump-back: [ctrl+o]
#jump-forward: [tab]
#edit: [e, i]
#yank: [y]
#visual: [v, V]

#line-numbers: ["#"]
#help: ["?"]
#quit: [q]
`
