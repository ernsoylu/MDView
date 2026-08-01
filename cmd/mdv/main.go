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
		fmt.Fprintln(os.Stderr, "       mdv update [--check]   update to the newest release")
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

	// "mdv update" replaces the binary from the newest release. A real file
	// by that name still wins, so the subcommand cannot shadow a document.
	if len(args) >= 1 && args[0] == "update" && !fileExists(args[0]) {
		os.Exit(runUpdate(len(args) > 1 && (args[1] == "--check" || args[1] == "-check")))
	}

	in, err := resolveInput(args, stdinPiped)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}

	th, err := pickTheme(*themeFlag, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
	if *widthFlag == 0 && cfg.Width > 0 {
		*widthFlag = cfg.Width
	}
	if err := run(in, th, cfg, *widthFlag, stdinPiped); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}
}

// run either starts the directory browser or shows a single document.
func run(in input, th theme.Theme, cfg config.Config, width int, stdinPiped bool) error {
	if in.browseRoot != "" {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("browsing needs a terminal; name a file to render")
		}
		return browse(in.browseRoot, in.browseList, in.browseTrunc, th, cfg, width)
	}
	doc := parser.Parse(in.src)

	// Not a terminal: dump the rendered document and exit. The mono color
	// profile strips styling and OSC 8 automatically in this case.
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		dump(doc, th, width)
		return nil
	}

	// A document read from a pipe leaves stdin exhausted, so the pager has
	// to take its keys from the terminal directly. Where there is no
	// terminal to take them from — no controlling tty, a sandbox, a CI
	// runner — there is nothing to drive a pager with, but the document is
	// already parsed and printing it beats failing with nothing at all.
	// "curl … | mdv" should always put the document on screen.
	if _, err := runPager(doc, th, in.name, in.path, cfg, width, stdinPiped); err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err, "(rendering as plain text)")
		dump(doc, th, width)
	}
	return nil
}

// input is either a document to render or a directory/file list to browse.
type input struct {
	src         []byte
	name, path  string
	browseRoot  string
	browseList  []ui.FileEntry
	browseTrunc bool
}

// resolveInput decides whether args mean browse, fetch, read a file, or
// read stdin. A directory, several files, or no argument on a terminal all
// go through the browser; one argument names a document to render.
func resolveInput(args []string, stdinPiped bool) (input, error) {
	if len(args) == 0 && !stdinPiped {
		return input{browseRoot: "."}, nil
	}
	if len(args) == 1 && args[0] != "-" && ui.IsDir(args[0]) {
		return input{browseRoot: args[0]}, nil
	}
	if len(args) > 1 {
		return multiFileInput(args)
	}
	if len(args) == 1 && fetch.IsRemote(args[0]) {
		return remoteInput(args[0])
	}
	if len(args) == 1 && args[0] != "-" {
		return fileInput(args[0])
	}
	if len(args) == 1 || (len(args) == 0 && stdinPiped) {
		return stdinInput()
	}
	flag.Usage()
	os.Exit(2)
	return input{}, nil
}

func multiFileInput(args []string) (input, error) {
	for _, a := range args {
		if fetch.IsRemote(a) {
			return input{}, fmt.Errorf("several files at once are local only; fetch one remote document at a time")
		}
	}
	list, err := ui.EntriesFromPaths(args)
	if err != nil {
		return input{}, err
	}
	return input{browseRoot: ".", browseList: list}, nil
}

func remoteInput(url string) (input, error) {
	src, name, err := fetch.Get(url)
	if err != nil {
		return input{}, err
	}
	return input{src: src, name: name}, nil
}

func fileInput(path string) (input, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return input{}, err
	}
	in := input{src: src, name: path}
	if abs, aerr := filepath.Abs(path); aerr == nil {
		in.path = abs
	}
	return in, nil
}

func stdinInput() (input, error) {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		return input{}, err
	}
	return input{src: src, name: "(stdin)"}, nil
}

// fileExists reports whether path names something on disk, so a document
// can outrank a subcommand of the same name.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dump writes the rendered document to stdout as plain text.
func dump(doc *parser.Doc, th theme.Theme, widthFlag int) {
	width := 80
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		width = c
	}
	if widthFlag > 0 {
		width = widthFlag
	}
	for _, ln := range render.Render(doc, th, width) {
		fmt.Println(ln.String())
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
		path, hardQuit, err := pickFile(root, entries, truncated, th)
		if err != nil {
			return err
		}
		if hardQuit || path == "" {
			return nil
		}
		if hard, err := openPicked(root, path, th, cfg, width); err != nil || hard {
			return err
		}
	}
}

func pickFile(root string, entries []ui.FileEntry, truncated bool, th theme.Theme) (path string, hardQuit bool, err error) {
	b := ui.NewBrowser(root, entries, truncated, th)
	final, err := tea.NewProgram(b, tea.WithAltScreen()).Run()
	if err != nil {
		return "", false, err
	}
	picked, ok := final.(ui.Browser)
	if !ok || picked.HardQuit() {
		return "", true, nil
	}
	path, chose := picked.Chosen()
	if !chose {
		return "", false, nil
	}
	return path, false, nil
}

func openPicked(root, path string, th theme.Theme, cfg config.Config, width int) (hardQuit bool, err error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	name := path
	if rel, relErr := filepath.Rel(root, path); relErr == nil {
		name = rel
	}
	return runPager(parser.Parse(src), th, name, path, cfg, width, false)
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
