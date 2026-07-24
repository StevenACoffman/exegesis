package book2skill

import (
	"sort"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// fencedLines returns the set of 0-based line indices (matching
// strings.Split(md, "\n")) that fall inside a fenced or indented code block.
// The document parsers consult it so a "## " or "# " line inside a code example
// is not mistaken for a heading — otherwise a shell comment like "## Boundary"
// inside an Execution segment truncates that segment, and a fenced heading in a
// generated document suppresses a genuinely custom section of the same name.
//
// goldmark supplies the code-block spans (CommonMark, no extensions). The parser
// is built per call: construction is microseconds on the small documents this
// package handles, and a package-level global would trip gochecknoglobals.
func fencedLines(md string) map[int]bool {
	source := []byte(md)
	root := goldmark.New().Parser().Parse(text.NewReader(source))
	starts := lineStarts(source)

	fenced := make(map[int]bool)
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if k := n.Kind(); k != ast.KindFencedCodeBlock && k != ast.KindCodeBlock {
			return ast.WalkContinue, nil
		}
		lines := n.Lines()
		for i := range lines.Len() {
			fenced[lineAt(starts, lines.At(i).Start)] = true
		}
		return ast.WalkContinue, nil
	})
	return fenced
}

// lineStarts returns the byte offset at which each line of src begins.
func lineStarts(src []byte) []int {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineAt returns the 0-based index of the line containing byte offset off, given
// the line-start offsets from lineStarts.
func lineAt(starts []int, off int) int {
	return sort.SearchInts(starts, off+1) - 1
}
