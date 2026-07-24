# mdv — a full-screen terminal Markdown viewer

[![CI](https://github.com/ernsoylu/MDView/actions/workflows/ci.yml/badge.svg)](https://github.com/ernsoylu/MDView/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ernsoylu/MDView)](https://github.com/ernsoylu/MDView/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`mdv` renders Markdown the way a terminal deserves: full-screen, fast, and
navigable like a browser.

```
mdv README.md
```

## Features

- **CommonMark + GFM** via goldmark: tables with box-drawing borders and
  alignment, task lists, strikethrough, autolinks — plus resolved entities
  and escapes.
- **Syntax highlighting** in fenced code blocks (chroma), with long lines
  chunk-wrapped, never lost.
- **Incremental search** (`/`) with smart-case, match highlighting, and
  `n`/`N` navigation.
- **Fuzzy table of contents** (`t`): type a few letters, jump to any heading.
- **Follow links like a browser**: press `f` for vimium-style hint labels or
  just click. Anchors and relative `.md` files open in mdv (GitHub-compatible
  heading slugs); web links open in your browser. `Ctrl+O`/`Tab` walk the
  jumplist back and forward — across documents.
- **Images in the terminal**: half-block mosaic anywhere colors work, real
  pixel graphics in kitty/ghostty (Unicode placeholders, so images scroll
  with the text).
- **LaTeX display math** rendered through the same image pipeline
  (go-latex), with raw-TeX fallback.
- **Live reload**: edits from any editor re-render in place, position kept —
  mdv doubles as a Markdown preview.
- **Edit in place**: `e` suspends the viewer and opens `$EDITOR` at the line
  you're reading (`vim +N` style).
- **Yank code** (`y`): the nearest code block goes to your clipboard via
  OSC 52.
- **Remembers your place** per file, restores it on reopen.
- **Themes**: adaptive light/dark default, or your own YAML theme.
- Wheel + click **mouse support**, OSC 8 **clickable links**, `NO_COLOR`
  respected, piped output degrades to clean plain text.

## Install

### Script (Linux, macOS, FreeBSD)

```sh
curl -fsSL https://raw.githubusercontent.com/ernsoylu/MDView/main/install.sh | sh
```

The script detects your OS and architecture, downloads the latest release,
verifies its checksum, installs `mdv` to `~/.local/bin` (override with
`MDV_INSTALL_DIR`), installs the man page, and adds the directory to your
PATH in `~/.zshrc` / `~/.bashrc` if needed — so `mdv example.md` just works
in a fresh shell.

### Go

```sh
go install github.com/ernsoylu/MDView/cmd/mdv@latest
```

### Manual

Grab an archive for your platform from the
[releases page](https://github.com/ernsoylu/MDView/releases/latest)
(Linux, macOS, Windows, FreeBSD — amd64 and arm64), unpack, and put `mdv`
on your PATH.

### From source

```sh
git clone https://github.com/ernsoylu/MDView && cd MDView
go build -ldflags="-s -w" -o bin/mdv ./cmd/mdv
```

## Usage

```
mdv [flags] file.md
command | mdv            # read from stdin
mdv file.md | less -R    # non-TTY output is plain text
```

| Flag | Meaning |
|---|---|
| `--theme <name>` | `default`, `plain`, or a path to a YAML theme file |
| `--width <n>` | maximum content width in columns (default: terminal width, capped at 120) |
| `--version` | print version and exit |

### Keys

| Keys | Action |
|---|---|
| `j`/`k`, `d`/`u`, `Space`/`b`, `g`/`G` | scroll: line, half page, page, top/bottom |
| `/` then `n`/`N` | incremental search; next/previous match |
| `t` | table of contents (type to fuzzy-filter, `Enter` jumps) |
| `f` | link hints — type the label to follow (clicking works too) |
| `Ctrl+O` / `Tab` | jumplist back / forward |
| `e` / `i` | edit at the current line with `$EDITOR` |
| `y` | yank the nearest code block to the clipboard |
| `?` | help overlay |
| `q` | quit |

Try the guided tour: `mdv examples/demo.md`.

## Configuration

On first run mdv creates **`~/.MDView/`** with two commented template files:

- **`~/.MDView/config.yaml`** — viewer settings; flags override them:

  ```yaml
  theme: ""        # default, plain, or a theme file (relative to ~/.MDView)
  width: 0         # max content width; 0 = terminal width, capped at 120
  editor: ""       # editor for e/i; empty uses $EDITOR, then vim
  images: auto     # auto, kitty, halfblock, off
  ```

- **`~/.MDView/theme.yaml`** — your theme. Uncomment any key to override
  the built-in adaptive theme:

  ```yaml
  heading1: { fg: "#88c0d0", bold: true, underline: true }
  codespan: { fg: "#d08770" }
  link: { fg: "#88c0d0", underline: true }
  chroma: nord     # code-highlighting palette, any chroma style name
  ```

  A complete example ships in [`examples/nord.yaml`](examples/nord.yaml):
  `mdv --theme examples/nord.yaml examples/demo.md`.

Reading positions persist separately under
`$XDG_STATE_HOME/mdv/positions.json` (default `~/.local/state/mdv/`).

## Terminal support

| Capability | Where |
|---|---|
| Colors | truecolor → 256 → 16 → mono degradation, `NO_COLOR` honored |
| Clickable links | any OSC 8 terminal (kitty, ghostty, iTerm2, WezTerm, foot, recent GNOME/Windows terminals) |
| Pixel images & math | kitty, ghostty (kitty graphics protocol) |
| Mosaic images & math | any color terminal |
| Clipboard yank | any OSC 52 terminal |

## Development

```sh
go test -race ./...                              # full suite
go test ./internal/render -run TestGolden -update  # refresh golden files
go test -fuzz=FuzzRender -fuzztime=30s ./internal/render
golangci-lint run
```

The architecture, design decisions, performance budgets, and roadmap live
in [CLAUDE.md](CLAUDE.md). Releases are cut by pushing a `v*` tag —
goreleaser builds the archives and publishes them.

## License

[MIT](LICENSE)
