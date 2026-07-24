package skilllint

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// This file is the goldmark-backed markdown structure parser used by mdutil.go.
// It replaces hand-rolled heading/link regexes with CommonMark AST facts, so a
// "#" inside a code fence is not a heading and a list item followed by "----" is
// not a Setext heading. It exposes only what the rubric needs; the slug policy
// (SlugifyHeading) and the unclosed-fence lint scanner stay in mdutil.go.
//
// The parser is built per call rather than as a package global: construction is
// microseconds, and a global would trip gochecknoglobals. GFM is intentionally
// NOT enabled — linkify would turn bare prose URLs into links and change what the
// link checkers see.

// mdLink is one parsed link or image reference.
type mdLink struct {
	Destination string
	IsImage     bool
}

// parse returns the CommonMark AST root for src along with the source bytes the
// nodes index into. Plain goldmark, no extensions — see the file comment.
func parse(src string) (ast.Node, []byte) {
	source := []byte(src)
	return goldmark.New().Parser().Parse(text.NewReader(source)), source
}

// headingTexts returns the rendered text of every heading (ATX and Setext) in
// document order. Headings inside code blocks are not heading nodes, so they are
// excluded. A code span within a heading contributes its content, matching the
// backtick-stripped text SlugifyHeading expects.
func headingTexts(src string) []string {
	root, source := parse(src)

	var out []string
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			out = append(out, nodeText(h, source))
		}
	}
	return out
}

// mdLinks returns every link and image destination in document order. Link syntax
// inside code spans or code blocks is not a link node, so it is excluded;
// CommonMark <autolinks> are ignored (the previous regex never matched them).
func mdLinks(src string) []mdLink {
	root, _ := parse(src)

	var out []mdLink
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Image:
			out = append(out, mdLink{Destination: string(v.Destination), IsImage: true})
			return ast.WalkSkipChildren, nil // alt text is not a link
		case *ast.Link:
			out = append(out, mdLink{Destination: string(v.Destination)})
		}
		return ast.WalkContinue, nil
	})
	return out
}

// nodeText concatenates the readable text of a subtree: Text and String literals
// (which include code-span contents, since a CodeSpan's child is a Text node).
func nodeText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := node.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(source))
		case *ast.String:
			b.Write(v.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}
