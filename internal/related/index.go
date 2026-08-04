package related

import (
	"fmt"
	"strings"
)

// Generated-region markers. `index` regenerates everything between them and
// preserves whatever a user adds below the end marker.
const (
	startMarker = "<!-- exegesis:index:start -->"
	endMarker   = "<!-- exegesis:index:end -->"
)

// Header carries the resolved title and author for the INDEX.md heading.
type Header struct {
	Title  string
	Author string
}

// Render returns the full INDEX.md: a generated region (title, skill list,
// relationship graph, dependency-ordered learning path) delimited by marker
// comments, followed by preserved — the caller's hand-added tail. Nodes are
// listed in slug order; the whole output is deterministic.
func Render(h Header, nodes []Node, preserved string) string {
	order, cyclic := LearningPath(nodes)
	var b strings.Builder
	b.WriteString(startMarker + "\n")
	fmt.Fprintf(&b, "# %s\n", title(h))
	if h.Author != "" {
		fmt.Fprintf(&b, "\nby %s\n", h.Author)
	}
	b.WriteString("\n## Skills\n\n")
	b.WriteString(skillList(nodes))
	b.WriteString("\n## Relationship graph\n\n```mermaid\n")
	b.WriteString(Mermaid(nodes))
	b.WriteString("```\n\n## Learning path\n\n")
	b.WriteString(learningList(order, cyclic))
	b.WriteString(endMarker + "\n")
	if preserved != "" {
		b.WriteString("\n" + preserved)
	}
	return b.String()
}

// Split returns the preserved tail of an existing INDEX.md — everything after
// the generated region's end marker. It is "" when the markers are absent (a
// hand-written or empty file), meaning a fresh generation replaces the whole
// file.
func Split(existing string) string {
	_, after, found := strings.Cut(existing, endMarker+"\n")
	if !found {
		if _, tail, ok := strings.Cut(existing, endMarker); ok {
			return strings.TrimPrefix(tail, "\n")
		}
		return ""
	}
	return strings.TrimPrefix(after, "\n")
}

// title resolves the heading text: the header title, or a placeholder when it is
// empty so the file is still well-formed.
func title(h Header) string {
	if h.Title == "" {
		return "Skills"
	}
	return h.Title
}

// skillList renders one bullet per node, slug-ordered, as
// "- **<slug>** — <description>".
func skillList(nodes []Node) string {
	if len(nodes) == 0 {
		return "_No skills found._\n"
	}
	var b strings.Builder
	for _, n := range sortedBySlug(nodes) {
		if n.Description == "" {
			fmt.Fprintf(&b, "- **%s**\n", n.Slug)
			continue
		}
		fmt.Fprintf(&b, "- **%s** — %s\n", n.Slug, n.Description)
	}
	return b.String()
}

// learningList renders the ordered learning path, with a warning note when the
// depends-on graph has a cycle.
func learningList(order, cyclic []string) string {
	var b strings.Builder
	if len(cyclic) > 0 {
		fmt.Fprintf(&b, "> ⚠️ depends-on cycle among: %s\n\n", strings.Join(cyclic, ", "))
	}
	for i, slug := range order {
		fmt.Fprintf(&b, "%d. %s\n", i+1, slug)
	}
	if len(order) == 0 {
		b.WriteString("_No skills found._\n")
	}
	return b.String()
}
