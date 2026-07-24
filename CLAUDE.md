# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Name:** `mdv` (MDView)
**Language:** Go (Golang 1.25+, per go.mod; older local toolchains auto-download it)
**Module:** `github.com/ernsoylu/MDView`
**Purpose:** A high-performance, full-screen terminal Markdown viewer built on the [CommonMark 0.31.2 specification](https://spec.commonmark.org/0.31.2/) **plus GFM extensions** (see Part 2 §1).
**Core Philosophy:** Fast terminal rendering, rich visual theming, seamless navigation, and lean external editor integration without bloating the binary with custom editing engines.

---

## Part 1: Mandatory AI Behavioral Rules (Karpathy Guidelines)

*Derived from Andrej Karpathy's observations on LLM coding pitfalls. All code generation, refactoring, and PRs MUST adhere to these four principles:*

### 1. Think Before Coding
- **State Assumptions Explicitly:** If requirements or edge cases are ambiguous, state your working assumptions before writing code. If uncertain, stop and ask for clarification rather than guessing.
- **Present Tradeoffs:** Do not silently choose an architectural implementation when multiple viable interpretations exist (e.g., terminal image protocol fallback strategies). Present the options concisely.
- **Surface Inconsistencies & Push Back:** If a requested feature violates the parsing spec or introduces unnecessary complexity, push back and explain the simpler or specification-compliant approach.

### 2. Simplicity First
- **Write Minimum Viable Code:** Implement the simplest solution that solves the immediate problem. Nothing speculative.
- **No Speculative Abstractions:** Do not create interfaces, factory patterns, or wrapper layers for single-use code. Do not add "flexibility" or "configurability" that was not explicitly requested.
- **No Defensive Bloat:** Avoid error handling for impossible scenarios. Trust internal guarantees.
- **The Senior Engineer Test:** If 200 lines could be written in 50 lines cleanly, rewrite it. If a senior Go engineer would call the implementation overengineered, simplify it immediately.

### 3. Surgical Changes
- **Touch Only What You Must:** Every modified line in a diff must trace directly to the immediate user request.
- **No Drive-by Refactoring:** Do not "improve" adjacent code, reformat untouched comments, or refactor existing code that is not currently broken. Match the existing repository style, even if you prefer an alternative convention.
- **Clean Up Only Your Mess:** If your changes render an import, variable, or struct field orphaned, delete it. Do NOT remove pre-existing dead code unless explicitly instructed to clean it up.

### 4. Goal-Driven Execution
- **Transform Tasks into Verifiable Goals:** Never execute an imperative loop without defining testable criteria first.
  - *Instead of:* "Add fuzzy TOC jump" → *Do:* "Write unit tests for fuzzy header matching, then implement scoring until tests pass."
  - *Instead of:* "Fix link backtracking" → *Do:* "Write a test simulating a 3-link jump history stack, assert `Pop()` returns correct offsets, and fix the state handler."
- **Multi-Step Plan Verification:** For complex features, list a concise checklist:
  1. [Step] → verify: [command/test]
  2. [Step] → verify: [command/test]

---

## Part 2: Architecture & Feature Specifications

*Items marked **(locked)** are settled design decisions — do not re-litigate them; push back explicitly if one must change.*

### 1. Parsing & Rendering Pipeline
- **Spec (locked):** CommonMark 0.31.2 **plus GFM** — tables, strikethrough, task lists, autolinks — via `yuin/goldmark` with `extension.GFM`. (Strict CommonMark alone contradicted the table/checkbox features; GFM resolves that.) Footnotes are a backlog candidate.
- **Pipeline (locked):** goldmark AST → **styled-line IR** → viewport slice. The IR is `[]render.Line`; each `Line` holds `[]Span{Text, Style, Link}` plus its 1-based **source line number**. This mapping powers `vim +N`, resize re-anchoring, and future TOC/search/link-hint features. Never bypass the IR by parsing rendered ANSI.
- **Syntax highlighting:** fenced/indented code via `alecthomas/chroma/v2` at the **token level** — tokens map to styles through the theme; chroma's own ANSI formatter is not used.
- **Determinism:** the renderer must stay deterministic — golden-file tests render `testdata/*.md` at fixed widths with `theme.Plain()`.

### 2. Terminal UI (TUI) & Visual Theming
- **Stack:** `bubbletea` (alt screen, resize, mouse) + `lipgloss` (styling). Full-screen on `mdv <file.md>`.
- **Chrome:** one-row status bar (filename, scroll position, key hints) and a `?` help overlay.
- **Resize:** re-render at the new width, then re-anchor the viewport to the source line that was previously at the top (via the IR's source mapping).
- **Color handling:** truecolor→256→16→mono degradation and `NO_COLOR` come free from lipgloss/termenv profiles; the default theme uses `AdaptiveColor` for light/dark backgrounds.
- **External themes (implemented v1.0):** `--theme` takes `default`, `plain`, or a YAML file path; keys (`heading1`…`heading6`, `emph`, `codespan`, `link`, …, `chroma`) overlay the default theme wholesale per key, unknown keys are errors. See `examples/nord.yaml`.
- **Config directory (implemented v1.1):** first run seeds `~/.MDView/` with commented `config.yaml` (`theme`, `width`, `editor`, `images`) and `theme.yaml` templates. Precedence: flags > `config.yaml` > `~/.MDView/theme.yaml` > builtin default; empty/commented files mean defaults, a malformed `config.yaml` warns and continues. (Deliberate deviation from XDG config dir per user decision; positions still use XDG state.)
- **Width:** content width = terminal width − 2, capped at 120 columns; piped output uses `$COLUMNS` or 80.
- **UTF-8 Symbols:** list bullets by depth (`•`, `◦`, `▪`), quote borders (`│`), checkboxes (`[✓]`, `[ ]`), table borders (`┌ ┬ ┐ ├ ┼ ┤ └ ┴ ┘ ─ │`), thematic break (`─`).
- **Wide content:** long code lines chunk-wrap and over-wide tables shrink+truncate cells with `…`; horizontal scrolling is an open question for later.

### 3. Search Engine (v0.2)
- **(locked)** `/` starts **incremental literal search** in the body with `n`/`N` next/previous and high-contrast match highlighting. Regex is a possible later opt-in; **glob syntax in body search is dropped** (globs are a filename idiom).
- **(locked)** **Fuzzy matching is for the header/TOC jump only** (ctrl-p style popup), not body text.

### 4. Navigation & Link Tracing
- **TOC (v0.2):** popup panel generated from `H1`–`H6` AST nodes; selecting an entry jumps the viewport.
- **Link selection (locked):** **hint mode** — `f` overlays letter labels on visible links (vimium-style) — plus **mouse click**. No Tab-cycling.
- **Jumplist (locked):** one unified history — link follows, TOC jumps, and search jumps all push; `Ctrl+O` goes back, `Ctrl+I` forward, restoring exact document + scroll state. Relative links (`./other.md`) and `#anchors` resolve with GitHub-style heading slugs.
- **URLs:** OSC 8 hyperlinks (`\x1b]8;;url\x1b\\label\x1b]8;;\x1b\\`) emitted at render time; suppressed when the color profile is mono (piped/`NO_COLOR`). `Enter`/click on an `http(s)` link opens the browser via `xdg-open` (v0.3).
- **Images (implemented v0.4):** an image-only paragraph with a **local** path renders as a block — **half-block mosaic** (`▀` cells, any color terminal) or **Kitty graphics protocol** with Unicode placeholders (U+10EEEE + row/col diacritics, id in the foreground color) when kitty/ghostty is detected, so images scroll with the buffer. Transmissions are written to `/dev/tty` once per image (keyed by path+mtime+size in `img.Registry`), never through the diffed frame. Everything else — remote URLs, unreadable files, inline images mid-text, mono profile — stays the styled placeholder. A dim caption follows each rendered image. Height caps at 24 cells. Sixel/iTerm2 and remote fetching: backlog.

### 5. Vim Editor Integration & Live Preview
- **No Embedded Editor:** do NOT implement custom text editing engines.
- **External Invocation (v0.3):** on `e`/`i`, suspend the alt screen, run `$EDITOR` (default `vim`) as `vim +<line> <file>` — the line comes from the IR's source mapping of the top visible line — and resume + re-render on exit.
- **Watch mode (v0.3):** `fsnotify` re-renders on file change while preserving the anchored position, so mdv works as a live preview next to any editor.
- **Vim RPC:** only if live-preview-in-vim is ever required; prefer msgpack-rpc to headless Neovim over re-implementing modal editing.

### 6. LaTeX Math (implemented v0.5)
- `$...$` (inline) and `$$...$$` (display) parsed by `internal/mathext`, a goldmark extension with conservative rules: dollar amounts (`$5 and $10`) stay text, and documents without math are unaffected.
- Display math rasterizes via [go-latex/latex](https://codeberg.org/go-latex/latex) v0.3.0 (`mtex` + `drawimg`, 28pt @ 130dpi) through the v0.4 image pipeline capped at 8 rows, recolored by `img.AlphaFromLuminance` so glyphs blend into light or dark backgrounds.
- **Subset caveat:** go-latex v0.3.0 *panics* on TeX it cannot typeset — including all superscripts/subscripts (`ast.Sup`/`ast.Sub` unimplemented upstream). `renderMath` recovers and falls back to the raw TeX styled like a code block; inline math always stays raw. Revisit the pin when upstream implements sup/sub (fractions, roots, Greek, and function notation do render today).

### 7. Keymap (implemented)
| Keys | Action |
|---|---|
| `j` / `↓`, `k` / `↑` | scroll one line |
| `d` / `Ctrl+D`, `u` / `Ctrl+U` | half page down / up |
| `Space` / `PgDn` / `Ctrl+F`, `b` / `PgUp` / `Ctrl+B` | full page down / up |
| `g` / `Home`, `G` / `End` | top / bottom |
| mouse wheel | scroll 3 lines |
| `/` | incremental search (type; `Enter` keep, `Esc` cancel+restore) |
| `n` / `N` | next / previous match (wraps) |
| `t` | TOC popup (type to fuzzy-filter; `Enter` jump, `Esc` close) |
| `Esc` | clear search highlights |
| `f` | link hints: overlay labels, type one to follow (mouse click also follows) |
| `Ctrl+O` / `Tab` | jumplist back / forward (`Ctrl+I` arrives as Tab in terminals) |
| `e` / `i` | suspend and edit at the current line via `$EDITOR` (default `vim +N`) |
| `y` | yank the nearest code block to the clipboard (OSC 52) |
| `?` | toggle help overlay |
| `q` / `Ctrl+C` | quit (persists reading position) |

---

## Part 3: Go Development Commands & Conventions

### Core Commands
- **Run App:** `go run ./cmd/mdv <path/to/file.md>` (also accepts stdin: `cat notes.md | go run ./cmd/mdv`). Flags: `--theme <default|plain|file.yaml>`, `--width <n>`, `--version`.
- **Build Binary:** `go build -ldflags="-s -w" -o bin/mdv ./cmd/mdv`
- **Run All Tests:** `go test -race ./...`
- **Run Specific Test:** `go test -v -run TestFunctionName ./path/to/pkg`
- **Update Golden Files:** `go test ./internal/render -run TestGolden -update` (after intentional renderer changes; review the diff)
- **Fuzz:** `go test -fuzz=FuzzRender -fuzztime=30s ./internal/render`
- **Benchmarking:** `go test -bench=. -benchmem ./internal/render`
- **Linting & Formatting:**
  ```bash
  gofmt -s -w .
  go vet ./...
  golangci-lint run
  ```

### Package Layout
```
cmd/mdv          CLI entry: flags, config precedence, TTY detection, piped-dump mode
internal/config  ~/.MDView seeding and config.yaml loading
internal/parser  goldmark setup, Doc with byte-offset → source-line index
internal/render  AST → styled-line IR: inline flattening, wrapping, tables, chroma
internal/theme   semantic Theme struct; Default() (adaptive) and Plain(); chroma palette
internal/ui      bubbletea model: viewport, keys, mouse, status bar, help overlay
```
Future packages (`internal/nav`, `internal/img`, `internal/editor`) are created when their milestone starts — never before.

### Testing Conventions
- Golden tests always use `theme.Plain()` (no ANSI, no OSC 8) so files stay readable and deterministic.
- Renderer invariants: no `\n` inside a `Line`; rendered width ≤ requested width (fuzz asserts no panics).
- **Performance budget:** parse+render of a 200 KB document must stay **under 100 ms** (`BenchmarkRenderProse` tracks this; don't gate CI on it). Currently ~47 ms. The budget covers prose-dominated documents: chroma tokenization dominates on pathologically code-dense input (`BenchmarkRender`, 1200 snippets, ~227 ms) — lazy viewport highlighting is the backlog lever if that ever matters in practice.
- Spans carry `*lipgloss.Style` pointers into the theme, never style values — copying styles by value made GC dominate render time (3× slowdown). Keep it that way.

### CI
`.github/workflows/ci.yml` runs gofmt -s check, `go vet`, `go test -race`, and golangci-lint on every push/PR. Pushing a `v*` tag triggers `.github/workflows/release.yml`, which runs goreleaser (`.goreleaser.yaml`: linux/darwin/windows × amd64/arm64, version stamped via `-X main.version`, archives include `docs/mdv.1` and `examples/`). The RTK shell wrapper can swallow tool exit codes — gate release-critical checks with `rtk proxy <cmd>`.

---

## Part 4: Roadmap & Status

- [x] **v0.1 — read-only pager:** GFM parse; styled-line IR with source mapping; wrap/lists/quotes/tables/task lists; chroma syntax highlighting; adaptive default theme + Plain; alt-screen pager with keymap + wheel; status bar; help overlay; resize re-anchoring; stdin + piped dump modes; OSC 8; golden/unit/fuzz/bench tests; CI.
- [x] **v0.2 — search + TOC:** incremental `/` with `n`/`N` and match highlighting; fuzzy TOC popup jump.
- [x] **v0.3 — links & flow:** hint mode + mouse follow; unified jumplist (`Ctrl+O`/`Ctrl+I`); relative-doc + GitHub-slug anchor resolution; `xdg-open` for URLs; `e` editor integration via `vim +N`; watch mode.
- [x] **v0.4 — images:** half-block mosaic fallback; Kitty protocol with Unicode placeholders.
- [x] **v0.5 — LaTeX math:** `$`/`$$` goldmark extension; go-latex/latex rendering through the image pipeline; raw-TeX fallback.
- [x] **v1.0 — polish:** external YAML themes; per-file reading-position persistence (XDG state dir); `y` yank code block via OSC 52; `--width` flag; goreleaser + man page.
- [x] **v1.1 — installability:** `~/.MDView` config dir (auto-seeded `config.yaml` + `theme.yaml`); `install.sh` (`curl | sh`, OS/arch detection, checksum verify, PATH setup in shell rc); README; FreeBSD builds; shellcheck in CI.
- **Backlog:** section folding (`za`/`zR`/`zM`), Sixel/iTerm2 images, remote image fetching, footnotes, regex search, horizontal scroll for wide content, lazy viewport syntax highlighting.
