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
	"github.com/ernsoylu/MDView/internal/fetch"
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
		fmt.Fprintln(os.Stderr, "usage: mdv [flags] [file.md | directory | files... | URL]")
		fmt.Fprintln(os.Stderr, "       markdown on stdin is read when piped")
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

	// A directory, several files, or no argument at all on a terminal all
	// go through the browser; one argument names a document to render.
	var browseRoot string
	var browseList []ui.FileEntry
	var browseTrunc bool

	var src []byte
	var name, path string
	var err error
	switch {
	case len(args) == 0 && !stdinPiped:
		browseRoot = "."
	case len(args) == 1 && args[0] != "-" && ui.IsDir(args[0]):
		browseRoot = args[0]
	case len(args) > 1:
		for _, a := range args {
			if fetch.IsRemote(a) {
				fmt.Fprintln(os.Stderr, "mdv: several files at once are local only; fetch one remote document at a time")
				os.Exit(2)
			}
		}
		browseRoot = "."
		browseList, err = ui.EntriesFromPaths(args)
	case len(args) == 1 && fetch.IsRemote(args[0]):
		src, name, err = fetch.Get(args[0])
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
	browsing := browseRoot != ""

	th, err := pickTheme(*themeFlag, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	if *widthFlag == 0 && cfg.Width > 0 {
		*widthFlag = cfg.Width
	}
	if browsing {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Fprintln(os.Stderr, "mdv: browsing needs a terminal; name a file to render")
			os.Exit(2)
		}
		if err := browse(browseRoot, browseList, browseTrunc, th, cfg, *widthFlag); err != nil {
			fmt.Fprintln(os.Stderr, "mdv:", err)
			os.Exit(1)
		}
		return
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

	if _, err := runPager(doc, th, name, path, cfg, *widthFlag, stdinPiped); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
}

// runPager shows one document full-screen, reporting whether the user left
// with ctrl+c rather than the quit key.
func runPager(doc *parser.Doc, th theme.Theme, name, path string, cfg config.Config, width int, stdinPiped bool) (hardQuit bool, err error) {
	keys, err := ui.LoadKeys(config.KeysPath())
	if err != nil {
		return false, err
	}
	m := ui.New(doc, th, name, path).WithStore(state.Open()).WithEditor(cfg.Editor).WithKeys(keys)
	if width > 0 {
		m = m.WithMaxWidth(width)
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
	if cfg.Mermaid {
		if mmdc := render.LookupMermaid(); mmdc != "" {
			m = m.WithMermaid(mmdc)
		} else {
			fmt.Fprintln(os.Stderr, "mdv: config: mermaid is on but mmdc is not on PATH; leaving fences as code")
		}
	}
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	if stdinPiped {
		opts = append(opts, tea.WithInputTTY())
	}
	final, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		return false, err
	}
	fm, ok := final.(ui.Model)
	return ok && fm.HardQuit(), nil
}

// browse alternates the file picker with the pager: quitting a document
// comes back to the list, and only the list — or ctrl+c — leaves mdv.
func browse(root string, entries []ui.FileEntry, truncated bool, th theme.Theme, cfg config.Config, width int) error {
	if entries == nil { // no explicit list: scan the tree
		var err error
		if entries, truncated, err = ui.FindMarkdown(root); err != nil {
			return err
		}
	}
	for {
		b := ui.NewBrowser(root, entries, truncated, th)
		final, err := tea.NewProgram(b, tea.WithAltScreen()).Run()
		if err != nil {
			return err
		}
		picked, ok := final.(ui.Browser)
		if !ok || picked.HardQuit() {
			return nil
		}
		path, chose := picked.Chosen()
		if !chose {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		name := path
		if rel, relErr := filepath.Rel(root, path); relErr == nil {
			name = rel
		}
		hard, perr := runPager(parser.Parse(src), th, name, path, cfg, width, false)
		if perr != nil {
			return perr
		}
		if hard {
			return nil
		}
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
