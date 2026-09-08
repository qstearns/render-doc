// Command render-doc renders a markdown file in the terminal.
//
// Fenced ```mermaid blocks are drawn as text diagrams by mermaid-ascii before
// glamour styles the document; a block mermaid-ascii can't draw is left as its
// source. glamour has no hook for custom code-block rendering, so the rewrite
// happens on the markdown text (the same approach as charmbracelet/glow#904).
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/diagram"
	"github.com/AlexanderGrooff/mermaid-ascii/pkg/render"
	"golang.org/x/term"
)

// fallbackWidth is used when no terminal is reachable to ask.
const fallbackWidth = 120

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "render-doc:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("render-doc", flag.ContinueOnError)
	var (
		width   int
		style   string
		ascii   bool
		noPager bool
		raw     bool
	)
	fs.IntVar(&width, "w", 0, "word-wrap `width` (default: the terminal's width)")
	fs.IntVar(&width, "width", 0, "alias for -w")
	fs.StringVar(&style, "s", "auto", "glamour `style`: auto, dark, light, notty, dracula, tokyo-night, pink, ascii, or a JSON file")
	fs.StringVar(&style, "style", "auto", "alias for -s")
	fs.BoolVar(&ascii, "ascii", false, "draw diagrams with plain ASCII instead of box-drawing characters")
	fs.BoolVar(&noPager, "no-pager", false, "write to stdout instead of opening the interactive viewer")
	fs.BoolVar(&raw, "raw", false, "leave mermaid fences as source")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: render-doc [flags] <file.md | ->")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("expected exactly one markdown file (or - for stdin)")
	}

	src, err := readInput(fs.Arg(0))
	if err != nil {
		return err
	}

	md := src
	if !raw {
		cfg := diagram.DefaultConfig()
		cfg.UseAscii = ascii
		md = rewriteMermaidFences(src, func(block string) (string, error) {
			return drawDiagram(block, cfg)
		})
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	resolvedStyle := resolveStyle(style, isTTY)
	if isTTY && !noPager {
		return viewMarkdown(md, resolvedStyle, width)
	}

	out, err := renderMarkdown(md, resolvedStyle, resolveWidth(width, isTTY))
	if err != nil {
		return err
	}
	_, err = io.WriteString(os.Stdout, out)
	return err
}

func readInput(path string) (string, error) {
	var (
		b   []byte
		err error
	)
	if path == "-" {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	return string(b), err
}

// drawDiagram renders one mermaid block. mermaid-ascii's parsers are the
// least battle-tested part of the chain, so a panic there is treated like any
// other unsupported diagram: the caller falls back to the source.
func drawDiagram(block string, cfg *diagram.Config) (out string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("mermaid-ascii panicked: %v", r)
		}
	}()
	return render.RenderDiagram(block, cfg)
}

// resolveStyle mirrors glow: "auto" means notty when stdout isn't a terminal,
// otherwise dark or light by asking the terminal for its background color.
func resolveStyle(style string, isTTY bool) glamour.TermRendererOption {
	switch {
	case style != "auto":
		return glamour.WithStylePath(style)
	case !isTTY:
		return glamour.WithStandardStyle(styles.NoTTYStyle)
	case lipgloss.HasDarkBackground(os.Stdin, os.Stdout):
		return glamour.WithStandardStyle(styles.DarkStyle)
	default:
		return glamour.WithStandardStyle(styles.LightStyle)
	}
}

// resolveWidth uses the full terminal width so wide diagrams aren't folded.
// When stdout isn't a terminal (a pager or pipe downstream) the controlling
// terminal is still the right thing to size for.
func resolveWidth(flag int, isTTY bool) int {
	if flag > 0 {
		return flag
	}
	if isTTY {
		if cols, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && cols > 0 {
			return cols
		}
	}
	if tty, err := os.Open("/dev/tty"); err == nil {
		defer tty.Close()
		if cols, _, err := term.GetSize(int(tty.Fd())); err == nil && cols > 0 {
			return cols
		}
	}
	return fallbackWidth
}

func renderMarkdown(md string, style glamour.TermRendererOption, width int) (string, error) {
	r, err := glamour.NewTermRenderer(style, glamour.WithWordWrap(width))
	if err != nil {
		return "", err
	}
	return r.Render(md)
}

// fence is one line that opens or closes a fenced code block.
type fence struct {
	indent string // leading spaces, at most three
	char   byte   // '`' or '~'
	n      int    // run length, at least three
	info   string // info string after the run, trimmed
}

// parseFence recognizes a fence line per CommonMark: a run of three or more
// backticks or tildes, indented by at most three spaces. A backtick fence's
// info string may not contain a backtick (that's inline code, not a fence).
func parseFence(line string) (fence, bool) {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	if len(indent) > 3 || trimmed == "" {
		return fence{}, false
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return fence{}, false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return fence{}, false
	}
	info := strings.TrimSpace(trimmed[n:])
	if c == '`' && strings.ContainsRune(info, '`') {
		return fence{}, false
	}
	return fence{indent: indent, char: c, n: n, info: info}, true
}

// closes reports whether f ends a block opened by open: same character, a run
// at least as long, and nothing else on the line.
func (f fence) closes(open fence) bool {
	return f.char == open.char && f.n >= open.n && f.info == ""
}

// lang is the first word of the info string, which is what marks a block as
// mermaid regardless of any attributes after it.
func (f fence) lang() string {
	lang, _, _ := strings.Cut(f.info, " ")
	return lang
}

// rewriteMermaidFences replaces each ```mermaid block in src with a plain
// code fence holding draw's rendering of it. A block draw rejects is copied
// through untouched, as is every other fence, so a ```mermaid line nested in
// a longer fence is never mistaken for a diagram. Line endings and a missing
// final newline are preserved.
func rewriteMermaidFences(src string, draw func(string) (string, error)) string {
	var out strings.Builder
	lines := strings.SplitAfter(src, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i := 0; i < len(lines); {
		open, ok := parseFence(lines[i])
		if !ok {
			out.WriteString(lines[i])
			i++
			continue
		}
		// An unterminated fence runs to the end of the document.
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if f, ok := parseFence(lines[j]); ok && f.closes(open) {
				end = j
				break
			}
		}
		next := min(end+1, len(lines))
		if open.lang() == "mermaid" {
			body := stripIndent(strings.Join(lines[i+1:end], ""), len(open.indent))
			if drawn, err := draw(body); err == nil {
				writeFence(&out, open.indent, drawn)
				i = next
				continue
			}
		}
		out.WriteString(strings.Join(lines[i:next], ""))
		i = next
	}
	return out.String()
}

// stripIndent removes up to n leading spaces from each line, as CommonMark
// does for the content of an indented fence.
func stripIndent(body string, n int) string {
	if n == 0 {
		return body
	}
	lines := strings.SplitAfter(body, "\n")
	for i, l := range lines {
		k := 0
		for k < n && k < len(l) && l[k] == ' ' {
			k++
		}
		lines[i] = l[k:]
	}
	return strings.Join(lines, "")
}

// writeFence emits drawn inside a plain fence, re-indented to sit where the
// original block sat (inside a list item, say).
func writeFence(out *strings.Builder, indent, drawn string) {
	out.WriteString(indent + "```\n")
	for _, l := range strings.SplitAfter(strings.TrimRight(drawn, "\n")+"\n", "\n") {
		if l != "" {
			out.WriteString(indent + l)
		}
	}
	out.WriteString(indent + "```\n")
}
