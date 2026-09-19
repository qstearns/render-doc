# render-doc

Render a markdown file in the terminal, with ` ```mermaid ` fences drawn as
text diagrams.

```
render-doc docs/rfcs/0005-mcp.md
cat notes.md | render-doc -
render-doc -w 100 -s light --no-pager README.md
```

It is [glamour](https://github.com/charmbracelet/glamour) (the renderer under
glow) plus [mermaid-ascii](https://github.com/AlexanderGrooff/mermaid-ascii)
in one binary. glamour has no hook for custom code-block rendering, so each
mermaid fence is drawn first and spliced back into the markdown as a plain
code block; a diagram mermaid-ascii can't draw (class and state diagrams,
subgraphs) is left as its source. glow was asked to do this in
[#904](https://github.com/charmbracelet/glow/pull/904) and never took it.

## Flags

| flag | default | |
|---|---|---|
| `-w`, `--width` | terminal width | word-wrap column; the full width so wide diagrams aren't folded |
| `-s`, `--style` | `auto` | glamour style: `auto`, `dark`, `light`, `notty`, `dracula`, `tokyo-night`, `pink`, `ascii`, or a JSON file. `auto` asks the terminal for its background and uses `notty` when stdout isn't a terminal |
| `--ascii` | off | draw diagrams with plain ASCII instead of box-drawing characters |
| `--no-pager` | off | write to stdout instead of the interactive viewer, which is used whenever stdout is a terminal |
| `--raw` | off | leave mermaid fences as source |

The interactive viewer rerenders the document when the terminal is resized.
Use arrow keys or `j`/`k` to scroll, space/`f` and `b` to move by a page,
`g`/`G` to jump to the top/bottom, and `q` to quit. An explicit `--width`
keeps the document layout fixed while the viewport itself still resizes.

Drag with the mouse to select text; double-click takes a word and
triple-click a line. Releasing the button copies the selection, `y` copies it
again, and `esc` clears it. Copying goes through OSC 52, so it works over ssh
but not in terminals that don't implement the escape (Terminal.app is the
common one) — there, hold <kbd>option</kbd> to use Terminal's own selection
instead, which bypasses the viewer entirely.

## Install

Through mise's go backend, in a project or in `~/.config/mise/config.toml`:

```toml
[tools]
"go:github.com/qstearns/render-doc" = "latest"
```

Or `go install github.com/qstearns/render-doc@latest`.

For local development, `mise run install` builds the working tree into
`~/.local/bin`; that copy shadows a mise-managed one if `~/.local/bin` comes
first on your PATH, so delete it when you're done.
