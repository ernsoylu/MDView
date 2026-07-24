package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/ernsoylu/MDView/internal/parser"
	"github.com/ernsoylu/MDView/internal/render"
	"github.com/ernsoylu/MDView/internal/theme"
	"github.com/ernsoylu/MDView/internal/ui"
)

func main() {
	args := os.Args[1:]
	stdinPiped := !term.IsTerminal(int(os.Stdin.Fd()))

	var src []byte
	var name string
	var err error
	switch {
	case len(args) == 1 && args[0] != "-":
		src, err = os.ReadFile(args[0])
		name = args[0]
	case len(args) == 1 || (len(args) == 0 && stdinPiped):
		src, err = io.ReadAll(os.Stdin)
		name = "(stdin)"
	default:
		fmt.Fprintln(os.Stderr, "usage: mdv <file.md>   (or pipe markdown on stdin)")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}

	doc := parser.Parse(src)
	th := theme.Default()

	// Not a terminal: dump the rendered document and exit. The mono color
	// profile strips styling and OSC 8 automatically in this case.
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		width := 80
		if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
			width = c
		}
		for _, ln := range render.Render(doc, th, width) {
			fmt.Println(ln.String())
		}
		return
	}

	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	if stdinPiped {
		opts = append(opts, tea.WithInputTTY())
	}
	if _, err := tea.NewProgram(ui.New(doc, th, name), opts...).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
}
