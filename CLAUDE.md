# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Name:** `mdv` (MDView)
**Language:** Go (Golang 1.22+)
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
- **Color handling:** truecolor→256→16→mono degradation and `NO_COLOR` come free from lipgloss/termenv profiles; the default theme uses `AdaptiveColor` for light/dark backgrounds. External JSON/YAML theme files: v1.0.
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
- **Images (locked fallback order):** styled placeholder `🖼 [Image: Alt] (url)` (v0.1) → **half-block mosaic** rendering, which works in any color terminal (v0.4) → **Kitty graphics protocol** using Unicode placeholders so images survive scrolling (v0.4). Sixel/iTerm2: backlog, optional.

### 5. Vim Editor Integration & Live Preview
- **No Embedded Editor:** do NOT implement custom text editing engines.
- **External Invocation (v0.3):** on `e`/`i`, suspend the alt screen, run `$EDITOR` (default `vim`) as `vim +<line> <file>` — the line comes from the IR's source mapping of the top visible line — and resume + re-render on exit.
- **Watch mode (v0.3):** `fsnotify` re-renders on file change while preserving the anchored position, so mdv works as a live preview next to any editor.
- **Vim RPC:** only if live-preview-in-vim is ever required; prefer msgpack-rpc to headless Neovim over re-implementing modal editing.

### 6. Keymap (implemented in v0.1)
| Keys | Action |
|---|---|
| `j` / `↓`, `k` / `↑` | scroll one line |
| `d` / `Ctrl+D`, `u` / `Ctrl+U` | half page down / up |
| `Space` / `PgDn` / `Ctrl+F`, `b` / `PgUp` / `Ctrl+B` | full page down / up |
| `g` / `Home`, `G` / `End` | top / bottom |
| mouse wheel | scroll 3 lines |
| `?` | toggle help overlay |
| `q` / `Ctrl+C` | quit |

Reserved for later: `/` `n` `N` (search), `t` (TOC), `f` (link hints), `Ctrl+O`/`Ctrl+I` (jumplist), `e`/`i` (editor), `y` (yank code block).

---

## Part 3: Go Development Commands & Conventions

### Core Commands
- **Run App:** `go run ./cmd/mdv <path/to/file.md>` (also accepts stdin: `cat notes.md | go run ./cmd/mdv`)
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
cmd/mdv          CLI entry: input handling, TTY detection, piped-dump mode
internal/parser  goldmark setup, Doc with byte-offset → source-line index
internal/render  AST → styled-line IR: inline flattening, wrapping, tables, chroma
internal/theme   semantic Theme struct; Default() (adaptive) and Plain(); chroma palette
internal/ui      bubbletea model: viewport, keys, mouse, status bar, help overlay
```
Future packages (`internal/nav`, `internal/img`, `internal/editor`) are created when their milestone starts — never before.

### Testing Conventions
- Golden tests always use `theme.Plain()` (no ANSI, no OSC 8) so files stay readable and deterministic.
- Renderer invariants: no `\n` inside a `Line`; rendered width ≤ requested width (fuzz asserts no panics).
- **Performance budget:** parse+render of a 200 KB document must stay **under 100 ms** (`BenchmarkRender` tracks this; don't gate CI on it).

### CI
`.github/workflows/ci.yml` runs gofmt -s check, `go vet`, `go test -race`, and golangci-lint on every push/PR. Release tooling (goreleaser, brew tap, man page) arrives with v1.0.

---

## Part 4: Roadmap & Status

- [ ] **v0.1 — read-only pager:** GFM parse; styled-line IR with source mapping; wrap/lists/quotes/tables/task lists; chroma syntax highlighting; adaptive default theme + Plain; alt-screen pager with keymap + wheel; status bar; help overlay; resize re-anchoring; stdin + piped dump modes; OSC 8; golden/unit/fuzz/bench tests; CI.
- [ ] **v0.2 — search + TOC:** incremental `/` with `n`/`N` and match highlighting; fuzzy TOC popup jump.
- [ ] **v0.3 — links & flow:** hint mode + mouse follow; unified jumplist (`Ctrl+O`/`Ctrl+I`); relative-doc + GitHub-slug anchor resolution; `xdg-open` for URLs; `e` editor integration via `vim +N`; watch mode.
- [ ] **v0.4 — images:** half-block mosaic fallback; Kitty protocol with Unicode placeholders.
- [ ] **v1.0 — polish:** external YAML themes; per-file reading-position persistence (XDG state dir); `y` yank code block via OSC 52; `--width` flag; goreleaser + man page.
- **Backlog:** section folding (`za`/`zR`/`zM`), Sixel/iTerm2 images, footnotes, regex search, horizontal scroll for wide content.
