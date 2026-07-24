# mdv Feature Demo

This document exercises every implemented feature. Open it full-screen with
`go run ./cmd/mdv examples/demo.md`, then press `?` for the key help, `t`
for the table of contents, and `/` to search. The word needle appears
throughout — search for it in lowercase, then as `Needle`, to see
smart-case in action.

## Typography

Plain paragraphs wrap to the terminal width. *Emphasis*, **strong**,
***both nested***, ~~strikethrough~~, and `inline code` all combine, even
**bold with *italic inside* and `code`**. Escapes stay literal: \*not
emphasized\*, and entities resolve: &copy; &amp; &rarr;.

A soft line break
is just a space, while a hard break\
forces a new line (needle #1 hides here).

Setext Heading Two
------------------

Headings also come in setext flavor, and this one is a TOC entry too.

### Deep Nesting

#### Level Four

##### Level Five

###### Level Six — the TOC indents each level

## Blockquotes

> A quote wraps like any paragraph when the text runs long enough to need it.
>
> > Nested quotes stack their bars.
> >
> > - Lists work inside quotes
> > - So does `code` and **style**
>
> ```go
> // Even fenced code lives happily inside a quote.
> fmt.Println("quoted needle")
> ```

## Lists Galore

- Unordered bullets cycle by depth
  - Second level switches glyph
    - Third level switches again
      - Fourth wraps around the cycle
- An item long enough to wrap onto a continuation line shows the hanging
  indent that keeps text aligned under the marker

8. Ordered lists honor their start number
9. Nine
10. Ten — double digits right-align the markers
11. Eleven (Needle #2 — capital N, exact-match it)

- [ ] An open task
- [x] A completed task
- [ ] Tasks mix freely with other items

Loose lists keep paragraph spacing:

- The first loose item

- The second one carries a second paragraph

  which sits indented under the same marker

## Code Showcase

```go
package main

import "fmt"

func main() {
	tabs := "expand to four spaces"
	fmt.Println("hello from mdv", tabs)
	// A deliberately long line to demonstrate chunk-wrapping of code that cannot word-wrap sensibly at all.
}
```

```python
def fib(n: int) -> int:
    return n if n < 2 else fib(n - 1) + fib(n - 2)  # needle in python
```

```json
{"name": "mdv", "version": "0.2", "gfm": true}
```

    An indented code block, no fence, no highlighting.

## Tables

| Left | Center | Right |
| :--- | :----: | ----: |
| a    |   b    |     c |
| wider cell | ✓ | 42 |

| Column | This one has deliberately verbose content to force the shrink-and-truncate behavior | Numbers |
| ------ | ----------------------------------------------------------------------------------- | ------: |
| one    | the widest column gives up width first, ellipsis marks the cut                       |    1000 |
| two    | needle #3 sits in a table cell                                                       |      99 |

## Links and Images

An [inline link](https://example.com), a [reference link][gold], an
autolink <https://example.com/angle>, and a bare URL that GFM linkifies:
https://example.com/bare. All emit OSC 8 hyperlinks — click them in a
modern terminal, or press `f` and type a label to follow one. This
[anchor link](#search-playground) jumps to the Search Playground below;
come back with `Ctrl+O`, go forward again with `Tab`. This one is long enough to chunk across lines:
https://example.com/a/very/long/path/segment/that/cannot/possibly/fit/on/a/single/terminal/line/anywhere

A local image on its own paragraph renders in the terminal — half-block
mosaic anywhere, real pixels in kitty/ghostty (note the transparent
corner). Remote images stay placeholders:

![A gradient with a transparent corner](gradient.png)

![The mdv logo, eventually](https://example.com/mdv.png)

[gold]: https://github.com/yuin/goldmark

## Unicode Stress

日本語のテキストは端末セル幅を二つ使うので、折り返し計算が正しいことを
ここで確認できます。Emoji count double-width too: 🎉 🚀 📚 — and the
status bar, tables, and wrapping all measure them with runewidth.

## Search Playground

Scatter zone: needle, NEEDLE, Needle, kneedle, needles. A lowercase
`/needle` query hits them all case-insensitively (five in this paragraph
alone); `/Needle` narrows to the exact capitals. Press `n` and `N` to hop
between hits — the count lives in the status bar. `Esc` clears the
highlights.

---

That horizontal rule above is a thematic break.

## Raw HTML

<details>
<summary>HTML blocks render dim and verbatim</summary>
mdv does not interpret HTML; it shows it faithfully.
</details>

Inline HTML like <kbd>q</kbd> passes through dimmed as well.

## Math Preview

Inline math $E = mc^2$ and display math stay raw TeX until v0.5:

$$
\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}
$$

The final needle is here. Press `g` to jump back to the top.
