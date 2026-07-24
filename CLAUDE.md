# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Name:** `mdv` (MDView)
**Language:** Go (Golang 1.22+)
**Purpose:** A high-performance, full-screen terminal Markdown viewer compliant with the [CommonMark 0.31.2 specification](https://spec.commonmark.org/0.31.2/).
**Core Philosophy:** Fast terminal rendering, rich visual theming, seamless navigation, and lean external editor integration without bloating the binary with custom editing engines.

**Current state:** This repository is spec-only so far — it contains this file and the MIT LICENSE. No Go module or source code exists yet. The architecture in Part 2 describes what is to be built; run `go mod init` and create `cmd/mdv/main.go` when starting implementation.

---

## Part 1: Mandatory AI Behavioral Rules (Karpathy Guidelines)

*Derived from Andrej Karpathy's observations on LLM coding pitfalls. All code generation, refactoring, and PRs MUST adhere to these four principles:*

### 1. Think Before Coding
- **State Assumptions Explicitly:** If requirements or edge cases are ambiguous, state your working assumptions before writing code. If uncertain, stop and ask for clarification rather than guessing.
- **Present Tradeoffs:** Do not silently choose an architectural implementation when multiple viable interpretations exist (e.g., terminal image protocol fallback strategies). Present the options concisely.
- **Surface Inconsistencies & Push Back:** If a requested feature violates CommonMark 0.31.2 specs or introduces unnecessary complexity, push back and explain the simpler or specification-compliant approach.

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
  - *Instead of:* "Add wildcard search" → *Do:* "Write unit tests for glob/wildcard search queries, then implement matching logic until tests pass."
  - *Instead of:* "Fix link backtracking" → *Do:* "Write a test simulating a 3-link jump history stack, assert `Pop()` returns correct offsets, and fix the state handler."
- **Multi-Step Plan Verification:** For complex features, list a concise checklist:
  1. [Step] → verify: [command/test]
  2. [Step] → verify: [command/test]

---

## Part 2: Architecture & Feature Specifications

### 1. Markdown Parsing Standard
- **Specification:** Strict adherence to **CommonMark 0.31.2**.
- **Parser Engine:** Prefer standard AST-based CommonMark parsers in Go (e.g., `yuin/goldmark`). Do not write a custom regex-based markdown parser from scratch unless extending AST node renderers.
- **Custom Renderers:** Implement terminal-specific AST node renderers that translate markdown tokens into styled ANSI escape sequences.

### 2. Terminal UI (TUI) & Visual Theming
- **Full-Screen Execution:** When invoked as `mdv <file.md>`, the app must immediately initialize an alternate screen buffer (via `tcell`, `bubbletea`, or termios) and render full-screen.
- **Theming & Color Usage:**
  - Use semantic color mapping (e.g., distinct hex/ANSI colors for headers, code blocks, blockquotes, bold/italic accents).
  - Support external JSON/YAML theme files or standard terminal color schemes (e.g., Solarized, Catppuccin, Gruvbox, Nord).
  - **UTF-8 Symbols:** Use clean UTF-8 glyphs for list bullets (`•`, `◦`, `▪`), quote borders (`│`), checkboxes (`[✓]`, `[ ]`), and table borders (`┌`, `┬`, `┐`, `├`, `┼`, `┤`, `└`, `┴`, `┘`).

### 3. Search Engine
- **Fuzzy Search:** Implement approximate string matching (e.g., Levenshtein distance or Smith-Waterman scoring) to highlight matching headers and body text in real-time.
- **Wildcard Option:** Support standard shell glob syntax (`*`, `?`, `[a-z]`) within the search input bar.
- **UI Integration:** Highlight all search hits in the viewport with a high-contrast background color; allow jumping between matches via `n`/`N` (next/previous).

### 4. Navigation & Link Tracing (Backtracking)
- **Table of Contents (TOC):** Dynamically generate a collapsible/floating TOC sidebar or popup panel from `H1`–`H6` AST nodes. Selecting a TOC entry must jump the viewport directly to that section.
- **Link Tracing & History Stack:**
  - When a user hits `Enter` on a local anchor link (`#section`) or relative document link (`./other.md`), push the current document path and scroll Y-offset onto a navigation history stack.
  - Pressing `Back` (e.g., `Backspace` or `Ctrl+O`) must pop the stack and restore the exact previous document and viewport scroll state.
- **Live Links:**
  - **URLs:** Emit terminal hyperlinks using **OSC 8 escape sequences** (`\x1b]8;;http://url\x1b\\label\x1b]8;;\x1b\\`) so modern terminals can natively click them.
  - **Images:** Support inline terminal image protocols where available (e.g., **Kitty graphics protocol**, **iTerm2 inline images**, or **Sixel** fallback). If unsupported by the host terminal, gracefully degrade to a styled placeholder: `🖼 [Image: Alt Text] (url)`.

### 5. Vim Editor Integration
- **No Embedded Editor:** Do NOT implement custom text manipulation or text-box editing engines.
- **External Invocation:** When the user triggers edit mode (e.g., pressing `e` or `i`), gracefully suspend the TUI alternate screen, execute `vim` (or `$EDITOR`) pointing to the current file at the exact current line number (e.g., `vim +<line_no> <file.md>`), and resume the TUI upon process exit.
- **Vim Libraries/RPC:** If advanced live-previewing is required, utilize lightweight Vim/Neovim RPC client libraries or embed headless Neovim instances via msgpack-rpc rather than re-implementing modal editing logic.

---

## Part 3: Go Development Commands & Conventions

- **Run App:** `go run cmd/mdv/main.go <path/to/file.md>`
- **Build Binary:** `go build -ldflags="-s -w" -o bin/mdv ./cmd/mdv`
- **Run All Tests:** `go test -v -race ./...`
- **Run Specific Test:** `go test -v -run TestFunctionName ./path/to/pkg`
- **Benchmarking:** `go test -bench=. -benchmem ./...`
- **Linting & Formatting:**
  ```bash
  gofmt -s -w .
  go vet ./...
  golangci-lint run
  ```
