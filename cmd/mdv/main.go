package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/ernsoylu/MDView/internal/config"
	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/state"
	"github.com/ernsoylu/MDView/internal/theme"
	"github.com/ernsoylu/MDView/internal/ui"
)

// version is stamped by goreleaser via -ldflags.
var version = "dev"

func main() {
	themeFlag := flag.String("theme", "", "theme: default, plain, or a path to a YAML theme file (default: ~/.MDView)")
	widthFlag := flag.Int("width", 0, "maximum content width in columns (0 = terminal width, capped at 120)")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: mdv [flags] <file.md>   (or pipe markdown on stdin)")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *versionFlag {
		fmt.Println("mdv", version)
		return
	}

	// ~/.MDView is created with commented templates on first run; a broken
	// config.yaml warns and falls back to defaults rather than blocking the
	// viewer.
	cfg, cfgErr := config.Ensure()
	if cfgErr != nil {
		fmt.Fprintln(os.Stderr, "mdv: config:", cfgErr, "(using defaults)")
		cfg = config.Config{}
	}

	args := flag.Args()
	stdinPiped := !term.IsTerminal(int(os.Stdin.Fd()))

	var src []byte
	var name, path string
	var err error
	switch {
	case len(args) == 1 && args[0] != "-":
		src, err = os.ReadFile(args[0])
		name = args[0]
		if abs, aerr := filepath.Abs(args[0]); aerr == nil {
			path = abs
		}
	case len(args) == 1 || (len(args) == 0 && stdinPiped):
		src, err = io.ReadAll(os.Stdin)
		name = "(stdin)"
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}

	th, err := pickTheme(*themeFlag, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	doc := parser.Parse(src)

	if *widthFlag == 0 && cfg.Width > 0 {
		*widthFlag = cfg.Width
	}

	// Not a terminal: dump the rendered document and exit. The mono color
	// profile strips styling and OSC 8 automatically in this case.
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		width := 80
		if c, cerr := strconv.Atoi(os.Getenv("COLUMNS")); cerr == nil && c > 0 {
			width = c
		}
		if *widthFlag > 0 {
			width = *widthFlag
		}
		for _, ln := range render.Render(doc, th, width) {
			fmt.Println(ln.String())
		}
		return
	}

	m := ui.New(doc, th, name, path).WithStore(state.Open()).WithEditor(cfg.Editor)
	if *widthFlag > 0 {
		m = m.WithMaxWidth(*widthFlag)
	}
	switch cfg.Images {
	case "", "auto":
	case "kitty":
		m = m.WithImages(render.ImagesKitty)
	case "halfblock":
		m = m.WithImages(render.ImagesHalfblock)
	case "off":
		m = m.WithImages(render.ImagesOff)
	default:
		fmt.Fprintf(os.Stderr, "mdv: config: unknown images mode %q (using auto)\n", cfg.Images)
	}
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	if stdinPiped {
		opts = append(opts, tea.WithInputTTY())
	}
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
}

// pickTheme resolves the theme with precedence: --theme flag, then
// config.yaml's theme (relative paths against ~/.MDView), then
// ~/.MDView/theme.yaml (a commented template loads as the default theme).
func pickTheme(flagValue string, cfg config.Config) (theme.Theme, error) {
	switch {
	case flagValue != "":
		return resolveTheme(flagValue, "")
	case cfg.Theme != "":
		return resolveTheme(cfg.Theme, config.Dir())
	}
	if tp := config.ThemePath(); tp != "" {
		if _, err := os.Stat(tp); err == nil {
			return theme.Load(tp)
		}
	}
	return theme.Default(), nil
}

func resolveTheme(name, baseDir string) (theme.Theme, error) {
	switch name {
	case "default":
		return theme.Default(), nil
	case "plain":
		return theme.Plain(), nil
	}
	if baseDir != "" && !filepath.IsAbs(name) {
		if _, err := os.Stat(name); err != nil {
			name = filepath.Join(baseDir, name)
		}
	}
	return theme.Load(name)
}
