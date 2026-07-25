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
- **Browse a tree**: `mdv` with no argument lists the markdown files under
  the current directory, filtered as you type; quitting a document returns
  to the list. Several files on the command line share that list.
- **Read remote docs**: `mdv https://…/README.md`, or
  `mdv github.com/owner/repo` for a repository's README.
- **Follow links like a browser**: press `f` for vimium-style hint labels or
  just click. Anchors and relative `.md` files open in mdv (GitHub-compatible
  heading slugs); web links open in your browser. `Ctrl+O`/`Tab` walk the
  jumplist back and forward — across documents.
- **Images in the terminal**: half-block mosaic anywhere colors work, real
  pixel graphics in kitty/ghostty (Unicode placeholders, so images scroll
  with the text).
- **LaTeX display math** rendered through the same image pipeline
  (go-latex), with raw-TeX fallback.
- **Mermaid diagrams** (opt-in) through the same pipeline, via mermaid-cli.
- **Live reload**: edits from any editor re-render in place, position kept —
  mdv doubles as a Markdown preview.
- **Edit in place**: `e` suspends the viewer and opens `$EDITOR` at the line
  you're reading — `vim +N` by default, adapted for editors that want the
  line elsewhere (`code --goto`, `subl`/`hx` `file:line`).
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
in a fresh shell. When it finishes it opens that release's notes in the mdv
it just installed.

### Go

```sh
go install github.com/ernsoylu/MDView/cmd/mdv@latest
```

Needs Go 1.25 or newer. On an older Go this still works with the default
`GOTOOLCHAIN=auto`, which fetches 1.25 for the build; if your distribution
ships Go with `GOTOOLCHAIN=local`, the build stops with *requires go >=
1.25.0* and the install script above is the easier route — it needs no Go at
all. The floor comes from dependencies (`golang.org/x/term`, `chroma`,
`go-latex`), not from mdv's own sources.

### Updating

```sh
mdv update
```

Checks the repository's latest release, verifies the download against the
release `checksums.txt`, and replaces the running binary in place — the same
assets and the same checksums the install script uses, so a binary installed
either way updates either way. `mdv update --check` only reports. If mdv
lives somewhere you cannot write, it says so and points at `sudo` or a
reinstall rather than half-applying.

A build from source reports its version as `dev` and is always considered
older than a release, so `mdv update` will move it onto one.

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
mdv                      # browse markdown under the current directory
mdv docs/                # browse a directory
mdv a.md b.md c.md       # several files: the list becomes your buffer list
mdv https://…/README.md  # fetch a URL
mdv github.com/owner/repo    # fetch a repository's README
curl -fsSL https://…/README.md | mdv   # read from stdin
command | mdv            # read from stdin
mdv file.md | less -R    # non-TTY output is plain text
mdv update               # update to the newest release
mdv update --check       # just report whether one is available
```

Reading from a pipe leaves stdin used up, so the pager takes its keys from
the terminal directly. Where there is no terminal to take them from — a
sandbox, a CI runner, no controlling tty — mdv prints the rendered document
as plain text rather than failing, so a piped invocation always produces
something.

With a directory — or no argument at all — mdv lists the markdown files
underneath, filtered as you type. `Enter` opens, `Esc` leaves, and quitting a
document comes back to the list; `Ctrl+C` leaves outright. Dot directories
and dependency trees (`node_modules`, `vendor`, `target`) are skipped.

Naming several files uses the same list, so it doubles as a buffer list:
quitting one document drops you back among the others.

Remote documents are fetched over HTTP with a 20s timeout and an 8 MiB cap.
`github.com/owner/repo` resolves to the repository's README, and a
`github.com/owner/repo/blob/…` link resolves to the file it points at.
Fetching is one document at a time — remote and local arguments cannot be
mixed — and relative links inside a fetched document do not resolve, the
same as for stdin.

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
| `v` / `V` | visual-line select (scroll keys extend); `y` copies the markdown source |
| `#` | toggle source line numbers |
| `?` | help overlay |
| `q` | quit |

Every key in this table is remappable — see `~/.MDView/keys.yaml` below.
`Ctrl+C` always quits.

Try the guided tour: `mdv examples/demo.md`.

## Configuration

On first run mdv creates **`~/.MDView/`** with three commented template files:

- **`~/.MDView/config.yaml`** — viewer settings; flags override them:

  ```yaml
  theme: ""        # default, plain, or a theme file (relative to ~/.MDView)
  width: 0         # max content width; 0 = terminal width, capped at 120
  editor: ""       # editor for e/i; empty uses $EDITOR, then vim
  images: auto     # auto, kitty, halfblock, off
  mermaid: false   # render mermaid fences (needs mmdc on PATH)
  ```

  `mermaid` is off by default on purpose: mermaid-cli drives a headless
  browser over content that came from the document, which is a larger thing
  to opt into than a colour scheme. With it off — or with `mmdc` not
  installed, or a diagram that does not compile — the fence stays a
  syntax-highlighted code block.

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

- **`~/.MDView/keys.yaml`** — your keymap. Listing keys for an action
  replaces its defaults entirely; an empty list leaves it unbound:

  ```yaml
  line-down: [ctrl+n, down]     # emacs-style movement
  line-up: [ctrl+p, up]
  yank: []                      # unbind
  ```

  Unknown action names, and a key claimed by two actions, are reported
  rather than silently ignored. The `?` overlay is generated from the live
  keymap, so it always shows the keys you actually have.

Reading positions persist separately under
`$XDG_STATE_HOME/mdv/positions.json` (default `~/.local/state/mdv/`).

## Terminal support

| Capability | Where |
|---|---|
| Colors | truecolor → 256 → 16 → mono degradation, `NO_COLOR` honored |
| Clickable links | any OSC 8 terminal (kitty, ghostty, iTerm2, WezTerm, foot, recent GNOME/Windows terminals) |
| Pixel images & math | kitty, ghostty, WezTerm (kitty graphics protocol) |
| Mosaic images & math | any color terminal, and inside tmux/screen |
| Clipboard yank | any OSC 52 terminal |

Inside tmux or screen mdv uses the mosaic: neither forwards kitty graphics
without being configured for it, and their panes inherit the outer
terminal's environment, so the environment alone cannot be trusted. Set
`images: kitty` in `config.yaml` to override the detection either way.

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
