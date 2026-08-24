package related_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/StevenACoffman/exegesis/internal/related"
)

func dep(target string) related.Edge {
	return related.Edge{Kind: related.DependsOn, Target: target, Rationale: "r"}
}

func TestLearningPath(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		nodes      []related.Node
		wantOrder  []string
		wantCyclic []string
	}{
		"prerequisite before dependent": {
			nodes:     []related.Node{{Slug: "a", Edges: []related.Edge{dep("b")}}, {Slug: "b"}},
			wantOrder: []string{"b", "a"},
		},
		"independent nodes sort lexicographically": {
			nodes:     []related.Node{{Slug: "z"}, {Slug: "x"}, {Slug: "m"}},
			wantOrder: []string{"m", "x", "z"},
		},
		"diamond resolves with sorted tie-break": {
			nodes: []related.Node{
				{Slug: "d", Edges: []related.Edge{dep("b"), dep("c")}},
				{Slug: "b", Edges: []related.Edge{dep("a")}},
				{Slug: "c", Edges: []related.Edge{dep("a")}},
				{Slug: "a"},
			},
			wantOrder: []string{"a", "b", "c", "d"},
		},
		"unknown target is ignored": {
			nodes:     []related.Node{{Slug: "a", Edges: []related.Edge{dep("ghost")}}},
			wantOrder: []string{"a"},
		},
		"cycle is reported, not fatal": {
			nodes: []related.Node{
				{Slug: "a", Edges: []related.Edge{dep("b")}},
				{Slug: "b", Edges: []related.Edge{dep("a")}},
			},
			wantOrder:  []string{"a", "b"},
			wantCyclic: []string{"a", "b"},
		},
		"only depends-on drives order": {
			// contrasts/composes edges must not constrain the path.
			nodes: []related.Node{
				{Slug: "a", Edges: []related.Edge{{Kind: related.ContrastsWith, Target: "b"}}},
				{Slug: "b"},
			},
			wantOrder: []string{"a", "b"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			order, cyclic, _ := related.LearningPath(tc.nodes)
			if strings.Join(order, ",") != strings.Join(tc.wantOrder, ",") {
				t.Errorf("order = %v, want %v", order, tc.wantOrder)
			}
			if strings.Join(cyclic, ",") != strings.Join(tc.wantCyclic, ",") {
				t.Errorf("cyclic = %v, want %v", cyclic, tc.wantCyclic)
			}
		})
	}
}

func TestMermaidDeterministicAndKnownOnly(t *testing.T) {
	t.Parallel()
	nodes := []related.Node{
		{Slug: "b", Title: "Bee", Edges: []related.Edge{dep("a")}},
		{
			Slug:  "a",
			Title: "Ay",
			Edges: []related.Edge{{Kind: related.ComposesWith, Target: "ghost"}},
		},
	}
	got := related.Mermaid(nodes)
	if got != related.Mermaid(nodes) {
		t.Fatal("Mermaid is not deterministic")
	}
	// Nodes are declared in slug order.
	if !strings.Contains(got, "graph TD\n  a[\"Ay\"]\n  b[\"Bee\"]\n") {
		t.Errorf("node declarations wrong or unordered:\n%s", got)
	}
	// Only the edge to the known slug is drawn.
	if !strings.Contains(got, "b -->|depends-on| a") {
		t.Errorf("missing known edge:\n%s", got)
	}
	if strings.Contains(got, "ghost") {
		t.Errorf("edge to unknown slug should be omitted:\n%s", got)
	}
}

// TestLearningPathReportsAbsentPrerequisites pins the defect the curated-market decision
// exposed. prereqs keeps only edges to known slugs, so a depends-on target absent from the
// tree vanished and its skill was emitted as though it had no prerequisite — at the front of
// the path, apparently ready to learn. Measured over the real market corpus, five skills
// were in that state.
func TestLearningPathReportsAbsentPrerequisites(t *testing.T) {
	t.Parallel()
	nodes := []related.Node{
		{Slug: "needs-outside", Edges: []related.Edge{
			{Kind: related.DependsOn, Target: "not-in-this-tree"},
		}},
		{Slug: "needs-nothing"},
		{Slug: "needs-inside", Edges: []related.Edge{
			{Kind: related.DependsOn, Target: "needs-nothing"},
		}},
		// A qualified target names the tree it means, so it is external by construction
		// and must not be reported as absent — DanglingEdges already excludes it.
		{Slug: "needs-other-tree", Edges: []related.Edge{
			{Kind: related.DependsOn, Target: "merged/all-books-v1/elsewhere"},
		}},
		// Only depends-on orders the path, so a non-ordering kind pointing outside is
		// merely unlisted rather than a missing prerequisite.
		{Slug: "informs-outside", Edges: []related.Edge{
			{Kind: related.Informs, Target: "also-not-here"},
		}},
	}
	order, cyclic, unresolved := related.LearningPath(nodes)

	if len(cyclic) != 0 {
		t.Errorf("cyclic = %v, want none", cyclic)
	}
	if want := []string{"needs-outside"}; !slices.Equal(unresolved, want) {
		t.Errorf("unresolved = %v, want %v", unresolved, want)
	}
	// The documented guarantee, and the one a plausible implementation breaks: a skill with
	// an unresolved prerequisite is still in the path. Reporting it must not remove it.
	if len(order) != len(nodes) {
		t.Fatalf("order has %d of %d slugs; reporting must not drop a node", len(order), len(nodes))
	}
	seen := map[string]bool{}
	for _, s := range order {
		seen[s] = true
	}
	for _, n := range nodes {
		if !seen[n.Slug] {
			t.Errorf("%s missing from the path", n.Slug)
		}
	}
}
