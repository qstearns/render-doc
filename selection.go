package main

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// selectionStyle inverts whatever sits under it, so the highlight reads the
// same against every glamour theme. lipgloss.StyleRanges drops the styling
// inside a range and re-renders it with the range's own style, which is what
// makes a bare reverse enough here.
var selectionStyle = lipgloss.NewStyle().Reverse(true)

// cell is a position in the rendered document: a line index, and a column
// measured in terminal cells rather than bytes or runes. Cells are the unit
// the mouse reports in and the unit ansi.Cut counts in, so staying in them
// keeps wide characters from shifting the two out of step.
type cell struct {
	line, col int
}

// before reports whether c comes earlier in the document than other.
func (c cell) before(other cell) bool {
	return c.line < other.line || (c.line == other.line && c.col < other.col)
}

// selection is the text between two cells, both ends included: dragging
// across a character selects it rather than stopping just short of it, which
// is what a terminal's own selection does.
type selection struct {
	anchor cell // where the drag started; may be after cursor
	cursor cell // where the pointer is now
	active bool // whether there is anything to highlight or copy
	drag   bool // whether the button is still down
}

// bounds returns the selection's ends in document order.
func (s selection) bounds() (start, end cell) {
	if s.cursor.before(s.anchor) {
		return s.cursor, s.anchor
	}
	return s.anchor, s.cursor
}

// span returns the half-open column range of line that falls inside the
// selection. Every line but the last runs to eol, which callers set to the
// line's own width when copying text and to the viewport's right edge when
// drawing, so a multi-line highlight squares off at the edge of the window
// the way a terminal's does.
func (s selection) span(line, eol int) (from, to int, ok bool) {
	start, end := s.bounds()
	if !s.active || line < start.line || line > end.line {
		return 0, 0, false
	}
	from, to = 0, eol
	if line == start.line {
		from = start.col
	}
	if line == end.line {
		to = min(end.col+1, eol)
	}
	if to <= from {
		return 0, 0, false
	}
	return from, to, true
}

// text returns the selected text, taken from the rendered document's lines
// with their styling stripped. Trailing spaces go too: glamour pads every
// line out to the render width, and that padding is not what anyone means to
// copy.
func (s selection) text(lines []string) string {
	if !s.active {
		return ""
	}
	start, end := s.bounds()
	var b strings.Builder
	for line := start.line; line <= min(end.line, len(lines)-1); line++ {
		if line > start.line {
			b.WriteByte('\n')
		}
		plain := ansi.Strip(lines[line])
		if from, to, ok := s.span(line, ansi.StringWidth(plain)); ok {
			b.WriteString(strings.TrimRight(ansi.Cut(plain, from, to), " "))
		}
	}
	return b.String()
}

// highlight inverts the selected cells of rows in place. rows are the
// viewport's rendered lines, so row i shows document line yOffset+i and its
// leftmost column is document column xOffset.
func (s selection) highlight(rows []string, yOffset, xOffset, width int) {
	if !s.active {
		return
	}
	for i, row := range rows {
		from, to, ok := s.span(yOffset+i, xOffset+width)
		if !ok {
			continue
		}
		from, to = max(from-xOffset, 0), min(to-xOffset, width)
		if to <= from {
			continue
		}
		rows[i] = lipgloss.StyleRanges(row, lipgloss.NewRange(from, to, selectionStyle))
	}
}

// charClass groups characters the way a double-click does: a run of word
// characters is a word, and a run of anything else that isn't a space is one
// too, which is what picks an identifier out of the punctuation around it.
type charClass int

const (
	classSpace charClass = iota
	classWord
	classOther
)

func classOf(r rune) charClass {
	switch {
	case unicode.IsSpace(r):
		return classSpace
	case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
		return classWord
	default:
		return classOther
	}
}

// wordAt returns the half-open cell range of the run of same-class characters
// under col in plain, which must already have its styling stripped. A column
// on a space or past the end of the line selects nothing, since a double
// click on glamour's padding otherwise hands back a screenful of blanks.
func wordAt(plain string, col int) (from, to int, ok bool) {
	// Walk grapheme clusters rather than runes so a combining mark stays with
	// the character it modifies, and columns keep counting the same way
	// ansi.Cut counts them.
	var classes []charClass
	// starts[i] is the column cluster i begins at. The extra last entry is the
	// width of the whole line, which is where the last cluster ends.
	starts, x := []int{0}, 0
	for g := uniseg.NewGraphemes(plain); g.Next(); {
		classes = append(classes, classOf(g.Runes()[0]))
		x += max(1, g.Width())
		starts = append(starts, x)
	}

	i := 0
	for i < len(classes) && starts[i+1] <= col {
		i++
	}
	if i == len(classes) || classes[i] == classSpace {
		return 0, 0, false
	}

	lo, hi := i, i+1
	for lo > 0 && classes[lo-1] == classes[i] {
		lo--
	}
	for hi < len(classes) && classes[hi] == classes[i] {
		hi++
	}
	return starts[lo], starts[hi], true
}
