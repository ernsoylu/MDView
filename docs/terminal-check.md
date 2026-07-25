# mdv terminal check

A single document that exercises the parts of mdv that only a real terminal
can confirm. Not shipped in releases — `.goreleaser.yaml` archives
`docs/mdv.1`, not this file.

Run it with:

    mdv docs/terminal-check.md

## 1. Text, tables, code

Emphasis: **bold**, *italic*, ~~struck~~, `code span`.

| Check | Expect |
|-------|--------|
| borders | box-drawing, aligned |
| colour | header brighter than body |

```go
func main() { fmt.Println("syntax highlighting") }
```

## 2. Links — press `f`

A [web link](https://example.com/mdv-check) and an [anchor](#5-hostile-escapes).

* `f` should overlay letter labels on both.
* Typing the web link's label opens a browser. **On macOS that exercises
  `open`**, which has never run on a Mac — before this it was hardcoded to
  `xdg-open` and did nothing at all.

## 3. Image — the kitty graphics path

![gradient](../examples/gradient.png)

* **In ghostty you should see a smooth blue-to-red gradient**, not blocky
  half-height characters. ghostty is detected via `TERM_PROGRAM=ghostty`,
  which was added blind and has never been tested on real ghostty.
* Blocky `▀` output means it fell back to the mosaic — detection missed.
* Inside tmux the mosaic is correct and deliberate.

## 4. Display math

$$
\frac{a+b}{c-d} = \sqrt{x} + \int f(x)
$$

Renders through the same image pipeline. Sub- and superscripts are not
supported upstream and fall back to raw TeX — that is expected, not a bug.

## 5. Hostile escapes

Every line below carries real escape bytes. **They must all appear as `?`
replacement characters**, and none of them may take effect.

Title hijack: ]0;MDV-CHECK-FAILED — the window title must not change.

Clipboard write: ]52;c;bWR2IGNoZWNrIGZhaWxlZA== — your clipboard must
be untouched.

Screen wipe and colour: [2J[31mRED[0m — nothing should clear or turn red.

```
code block ]0;MDV-CHECK-FAILED [41m
```

<div>]0;MDV-CHECK-FAILED</div>

[link with a breakout destination](http://example.com\]0;MDV-CHECK-FAILED)

## 6. Checklist

- [ ] tables, code highlighting and colours look right
- [ ] `f` labels both links; following the web one opens a browser (macOS: `open`)
- [ ] `t` lists the six headings and jumps
- [ ] `/` finds a word and reads "1 match" for a single hit
- [ ] the gradient is a real image in ghostty
- [ ] the equation renders as an image
- [ ] section 5 shows `?` characters and the window title is unchanged
- [ ] `e` opens $EDITOR at the right line, and returning re-renders
- [ ] `q` quits; from `mdv .` it returns to the file list instead
