package main

import "testing"

// The lines a viewer holds are glamour's output: indented by its margin and
// padded out to the render width.
var selectionLines = []string{
	"  hello world  ",
	"               ",
	"  second line  ",
}

func TestSelectionText(t *testing.T) {
	tests := []struct {
		name string
		sel  selection
		want string
	}{
		{
			name: "within one line",
			sel:  selection{anchor: cell{0, 2}, cursor: cell{0, 6}, active: true},
			want: "hello",
		},
		{
			name: "dragged backwards",
			sel:  selection{anchor: cell{0, 6}, cursor: cell{0, 2}, active: true},
			want: "hello",
		},
		{
			name: "across lines, keeping the blank one and dropping the padding",
			sel:  selection{anchor: cell{0, 8}, cursor: cell{2, 7}, active: true},
			want: "world\n\n  second",
		},
		{
			name: "past the end of a line",
			sel:  selection{anchor: cell{2, 9}, cursor: cell{2, 14}, active: true},
			want: "line",
		},
		{
			name: "past the end of the document",
			sel:  selection{anchor: cell{2, 0}, cursor: cell{9, 0}, active: true},
			want: "  second line",
		},
		{
			name: "inactive",
			sel:  selection{anchor: cell{0, 2}, cursor: cell{0, 6}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sel.text(selectionLines); got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectionHighlight(t *testing.T) {
	tests := []struct {
		name             string
		rows             []string
		sel              selection
		yOffset, xOffset int
		want             []string
	}{
		{
			name: "one row, leaving the others alone",
			rows: []string{selectionLines[0], selectionLines[2]},
			sel:  selection{anchor: cell{0, 2}, cursor: cell{0, 6}, active: true},
			want: []string{"  \x1b[7mhello\x1b[m world  ", "  second line  "},
		},
		{
			name: "squared off at the right edge across rows",
			rows: []string{selectionLines[0], selectionLines[2]},
			sel:  selection{anchor: cell{0, 8}, cursor: cell{1, 7}, active: true},
			want: []string{"  hello \x1b[7mworld  \x1b[m", "\x1b[7m  second\x1b[m line  "},
		},
		{
			name:    "shifted by the scroll offsets",
			rows:    []string{"second line  "},
			sel:     selection{anchor: cell{2, 2}, cursor: cell{2, 7}, active: true},
			yOffset: 2,
			xOffset: 2,
			want:    []string{"\x1b[7msecond\x1b[m line  "},
		},
		{
			name: "not at all when inactive",
			rows: []string{selectionLines[0]},
			sel:  selection{anchor: cell{0, 2}, cursor: cell{0, 6}},
			want: []string{"  hello world  "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := append([]string(nil), tt.rows...)
			tt.sel.highlight(rows, tt.yOffset, tt.xOffset, 15)
			for i := range rows {
				if rows[i] != tt.want[i] {
					t.Errorf("row %d = %q, want %q", i, rows[i], tt.want[i])
				}
			}
		})
	}
}

func TestWordAt(t *testing.T) {
	//              0123456789
	const line = "  foo_bar1, baz"

	tests := []struct {
		name     string
		line     string
		col      int
		from, to int
		ok       bool
	}{
		{name: "inside a word", line: line, col: 5, from: 2, to: 10, ok: true},
		{name: "at a word's first cell", line: line, col: 2, from: 2, to: 10, ok: true},
		{name: "on punctuation", line: line, col: 10, from: 10, to: 11, ok: true},
		{name: "on the margin", line: line, col: 0, ok: false},
		{name: "between words", line: line, col: 11, ok: false},
		{name: "past the end", line: line, col: 40, ok: false},
		{name: "on a wide character", line: "日本 ok", col: 2, from: 0, to: 4, ok: true},
		{name: "across a combining mark", line: "e\u0301clair x", col: 3, from: 0, to: 6, ok: true},
		{name: "after wide characters", line: "日本 ok", col: 5, from: 5, to: 7, ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, ok := wordAt(tt.line, tt.col)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && (from != tt.from || to != tt.to) {
				t.Errorf("range = [%d,%d), want [%d,%d)", from, to, tt.from, tt.to)
			}
		})
	}
}
