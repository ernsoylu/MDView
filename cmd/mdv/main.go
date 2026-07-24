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

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/state"
	"github.com/ernsoylu/MDView/internal/theme"
	"github.com/ernsoylu/MDView/internal/ui"
)

// version is stamped by goreleaser via -ldflags.
var version = "dev"

func main() {
	themeFlag := flag.String("theme", "default", "theme: default, plain, or a path to a YAML theme file")
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

	th, err := resolveTheme(*themeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	doc := parser.Parse(src)

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

	m := ui.New(doc, th, name, path).WithStore(state.Open())
	if *widthFlag > 0 {
		m = m.WithMaxWidth(*widthFlag)
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

func resolveTheme(name string) (theme.Theme, error) {
	switch name {
	case "default":
		return theme.Default(), nil
	case "plain":
		return theme.Plain(), nil
	default:
		return theme.Load(name)
	}
}
