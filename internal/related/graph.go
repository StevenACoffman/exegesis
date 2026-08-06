package related

import (
	"fmt"
	"sort"
	"strings"
)

// Node is one skill in the index: its slug, human title, description, and the
// related-skill edges parsed from its `## Related skills` section.
type Node struct {
	Slug        string
	Title       string
	Description string
	Edges       []Edge
}

// DanglingEdge is one edge whose target is not a skill in the tree — an edge the
// graph cannot render, so LearningPath and Mermaid drop it. Reporting it is the
// only way the loss becomes visible.
type DanglingEdge struct {
	Source string // slug of the skill whose section holds the edge
	Edge   Edge   // the edge itself, unknown target included
}

// LearningPath returns the node slugs in dependency order — every skill appears
// after each skill it depends-on — tie-broken lexicographically by slug so the
// output is deterministic. Only DependsOn edges to known slugs are followed.
//
// If the depends-on edges contain a cycle, the still-unresolved slugs are
// appended in slug order and returned as cyclic (the caller renders a warning
// rather than failing). cyclic is empty when the graph is acyclic.
//
// Ensures: order is a permutation of every node slug; for an acyclic graph, no
// slug precedes one it depends-on.
func LearningPath(nodes []Node) (order, cyclic []string) {
	slugs, known := slugSet(nodes)
	indegree := make(map[string]int, len(slugs))
	successors := make(map[string][]string, len(slugs))
	for _, n := range nodes {
		for _, prereq := range prereqs(n, known) {
			successors[prereq] = append(successors[prereq], n.Slug)
			indegree[n.Slug]++
		}
	}
	order = make([]string, 0, len(slugs))
	emitted := make(map[string]bool, len(slugs))
	for len(order) < len(slugs) {
		next := smallestReady(slugs, indegree, emitted)
		if next == "" {
			break // a cycle blocks every remaining node
		}
		order = append(order, next)
		emitted[next] = true
		for _, s := range successors[next] {
			indegree[s]--
		}
	}
	for _, s := range slugs {
		if !emitted[s] {
			cyclic = append(cyclic, s)
		}
	}
	order = append(order, cyclic...)
	return order, cyclic
}

// Mermaid renders a `graph TD` of every edge to a known slug, deterministically
// ordered (nodes by slug, then edges by source, kind, target). The node label
// is the skill's title.
func Mermaid(nodes []Node) string {
	_, known := slugSet(nodes)
	var b strings.Builder
	b.WriteString("graph TD\n")
	for _, n := range sortedBySlug(nodes) {
		fmt.Fprintf(&b, "  %s[%q]\n", n.Slug, mermaidLabel(n))
	}
	for _, line := range edgeLines(nodes, known) {
		b.WriteString(line)
	}
	return b.String()
}

// DanglingEdges returns every edge in nodes whose target is not one of nodes' own
// slugs — exactly the edges LearningPath and Mermaid silently drop — ordered by
// source, then kind, then target so a report of them is deterministic.
//
// Ensures: the result is empty iff Mermaid renders an edge line for every edge in
// nodes; it is pure.
func DanglingEdges(nodes []Node) []DanglingEdge {
	_, known := slugSet(nodes)
	var out []DanglingEdge
	for _, n := range nodes {
		for _, e := range n.Edges {
			if !known[e.Target] {
				out = append(out, DanglingEdge{Source: n.Slug, Edge: e})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Edge.Kind != b.Edge.Kind {
			return a.Edge.Kind < b.Edge.Kind
		}
		return a.Edge.Target < b.Edge.Target
	})
	return out
}

// UnknownSlugs returns the members of want that are not slugs in nodes,
// deduplicated and sorted. It is the pre-write counterpart of DanglingEdges: a
// command about to record an edge asks it whether the endpoints exist, instead of
// discovering the answer from a later index that quietly omits them.
//
// Ensures: the result is empty iff every want is a slug in nodes; it is pure.
func UnknownSlugs(nodes []Node, want []string) []string {
	_, known := slugSet(nodes)
	seen := make(map[string]bool, len(want))
	var out []string
	for _, w := range want {
		if known[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// prereqs returns the depends-on targets of n that are known slugs.
func prereqs(n Node, known map[string]bool) []string {
	var out []string
	for _, e := range n.Edges {
		if e.Kind == DependsOn && known[e.Target] {
			out = append(out, e.Target)
		}
	}
	return out
}

// smallestReady returns the lexicographically smallest not-yet-emitted slug with
// no remaining prerequisites, or "" when none is ready.
func smallestReady(slugs []string, indegree map[string]int, emitted map[string]bool) string {
	for _, s := range slugs { // slugs is pre-sorted, so the first match is smallest
		if !emitted[s] && indegree[s] == 0 {
			return s
		}
	}
	return ""
}

// edgeLines returns the sorted Mermaid edge lines for edges to known slugs.
func edgeLines(nodes []Node, known map[string]bool) []string {
	var lines []string
	for _, n := range nodes {
		for _, e := range n.Edges {
			if known[e.Target] {
				lines = append(lines, fmt.Sprintf("  %s -->|%s| %s\n", n.Slug, e.Kind, e.Target))
			}
		}
	}
	sort.Strings(lines)
	return lines
}

// slugSet returns the sorted slugs and a membership set.
func slugSet(nodes []Node) (slugs []string, known map[string]bool) {
	known = make(map[string]bool, len(nodes))
	slugs = make([]string, 0, len(nodes))
	for _, n := range nodes {
		if !known[n.Slug] {
			known[n.Slug] = true
			slugs = append(slugs, n.Slug)
		}
	}
	sort.Strings(slugs)
	return slugs, known
}

// sortedBySlug returns nodes ordered by slug.
func sortedBySlug(nodes []Node) []Node {
	out := make([]Node, len(nodes))
	copy(out, nodes)
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// mermaidLabel is the node's title with double quotes neutralized so the label
// stays valid inside a quoted Mermaid node.
func mermaidLabel(n Node) string {
	label := n.Title
	if label == "" {
		label = n.Slug
	}
	return strings.ReplaceAll(label, `"`, "'")
}
