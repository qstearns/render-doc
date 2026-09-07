package main

import (
	"errors"
	"strings"
	"testing"
)

// stubDraw wraps a block's trimmed source in brackets so tests can see what
// was passed, and rejects class diagrams the way mermaid-ascii does.
func stubDraw(block string) (string, error) {
	if strings.Contains(block, "classDiagram") {
		return "", errors.New("unsupported diagram")
	}
	return "[" + strings.TrimSpace(block) + "]\n", nil
}

func TestRewriteMermaidFences(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			name: "plain fence is drawn",
			in:   "# T\n\n```mermaid\ngraph LR\n  A --> B\n```\n\ntail\n",
			want: "# T\n\n```\n[graph LR\n  A --> B]\n```\n\ntail\n",
		},
		{
			name: "rejected block is copied through",
			in:   "```mermaid\nclassDiagram\n  A <|-- B\n```\n",
			want: "```mermaid\nclassDiagram\n  A <|-- B\n```\n",
		},
		{
			name: "other fences are untouched",
			in:   "```go\nfunc main() {}\n```\n",
			want: "```go\nfunc main() {}\n```\n",
		},
		{
			name: "mermaid line nested in a longer fence is not a diagram",
			in:   "````md\n```mermaid\ngraph LR\n```\n````\n",
			want: "````md\n```mermaid\ngraph LR\n```\n````\n",
		},
		{
			name: "indented fence keeps its indent and strips it from the body",
			in:   "- item\n\n  ```mermaid\n  graph LR\n    A --> B\n  ```\n",
			want: "- item\n\n  ```\n  [graph LR\n    A --> B]\n  ```\n",
		},
		{
			name: "tilde fences and info attributes",
			in:   "~~~mermaid {.x}\ngraph TD\n~~~\n",
			want: "```\n[graph TD]\n```\n",
		},
		{
			name: "unterminated fence runs to end of document",
			in:   "```mermaid\ngraph LR\n  A --> B",
			want: "```\n[graph LR\n  A --> B]\n```\n",
		},
		{
			name: "missing final newline is preserved on prose",
			in:   "one\ntwo",
			want: "one\ntwo",
		},
		{
			name: "short closing run does not close",
			in:   "````mermaid\ngraph LR\n```\nstill inside\n````\n",
			want: "```\n[graph LR\n```\nstill inside]\n```\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rewriteMermaidFences(c.in, stubDraw); got != c.want {
				t.Errorf("\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

func TestParseFence(t *testing.T) {
	if _, ok := parseFence("    ```mermaid"); ok {
		t.Error("four-space indent is an indented code block, not a fence")
	}
	if _, ok := parseFence("``` a ` b"); ok {
		t.Error("backtick in a backtick fence's info string is inline code")
	}
	if f, ok := parseFence("~~~ a ` b"); !ok || f.lang() != "a" {
		t.Error("tilde fences may carry backticks in the info string")
	}
}
